package manager

import (
	"errors"
	"sync"

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
}

func NewManager(minRunningWorkers, nonBlockingBufferSize uint, logger *zap.Logger) *ServicesManager {
	return &ServicesManager{
		servicesHealthCheck: make(map[timesutil.Duration][]*service.Service),
		schedulers:          make(map[timesutil.Duration]*scheduler),
		serviceGroups:       make(map[string]*ServiceGroup),
		log:                 logger.Sugar(),
		mutex:               sync.RWMutex{},
		pool:                *pool.NewWorkerPool(minRunningWorkers, nonBlockingBufferSize),
		stop:                sync.Once{},
		wg:                  sync.WaitGroup{},
	}
}

// Start begins scheduling health checks for all services according to their configured intervals.
// It ensures that the scheduling logic is only executed once, even if called multiple times.
func (sm *ServicesManager) Start() {
	sm.pool.Start()
}

func (sm *ServicesManager) Stop() {
	sm.stop.Do(func() {
		for _, scheduler := range sm.schedulers {
			close(scheduler.quit)
		}
		sm.wg.Wait()
		sm.pool.Stop()
		sm.log.Debug("successfully closed manager")
	})
}

func (sm *ServicesManager) RegisterService(newService *service.Service, locked bool) {
	// TODO: Better way to do this? reflect over this
	if !locked {
		sm.mutex.Lock()
		defer sm.mutex.Unlock()
	}

	exists, oldSvc, _ := sm.serviceExistsUnlocked(newService)

	if exists { // update service if already exists
		sm.updateServiceUnlocked(oldSvc, newService)
		return
	} else if _, ok := sm.servicesHealthCheck[newService.Interval]; !ok { // first service on interval
		scheduler := newScheduler(newService.Interval)
		sm.schedulers[newService.Interval] = scheduler
		sm.schedulerLoop(scheduler)
		sm.servicesHealthCheck[newService.Interval] = make([]*service.Service, 0)
		sm.log.Debugf("new scheduler on interval: %v", scheduler.interval.String())
	}

	if _, ok := sm.serviceGroups[newService.Fqdn]; !ok {
		sm.serviceGroups[newService.Fqdn] = new(ServiceGroup)
		newGroup := NewEmptyServiceGroup()
		newGroup.OnPromotion = func(event *PromotionEvent) {
			sm.log.Debugf("received promotion event for service: %v, OldActive: %v, NewActive: %v", event.Service, event.OldActive.Datacenter, event.NewActive.Datacenter)
			sm.handlePromotion(event)
		}
		sm.serviceGroups[newService.Fqdn] = newGroup
		sm.log.Debugf("new service group, for service: %v", newService.Fqdn)
	}

	sm.servicesHealthCheck[newService.Interval] = append(sm.servicesHealthCheck[newService.Interval], newService)
	sm.serviceGroups[newService.Fqdn].RegisterService(newService)
	newService.SetHealthChangeCallback(func(healthy bool) {
		sm.log.Debugf("received health-change for service: %v:%v", newService.Fqdn, newService.Datacenter)
		sm.serviceGroups[newService.Fqdn].OnServiceHealthChange(newService, healthy)
		sm.DNSUpdate(newService, healthy)
	})

	sm.log.Debugf("Service: %v:%v registered", newService.Fqdn, newService.Datacenter)
}

// removes the service from its healthcheck queue
func (sm *ServicesManager) RemoveService(service *service.Service, locked bool) error {
	// TODO: Better way to do this? reflect over this
	if !locked {
		sm.mutex.Lock()
		defer sm.mutex.Unlock()
	}

	exists, _, removeIdx := sm.serviceExistsUnlocked(service)
	if !exists {
		return ErrServiceNotFound
	}

	newQueue := utils.RemoveIndexFromSlice(sm.servicesHealthCheck[service.Interval], removeIdx)
	if len(newQueue) == 0 {
		delete(sm.servicesHealthCheck, service.Interval)
		close(sm.schedulers[service.Interval].quit) // signal scheduler to stop
		delete(sm.schedulers, service.Interval)
	} else {
		sm.servicesHealthCheck[service.Interval] = newQueue
	}
	sm.log.Debugf("Service: %v:%v removed", service.Fqdn, service.Datacenter)

	return nil
}

// updates an existing service with new configuration
// assumes sm.mutex is held by the caller
func (sm *ServicesManager) updateServiceUnlocked(old, new *service.Service) {
	if old.Interval != new.Interval { // move service from old scheduler to new scheduler
		sm.RemoveService(old, true)
		sm.RegisterService(new, true)
		new.Copy(old)
	}

	queue := sm.servicesHealthCheck[new.Interval]
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
						close(scheduler.quit) // make sure to close the schedulers channel
						return
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
	queue, ok := sm.servicesHealthCheck[service.Interval]
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

func (sm *ServicesManager) handlePromotion(event *PromotionEvent) {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	activeInterval := event.OldActive.Interval
	demotedInterval := event.NewActive.Interval // the interval has not been updated yet, there this looks a bit backwards

	sm.log.Infof("Promoting service %v:%v (interval: %v -> %v)",
		event.NewActive.Fqdn, event.NewActive.Datacenter,
		event.NewActive.Interval, activeInterval)

	sm.log.Infof("Demoting service %v:%v (interval: %v -> %v)",
		event.OldActive.Fqdn, event.OldActive.Datacenter,
		event.OldActive.Interval, demotedInterval)

	sm.moveServiceToInterval(event.NewActive, activeInterval)
	sm.moveServiceToInterval(event.OldActive, demotedInterval)
}

func (sm *ServicesManager) moveServiceToInterval(svc *service.Service, newInterval timesutil.Duration) {
	oldInterval := svc.Interval

	if queue, ok := sm.servicesHealthCheck[oldInterval]; ok {
		// remove from old interval queue
		for idx, qService := range queue {
			if qService.Fqdn == svc.Fqdn && qService.Datacenter == svc.Datacenter {
				newQueue := utils.RemoveIndexFromSlice(queue, idx)

				if len(newQueue) == 0 { // cleanup interval if empty
					delete(sm.servicesHealthCheck, oldInterval)
					if scheduler, ok := sm.schedulers[oldInterval]; ok {
						close(scheduler.quit)
						delete(sm.schedulers, oldInterval)
					}
				} else {
					sm.servicesHealthCheck[oldInterval] = newQueue
				}
				break
			}
		}
	}

	if _, ok := sm.servicesHealthCheck[newInterval]; !ok {
		scheduler := newScheduler(newInterval)
		sm.schedulers[newInterval] = scheduler
		sm.schedulerLoop(scheduler)
		sm.servicesHealthCheck[newInterval] = make([]*service.Service, 0)
		sm.log.Debugf("new scheduler on interval: %v", scheduler.interval.String())
	}
	svc.Interval = newInterval
	sm.servicesHealthCheck[newInterval] = append(sm.servicesHealthCheck[newInterval], svc)
}
