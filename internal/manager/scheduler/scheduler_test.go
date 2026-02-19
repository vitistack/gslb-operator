package scheduler

import (
	"sync"
	"testing"
	"time"

	"github.com/vitistack/gslb-operator/internal/model"
	"github.com/vitistack/gslb-operator/internal/service"
	"github.com/vitistack/gslb-operator/internal/utils/timesutil"
)

var genericGSLBConfig = model.GSLBConfig{
	ServiceID:  "123",
	Ip:         "192.168.1.1",
	Port:       "80",
	Datacenter: "dc1",
	Interval:   timesutil.Duration(time.Second),
	Priority:   1,
	CheckType:  "TCP-FULL",
}

var genericGSLBConfig2 = model.GSLBConfig{
	ServiceID:  "456",
	Ip:         "192.168.1.2",
	Port:       "80",
	Datacenter: "dc2",
	Interval:   timesutil.Duration(time.Second),
	Priority:   1,
	CheckType:  "TCP-FULL",
}

func TestNewScheduler(t *testing.T) {
	tests := []struct {
		name string // description of this test case
		// Named input parameters for target function.
		interval time.Duration
		want     *Scheduler
	}{
		{
			name:     "empty-scheduler",
			interval: time.Second * 10,
			want: &Scheduler{
				interval:    time.Second * 10,
				maxOffSets:  20,
				nextOffset:  0,
				jitterRange: time.Second,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewScheduler(tt.interval, &sync.WaitGroup{})

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

func TestScheduleService(t *testing.T) {
	svc, err := service.NewServiceFromGSLBConfig(genericGSLBConfig)
	if err != nil {
		t.Fatalf("could not create test service: %s", err.Error())
	}

	receivedTick := false

	wg := sync.WaitGroup{}
	scheduler := NewScheduler(time.Duration(svc.GetDefaultInterval()), &wg)
	scheduler.OnTick = func(s *service.Service) {
		receivedTick = true
	}
	defer scheduler.Stop()

	scheduler.ScheduleService(svc)
	if !scheduler.isRunning {
		t.Errorf("scheduler is not running, expected: isRunning == true, but got: isRunning == false")
	}

	if len(scheduler.heap) == 0 && !receivedTick {
		t.Errorf("scheduler is running, but heap size is 0, means scheduler has pop'ed the heap before received tick")
	}

}

func TestScheduler_RemoveService(t *testing.T) {
	svc1, _ := service.NewServiceFromGSLBConfig(genericGSLBConfig)
	svc2, _ := service.NewServiceFromGSLBConfig(genericGSLBConfig2)
	tests := []struct {
		name string // description of this test case
		// Named input parameters for receiver constructor.
		interval time.Duration
		wg       *sync.WaitGroup
		// Named input parameters for target function.
		svc          *service.Service
		want         bool
		addSecond    bool
		removeSecond bool
	}{
		{
			name:         "only-one",
			interval:     time.Second,
			wg:           &sync.WaitGroup{},
			svc:          svc1,
			want:         true,
			addSecond:    false,
			removeSecond: false,
		},
		{
			name:         "add-second-remove-first",
			interval:     time.Second,
			wg:           &sync.WaitGroup{},
			svc:          svc1,
			want:         false,
			addSecond:    true,
			removeSecond: false,
		},
		{
			name:         "add-second-remove-second",
			interval:     time.Second,
			wg:           &sync.WaitGroup{},
			svc:          svc1,
			want:         false,
			addSecond:    true,
			removeSecond: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := NewScheduler(tt.interval, tt.wg)

			s.ScheduleService(tt.svc)
			var got bool
			if tt.addSecond {
				s.ScheduleService(svc2)
			}

			if tt.removeSecond {
				got = s.RemoveService(svc2)
			} else {
				got = s.RemoveService(tt.svc)
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
