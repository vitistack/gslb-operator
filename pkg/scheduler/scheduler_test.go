package scheduler

import (
	"sync"
	"testing"
	"time"
)

func TestNewScheduler(t *testing.T) {
	tests := []struct {
		name string // description of this test case
		// Named input parameters for target function.
		interval time.Duration
		want     *Scheduler[int]
	}{
		{
			name:     "empty-scheduler",
			interval: time.Second * 10,
			want: &Scheduler[int]{
				interval:    time.Second * 10,
				maxOffSets:  20,
				nextOffset:  0,
				jitterRange: time.Second,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewScheduler[int](tt.interval, &sync.WaitGroup{})

			if got.interval != tt.want.interval {
				t.Errorf("expected interval: %v, got: %v", tt.want.interval, got.interval)
			}

			if got.maxOffSets != tt.want.maxOffSets {
				t.Errorf("expected maxOffSets: %v, got: %v", tt.want.maxOffSets, got.maxOffSets)
			}

			if got.nextOffset != tt.want.nextOffset {
				t.Errorf("expected nextOffset: %v, got: %v", tt.want.nextOffset, got.nextOffset)
			}

			if got.jitterRange != tt.want.jitterRange {
				t.Errorf("expected maxOffSets: %v, got: %v", tt.want.maxOffSets, got.maxOffSets)
			}
		})
	}
}

func TestScheduleItem(t *testing.T) {
	receivedTick := false

	wg := sync.WaitGroup{}
	scheduler := NewScheduler[int](time.Millisecond, &wg)
	scheduler.onTick = func(int) {
		receivedTick = true
	}
	defer scheduler.Stop()

	scheduler.Schedule(1)
	if !scheduler.isRunning {
		t.Errorf("scheduler is not running, expected: isRunning == true, but got: isRunning == false")
	}

	if len(scheduler.heap) == 0 && !receivedTick {
		t.Errorf("scheduler is running, but heap size is 0, means scheduler has pop'ed the heap before received tick")
	}

}

func TestScheduler_RemoveService(t *testing.T) {
	first := 1
	second := 2
	tests := []struct {
		name string // description of this test case
		// Named input parameters for receiver constructor.
		interval time.Duration
		wg       *sync.WaitGroup
		// Named input parameters for target function.
		item         int
		want         bool
		addSecond    bool
		removeSecond bool
	}{
		{
			name:         "only-one",
			interval:     time.Second,
			wg:           &sync.WaitGroup{},
			item:         1,
			want:         true,
			addSecond:    false,
			removeSecond: false,
		},
		{
			name:         "add-second-remove-first",
			interval:     time.Second,
			wg:           &sync.WaitGroup{},
			item:         1,
			want:         false,
			addSecond:    true,
			removeSecond: false,
		},
		{
			name:         "add-second-remove-second",
			interval:     time.Second,
			wg:           &sync.WaitGroup{},
			item:         1,
			want:         false,
			addSecond:    true,
			removeSecond: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := NewScheduler[int](tt.interval, tt.wg)

			s.Schedule(tt.item)
			var got bool
			if tt.addSecond {
				s.Schedule(second)
			}

			if tt.removeSecond {
				got = s.Remove(func(i int) bool { return i == second })
			} else {
				got = s.Remove(func(i int) bool { return i == first })
				if s.heap.Peek().shouldReSchedule {
					t.Errorf("scheduled service are set to be rescheduled after remove has been called")
				}
			}

			if got != tt.want {
				t.Errorf("RemoveService() = %v, but wanted %v", got, tt.want)
			}
		})
	}
}
