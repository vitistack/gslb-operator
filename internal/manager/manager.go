package manager

import (
	"errors"
	"fmt"
	"sync"

	"github.com/vitistack/gslb-operator/internal/model"
	"github.com/vitistack/gslb-operator/internal/service"
	"github.com/vitistack/gslb-operator/internal/utils"
	"github.com/vitistack/gslb-operator/internal/utils/timesutil"
	"github.com/vitistack/gslb-operator/pkg/pool"
	"go.uber.org/zap"
)

// Responsible for managing services, on scheduling services for health checks
type ServicesManager struct {
	// servicesHealthCheck maps check intervals to services that should be checked at that interval.
	servicesHealthCheck map[timesutil.Duration][]*service.Service
	schedulers          map[timesutil.Duration]*scheduler // wrapped scheduler for services
	serviceGroups       map[string]*ServiceGroup
	log                 *zap.SugaredLogger
	mutex               sync.RWMutex
	stop                sync.Once
	pool                pool.WorkerPool
	wg                  sync.WaitGroup
	DNSUpdate           func(*service.Service, bool)
	dryrun              bool
}

func NewManager(logger *zap.Logger, opts ...serviceManagerOption) *ServicesManager {
	cfg := managerConfig{
		MinRunningWorkers:     100,
		NonBlockingBufferSize: 110,
		DryRun:                false,
	}

	for _, opt := range opts {
		opt(&cfg)
	}

	if cfg.DryRun {
		logger.Warn("dry-run enabled")
	}

	return &ServicesManager{
		servicesHealthCheck: make(map[timesutil.Duration][]*service.Service),
		schedulers:          make(map[timesutil.Duration]*scheduler),
		serviceGroups:       make(map[string]*ServiceGroup),
		log:                 logger.Sugar(),
		mutex:               sync.RWMutex{},
		pool:                *pool.NewWorkerPool(cfg.MinRunningWorkers, cfg.NonBlockingBufferSize),
		stop:                sync.Once{},
		wg:                  sync.WaitGroup{},
		dryrun:              cfg.DryRun,
	}
}

// Start begins scheduling health checks for all services according to their configured intervals.
// It ensures that the scheduling logic is only executed once, even if called multiple times.
func (sm *ServicesManager) Start() {
	sm.pool.Start()
}

func (sm *ServicesManager) Stop() {
	sm.pool.Stop()
	sm.stop.Do(func() {
		for _, scheduler := range sm.schedulers {
			close(scheduler.quit)
		}
		sm.wg.Wait()
		sm.log.Debug("successfully closed manager")
	})
}

func (sm *ServicesManager) RegisterService(serviceCfg model.GSLBConfig, locked bool) (*service.Service, error) {
	// TODO: Better way to do this? reflect over this
	if !locked {
		sm.mutex.Lock()
		defer sm.mutex.Unlock()
	}
	newService, err := service.NewServiceFromGSLBConfig(serviceCfg, sm.log, sm.dryrun)
	if err != nil {
		return nil, fmt.Errorf("unable to register service: %s", err.Error())
	}

	exists, oldSvc, _ := sm.serviceExistsUnlocked(newService)

	if exists { // update service if already exists
		sm.updateServiceUnlocked(oldSvc, newService)
		return newService, nil
	}

	if _, ok := sm.servicesHealthCheck[newService.ScheduledInterval]; !ok { // first service on interval
		sm.newScheduler(newService.ScheduledInterval)
	}

	fqdn := newService.Fqdn
	serviceGroup, ok := sm.serviceGroups[fqdn]
	if !ok {
		serviceGroup = sm.newServiceGroup(fqdn)
		sm.log.Debugf("new service group, for service: %v", newService.Fqdn)
	}

	sm.servicesHealthCheck[newService.ScheduledInterval] = append(sm.servicesHealthCheck[newService.ScheduledInterval], newService)
	serviceGroup.RegisterService(newService)

	newService.SetHealthChangeCallback(func(healthy bool) {
		sm.log.Debugf("received health-change for service: %v:%v (healthy: %v)", newService.Fqdn, newService.Datacenter, healthy)
		sm.serviceGroups[newService.Fqdn].OnServiceHealthChange(newService, healthy)
	})

	sm.log.Debugf("Service: %v:%v registered", newService.Fqdn, newService.Datacenter)
	return newService, nil
}

