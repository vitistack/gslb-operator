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
		[]string{"memberOf",  "endpoint", "datacenter", "status"},
	)

	healthCheckDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name: "healthcheck_duration_ms",
			Help: "Health check duration",
			Buckets: []float64{1, 5, 25, 50, 100, 250, 500, 1000, 2500, 5000},
		},
		[]string{"memberOf", "endpoint", "datacenter"},
	)
)

