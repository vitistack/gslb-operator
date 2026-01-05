package service

import (
	"fmt"
	"math/rand"
	"net"
	"time"

	"github.com/vitistack/gslb-operator/internal/checks"
	"github.com/vitistack/gslb-operator/internal/model"
	"github.com/vitistack/gslb-operator/internal/utils/timesutil"
	"go.uber.org/zap"
)

type HealthChangeCallback func(healthy bool)

type Service struct {
	addr                 string
	Fqdn                 string
	Port                 string
	Datacenter           string
	ScheduledInterval    timesutil.Duration
	defaultInterval      timesutil.Duration
	priority             int
	FailureThreshold     int
	failureCount         int
	check                func() error // TCP - half/full, HTTP(S)
	healthChangeCallback HealthChangeCallback
	isHealthy            bool
	log                  *zap.SugaredLogger
}

func NewServiceFromGSLBConfig(config model.GSLBConfig, logger *zap.SugaredLogger, dryRun bool) (*Service, error) {
	ip := net.ParseIP(config.Ip)
	if ip == nil {
		return nil, ErrUnableToParseIpAddr
	}

	addr, err := net.ResolveTCPAddr("tcp", fmt.Sprintf("%v:%v", ip.String(), config.Port))
	if err != nil {
		return nil, ErrUnableToResolveAddr
	}
	interval := CalculateInterval(config.Priority, config.Interval)
	svc := &Service{
		addr:              addr.String(),
		Fqdn:              config.Fqdn,
		Port:              config.Port,
		Datacenter:        config.Datacenter,
		ScheduledInterval: interval,
		defaultInterval:   interval,
		priority:          config.Priority,
		FailureThreshold:  3,
		failureCount:      3,
		isHealthy:         false,
		log:               logger,
	}

	switch {
	case dryRun:
		svc.check = checks.DryRun()

	case config.Type == "HTTP":
		svc.check = checks.HTTPCheck(svc.addr, checks.DEFAULT_TIMEOUT)

	case config.Type == "TCP-FULL":
		svc.check = checks.TCPFull(svc.addr, checks.DEFAULT_TIMEOUT)

	case config.Type == "TCP-HALF":
		svc.check = checks.TCPHalf(svc.addr, checks.DEFAULT_TIMEOUT)

	default:
		svc.check = checks.TCPFull(svc.addr, checks.DEFAULT_TIMEOUT)
	}

	return svc, nil
}

// 5s, 15s, 45s, checks.MAX_CHECK_INTERVAL.
// Exponential growth of duration based on priority. Up to checks.MAX_CHECK_INTERVAL
func CalculateInterval(priority int, baseInterval timesutil.Duration) timesutil.Duration {
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

	jitter := float64(interval) * 0.1 * (2*rand.Float64() - 1)
	interval = time.Duration(float64(interval) + jitter) // Adds a ±10% jitter to interval

	return timesutil.Duration(interval)
}

// this is different from s.Interval. Because that is the interval the service is currently scheduled
// its base intervall is the intervall which resides in the services' GSLB - config in the dns - zone
func (s *Service) GetBaseInterval() timesutil.Duration {
	scaleFactor := 3.0
	multiplier := 1.0
	for i := 1; i < s.priority; i++ {
		multiplier *= scaleFactor
	}

	baseInterval := max(time.Duration(float64(s.defaultInterval)/multiplier), time.Second*5)

	return timesutil.Duration(baseInterval.Round(time.Second))
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
	s.log.Debugf("Health-Check on Service: %v:%v Successfull", s.addr, s.Datacenter)
	if s.isHealthy { // already healthy
		s.failureCount = 0
		return
	}

	if s.failureCount > 0 {
		s.failureCount--
	}

	if s.failureCount == 0 {
		s.isHealthy = true
		s.healthChangeCallback(true)
	}
}

// called when healthcheck fails
func (s *Service) OnFailure(err error) {
	s.log.Debugf("Health-Check on Service: %v:%v Failed: %s", s.addr, s.Datacenter, err.Error())
	if !s.isHealthy { // already unhealthy
		s.failureCount = s.FailureThreshold
		return
	}

	if s.failureCount < s.FailureThreshold {
		s.failureCount++
	}

	if s.failureCount == s.FailureThreshold { // threshold reached, service is considered down
		s.isHealthy = false
		s.healthChangeCallback(false)
	}
}

func (s *Service) SetCheck(check func() error) {
	s.check = check
}

func (s *Service) SetHealthChangeCallback(callback HealthChangeCallback) {
	s.healthChangeCallback = callback
}

func (s *Service) IsHealthy() bool {
	return s.isHealthy
}

func (s *Service) GetPriority() int {
	return s.priority
}

func (s *Service) GetIP() (string, error) {
	ip, _, err := net.SplitHostPort(s.addr)
	if err != nil {
		return "", fmt.Errorf("could not read ip from network address: %s: %s", s.addr, err.Error())
	}
	return ip, nil
}

func (s *Service) GetDefaultInterval() timesutil.Duration {
	return s.defaultInterval
}

// copies necessary private values from old, to the service pointed to by s
func (s *Service) Copy(old *Service) *Service {
	s.isHealthy = old.isHealthy
	s.failureCount = old.failureCount
	s.healthChangeCallback = old.healthChangeCallback
	s.defaultInterval = old.defaultInterval
	return s
}
