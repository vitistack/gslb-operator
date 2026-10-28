package metrics

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/vitistack/gslb-operator/pkg/rest/router"
)

func Register(parent *router.RouteGroup) *router.RouteGroup {
	g := parent.Group("/metrics").Public()

	g.Handle( // static wiring of prometheus metrics handler
		http.MethodGet,
		"",
		promhttp.Handler(),
	)

	return g
}
