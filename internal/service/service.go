package service

import (
	"fmt"
	"log/slog"
	"net"
	"time"

	"github.com/vitistack/gslb-operator/internal/checks"
	"github.com/vitistack/gslb-operator/internal/model"
	"github.com/vitistack/gslb-operator/internal/utils/timesutil"
	"github.com/vitistack/gslb-operator/pkg/bslog"
)

const DEFAULT_FAILURE_THRESHOLD = 3

type HealthChangeCallback func(healthy bool)

type Service struct {
	id                   string
	addr                 string
	Fqdn                 string
	MemberOf             string
	Datacenter           string
	checkType            string
	ScheduledInterval    timesutil.Duration
	defaultInterval      timesutil.Duration
	priority             int
	FailureThreshold     int
	failureCount         int
	check                func() error // TCP - half/full, HTTP(S)
	healthChangeCallback HealthChangeCallback
	isHealthy            bool
}

func NewServiceFromGSLBConfig(config model.GSLBConfig, dryRun bool) (*Service, error) {
	ip := net.ParseIP(config.Ip)
	if ip == nil {
		return nil, ErrUnableToParseIpAddr
	}

	addr, err := net.ResolveTCPAddr("tcp", fmt.Sprintf("%v:%v", ip.String(), config.Port))
	if err != nil {
		return nil, ErrUnableToResolveAddr
	}

	if config.ServiceID == "" {
		return nil, ErrEmptyServiceId
	}

	interval := CalculateInterval(config.Priority, config.Interval)
	svc := &Service{
		id:                config.ServiceID,
		addr:              addr.String(),
		Fqdn:              config.Fqdn,
		MemberOf:          config.MemberOf,
		Datacenter:        config.Datacenter,
		checkType:         config.CheckType,
		ScheduledInterval: interval,
		defaultInterval:   interval,
		priority:          config.Priority,
		FailureThreshold:  config.FailureThreshold,
		failureCount:      config.FailureThreshold,
		isHealthy:         false,
	}

	switch {
	case dryRun:
		svc.check = checks.DryRun()

	case config.CheckType == checks.HTTPS:
		svc.check = checks.HTTPCheck("https://"+svc.Fqdn, checks.DEFAULT_TIMEOUT)

	case config.CheckType == checks.HTTP:
		svc.check = checks.HTTPCheck("https://"+svc.Fqdn, checks.DEFAULT_TIMEOUT)

	case config.CheckType == checks.TCP_FULL:
		svc.check = checks.TCPFull(svc.addr, checks.DEFAULT_TIMEOUT)

	case config.CheckType == checks.TCP_HALF:
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
	bslog.Debug("Health-Check Successfull", slog.Any("service",s))
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
	bslog.Debug("Health-Check Failed", slog.Any("service", s), slog.String("error", err.Error()))
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

func (s *Service) GetID() string {
	return s.id
}

func (s *Service) ConfigChanged(other *Service) bool {
	if s.Fqdn != other.Fqdn ||
		s.addr != other.addr ||
		s.Datacenter != other.Datacenter ||
		s.FailureThreshold != other.FailureThreshold ||
		s.priority != other.priority ||
		s.checkType != other.checkType {
		return true
	}
	return false
}

// updates the configuration values of s with the values of new
func (s *Service) Assign(new *Service) {
	s.addr = new.addr
	s.check = new.check
	s.MemberOf = new.MemberOf
	s.priority = new.priority
	s.Datacenter = new.Datacenter
	s.defaultInterval = new.defaultInterval
	s.FailureThreshold = new.FailureThreshold
}

func (s *Service) LogValue() slog.Value {
    return slog.GroupValue(
        slog.String("id", s.id),
        slog.String("memberOf", s.MemberOf),
        slog.String("fqdn", s.Fqdn),
        slog.String("datacenter", s.Datacenter),
    )
}
