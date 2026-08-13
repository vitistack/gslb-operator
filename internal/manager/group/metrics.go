package group

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	serviceGroupMembers = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "service_group_members",
			Help: "Number of members in each service group",
		},
		[]string{"memberOf"},
	)
)
