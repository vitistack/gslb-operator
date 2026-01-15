package manager

import (
	"testing"
	"time"

	"github.com/vitistack/gslb-operator/internal/model"
	"github.com/vitistack/gslb-operator/internal/service"
	"github.com/vitistack/gslb-operator/internal/utils/timesutil"
	"go.uber.org/zap"
)

var logger, _ = zap.NewDevelopment()

var genericGSLBConfig = model.GSLBConfig{
	Fqdn:       "test.example.com",
	Ip:         "192.168.1.1",
	Port:       "80",
	Datacenter: "dc1",
	Interval:   timesutil.Duration(5 * time.Second),
	Priority:   1,
	Type:       "TCP-FULL",
}

func TestNewManager(t *testing.T) {
	manager := NewManager(
		logger,
		WithMinRunningWorkers(5),
		WithNonBlockingBufferSize(6),
	)

	if manager == nil {
		t.Fatal("NewManager returned nil")
	}

	if manager.servicesHealthCheck == nil {
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
		logger,
		WithMinRunningWorkers(2),
		WithNonBlockingBufferSize(10),
	)

	svc, _ := manager.RegisterService(genericGSLBConfig)

	manager.mutex.RLock()
	services, ok := manager.servicesHealthCheck[svc.ScheduledInterval]
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
		logger,
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

func TestServicesManager_serviceExistsUnlocked(t *testing.T) {
	tests := []struct {
		name string // description of this test case
		// Named input parameters for receiver constructor.
		logger *zap.Logger
		opts   []serviceManagerOption
		// Named input parameters for target function.
		want  bool
		want2 *service.Service
		want3 int
	}{
		{
			name:   "one-service",
			logger: logger,
			opts:   []serviceManagerOption{WithDryRun(true)},
			want:   true,
			want3:  0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sm := NewManager(tt.logger, tt.opts...)
			tt.want2, _ = sm.RegisterService(genericGSLBConfig)

			svc, _ := service.NewServiceFromGSLBConfig(genericGSLBConfig, logger.Sugar(), true)

			got, got2, got3 := sm.serviceExistsUnlocked(svc)
			// TODO: update the condition below to compare got with tt.want.
			if got != tt.want {
				t.Errorf("serviceExistsUnlocked() = %v, want %v", got, tt.want)
			}
			if got2 != tt.want2 {
				t.Errorf("serviceExistsUnlocked() = %v, want %v", got2, tt.want2)
			}
			if got3 != tt.want3 {
				t.Errorf("serviceExistsUnlocked() = %v, want %v", got3, tt.want3)
			}
		})
	}
}
