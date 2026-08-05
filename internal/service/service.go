package service

import (
	"fmt"
	"log/slog"
	"reflect"
	"strings"
	"time"

	"github.com/vitistack/gslb-operator/internal/checks"
	"github.com/vitistack/gslb-operator/internal/config"
	dnsviews "github.com/vitistack/gslb-operator/internal/dns/views"
	"github.com/vitistack/gslb-operator/internal/model"
	"github.com/vitistack/gslb-operator/internal/utils/ip"
	"github.com/vitistack/gslb-operator/internal/utils/timesutil"
)

const DEFAULT_FAILURE_THRESHOLD = 3

type HealthChangeCallback func(*HealthChangeEvent)
type FailureCountCallback func(*model.GSLBService)

type HealthChangeEvent struct {
	Svc     *Service
	Healthy bool
}

type Service struct {
	id                   string
	address              ip.Address
	Fqdn                 string
	MemberOf             string
	Port                 string
	Datacenter           string
	Views                []string
	checkType            string
	checkScript          *model.LuaScript // lua script for HTTPS response validation
	ScheduledInterval    timesutil.Duration
	defaultInterval      timesutil.Duration
	priority             int
	FailureThreshold     int
	failureCount         int
	checker              checks.Checker
	onHealthChange       HealthChangeCallback
	onFailureCountUpdate FailureCountCallback
	isHealthy            bool
	dryRun               bool
}

func NewServiceFromGSLBConfig(cfg model.GSLBConfig, opts ...ServiceOption) (*Service, error) {
	if cfg.ServiceID == "" {
		return nil, ErrEmptyServiceId
	}

	port := ":443"
	if cfg.Port != "" && cfg.Port != "443" {
		port = fmt.Sprintf(":%s", cfg.Port)
	}

	validViews := make([]string, 0, len(cfg.Views))
	if config.SplitDNS().Enable() {
		for _, view := range cfg.Views {
			if dnsviews.Valid(view) {
				validViews = append(validViews, view)
			}
		}
	}

	if len(validViews) == 0 {
		validViews = append(validViews, config.SplitDNS().DefaultView())
	}

	interval := CalculateInterval(cfg.Priority, cfg.Interval)
	svc := &Service{
		id:                cfg.ServiceID,
		address:           cfg.Address,
		Fqdn:              cfg.Fqdn,
		MemberOf:          cfg.MemberOf,
		Port:              port,
		Datacenter:        cfg.Datacenter,
		Views:             cfg.Views,
		checkType:         cfg.CheckType,
		checkScript:       cfg.Script,
		ScheduledInterval: interval,
		defaultInterval:   interval,
		priority:          cfg.Priority,
		FailureThreshold:  cfg.FailureThreshold,
		failureCount:      cfg.FailureThreshold, // need to succeed check N times before healthy!
		isHealthy:         false,
		dryRun:            false,
	}

	for _, opt := range opts {
		opt(svc)
	}

	switch {
	case svc.dryRun:
		svc.checker = &checks.DryRun{}

	case cfg.CheckType == checks.HTTPS:
		svc.checker = checks.NewHTTPChecker("https://"+svc.Fqdn, checks.DEFAULT_TIMEOUT, cfg.Script)

	case cfg.CheckType == checks.HTTP:
		svc.checker = checks.NewHTTPChecker("https://"+svc.Fqdn, checks.DEFAULT_TIMEOUT, cfg.Script)

	case cfg.CheckType == checks.TCP_FULL:
		svc.checker = checks.NewTCPFullChecker(svc.address.PrimaryTCPAddr(svc.Port), checks.DEFAULT_TIMEOUT)

	case cfg.CheckType == checks.TCP_HALF:
		svc.checker = checks.NewTCPHalfChecker(svc.address.PrimaryTCPAddr(svc.Port), checks.DEFAULT_TIMEOUT)

	default:
		svc.checker = checks.NewTCPFullChecker(svc.address.PrimaryTCPAddr(svc.Port), checks.DEFAULT_TIMEOUT)
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
// its base intervall is the intervall which resides in the services' GSLB - cfg in the dns - zone
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
	return s.checker.Check()
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
		s.onHealthChange(&HealthChangeEvent{
			Svc:     s,
			Healthy: true,
		})
	} else {
		s.onFailureCountUpdate(s.GSLBService())
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
		s.onHealthChange(&HealthChangeEvent{
			Svc:     s,
			Healthy: false,
		})
	} else {
		s.onFailureCountUpdate(s.GSLBService())
	}
}

