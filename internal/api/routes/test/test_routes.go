package test_routes

import (
	"log/slog"
	"net/http"

	"github.com/vitistack/gslb-operator/internal/api/routes"
	v1 "github.com/vitistack/gslb-operator/internal/api/routes/v1"
	"github.com/vitistack/gslb-operator/internal/config"
	"github.com/vitistack/gslb-operator/pkg/bslog"
	"github.com/vitistack/gslb-operator/pkg/rest/middleware"
	"github.com/vitistack/gslb-operator/pkg/rest/router"
)

func Register(parent *router.RouteGroup, deps routes.Dependencies) *router.RouteGroup {
	if !config.DevMode() {
		return nil
	}

	deps.SecuredChain = middleware.Passthrough()

	g := parent.VersionGroup("/test").Public()
	Mount(g, deps, handlers(deps))
	bslog.Warn("mounted unauthenticated test API mirror", slog.String("prefix", g.Prefix()))
	return g
}

func Mount(g *router.RouteGroup, deps routes.Dependencies, handlers Handlers) {
	v1.Mount(g, deps, handlers.Handlers)

	g.GET("/ping", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("pong"))
	}).Public()
}

func handlers(deps routes.Dependencies) Handlers {
	h := Handlers{Handlers: v1.DefaultHandlers(deps)}
	return h
}
