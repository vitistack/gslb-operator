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

// wrapper for service which is scheduled on the heap
type ScheduledService struct {
	service       *service.Service
	nextCheckTime time.Time
	offsett       time.Duration

	// only used when RemoveService wants to remove the service
	// that is currently at the top of the heap
	shouldReSchedule bool
}

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
	wg   *sync.WaitGroup
	mu   sync.Mutex

	isRunning bool

	// action to take when ScheduledService.nectCheckTime has been reached
	OnTick func(*service.Service)
}

func NewScheduler(interval time.Duration, wg *sync.WaitGroup) *Scheduler {
	h := make(ServiceHeap, 0)

	maxOffsets := int(interval.Seconds() * 2)

	return &Scheduler{
		interval:    interval,
		maxOffSets:  maxOffsets,
		nextOffset:  0,
		heap:        h,
		jitterRange: time.Duration(float64(interval) * 0.1),
		stop:        make(chan struct{}),
		wg:          wg,
		mu:          sync.Mutex{},
		isRunning:   false,
	}
}

func (s *Scheduler) Stop() {
	close(s.stop)
}

// schedule a new service on the heap
func (s *Scheduler) ScheduleService(svc *service.Service) {
	s.mu.Lock()
	defer s.mu.Unlock()
	offset := OFFSET * time.Duration(s.nextOffset)
	scheduled := ScheduledService{
		service:          svc,
		nextCheckTime:    time.Now().Add(offset).Add(s.newCheckInterval()),
		shouldReSchedule: true,
	}

	s.nextOffset = (s.nextOffset + 1) % s.maxOffSets

	heap.Push(&s.heap, &scheduled)
	if s.heap.Len() == 1 {
		s.startLoop() // restart the loop when we have scheduled services in the heap
	}
}

func (s *Scheduler) RemoveService(svc *service.Service) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	idx := s.heap.GetServiceIndex(svc)
	if idx == -1 {
		return s.heap.Len() == 0
	}
	if idx == 0 {
		s.heap[0].shouldReSchedule = false
	} else {
		heap.Remove(&s.heap, idx)
	}

	return s.heap.Len() == 0
}

// re-schedule the service at the top of the heap
func (s *Scheduler) reSchedule() {
	s.mu.Lock()
	defer s.mu.Unlock()
	top := heap.Pop(&s.heap).(*ScheduledService)

	if !top.shouldReSchedule {
		// just return since the service is now removed from the heap
		return
	}

	top.nextCheckTime = time.Now().Add(s.newCheckInterval()) // initiate the next checktime

	heap.Push(&s.heap, top)

	if s.heap.Len() == 0 {
		s.startLoop()
	}
}

func (s *Scheduler) startLoop() {
	if s.isRunning {
		return
	}
	s.isRunning = true
	s.loop()
}

func (s *Scheduler) loop() {
	s.wg.Go(func() {
		defer func() {
			s.mu.Lock()
			s.isRunning = false
			s.mu.Unlock()
		}()

		for {
			s.mu.Lock()
			if s.heap.Len() == 0 { // no need to infinitly run on an empty queue
				s.mu.Unlock()
				break
			}

			next := s.heap.Peek()
			s.mu.Unlock()
			if next.nextCheckTime.Before(time.Now()) { // check time already past, do action immediately and reschedule
				s.OnTick(next.service)
				s.reSchedule()
			} else {
				timeUntil := time.Until(next.nextCheckTime)
				select {
				case <-s.stop:
					return
				case <-time.After(timeUntil):
					s.OnTick(next.service)
					s.reSchedule()
				}
			}
		}
	})
}

func (s *Scheduler) newCheckInterval() time.Duration {
	jitter := time.Duration((rand.Float64()*2 - 1) * float64(s.jitterRange)) // get a random jitter inside the jitter range
	return (s.interval + jitter).Round(time.Second / 10)                     // create the interval duration, rounded to the nearest 10th of a second
}
