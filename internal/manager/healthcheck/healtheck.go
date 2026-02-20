package healthcheck

import (
	"time"

	"github.com/vitistack/gslb-operator/internal/service"
)

// the result of a singular health-check
type Result struct {
	Success   bool
	timeTaken time.Duration
}

type HealthCheckJob struct {
	service   *service.Service
	metrics   *HealthCheckMetricsCounter
	lastCheck time.Time
}

func NewJob(svc *service.Service) *HealthCheckJob {
	return &HealthCheckJob{
		service: svc,
		metrics: NewMetricsCounter(1_000),
	}
}

func (hj *HealthCheckJob) Execute() error {
	hj.lastCheck = time.Now()
	return hj.service.Execute()
}

func (hj *HealthCheckJob) OnSuccess() {
	hj.metrics.Record(&Result{
		Success:   true,
		timeTaken: time.Since(hj.lastCheck),
	})
	hj.service.OnSuccess()
}

func (hj *HealthCheckJob) OnFailure(err error) {
	hj.metrics.Record(&Result{
		Success:   false,
		timeTaken: time.Since(hj.lastCheck),
	})
	hj.service.OnFailure(err)
}
