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

	serviceGroupMembers = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "service_group_members",
			Help: "Number of members in each service group",
		},
		[]string{"memberOf"},
	)
)
