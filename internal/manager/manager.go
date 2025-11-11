package manager

import (
	"errors"
	"sync"
	"time"

	"github.com/vitistack/gslb-operator/internal/service"
	"github.com/vitistack/gslb-operator/internal/utils"
	"github.com/vitistack/gslb-operator/internal/utils/timesutil"
	"github.com/vitistack/gslb-operator/pkg/pool"
)

// Responsible for managing services, on scheduling services for health checks
type ServicesManager struct {
	// servicesHealthCheck maps check intervals to services that should be checked at that interval.
	servicesHealthCheck map[timesutil.Duration][]*service.Service
	start               sync.Once
	stop                sync.Once
	pool                pool.WorkerPool
	quit                chan struct{}
	wg                  sync.WaitGroup
}

func NewManager(minRunningWorkers, nonBlockingBufferSize uint) *ServicesManager {
	return &ServicesManager{
		servicesHealthCheck: make(map[timesutil.Duration][]*service.Service),
		pool:                *pool.NewWorkerPool(minRunningWorkers, nonBlockingBufferSize),
		start:               sync.Once{},
		stop:                sync.Once{},
		quit:                make(chan struct{}),
		wg:                  sync.WaitGroup{},
	}
}

// Start begins scheduling health checks for all services according to their configured intervals.
// It ensures that the scheduling logic is only executed once, even if called multiple times.
func (sm *ServicesManager) Start() {
	sm.pool.Start()
	sm.start.Do(func() {
		for duration, services := range sm.servicesHealthCheck {
			ticker := time.NewTicker(time.Duration(duration))
			sm.schedulerLoop(ticker, services)
		}
	})
}

func (sm *ServicesManager) Stop() {
	sm.stop.Do(func() {
		close(sm.quit)
		sm.wg.Wait()
		sm.pool.Stop()
	})
}

func (sm *ServicesManager) RegisterService(service *service.Service) {
	sm.servicesHealthCheck[service.Interval] = append(sm.servicesHealthCheck[service.Interval], service)
}

// removes the service from its healthcheck queue
func (sm *ServicesManager) RemoveService(service *service.Service) {
	queue, ok := sm.servicesHealthCheck[service.Interval]
	if !ok {
		return
	}

	removeIdx := -1
	for idx, svc := range queue {
		if svc.Addr == service.Addr && svc.Datacenter == service.Datacenter {
			removeIdx = idx
			break
		}
	}
	sm.servicesHealthCheck[service.Interval] = utils.RemoveIndexFromSlice(queue, removeIdx)
}

func (sm *ServicesManager) schedulerLoop(ticker *time.Ticker, services []*service.Service) {
	sm.wg.Go(func() {
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				for i := range services {
					err := sm.pool.Put(services[i])
					if errors.Is(err, pool.ErrPutOnClosedPool) {
						ticker.Stop()
						return
					}
				}
			case <-sm.quit:
				return
			}
		}
	})
}
