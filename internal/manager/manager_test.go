package manager

import (
	"testing"
	"time"

	"github.com/vitistack/gslb-operator/internal/model"
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

	svc, _ := manager.RegisterService(genericGSLBConfig, false)

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
	manager := NewManager(
		logger,
		WithMinRunningWorkers(2),
		WithNonBlockingBufferSize(10),
	)

	manager.RegisterService(genericGSLBConfig, false)
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
