package service

import (
	"time"

	"github.com/vitistack/gslb-operator/internal/checks"
	"github.com/vitistack/gslb-operator/internal/model"
	"github.com/vitistack/gslb-operator/internal/utils/timesutil"
)

type HealthCallback func(health bool)

type Service struct {
	Addr             string
	Datacenter       string
	Interval         timesutil.Duration
	FailureThreshold int
	failureCount     int
	check            func() error // TCP - half/full, HTTP(S)
	healthCallback   HealthCallback
	isHealthy        bool
}

func NewServiceFromGSLBConfig(config model.GSLBConfig) *Service {
	svc := &Service{
		Addr:             config.Address,
		Datacenter:       config.Datacenter,
		Interval:         calculateInterval(config.Priority, config.Interval),
		FailureThreshold: 3,
		failureCount:     3,
		isHealthy:        false,
	}

	switch config.Type {
	case "HTTP":
		svc.check = checks.HTTPCheck(svc.Addr, checks.DEFAULT_TIMEOUT)

	case "TCP-FULL":
		svc.check = checks.TCPFull(svc.Addr, checks.DEFAULT_TIMEOUT)

	case "TCP-HALF":
		svc.check = checks.TCPHalf(svc.Addr, checks.DEFAULT_TIMEOUT)

	default:
		svc.check = checks.TCPFull(svc.Addr, checks.DEFAULT_TIMEOUT)
	}

	return svc
}

// 5s, 15s, 45s, checks.MAX_CHECK_INTERVAL.
// Exponential growth of duration based on priority. Up to checks.MAX_CHECK_INTERVAL
func calculateInterval(priority int, baseInterval timesutil.Duration) timesutil.Duration {
	scaleFactor := 3.0

	if priority < 1 {
		priority = 1
	}
	// Calculate: baseInterval * (scaleFactor ^ (priority - 1))
	multiplier := 1.0
	for i := 1; i < priority; i++ {
		multiplier *= scaleFactor
	}

	interval := time.Duration(float64(baseInterval) * multiplier)
	if interval > checks.MAX_CHECK_INTERVAL {
		return timesutil.Duration(checks.MAX_CHECK_INTERVAL)
	}

	return timesutil.Duration(interval)
}

// checks health of service
func (s *Service) Execute() error {
	return s.check()
}

/*
start values:
	- count = 3
	- healthy = false

OnFailure : count = 3, healthy = false

OnSuccess : count = 2, healthy = false
OnFailure : count = 3, healthy = false

OnSuccess : count = 2, healthy = false
OnSuccess : count = 1, healthy = false
OnFailure : count = 3, healthy = false

OnSuccess : count = 2, healthy = false
OnSuccess : count = 1, healthy = false
OnSuccess : count = 0, healthy = true -> update DNS

OnSuccess : count = 0, healthy = true

OnFailure : count = 1, healthy = true
OnSuccess : count = 0, healthy = true

OnFailure : count = 1, healthy = true
OnFailure : count = 2, healthy = true
OnSuccess : count = 0, healthy = true

OnFailure : count = 1, healthy = true
OnFailure : count = 2, healthy = true
OnFailure : count = 3, healthy = false -> update DNS
*/

// called when healthcheck is successful
func (s *Service) OnSuccess() {
	if s.isHealthy { // already healthy
		s.failureCount = 0
		return
	}

	if s.failureCount > 0 {
		s.failureCount--
	}

	if s.failureCount == 0 {
		s.isHealthy = true
		s.healthCallback(true)
	}
}

// called when healthcheck fails
func (s *Service) OnFailure(err error) {
	if !s.isHealthy { // already unhealthy
		s.failureCount = s.FailureThreshold
		return
	}

	if s.failureCount < s.FailureThreshold {
		s.failureCount++
	}

	if s.failureCount == s.FailureThreshold { // threshold reached, service is considered down
		s.isHealthy = false
		s.healthCallback(false)
	}
}

func (s *Service) SetCheck(check func() error) {
	s.check = check
}

func (s *Service) SetHealthCheckCallback(callback HealthCallback) {
	s.healthCallback = callback
}

func (s *Service) IsHealthy() bool {
	return s.isHealthy
}

// copies private values from old, to the service pointed to by s
func (s *Service) Copy(old *Service) *Service {
	s.check = old.check
	s.isHealthy = old.isHealthy
	s.failureCount = old.failureCount
	s.healthCallback = old.healthCallback
	return s
}
