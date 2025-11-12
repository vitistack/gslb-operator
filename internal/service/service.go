package service

import (
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
		FailureThreshold: 3,
		failureCount:     0,
		isHealthy:        false, // TODO!!!!!! Haakon
	}

	switch config.Type {
	case "HTTP":
		svc.check = checks.HTTPCheck(svc.Addr, checks.DEFAULT_TIMEOUT)

	case "TCP-FULL":
		svc.check = checks.TCPFull(svc.Addr, checks.DEFAULT_TIMEOUT)

	case "TCP-HALF":
		svc.check = checks.TCPHalf(svc.Addr, checks.DEFAULT_TIMEOUT)
	}

	return svc
}

// checks health of service
func (s *Service) Execute() error {
	return s.check()
}

// called when healthcheck is successful
func (s *Service) OnSuccess() {
	s.failureCount--
	if !s.isHealthy && (s.failureCount%s.FailureThreshold == s.FailureThreshold) {
		s.isHealthy = true
		s.failureCount = 0
		s.healthCallback(true)
	}
}

// called when healthcheck fails
func (s *Service) OnFailure(err error) {
	s.failureCount++
	if s.failureCount%s.FailureThreshold == 0 { // threshold reached, service is considered down
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
	s.failureCount = old.failureCount
	s.check = old.check
	s.healthCallback = old.healthCallback
	s.isHealthy = old.isHealthy
	return s
}
