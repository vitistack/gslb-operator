package service

import (
	"errors"
	"log"
	"testing"
	"time"

	"github.com/vitistack/gslb-operator/internal/checks"
	"github.com/vitistack/gslb-operator/internal/model"
	"github.com/vitistack/gslb-operator/internal/utils/timesutil"
	"go.uber.org/zap"
)

type Test struct {
	Name            string
	InputService    Service
	ExpectedHealthy bool
	FailureCount    int
}

func TestCalculateInterval(t *testing.T) {
	interval := CalculateInterval(1, timesutil.FromDuration(time.Second*5))
	if interval != timesutil.Duration(checks.MIN_CHECK_INTERVAL) {
		t.Errorf("expected %v, but got: %v", checks.MIN_CHECK_INTERVAL.String(), interval.String())
	}

	interval = CalculateInterval(0, timesutil.FromDuration(time.Second*5))
	if interval != timesutil.Duration(checks.MIN_CHECK_INTERVAL) {
		t.Errorf("expected %v, but got: %v", checks.MIN_CHECK_INTERVAL.String(), interval.String())
	}

	interval = CalculateInterval(2, timesutil.FromDuration(time.Second*5))
	if interval != timesutil.FromDuration(checks.MIN_CHECK_INTERVAL*3) {
		t.Errorf("expected %v, but got: %v", (checks.MIN_CHECK_INTERVAL * 3).String(), interval.String())
	}

	interval = CalculateInterval(3, timesutil.FromDuration(time.Second*5))
	if interval != timesutil.FromDuration(checks.MIN_CHECK_INTERVAL*9) {
		t.Errorf("expected %v, but got: %v", (checks.MIN_CHECK_INTERVAL * 9).String(), interval.String())
	}

	interval = CalculateInterval(10, timesutil.FromDuration(time.Second*5))
	if interval != timesutil.FromDuration(checks.MAX_CHECK_INTERVAL) {
		t.Errorf("expected %v, but got: %v", checks.MAX_CHECK_INTERVAL.String(), interval.String())
	}
}

func TestOnSuccess(t *testing.T) {
	svc0 := Service{
		failureCount: 0,
		healthChangeCallback: func(health bool) {

		},
		FailureThreshold: 3,
		isHealthy:        false,
	}
	svc1 := Service{
		failureCount: 1,
		healthChangeCallback: func(health bool) {

		},
		FailureThreshold: 3,
		isHealthy:        false,
	}
	svc2 := Service{
		failureCount: 2,
		healthChangeCallback: func(health bool) {

		},
		FailureThreshold: 3,
		isHealthy:        false,
	}
	svc3 := Service{
		failureCount: 3,
		healthChangeCallback: func(health bool) {

		},
		FailureThreshold: 3,
		isHealthy:        false,
	}
	svc4 := Service{
		failureCount: 0,
		healthChangeCallback: func(health bool) {

		},
		FailureThreshold: 3,
		isHealthy:        true,
	}

	tests := []Test{
		{
			Name:            "failure-count-on-0-unHealthy",
			InputService:    svc0,
			ExpectedHealthy: true,
			FailureCount:    0,
		},
		{
			Name:            "failure-count-on-1-unHealthy",
			InputService:    svc1,
			ExpectedHealthy: true,
			FailureCount:    1,
		},
		{
			Name:            "failure-count-on-2-unHealthy",
			InputService:    svc2,
			ExpectedHealthy: false,
			FailureCount:    2,
		},
		{
			Name:            "failure-count-on-3-unHealthy",
			InputService:    svc3,
			ExpectedHealthy: false,
			FailureCount:    3,
		},
		{
			Name:            "failure-count-on-0-healthy",
			InputService:    svc4,
			ExpectedHealthy: true,
			FailureCount:    0,
		},
	}
	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			test.InputService.OnSuccess()
			if test.InputService.IsHealthy() != test.ExpectedHealthy {
				t.Errorf("Expected health: %v, but got: %v. With failureCount set to: %v before OnSuccess()", test.ExpectedHealthy, test.InputService.IsHealthy(), test.InputService.failureCount+1)
			}
		})
	}

	svc0.isHealthy = true
	for range svc0.FailureThreshold - 1 {
		svc0.OnFailure(errors.New("test error"))
	}
	log.Printf("count: %v", svc0.failureCount)
	svc0.OnSuccess()
	log.Printf("count: %v", svc0.failureCount)

	if !svc0.isHealthy {
		t.Errorf("Expected health: %v, but got: %v. After 2x OnFailure before OnSuccess()", true, svc0.IsHealthy())
	}

	for range svc0.FailureThreshold {
		svc0.OnFailure(errors.New("test error"))
	}
	log.Printf("count: %v", svc0.failureCount)
	svc0.OnSuccess()
	log.Printf("count: %v", svc0.failureCount)

	if svc0.isHealthy {
		t.Fatalf("Expected health: %v, but got: %v. After 3x OnFailure before OnSuccess()", false, svc0.IsHealthy())
	}
}

