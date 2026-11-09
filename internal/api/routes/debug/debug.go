package debug

import (
	"net/http"
	"net/http/pprof"

	"github.com/vitistack/gslb-operator/internal/config"
	"github.com/vitistack/gslb-operator/pkg/bslog"
	"github.com/vitistack/gslb-operator/pkg/rest/router"
)

// Register wires v1's endpoints under parent as a version-tagged "/v1" group.
func Register(parent *router.RouteGroup) *router.RouteGroup {
	var g *router.RouteGroup
	if config.DevMode() {
		g = parent.Group("/debug").Public()

		g.Handle(http.MethodGet, "/pprof/", http.HandlerFunc(pprof.Index))
		g.Handle(http.MethodGet, "/pprof/heap/", pprof.Handler("heap"))

		bslog.Info("mounted debug API")
	}

	return g
}
