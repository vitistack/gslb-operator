package scheduler

import (
	"container/heap"
	"sync"
	"time"

	"github.com/vitistack/gslb-operator/internal/service"
	"github.com/vitistack/gslb-operator/internal/utils/timesutil"
)

// wrapper for scheduling services' ticker and quit
// TODO: sub-interval scheduling, e.g. wait 1s then schedule a new task on new timer
type Scheduler struct {
	// base-interval that the service distributes services on
	interval    timesutil.Duration
	heap        ServiceHeap
	jitterRange time.Duration
	timer       *time.Timer
	stop        chan struct{}
	wg          sync.WaitGroup
	mu          sync.Mutex
}

func NewScheduler(interval timesutil.Duration) *Scheduler {
	h := make(ServiceHeap, 0)
	heap.Init(&h)
	return &Scheduler{
		interval:    interval,
		heap:        h,
		jitterRange: time.Duration(float64(interval) * 0.1),
		stop:        make(chan struct{}),
		wg:          sync.WaitGroup{},
		mu:          sync.Mutex{},
	}
}

func (s *Scheduler) Stop() {

}

func (s *Scheduler) ScheduleService(svc *service.Service) {

}

func (s *Scheduler) Loop() {

}
