package v1

import (
	"log/slog"

	"github.com/vitistack/gslb-operator/internal/api/routes"
	"github.com/vitistack/gslb-operator/pkg/bslog"
	"github.com/vitistack/gslb-operator/pkg/rest/router"
)

// Register wires v1's endpoints under parent as a version-tagged "/v1" group.
func Register(parent *router.RouteGroup, deps routes.Dependencies) *router.RouteGroup {
	g := parent.VersionGroup("/v1")
	Mount(g, deps, DefaultHandlers(deps))
	bslog.Info("mounted API version", slog.String("version", "v1"), slog.String("prefix", g.Prefix()))
	return g
}

// RegisterLatest mounts the same v1 endpoints directly under parent, with no
// "/vN" segment, for the unversioned "latest" alias (e.g. /api/*)
func RegisterLatest(parent *router.RouteGroup, deps routes.Dependencies) *router.RouteGroup {
	g := parent.Group("")
	Mount(g, deps, DefaultHandlers(deps))
	return g
}
