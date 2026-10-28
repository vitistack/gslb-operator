package debug

import (
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

		g.GET("/pprof/", pprof.Index)
		g.GET("/pprof/heap/", pprof.Handler("heap").ServeHTTP)

		bslog.Info("mounted debug API")
	}

	return g
}