// removes the service from its healthcheck queue
func (sm *ServicesManager) RemoveService(service *service.Service, locked bool) error {
	if !locked {
		sm.mutex.Lock()
		defer sm.mutex.Unlock()
	}

	exists, _, removeIdx := sm.serviceExistsUnlocked(service)
	if !exists { // cannpt remove something that does not exists
		return ErrServiceNotFound
	}

	group := sm.serviceGroups[service.Fqdn]
	group.RemoveService(service) // registered in group
	if len(group.Members) == 0 {
		delete(sm.serviceGroups, service.Fqdn)
	}
	newQueue := utils.RemoveIndexFromSlice(sm.servicesHealthCheck[service.ScheduledInterval], removeIdx)
	if len(newQueue) == 0 {
		sm.cleanupInterval(service.ScheduledInterval)
	} else {
		sm.servicesHealthCheck[service.ScheduledInterval] = newQueue
	}
	sm.log.Debugf("Service: %v:%v removed", service.Fqdn, service.Datacenter)

	return nil
}

// updates an existing service with new configuration
// assumes sm.mutex is held by the caller
func (sm *ServicesManager) updateServiceUnlocked(old, new *service.Service) {
	if old == new {
		return
	}

	if old.ScheduledInterval != new.ScheduledInterval { // move service from old scheduler to new scheduler
		sm.moveServiceToInterval(old, new.ScheduledInterval)
	}

	if old.Fqdn != new.Fqdn {
		group := sm.serviceGroups[old.Fqdn]
		group.RemoveService(old)

		newGroup, ok := sm.serviceGroups[new.Fqdn]
		if !ok {
			newGroup = sm.newServiceGroup(new.Fqdn)
		}
		newGroup.RegisterService(new)
	}

	queue := sm.servicesHealthCheck[new.ScheduledInterval]
	for idx, svc := range queue {
		if svc.Fqdn == new.Fqdn && svc.Datacenter == new.Datacenter {
			new.Copy(svc)
			queue[idx] = new
			break
		}
	}
	sm.log.Debugf("Service: %v:%v updated", old.Fqdn, old.Datacenter)
}

func (sm *ServicesManager) schedulerLoop(scheduler *scheduler) {
	sm.wg.Go(func() {
		defer scheduler.Stop()
		for {
			select {
			case <-scheduler.ticker.C:
				sm.mutex.RLock()
				services := make([]*service.Service, len(sm.servicesHealthCheck[scheduler.interval]))
				copy(services, sm.servicesHealthCheck[scheduler.interval]) // copy to not hold the lock while iterating services
				sm.mutex.RUnlock()

				for i := range services {
					sm.log.Debugf("checking service: %v:%v", services[i].Fqdn, services[i].Datacenter)
					err := sm.pool.Put(services[i])
					if errors.Is(err, pool.ErrPutOnClosedPool) {
						sm.log.Errorf("failed to execute health check, pool is closed")
					}
				}

			case <-scheduler.quit: // stops a specific scheduler
				sm.log.Debugf("Scheduler on interval: %v closed", scheduler.interval.String())
				return
			}
		}
	})
}

// WARNING, ONLY CALL THIS FUNCTION IF YOU KNOW WHAT YOU ARE DOING.
// NEEDS TO HOLD sm.mutex BEFORE A CALL TO THIS FUNCTION IS MADE
// A service is considered to exist if a registered service has the same Fqdn and Datacenter field as the service parameter
func (sm *ServicesManager) serviceExistsUnlocked(service *service.Service) (exists bool, svc *service.Service, index int) {
	queue, ok := sm.servicesHealthCheck[service.ScheduledInterval]
	if !ok {
		exists = false
		return exists, nil, -1
	}

	for idx, s := range queue {
		if service.Fqdn == s.Fqdn && service.Datacenter == s.Datacenter {
			exists = true
			svc = s
			index = idx
			return
		}
	}

	return false, nil, -1
}

