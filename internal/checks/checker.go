package checks

import (
	"time"
)

type Checker interface {
	Check() *HealthCheckResult
	Roundtrip() time.Duration
}

type HealthCheckResult struct {
	// populated if an error happened during the health-check
	// e.g. failed healthcheck
	err error

	// wether the healthcheck was successfull or not
	Success bool

	// total checktime for healthcheck
	CheckTime float64
}

func (hr *HealthCheckResult) Err() error {
	return hr.err
}

func (hr *HealthCheckResult) Error() string {
	if hr.err != nil {
		return hr.err.Error()
	}
	return ""
}
