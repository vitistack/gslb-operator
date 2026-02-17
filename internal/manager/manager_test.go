package manager

import (
	"testing"
	"time"

	"github.com/vitistack/gslb-operator/internal/model"
	"github.com/vitistack/gslb-operator/internal/service"
	"github.com/vitistack/gslb-operator/internal/utils/timesutil"
)

var genericGSLBConfig = model.GSLBConfig{
	ServiceID:  "123-test-456",
	MemberOf:   "example.com",
	Fqdn:       "test.example.com",
	Ip:         "192.168.1.1",
	Port:       "80",
	Datacenter: "dc1",
	Interval:   timesutil.Duration(5 * time.Second),
	Priority:   1,
	CheckType:  "TCP-FULL",
}

func TestNewManager(t *testing.T) {
	manager := NewManager(
		WithMinRunningWorkers(5),
		WithNonBlockingBufferSize(6),
	)

	if manager == nil {
		t.Fatal("NewManager returned nil")
	}

	if manager.scheduledServices == nil {
		t.Error("servicesHealthCheck map not initialized")
	}

	if manager.schedulers == nil {
		t.Error("schedulers map not initialized")
	}

	if manager.pool.NumWorkers() != 0 {
		t.Error("pool should not have workers before Start()")
	}
}

func TestRegister(t *testing.T) {
	manager := NewManager(
		WithMinRunningWorkers(2),
		WithNonBlockingBufferSize(10),
	)

	svc, _ := manager.RegisterService(genericGSLBConfig)

	manager.mutex.RLock()
	services, ok := manager.scheduledServices[svc.ScheduledInterval]
	manager.mutex.RUnlock()

	if !ok {
		t.Error("service interval not found in health check map")
	}

	if len(services) == 0 {
		t.Error("expected registered service but got 0")
	}

	if services[0] != svc {
		t.Error("registered service is not the same as expected")
	}
}

func TestStartAndStop(t *testing.T) {
	manager := NewManager(
		WithMinRunningWorkers(2),
		WithNonBlockingBufferSize(10),
	)

	manager.RegisterService(genericGSLBConfig)
	manager.Start()

	// Give it a moment to start
	time.Sleep(100 * time.Millisecond)

	if manager.pool.NumWorkers() < 2 {
		t.Errorf("expected at least 2 workers, got %d", manager.pool.NumWorkers())
	}

	manager.Stop()

	// Give it a moment to stop
	time.Sleep(100 * time.Millisecond)

	// Pool should be stopped
	if manager.pool.NumWorkers() != 0 {
		t.Errorf("expected 0 workers after stop, got %d", manager.pool.NumWorkers())
	}
}

