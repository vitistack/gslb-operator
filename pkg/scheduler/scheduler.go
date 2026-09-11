package scheduler

import (
	"container/heap"
	"log/slog"
	"math/rand/v2"
	"sync"
	"time"

	"github.com/vitistack/gslb-operator/pkg/bslog"
)

const OFFSETS_PER_SECOND = 2
const OFFSET = time.Second / OFFSETS_PER_SECOND

// wrapper for service which is scheduled on the heap
type ScheduledItem[T any] struct {
	item          T
	nextCheckTime time.Time
	offsett       time.Duration

	// only used when RemoveService wants to remove the service
	// that is currently at the top of the heap
	shouldReSchedule bool
}

type Scheduler[T any] struct {
	// base-interval that the service distributes services on
	interval time.Duration

	// maximum number of offsets possible for the interval
	maxOffSets int

	// next offsett to schedule a service on
	nextOffset int

	// queue were the next scheduled service is on top of the heap
	heap ItemHeap[T]

	// random jitter to spread out scheduled service on interval and sub-tick
	jitterRange time.Duration

	stop chan struct{} // signal stop
	wg   *sync.WaitGroup
	mu   sync.Mutex

	isRunning bool

	// action to take when ScheduledService.nectCheckTime has been reached
	onTick func(T)
}

func NewScheduler[T any](interval time.Duration, wg *sync.WaitGroup) Scheduler[T] {
	h := make(ItemHeap[T], 0)

	maxOffsets := int(interval.Seconds() * 2)
	if maxOffsets == 0 {
		maxOffsets = 1
	}

	return Scheduler[T]{
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

func (s *Scheduler[T]) Stop() {
	close(s.stop)
}

func (s *Scheduler[T]) OnTick(fn func(T)) {
	s.onTick = fn
}

// schedule a new service on the heap
func (s *Scheduler[T]) Schedule(item T) {
	s.mu.Lock()
	defer s.mu.Unlock()
	offset := OFFSET * time.Duration(s.nextOffset)
	scheduled := ScheduledItem[T]{
		item:             item,
		nextCheckTime:    time.Now().Add(offset).Add(s.newTickInterval()),
		shouldReSchedule: true,
	}

	s.nextOffset = (s.nextOffset + 1) % s.maxOffSets

	heap.Push(&s.heap, &scheduled)
	if s.heap.Len() == 1 {
		s.startLoop() // restart the loop when we have scheduled services in the heap
	}
}

// cmp is a function used to compare scheduled items if it exists or not.
func (s *Scheduler[T]) Remove(cmp func(T) bool) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	idx := s.heap.GetIndex(
		func(scheduled *ScheduledItem[T]) bool {
			return cmp(scheduled.item)
		},
	)

	switch idx {
	case -1:
		return s.heap.Len() == 0
	case 0:
		s.heap[0].shouldReSchedule = false
		if len(s.heap) == 1 {
			return true
		}
	default:
		heap.Remove(&s.heap, idx)
	}

	return s.heap.Len() == 0
}

func (s *Scheduler[T]) Has(cmp func(T) bool) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.heap.GetIndex(func(si *ScheduledItem[T]) bool { return cmp(si.item) }) != -1
}

// re-schedule the service at the top of the heap
func (s *Scheduler[T]) reSchedule(item *ScheduledItem[T]) {
	s.mu.Lock()
	defer s.mu.Unlock()

	idx := s.heap.GetIndex(func(si *ScheduledItem[T]) bool { return si == item })
	if idx == -1 {
		// item was removed concurrently
		return
	}

	if !item.shouldReSchedule {
		// just return since the service is now removed from the heap
		return
	}

	item.nextCheckTime = time.Now().Add(s.newTickInterval()) // initiate the next checktime

	heap.Push(&s.heap, item)

	if s.heap.Len() == 0 {
		s.startLoop()
	}
}

func (s *Scheduler[T]) startLoop() {
	if s.isRunning {
		return
	}
	s.isRunning = true
	s.loop()
}

func (s *Scheduler[T]) loop() {
	s.wg.Go(func() {
		defer func() {
			s.mu.Lock()
			if s.heap.Len() == 0 {
				s.isRunning = false
			}
			s.mu.Unlock()
			bslog.Debug("scheduler closed", slog.String("interval", s.interval.String()))
		}()

		for {
			select {
			case <-s.stop: // check stop
				bslog.Debug("got stop, exiting scheduler...")
				return
			default:
			}

			s.mu.Lock()
			if s.heap.Len() == 0 { // no need to infinitly run on an empty queue
				s.isRunning = false
				s.mu.Unlock()
				return
			}

			next := s.heap.Peek()
			s.mu.Unlock()
			if next.nextCheckTime.Before(time.Now()) { // check time already past, do action immediately and reschedule
				if s.onTick != nil {
					s.onTick(next.item)
				}

				select {
				case <-s.stop: // check stop
					bslog.Debug("got stop, exiting scheduler...")
					return
				default:
				}

				s.reSchedule(next)
			} else {
				timeUntil := time.Until(next.nextCheckTime)
				select {
				case <-s.stop:
					bslog.Debug("got stop, exiting scheduler...")
					return
				case <-time.After(timeUntil):
					if s.onTick != nil {
						s.onTick(next.item)
					}

					select {
					case <-s.stop: // check stop
						bslog.Debug("got stop, exiting scheduler...")
						return
					default:
					}

					s.reSchedule(next)
				}
			}
		}
	})
}

func (s *Scheduler[T]) newTickInterval() time.Duration {
	jitter := time.Duration((rand.Float64()*2 - 1) * float64(s.jitterRange)) // get a random jitter inside the jitter range
	return (s.interval + jitter).Round(time.Second / 10)                     // create the interval duration, rounded to the nearest 10th of a second
}
