package healthcheck

import (
	"log/slog"
	"time"

	"github.com/vitistack/gslb-operator/internal/service"
	"github.com/vitistack/gslb-operator/pkg/bslog"
)

type HealthCheckJob struct {
	Service   *service.Service
	lastCheck time.Time
}

func NewJob(svc *service.Service) *HealthCheckJob {
	return &HealthCheckJob{
		Service: svc,
	}
}

func (hj *HealthCheckJob) Execute() error {
	hj.lastCheck = time.Now()
	err := hj.Service.Execute()

	checkTimeMs := float64(time.Since(hj.lastCheck).Milliseconds())

	bslog.Debug("check complete", slog.Float64("duration_ms", checkTimeMs))
	healthCheckDuration.WithLabelValues(
		hj.Service.MemberOf,
		hj.Service.Fqdn,
		hj.Service.Datacenter).
		Observe(checkTimeMs)
	return err
}

func (hj *HealthCheckJob) OnSuccess() {
	healthChecksTotal.WithLabelValues(hj.Service.MemberOf,
		hj.Service.Fqdn,
		hj.Service.Datacenter,
		"success").
		Inc()
	hj.Service.OnSuccess()
}

func (hj *HealthCheckJob) OnFailure(err error) {
	healthChecksTotal.WithLabelValues(hj.Service.MemberOf,
		hj.Service.Fqdn,
		hj.Service.Datacenter,
		"failure").
		Inc()
	hj.Service.OnFailure(err)
}
