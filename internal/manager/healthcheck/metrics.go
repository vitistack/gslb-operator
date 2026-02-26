package healthcheck

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	healthChecksTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "healthcheck_total",
			Help: "Total health checks performed",
		},
		[]string{"service", "success"},
	)

	healthCheckDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name: "healthcheck_duration_ms",
			Help: "Health check duration",
		},
		[]string{"service"},
	)
)

