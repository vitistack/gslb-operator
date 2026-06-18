package healthcheck

import (
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/vitistack/gslb-operator/internal/checks"
	"github.com/vitistack/gslb-operator/internal/service"
	"github.com/vitistack/gslb-operator/pkg/bslog"
)

type HealthCheckJob struct {
	Service   *service.Service
	lastCheck time.Time
	duration  float64
}

func NewJob(svc *service.Service) *HealthCheckJob {
	return &HealthCheckJob{
		Service: svc,
	}
}

func (hj *HealthCheckJob) Execute() error {
	err := hj.Service.Execute()
	result, ok := errors.AsType[*checks.HealthCheckResult](err)
	if !ok {
		return fmt.Errorf("health-check returned wrong error type: %T", err)
	}

	hj.duration = result.CheckTime
	healthCheckDuration.WithLabelValues(
		hj.Service.MemberOf,
		hj.Service.Fqdn,
		hj.Service.Datacenter).
		Observe(result.CheckTime)

	return result.Err()
}

func (hj *HealthCheckJob) OnSuccess() {
	bslog.HealthCheck("Health-Check Successful",
		slog.Float64("duration_ms", hj.duration),
		slog.Any("service", hj.Service),
	)

	healthChecksTotal.WithLabelValues(hj.Service.MemberOf,
		hj.Service.Fqdn,
		hj.Service.Datacenter,
		"success").
		Inc()
	hj.Service.OnSuccess()
}

func (hj *HealthCheckJob) OnFailure(err error) {
	bslog.HealthCheck("Health-Check Failed",
		slog.Float64("duration_ms", hj.duration),
		slog.Any("service", hj.Service),
		slog.String("error", err.Error()),
	)

	healthChecksTotal.WithLabelValues(hj.Service.MemberOf,
		hj.Service.Fqdn,
		hj.Service.Datacenter,
		"failure").
		Inc()
	hj.Service.OnFailure(err)
}
