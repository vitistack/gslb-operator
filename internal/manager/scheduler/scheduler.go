package scheduler

import (
	"container/heap"
	"math/rand/v2"
	"sync"
	"time"

	"github.com/vitistack/gslb-operator/internal/service"
)

const OFFSETS_PER_SECOND = 2
const OFFSET = time.Second / OFFSETS_PER_SECOND

// TODO: sub-interval scheduling, e.g. wait 1s then schedule a new task on new timer
type Scheduler struct {
	// base-interval that the service distributes services on
	interval time.Duration

	// maximum number of offsets possible for the interval
	maxOffSets int

	// next offsett to schedule a service on
	nextOffset int

	// queue were the next scheduled service is on top of the heap
	heap ServiceHeap

	// random jitter to spread out scheduled service on interval and sub-tick
	jitterRange time.Duration

	stop chan struct{}
	wg   sync.WaitGroup
	mu   sync.Mutex

	// action to take when ScheduledService.nectCheckTime has been reached
	OnTick func(*service.Service)
}

// wrapper for service which is scheduled on the heap
type ScheduledService struct {
	service       *service.Service
	nextCheckTime time.Time
	offsett       time.Duration
}

func NewScheduler(interval time.Duration) *Scheduler {
	h := make(ServiceHeap, 0)

	maxOffsets := int(interval.Seconds() * 2)

	return &Scheduler{
		interval:    interval,
		maxOffSets:  maxOffsets,
		nextOffset:  0,
		heap:        h,
		jitterRange: time.Duration(float64(interval) * 0.1),
		stop:        make(chan struct{}),
		wg:          sync.WaitGroup{},
		mu:          sync.Mutex{},
	}
}

func (s *Scheduler) Stop() {
	close(s.stop)
	s.wg.Wait()
}

// schedule a new service on the heap
func (s *Scheduler) ScheduleService(svc *service.Service) {
	s.mu.Lock()
	defer s.mu.Unlock()
	offset := OFFSET * time.Duration(s.nextOffset)
	scheduled := ScheduledService{
		service:       svc,
		nextCheckTime: time.Now().Add(offset).Add(s.newCheckInterval()),
	}

	s.nextOffset = (s.nextOffset + 1) % s.maxOffSets

	heap.Push(&s.heap, &scheduled)
	if s.heap.Len() == 1 {
		s.Loop() // restart the loop when we have scheduled services in the heap
	}
}

func (s *Scheduler) RemoveService(svc *service.Service) {
	s.mu.Lock()
	defer s.mu.Unlock()
	idx := s.heap.GetServiceIndex(svc)
	if idx == -1 {
		return
	}
	heap.Remove(&s.heap, idx)
}

// re-schedule an already existing scheduled service
func (s *Scheduler) reScheduleService(reSchedule *ScheduledService) {
	reSchedule.nextCheckTime = time.Now().Add(s.newCheckInterval()) // initiate the next checktime

	s.mu.Lock()
	defer s.mu.Unlock()
	heap.Push(&s.heap, reSchedule)

	if s.heap.Len() == 0 {
		s.Loop()
	}
}

func (s *Scheduler) Loop() {
	s.wg.Go(func() {
		for {
			s.mu.Lock()
			if s.heap.Len() == 0 { // no need to infinetly run on an empty queue
				s.mu.Unlock()
				break
			}

			next := heap.Pop(&s.heap).(*ScheduledService)
			s.mu.Unlock()
			if next.nextCheckTime.Before(time.Now()) { // check time already past, do action immediately and reschedule
				s.OnTick(next.service)
				s.reScheduleService(next)
			} else {
				timeUntil := time.Until(next.nextCheckTime)
				select {
				case <-s.stop:
					break
				case <-time.After(timeUntil):
					s.OnTick(next.service)
					s.reScheduleService(next)
				}
			}
		}
	})
}

func (s *Scheduler) newCheckInterval() time.Duration {
	jitter := time.Duration((rand.Float64()*2 - 1) * float64(s.jitterRange)) // get a random jitter inside the jitter range
	return (s.interval + jitter).Round(time.Second / 10)                     // create the interval duration, rounded to the nearest 10th of a second
}
