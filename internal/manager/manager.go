package manager

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/vitistack/gslb-operator/internal/manager/scheduler"
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
	schedulers          map[timesutil.Duration]*scheduler.Scheduler // wrapped scheduler for services
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
		schedulers:          make(map[timesutil.Duration]*scheduler.Scheduler),
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

		for interval, scheduler := range sm.schedulers {
			scheduler.Stop()
			sm.log.Debugf("successfully closed scheduler on interval: %s", interval.String())
		}

		sm.wg.Wait()
		sm.log.Debug("successfully closed manager")
	})
}

func (sm *ServicesManager) RegisterService(serviceCfg model.GSLBConfig) (*service.Service, error) {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	newService, err := service.NewServiceFromGSLBConfig(serviceCfg, sm.log, sm.dryrun)
	if err != nil {
		return nil, fmt.Errorf("unable to register service: %s", err.Error())
	}

	exists, oldSvc, _ := sm.serviceExistsUnlocked(newService)

	if exists { // update service if already exists
		sm.updateServiceUnlocked(oldSvc, newService)
		return newService, nil
	}

	// set healthchange callback action
	newService.SetHealthChangeCallback(func(healthy bool) {
		sm.log.Debugf("received health-change for service: %v:%v (healthy: %v)", newService.MemberOf, newService.Datacenter, healthy)
		sm.serviceGroups[newService.MemberOf].OnServiceHealthChange(newService, healthy)
	})

	// create new scheduler if needed, and schedule service
	scheduler := sm.newScheduler(newService.ScheduledInterval)
	if _, ok := sm.servicesHealthCheck[newService.ScheduledInterval]; !ok { // first service on interval
		scheduler.ScheduleService(newService)
	}
	scheduler.ScheduleService(newService)

	// create new service group if needed, and register service in group
	memberOf := newService.MemberOf
	serviceGroup, ok := sm.serviceGroups[memberOf]
	if !ok {
		serviceGroup = sm.newServiceGroup(memberOf)
		sm.log.Debugf("new service group, for service: %v", newService.MemberOf)
	}
	serviceGroup.RegisterService(newService)

	sm.servicesHealthCheck[newService.ScheduledInterval] = append(sm.servicesHealthCheck[newService.ScheduledInterval], newService)
	sm.log.Debugf("Service: %v:%v registered", newService.MemberOf, newService.Datacenter)
	return newService, nil
}

// removes the service from its healthcheck queue
func (sm *ServicesManager) RemoveService(service *service.Service, locked bool) error {
	if !locked {
		sm.mutex.Lock()
		defer sm.mutex.Unlock()
	}

	exists, _, removeIdx := sm.serviceExistsUnlocked(service)
	if !exists { // cannot remove something that does not exists
		return ErrServiceNotFound
	}

	scheduler := sm.schedulers[service.ScheduledInterval]
	scheduler.RemoveService(service)

	group := sm.serviceGroups[service.MemberOf]
	group.RemoveService(service) // registered in group
	if len(group.Members) == 0 {
		delete(sm.serviceGroups, service.MemberOf)
	}
	newQueue := utils.RemoveIndexFromSlice(sm.servicesHealthCheck[service.ScheduledInterval], removeIdx)
	if len(newQueue) == 0 {
		sm.cleanupInterval(service.ScheduledInterval)
	} else {
		sm.servicesHealthCheck[service.ScheduledInterval] = newQueue
	}
	sm.log.Debugf("Service: %v:%v removed", service.MemberOf, service.Datacenter)

	return nil
}

// updates an existing service with new configuration
// assumes sm.mutex is held by the caller
func (sm *ServicesManager) updateServiceUnlocked(old, new *service.Service) {
	if old == new {
		return
	}

	if !old.ConfigChanged(new) { // nothing to do
		return
	}

	oldDefaultInterval, newDefaultInterval := old.GetDefaultInterval(), new.GetDefaultInterval()
	if oldDefaultInterval != newDefaultInterval && oldDefaultInterval == old.ScheduledInterval {
		// we need to move the service to a new interval
		// otherwise the service will get rescheduled back to its default interval on its own, when it is needed
		sm.moveServiceToInterval(old, newDefaultInterval)
	}

	if old.MemberOf != new.MemberOf {
		oldGroup := sm.serviceGroups[old.MemberOf]
		oldGroup.RemoveService(old)

		if len(oldGroup.Members) == 0 {
			// TODO: delete service group
			sm.log.Error("TODO: delete service group")
		}
	}

	old.Update(new)

	sm.log.Debugf("Service: %s updated", old.GetID())
}

// WARNING, ONLY CALL THIS FUNCTION IF YOU KNOW WHAT YOU ARE DOING.
// NEEDS TO HOLD sm.mutex BEFORE A CALL TO THIS FUNCTION IS MADE
// A service is considered to exist if a registered service has the same Fqdn and Datacenter field as the service parameter
func (sm *ServicesManager) serviceExistsUnlocked(service *service.Service) (bool, *service.Service, int) {
	queue, ok := sm.servicesHealthCheck[service.ScheduledInterval]
	if !ok {
		return false, nil, -1
	}

	for idx, s := range queue {
		if s.GetID() == service.GetID() {
			return true, s, idx
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

func (sm *ServicesManager) newServiceGroup(memberOf string) *ServiceGroup {
	sm.serviceGroups[memberOf] = new(ServiceGroup)
	newGroup := NewEmptyServiceGroup()

	newGroup.OnPromotion = func(event *PromotionEvent) {
		if event.NewActive == nil {
			sm.log.Warnf("no active sites available for: %s", event.Service)
		}
		sm.handlePromotion(event)
	}
	sm.serviceGroups[memberOf] = newGroup

	return newGroup
}

// creates a new scheduler, and starts its loop
func (sm *ServicesManager) newScheduler(interval timesutil.Duration) *scheduler.Scheduler {
	if scheduler, ok := sm.schedulers[interval]; ok { // scheduler already exists
		return scheduler
	}

	scheduler := scheduler.NewScheduler(time.Duration(interval))
	sm.schedulers[interval] = scheduler
	sm.servicesHealthCheck[interval] = make([]*service.Service, 0)

	scheduler.OnTick = func(s *service.Service) {
		sm.log.Debugf("checking service: %v:%v", s.Fqdn, s.Datacenter)
		err := sm.pool.Put(s)
		if errors.Is(err, pool.ErrPutOnClosedPool) {
			sm.log.Errorf("failed to execute health check, pool is closed")
		}
	}
	scheduler.Loop()

	sm.log.Debugf("new scheduler on interval: %v", interval.String())

	return scheduler
}

func (sm *ServicesManager) cleanupInterval(interval timesutil.Duration) {
	delete(sm.servicesHealthCheck, interval)
	if scheduler, ok := sm.schedulers[interval]; ok {
		scheduler.Stop()
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
