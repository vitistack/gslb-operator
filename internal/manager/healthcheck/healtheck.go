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

	bslog.Debug("check complete", slog.Float64("duration_ms", float64(time.Since(hj.lastCheck).Milliseconds())))
	return err
}

func (hj *HealthCheckJob) OnSuccess() {
	healthChecksTotal.WithLabelValues(hj.Service.String(), "success").Inc()
	healthCheckDuration.WithLabelValues(hj.Service.String()).Observe(float64(time.Since(hj.lastCheck).Milliseconds()))
	hj.Service.OnSuccess()
}

func (hj *HealthCheckJob) OnFailure(err error) {
	healthChecksTotal.WithLabelValues(hj.Service.String(), "failure").Inc()
	healthCheckDuration.WithLabelValues(hj.Service.String()).Observe(float64(time.Since(hj.lastCheck).Milliseconds()))
	hj.Service.OnFailure(err)
}
