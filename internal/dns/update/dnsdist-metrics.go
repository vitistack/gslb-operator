package update

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	serverUpMetric = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "dnsdist_server_up",
			Help: "Whether the dnsdist backend server is reachable (1=up, 0=down)",
		},
		[]string{"dnsdist_server"},
	)

	//serverConnectErrorsMetric = promauto.NewCounterVec(
	//	prometheus.CounterOpts{
	//		Name: "dnsdist_server_connect_errors_total",
	//		Help: "Total number of connection errors per dnsdist backend server",
	//	},
	//	[]string{"dnsdist_server"},
	//)
)
