package manager

import (
	"testing"
	"time"

	"github.com/vitistack/gslb-operator/internal/model"
	"github.com/vitistack/gslb-operator/internal/service"
	"github.com/vitistack/gslb-operator/internal/utils/timesutil"
)

var genericGSLBConfig = model.GSLBConfig{
	Address:    "192.168.1.1:80",
	Datacenter: "dc1",
	Interval:   timesutil.Duration(5 * time.Second),
	Priority:   1,
	Type:       "TCP-FULL",
}

func TestNewManager(t *testing.T) {
	manager := NewManager(5, 5)

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
	manager := NewManager(2, 10)

	svc := &service.Service{
		Addr:             "192.168.1.1:80",
		Datacenter:       "dc1",
		Interval:         timesutil.Duration(5 * time.Second),
		FailureThreshold: 3,
	}

	manager.RegisterService(svc, false)

	manager.mutex.RLock()
	services, ok := manager.servicesHealthCheck[svc.Interval]
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
	manager := NewManager(2, 10)

	svc := service.NewServiceFromGSLBConfig(genericGSLBConfig)
	svc.Interval = timesutil.Duration(5 * time.Second)

	manager.RegisterService(svc, false)
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