func (s *Service) SetHealthChangeCallback(callback HealthChangeCallback) {
	s.onHealthChange = callback
}

func (s *Service) SetFailureCountCallback(callback FailureCountCallback) {
	s.onFailureCountUpdate = callback
}

func (s *Service) IsHealthy() bool {
	return s.isHealthy
}

func (s *Service) GetPriority() int {
	return s.priority
}

func (s *Service) GetAddress() ip.Address {
	return s.address
}

func (s *Service) GetDefaultInterval() timesutil.Duration {
	return s.defaultInterval
}

func (s *Service) GetID() string {
	return s.id
}

func (s *Service) GetFailureCount() int {
	return s.failureCount
}

func (s *Service) GetAverageRoundtrip() time.Duration {
	return s.checker.Roundtrip()
}

func (s *Service) ConfigChanged(other model.GSLBConfig) bool {
	cfgSelf := s.GSLBConfig()
	other.Interval = CalculateInterval(other.Priority, other.Interval)
	return !reflect.DeepEqual(cfgSelf, other)
	//if s.Fqdn != other.Fqdn ||
	//	s.addr.String() != other.addr.String() ||
	//	s.Datacenter != other.Datacenter ||
	//	s.FailureThreshold != other.FailureThreshold ||
	//	s.priority != other.priority ||
	//	s.checkType != other.checkType ||
	//	s.checkScript != other.checkScript {
	//	return true
	//}
	//return false
}

// updates the cfguration values of s with the values of new
func (s *Service) Assign(new *Service) {
	s.Fqdn = new.Fqdn
	s.Port = new.Port
	s.checker = new.checker
	s.MemberOf = new.MemberOf
	s.priority = new.priority
	s.checkType = new.checkType
	s.checkScript = new.checkScript
	s.Datacenter = new.Datacenter
	s.defaultInterval = new.defaultInterval
	s.FailureThreshold = new.FailureThreshold
}

func (s *Service) LogValue() slog.Value {
	if s == nil {
		return slog.StringValue("nil")
	}

	return slog.GroupValue(
		slog.String("id", s.id),
		slog.String("memberOf", s.MemberOf),
		slog.String("fqdn", s.Fqdn),
		slog.String("datacenter", s.Datacenter),
		slog.String("address", s.address.String()),
	)
}

// satisfies the stringer interface to allow passing s for %v in formatted strings
func (s *Service) String() string {
	return fmt.Sprintf("%s:%s:%s:%s:%s", s.id, s.MemberOf, s.Fqdn, s.Datacenter, s.address.String())
}

func (s *Service) GSLBService() *model.GSLBService {
	out := &model.GSLBService{
		ID:           s.id,
		MemberOf:     s.MemberOf,
		Fqdn:         s.Fqdn,
		Port:         s.Port,
		Datacenter:   s.Datacenter,
		Views:        s.Views,
		Address:      s.address,
		IsHealthy:    s.isHealthy,
		FailureCount: s.failureCount,
	}

	return out
}

func (s *Service) GSLBConfig() model.GSLBConfig {
	return model.GSLBConfig{
		ServiceID:        s.id,
		MemberOf:         s.MemberOf,
		Fqdn:             s.Fqdn,
		Address:          s.address,
		Port:             strings.Trim(s.Port, ":"),
		Datacenter:       s.Datacenter,
		Views:            s.Views,
		Interval:         s.defaultInterval,
		Priority:         s.priority,
		FailureThreshold: s.FailureThreshold,
		CheckType:        s.checkType,
		Script:           s.checkScript,
	}
}