func TestServicesManager_updateServiceUnlocked(t *testing.T) {
	tests := []struct {
		name string // description of this test case
		old  model.GSLBConfig
		new  model.GSLBConfig
	}{
		{
			name: "update-priority",
			old:  genericGSLBConfig,
			new: model.GSLBConfig{
				ServiceID:  "123-test-456",
				MemberOf:   "example.com",
				Fqdn:       "test.example.com",
				Ip:         "192.168.1.1",
				Port:       "80",
				Datacenter: "dc1",
				Interval:   timesutil.Duration(5 * time.Second),
				Priority:   2,
				CheckType:  "TCP-FULL",
			},
		},
		{
			name: "update-dc",
			old:  genericGSLBConfig,
			new: model.GSLBConfig{
				ServiceID:  "123-test-456",
				MemberOf:   "example.com",
				Fqdn:       "test.example.com",
				Ip:         "192.168.1.1",
				Port:       "80",
				Datacenter: "dc2",
				Interval:   timesutil.Duration(5 * time.Second),
				Priority:   1,
				CheckType:  "TCP-FULL",
			},
		},
		{
			name: "update-ip",
			old:  genericGSLBConfig,
			new: model.GSLBConfig{
				ServiceID:  "123-test-456",
				MemberOf:   "example.com",
				Fqdn:       "test.example.com",
				Ip:         "192.168.1.2",
				Port:       "80",
				Datacenter: "dc1",
				Interval:   timesutil.Duration(5 * time.Second),
				Priority:   1,
				CheckType:  "TCP-FULL",
			},
		},
		{
			name: "update-check-type",
			old:  genericGSLBConfig,
			new: model.GSLBConfig{
				ServiceID:  "123-test-456",
				MemberOf:   "example.com",
				Fqdn:       "test.example.com",
				Ip:         "192.168.1.1",
				Port:       "80",
				Datacenter: "dc1",
				Interval:   timesutil.Duration(5 * time.Second),
				Priority:   1,
				CheckType:  "TCP-HALF",
			},
		},
		{
			name: "update-member-of",
			old:  genericGSLBConfig,
			new: model.GSLBConfig{
				ServiceID:  "123-test-456",
				MemberOf:   "example.example.com",
				Fqdn:       "test.example.com",
				Ip:         "192.168.1.1",
				Port:       "80",
				Datacenter: "dc1",
				Interval:   timesutil.Duration(5 * time.Second),
				Priority:   1,
				CheckType:  "TCP-FULL",
			},
		},
		{
			name: "update-fqdn",
			old:  genericGSLBConfig,
			new: model.GSLBConfig{
				ServiceID:  "123-test-456",
				MemberOf:   "example.example.com",
				Fqdn:       "testing.example.com",
				Ip:         "192.168.1.1",
				Port:       "80",
				Datacenter: "dc1",
				Interval:   timesutil.Duration(5 * time.Second),
				Priority:   1,
				CheckType:  "TCP-FULL",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sm := NewManager(WithDryRun(true))
			sm.Start()

			sm.DNSUpdate = func(s *service.Service, b bool) {

			}
			old, err := sm.RegisterService(tt.old)
			if err != nil {
				t.Fatalf("could not create service during testing: %s", err.Error())
			}

			new, err := service.NewServiceFromGSLBConfig(tt.new, service.WithDryRunChecks(true))
			if err != nil {
				t.Fatalf("could not create service during testing: %s", err.Error())
			}
			sm.updateService(old, new)

			if old.ConfigChanged(new) {
				t.Error("still pending config changes after update")
			}

			_, interval, svc := sm.scheduledServices.Search(old.GetID())
			if interval != new.GetDefaultInterval() {
				t.Errorf("the service was not located at its correct interval, expected: %s but got: %s", new.GetDefaultInterval(), interval)
			}

			if svc != old {
				t.Fatal("scheduled service changed pointer after update")
			}
		})
	}
}

func TestServicesManager_moveServiceToInterval(t *testing.T) {
	tests := []struct {
		name string // description of this test case
		// Named input parameters for target function.
		config      model.GSLBConfig
		newInterval timesutil.Duration
		shouldExist bool
	}{
		{
			name:        "change-to-non-existing-interval",
			config:      genericGSLBConfig,
			newInterval: timesutil.FromDuration(time.Second),
		},
		{
			name:        "change-to-existing-interval",
			config:      genericGSLBConfig,
			newInterval: timesutil.FromDuration(time.Second),
			shouldExist: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sm := NewManager(WithDryRun(true))
			svc, _ := sm.RegisterService(tt.config)

			if tt.shouldExist {
				sm.newScheduler(tt.newInterval)
			}
			sm.moveServiceToInterval(svc, tt.newInterval)

			_, interval, _ := sm.scheduledServices.Search(svc.GetID())
			if interval != tt.newInterval {
				t.Errorf("expected new interval: %s but got: %s", tt.newInterval.String(), interval.String())
			}

			_, ok := sm.schedulers[tt.newInterval]
			if !ok {
				t.Error("scheduler on new interval does not exist")
			}
		})
	}
}
