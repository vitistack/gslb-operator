package service

import (
	"fmt"
	"log/slog"
	"reflect"
	"strings"
	"sync"
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
	fqdn                 string
	memberOf             string
	port                 string
	datacenter           string
	views                []string
	checkType            string
	checkScript          *model.LuaScript // lua script for HTTPS response validation
	ScheduledInterval    timesutil.Duration
	defaultInterval      timesutil.Duration
	priority             int
	failureThreshold     int
	failureCount         int
	checker              checks.Checker
	onHealthChange       HealthChangeCallback
	onFailureCountUpdate FailureCountCallback
	isHealthy            bool
	dryRun               bool
	mu                   *sync.Mutex
}

func NewServiceFromGSLBConfig(cfg model.GSLBConfig, opts ...ServiceOption) (*Service, error) {
	if strings.Trim(cfg.ServiceID, " ") == "" {
		return nil, ErrEmptyServiceId
	}

	port := ":443"
	if cfg.Port != "" && cfg.Port != "443" {
		port = fmt.Sprintf(":%s", cfg.Port)
	}

	validviews := make([]string, 0, len(cfg.Views))
	if config.DNS().Enable() {
		for _, view := range cfg.Views {
			if dnsviews.Valid(view) {
				validviews = append(validviews, view)
			}
		}
	}

	if len(validviews) == 0 {
		validviews = append(validviews, config.DNS().DefaultView())
	}

	interval := CalculateInterval(cfg.Priority, cfg.Interval)
	svc := &Service{
		id:                cfg.ServiceID,
		address:           cfg.Address,
		fqdn:              cfg.Fqdn,
		memberOf:          cfg.MemberOf,
		port:              port,
		datacenter:        cfg.Datacenter,
		views:             validviews,
		checkType:         cfg.CheckType,
		checkScript:       cfg.Script,
		ScheduledInterval: interval,
		defaultInterval:   interval,
		priority:          cfg.Priority,
		failureThreshold:  cfg.FailureThreshold,
		failureCount:      cfg.FailureThreshold, // need to succeed check N times before healthy!
		isHealthy:         false,
		dryRun:            false,
		mu:                &sync.Mutex{},
	}

	for _, opt := range opts {
		opt(svc)
	}

	switch {
	case svc.dryRun:
		svc.checker = &checks.DryRun{}

	case cfg.CheckType == checks.HTTPS:
		svc.checker = checks.NewHTTPChecker("https://"+svc.fqdn, checks.DEFAULT_TIMEOUT, cfg.Script)

	case cfg.CheckType == checks.HTTP:
		svc.checker = checks.NewHTTPChecker("http://"+svc.fqdn, checks.DEFAULT_TIMEOUT, cfg.Script)

	case cfg.CheckType == checks.TCP_FULL:
		svc.checker = checks.NewTCPFullChecker(svc.address.PrimaryTCPAddr(svc.port), checks.DEFAULT_TIMEOUT)

	case cfg.CheckType == checks.TCP_HALF:
		svc.checker = checks.NewTCPHalfChecker(svc.address.PrimaryTCPAddr(svc.port), checks.DEFAULT_TIMEOUT)

	default:
		svc.checker = checks.NewTCPFullChecker(svc.address.PrimaryTCPAddr(svc.port), checks.DEFAULT_TIMEOUT)
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
	s.mu.Lock()
	checker := s.checker
	s.mu.Unlock()
	return checker.Check()
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
	s.mu.Lock()
	if s.isHealthy { // already healthy
		previousFailureCount := s.failureCount
		s.failureCount = 0

		s.mu.Unlock()
		if previousFailureCount > 0 {
			s.onFailureCountUpdate(s.GSLBService())
		}

		return
	}

	if s.failureCount > 0 {
		s.failureCount--
	}

	becameHealthy := s.failureCount == 0
	if becameHealthy {
		s.isHealthy = true
	}
	s.mu.Unlock()

	if s.failureCount == 0 {
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
	s.mu.Lock()
	if !s.isHealthy { // already unhealthy
		s.failureCount = s.failureThreshold
		s.mu.Unlock()
		return
	}

	if s.failureCount < s.failureThreshold {
		s.failureCount++
	}

	becameUnHalthy := s.failureCount == s.failureThreshold
	if becameUnHalthy {
		s.isHealthy = false
	}
	s.mu.Unlock()

	if becameUnHalthy { // threshold reached, service is considered down
		s.onHealthChange(&HealthChangeEvent{
			Svc:     s,
			Healthy: false,
		})
	} else {
		s.onFailureCountUpdate(s.GSLBService())
	}
}

func (s *Service) SetHealthChangeCallback(callback HealthChangeCallback) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onHealthChange = callback
}

func (s *Service) SetFailureCountCallback(callback FailureCountCallback) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onFailureCountUpdate = callback
}

