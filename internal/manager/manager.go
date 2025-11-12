package manager

import (
	"errors"
	"sync"

	"github.com/vitistack/gslb-operator/internal/service"
	"github.com/vitistack/gslb-operator/internal/utils"
	"github.com/vitistack/gslb-operator/internal/utils/timesutil"
	"github.com/vitistack/gslb-operator/pkg/pool"
)

// Responsible for managing services, on scheduling services for health checks
type ServicesManager struct {
	// servicesHealthCheck maps check intervals to services that should be checked at that interval.
	servicesHealthCheck map[timesutil.Duration][]*service.Service
	schedulers          map[timesutil.Duration]*scheduler // wrapped scheduler for services
	mutex               sync.RWMutex
	stop                sync.Once
	pool                pool.WorkerPool
	wg                  sync.WaitGroup
}

func NewManager(minRunningWorkers, nonBlockingBufferSize uint) *ServicesManager {
	return &ServicesManager{
		servicesHealthCheck: make(map[timesutil.Duration][]*service.Service),
		schedulers:          make(map[timesutil.Duration]*scheduler),
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
	})
}

func (sm *ServicesManager) RegisterService(newService *service.Service) {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	exists, oldSvc, _ := sm.serviceExistsUnlocked(newService)

	if exists { // update service if already exists
		sm.updateServiceUnlocked(oldSvc, newService)
		return
	} else if _, ok := sm.servicesHealthCheck[newService.Interval]; !ok { // first service on interval
		scheduler := newScheduler(newService.Interval)
		sm.schedulers[newService.Interval] = scheduler
		sm.schedulerLoop(scheduler)
		sm.servicesHealthCheck[newService.Interval] = make([]*service.Service, 0)
	}

	sm.servicesHealthCheck[newService.Interval] = append(sm.servicesHealthCheck[newService.Interval], newService)
}

// removes the service from its healthcheck queue
func (sm *ServicesManager) RemoveService(service *service.Service) error {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

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

	return nil
}

// updates an existing service with new configuration
// assumes sm.mutex is held by the caller
func (sm *ServicesManager) updateServiceUnlocked(old, new *service.Service) {
	if old.Interval != new.Interval { // move service from old scheduler to new scheduler
		// TODO: This breaks, becausee these functions try to hold a lock which is held by the caller of this function.
		sm.RemoveService(old)
		sm.RegisterService(new)
		new.Copy(old)
	}

	queue := sm.servicesHealthCheck[new.Interval]
	for idx, svc := range queue {
		if svc.Addr == new.Addr && svc.Datacenter == new.Datacenter {
			new.Copy(svc)
			queue[idx] = new
			break
		}
	}
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
					err := sm.pool.Put(services[i])
					if errors.Is(err, pool.ErrPutOnClosedPool) {
						close(scheduler.quit) // make sure to close the schedulers channel
						return
					}
				}

			case <-scheduler.quit: // stops a specific scheduler
				return
			}
		}
	})
}

// WARNING, ONLY CALL THIS FUNCTION IF YOU KNOW WHAT YOU ARE DOING.
// NEEDS TO HOLD sm.mutex BEFORE A CALL TO THIS FUNCTION IS MADe
// A service is considered to exist if a registered service has the same Addr and Datacenter field as the service parameter
func (sm *ServicesManager) serviceExistsUnlocked(service *service.Service) (exists bool, svc *service.Service, index int) {
	queue, ok := sm.servicesHealthCheck[service.Interval]
	if !ok {
		exists = false
		return exists, nil, -1
	}

	for idx, s := range queue {
		if service.Addr == s.Addr && service.Datacenter == s.Datacenter {
			exists = true
			svc = s
			index = idx
			return
		}
	}

	return false, nil, -1
}
