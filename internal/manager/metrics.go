package manager

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	workerPoolSize = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "worker_pool_size_total",
		Help: "Number of running workers that perform health checks",
	})

	serviceGroups = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "service_groups_total",
		Help: "Number of service groups",
	})
)
