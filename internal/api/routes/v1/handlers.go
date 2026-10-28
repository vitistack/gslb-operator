package v1

import (
	"net/http"

	"github.com/vitistack/gslb-operator/internal/api/handlers/v1/dnsdist"
	"github.com/vitistack/gslb-operator/internal/api/handlers/v1/service"
	"github.com/vitistack/gslb-operator/internal/api/handlers/v1/spoofs"
	"github.com/vitistack/gslb-operator/internal/api/routes"
	"github.com/vitistack/gslb-operator/pkg/auth/authz"
	"github.com/vitistack/gslb-operator/pkg/rest/router"
)

// Handlers is every endpoint v1 serves. A later version overrides only the
// field(s) it changed and leaves the rest as v1's defaults.
type Handlers struct {
	Token          http.HandlerFunc
	Spoofs         http.HandlerFunc
	OverrideGet    http.HandlerFunc
	OverrideCreate http.HandlerFunc
	OverrideDelete http.HandlerFunc
	ServiceStatus  http.HandlerFunc
	DNSDistConfig  http.HandlerFunc
}

// DefaultHandlers builds v1's own handlers from deps.
func DefaultHandlers(deps routes.Dependencies) Handlers {
	statusHandler := service.NewGSLBServiceHandler(deps.StatusRepo)
	spoofsHandler := spoofs.NewSpoofsService(deps.ServicesStore, deps.Manager)
	dnsdistHandler := dnsdist.NewDNSDistService(deps.ServicesStore)

	return Handlers{
		Token:          deps.AuthService.Token,
		Spoofs:         spoofsHandler.GetFQDNSpoof,
		OverrideGet:    spoofsHandler.GetOverride,
		OverrideCreate: spoofsHandler.CreateOverride,
		OverrideDelete: spoofsHandler.DeleteOverride,
		ServiceStatus:  statusHandler.GetServiceStatus,
		DNSDistConfig:  dnsdistHandler.GetConfig,
	}
}

// Mount wires routes onto g using whatever is in h, so callers can swap
// individual handlers (e.g. for v2) before calling this.
func Mount(g *router.RouteGroup, deps routes.Dependencies, h Handlers) {
	authRouter := g.Group("/auth")
	authRouter.POST("/token", h.Token).Public()

	gslbServiceRouter := g.Group("/service/{memberOf}").Use(deps.SecuredChain)
	gslbServiceRouter.GET("/spoofs", h.Spoofs).Use(deps.SecuredChain).Action(authz.SpoofsRead)
	gslbServiceRouter.GET("/override", h.OverrideGet).Action(authz.OverrideRead)
	gslbServiceRouter.POST("override", h.OverrideCreate).Action(authz.OverrideCreate)
	gslbServiceRouter.DELETE("/override", h.OverrideDelete).Action(authz.OverrideDelete)

	if deps.StatusEnabled {
		gslbServiceRouter.GET("/status", h.ServiceStatus).Action(authz.StatusRead)
	}

	dnsdistRouter := g.Group("/dnsdist").Use(deps.SecuredChain)
	dnsdistRouter.GET("/config", h.DNSDistConfig).Action(authz.SpoofsList)
}