// re-schedules the relevant services in the PromotionEvent
func (sm *ServicesManager) handlePromotion(event *PromotionEvent) {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	// No-op: nothing to change
	if event.NewActive == event.OldActive {
		sm.log.Debugf("promotion no-op for service %s (unchanged active)", event.Service)
		return
	}

	var baseInterval timesutil.Duration
	var demotedInterval timesutil.Duration

	msg := "received promotion event for service: " + event.Service + ": "
	if event.OldActive != nil { // set baseInterval
		msg += " OldActive: " + event.OldActive.Datacenter + " "
		baseInterval = event.OldActive.GetBaseInterval()
	}
	if event.NewActive != nil {
		msg += "NewActive: " + event.NewActive.Datacenter
		if baseInterval == 0 {
			baseInterval = event.NewActive.GetBaseInterval()
		}
	}
	sm.log.Debug(msg)

	if event.OldActive != nil && event.NewActive != nil { // just swap, and do dns updates
		demotedInterval = event.NewActive.ScheduledInterval

		sm.log.Infof("Demoting service %v:%v (interval: %v -> %v)",
			event.OldActive.Fqdn, event.OldActive.Datacenter,
			event.OldActive.ScheduledInterval, demotedInterval)
		sm.moveServiceToInterval(event.OldActive, demotedInterval)
		sm.DNSUpdate(event.OldActive, false)

		sm.log.Infof("Promoting service %v:%v (interval: %v -> %v)",
			event.NewActive.Fqdn, event.NewActive.Datacenter,
			event.NewActive.ScheduledInterval, baseInterval)
		sm.moveServiceToInterval(event.NewActive, baseInterval)
		sm.DNSUpdate(event.NewActive, true)
		return
	}

	if event.NewActive != nil { // first service to come up when all services are down
		sm.log.Infof("Promoting service %s:%s to new active", event.NewActive.Fqdn, event.NewActive.Datacenter)
		sm.moveServiceToInterval(event.NewActive, baseInterval)
		sm.DNSUpdate(event.NewActive, true)
		return
	}

	if event.OldActive != nil { // no service to take over
		sm.log.Warnf("No sites available for %s", event.Service)
		sm.DNSUpdate(event.OldActive, false)
		return
	}
}

func (sm *ServicesManager) newServiceGroup(fqdn string) *ServiceGroup {
	sm.serviceGroups[fqdn] = new(ServiceGroup)
	newGroup := NewEmptyServiceGroup()

	newGroup.OnPromotion = func(event *PromotionEvent) {
		if event.NewActive == nil {
			sm.log.Warnf("no active sites available for: %s", event.Service)
		}
		sm.handlePromotion(event)
	}
	sm.serviceGroups[fqdn] = newGroup

	return newGroup
}

// creates a new scheduler, and starts its loop
func (sm *ServicesManager) newScheduler(interval timesutil.Duration) *scheduler {
	scheduler := newScheduler(interval)
	sm.schedulers[interval] = scheduler
	sm.servicesHealthCheck[interval] = make([]*service.Service, 0)
	sm.schedulerLoop(scheduler)
	sm.log.Debugf("new scheduler on interval: %v", scheduler.interval.String())

	return scheduler
}

func (sm *ServicesManager) cleanupInterval(interval timesutil.Duration) {
	delete(sm.servicesHealthCheck, interval)
	if scheduler, ok := sm.schedulers[interval]; ok {
		close(scheduler.quit)
		delete(sm.schedulers, interval)
	}
	sm.log.Debugf("deleted scheduler on interval: %v", interval.String())
}

func (sm *ServicesManager) moveServiceToInterval(svc *service.Service, newInterval timesutil.Duration) {
	oldInterval := svc.ScheduledInterval
	if oldInterval == newInterval {
		return // already scheduled on this interval
	}

	if queue, ok := sm.servicesHealthCheck[oldInterval]; ok {
		// remove from old interval queue
		for idx, qService := range queue {
			if qService.Fqdn == svc.Fqdn && qService.Datacenter == svc.Datacenter {
				newQueue := utils.RemoveIndexFromSlice(queue, idx)

				if len(newQueue) == 0 { // cleanup interval if empty
					sm.cleanupInterval(oldInterval)
				} else {
					sm.servicesHealthCheck[oldInterval] = newQueue
				}
				break
			}
		}
	}

	if _, ok := sm.servicesHealthCheck[newInterval]; !ok {
		sm.newScheduler(newInterval)
	}

	sm.servicesHealthCheck[newInterval] = append(sm.servicesHealthCheck[newInterval], svc)
	svc.ScheduledInterval = newInterval
}