func TestOnFailure(t *testing.T) {
	svc0 := Service{
		failureCount: 0,
		healthChangeCallback: func(health bool) {

		},
		FailureThreshold: 3,
		isHealthy:        true,
	}
	svc1 := Service{
		failureCount: 1,
		healthChangeCallback: func(health bool) {

		},
		FailureThreshold: 3,
		isHealthy:        true,
	}
	svc2 := Service{
		failureCount: 2,
		healthChangeCallback: func(health bool) {

		},
		FailureThreshold: 3,
		isHealthy:        true,
	}
	svc3 := Service{
		failureCount: 3,
		healthChangeCallback: func(health bool) {

		},
		FailureThreshold: 3,
		isHealthy:        true,
	}
	svc4 := Service{
		failureCount: 0,
		healthChangeCallback: func(health bool) {

		},
		FailureThreshold: 3,
		isHealthy:        false,
	}

	tests := []Test{
		{
			Name:            "failure-count-on-0-Healthy",
			InputService:    svc0,
			ExpectedHealthy: true,
			FailureCount:    0,
		},
		{
			Name:            "failure-count-on-1-Healthy",
			InputService:    svc1,
			ExpectedHealthy: true,
			FailureCount:    1,
		},
		{
			Name:            "failure-count-on-2-Healthy",
			InputService:    svc2,
			ExpectedHealthy: false,
			FailureCount:    2,
		},
		{
			Name:            "failure-count-on-3-Healthy",
			InputService:    svc3,
			ExpectedHealthy: false,
			FailureCount:    3,
		},
		{
			Name:            "failure-count-on-0-unHealthy",
			InputService:    svc4,
			ExpectedHealthy: false,
			FailureCount:    0,
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			test.InputService.OnFailure(errors.New("test"))
			if test.InputService.IsHealthy() != test.ExpectedHealthy {
				t.Errorf("Expected health: %v, but got: %v. With failureCount set to: %v before OnFailure()", test.ExpectedHealthy, test.InputService.IsHealthy(), test.FailureCount)
			}
		})
	}

	svc0.isHealthy = false
	svc0.failureCount = 3
	for range svc0.FailureThreshold - 1 {
		svc0.OnSuccess()
	}
	log.Printf("count: %v", svc0.failureCount)
	svc0.OnFailure(errors.New("test"))
	log.Printf("count: %v", svc0.failureCount)

	if svc0.isHealthy {
		t.Errorf("Expected health: %v, but got: %v. After 2x OnSuccess() before OnFailure()", false, svc0.IsHealthy())
	}

	for range svc0.FailureThreshold {
		svc0.OnSuccess()
	}
	log.Printf("count: %v", svc0.failureCount)
	svc0.OnFailure(errors.New("test"))
	log.Printf("count: %v", svc0.failureCount)

	if !svc0.isHealthy {
		t.Fatalf("Expected health: %v, but got: %v. After 3x OnSuccess() before OnFailure()", true, svc0.IsHealthy())
	}
}

func TestService_GetBaseInterval(t *testing.T) {
	log, _ := zap.NewDevelopment()
	tests := []struct {
		name string // description of this test case
		// Named input parameters for receiver constructor.
		config model.GSLBConfig
		logger *zap.SugaredLogger
		dryRun bool
		want   timesutil.Duration
	}{
		{
			name: "baseinterval-5-priority-1",
			config: model.GSLBConfig{
				Fqdn:       "test.nhn.no",
				Ip:         "127.0.0.1",
				Port:       "80",
				Datacenter: "Abels1",
				Interval:   timesutil.FromDuration(time.Second * 5),
				Priority:   1,
				Type:       "TCP-FULL",
			},
			logger: log.Sugar(),
			dryRun: true,
			want:   timesutil.FromDuration(time.Second * 5),
		},
		{
			name: "baseinterval-5-priority-2",
			config: model.GSLBConfig{
				Fqdn:       "test.nhn.no",
				Ip:         "127.0.0.1",
				Port:       "80",
				Datacenter: "Abels1",
				Interval:   timesutil.FromDuration(time.Second * 5),
				Priority:   2,
				Type:       "TCP-FULL",
			},
			logger: log.Sugar(),
			dryRun: true,
			want:   timesutil.FromDuration(time.Second * 5),
		},
		{
			name: "baseinterval-5-priority-3",
			config: model.GSLBConfig{
				Fqdn:       "test.nhn.no",
				Ip:         "127.0.0.1",
				Port:       "80",
				Datacenter: "Abels1",
				Interval:   timesutil.FromDuration(time.Second * 5),
				Priority:   3,
				Type:       "TCP-FULL",
			},
			logger: log.Sugar(),
			dryRun: true,
			want:   timesutil.FromDuration(time.Second * 5),
		},
		{
			name: "baseinterval-5-priority-4",
			config: model.GSLBConfig{
				Fqdn:       "test.nhn.no",
				Ip:         "127.0.0.1",
				Port:       "80",
				Datacenter: "Abels1",
				Interval:   timesutil.FromDuration(time.Second * 5),
				Priority:   4,
				Type:       "TCP-FULL",
			},
			logger: log.Sugar(),
			dryRun: true,
			want:   timesutil.FromDuration(time.Second * 5),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, err := NewServiceFromGSLBConfig(tt.config, tt.logger, tt.dryRun)
			if err != nil {
				t.Fatalf("could not construct receiver type: %v", err)
			}
			got := s.GetBaseInterval()

			log.Sugar().Debug(got, tt.want, s.ScheduledInterval)
			if got != tt.want {
				t.Errorf("GetBaseInterval() = %v, want %v", got, tt.want)
			}
		})
	}
}