func (s *Service) IsHealthy() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.isHealthy
}

func (s *Service) GetFqdn() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.fqdn
}

func (s *Service) GetMemberOf() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.memberOf
}

func (s *Service) GetPort() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.port
}

func (s *Service) GetDatacenter() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.datacenter
}

func (s *Service) GetViews() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.views
}

func (s *Service) GetPriority() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.priority
}

func (s *Service) GetAddress() ip.Address {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.address
}

func (s *Service) GetDefaultInterval() timesutil.Duration {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.defaultInterval
}

func (s *Service) GetID() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.id
}

func (s *Service) GetFailureCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.failureCount
}

func (s *Service) GetFailureThreshold() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.failureThreshold
}

func (s *Service) GetAverageRoundtrip() time.Duration {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.checker.Roundtrip()
}

func (s *Service) ConfigChanged(other model.GSLBConfig) bool {
	cfgSelf := s.GSLBConfig()
	other.Interval = CalculateInterval(other.Priority, other.Interval)
	return !reflect.DeepEqual(cfgSelf, other)
	//if s.fqdn != other.fqdn ||
	//	s.addr.String() != other.addr.String() ||
	//	s.datacenter != other.datacenter ||
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
	s.mu.Lock()
	defer s.mu.Unlock()
	s.fqdn = new.fqdn
	s.port = new.port
	s.views = new.views
	s.checker = new.checker
	s.memberOf = new.memberOf
	s.priority = new.priority
	s.checkType = new.checkType
	s.checkScript = new.checkScript
	s.datacenter = new.datacenter
	s.defaultInterval = new.defaultInterval
	s.failureThreshold = new.failureThreshold
}

func (s *Service) LogValue() slog.Value {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s == nil {
		return slog.StringValue("nil")
	}

	return slog.GroupValue(
		slog.String("id", s.id),
		slog.String("memberOf", s.memberOf),
		slog.String("fqdn", s.fqdn),
		slog.String("datacenter", s.datacenter),
		slog.String("address", s.address.String()),
	)
}

// satisfies the stringer interface to allow passing s for %v in formatted strings
func (s *Service) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return fmt.Sprintf("%s:%s:%s:%s:%s", s.id, s.memberOf, s.fqdn, s.datacenter, s.address.String())
}

func (s *Service) GSLBService() *model.GSLBService {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := &model.GSLBService{
		ID:           s.id,
		MemberOf:     s.memberOf,
		Fqdn:         s.fqdn,
		Port:         s.port,
		Datacenter:   s.datacenter,
		Views:        s.views,
		Address:      s.address,
		IsHealthy:    s.isHealthy,
		FailureCount: s.failureCount,
	}

	return out
}

func (s *Service) GSLBConfig() model.GSLBConfig {
	s.mu.Lock()
	defer s.mu.Unlock()
	return model.GSLBConfig{
		ServiceID:        s.id,
		MemberOf:         s.memberOf,
		Fqdn:             s.fqdn,
		Address:          s.address,
		Port:             strings.Trim(s.port, ":"),
		Datacenter:       s.datacenter,
		Views:            s.views,
		Interval:         s.defaultInterval,
		Priority:         s.priority,
		FailureThreshold: s.failureThreshold,
		CheckType:        s.checkType,
		Script:           s.checkScript,
	}
}
