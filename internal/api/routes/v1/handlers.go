package v1

import (
	"log/slog"

	"github.com/vitistack/gslb-operator/internal/api/handlers/v1/admin"
	"github.com/vitistack/gslb-operator/internal/api/handlers/v1/auth"
	"github.com/vitistack/gslb-operator/internal/api/handlers/v1/dnsdist"
	"github.com/vitistack/gslb-operator/internal/api/handlers/v1/service"
	"github.com/vitistack/gslb-operator/internal/api/handlers/v1/spoofs"
	"github.com/vitistack/gslb-operator/internal/api/routes"
	"github.com/vitistack/gslb-operator/internal/config"
	"github.com/vitistack/gslb-operator/pkg/auth/authc"
	"github.com/vitistack/gslb-operator/pkg/bslog"
	"github.com/vitistack/gslb-operator/pkg/rest/middleware"
	"github.com/vitistack/gslb-operator/pkg/rest/router"
)

// Handlers is every endpoint v1 serves. A later version overrides only the
// field(s) it changed and leaves the rest as v1's defaults.
type Handlers struct {
	APIEnrollmentsGet router.HandlerFunc
	APIEnrollmentGet  router.HandlerFunc
	GrantsPut         router.HandlerFunc
	Token             router.HandlerFunc
	Spoofs            router.HandlerFunc
	OverrideGet       router.HandlerFunc
	OverrideCreate    router.HandlerFunc
	OverrideDelete    router.HandlerFunc
	ServicesGet       router.HandlerFunc
	ServiceGet        router.HandlerFunc
	StatusGet         router.HandlerFunc
	ServiceStatusGet  router.HandlerFunc
	MembersGet        router.HandlerFunc
	MembersIdGet      router.HandlerFunc
	DNSDistConfigGet  router.HandlerFunc
}

// DefaultHandlers builds v1's own handlers from deps.
func DefaultHandlers(deps *routes.Dependencies) Handlers {
	authHandler, err := auth.Init(deps.AuthStore)
	if err != nil {
		bslog.Fatal("failed to initialize auth handler", slog.String("reason", err.Error()))
	}

	deps.AuthEnforcer = authHandler.Enforcer()
	deps.SecuredChain = middleware.Chain(authHandler.Verify())

	adminHandler := admin.NewAdminService(authc.NewRegistry(deps.AuthStore))
	serviceHandler := service.NewGSLBServiceHandler(deps.StatusRepo, deps.ServicesStore)
	spoofsHandler := spoofs.NewSpoofsService(deps.ServicesStore, deps.Manager)
	dnsdistHandler := dnsdist.NewDNSDistService(deps.ServicesStore)

	return Handlers{
		APIEnrollmentsGet: adminHandler.GetAPIEnrollments,
		APIEnrollmentGet:  adminHandler.GetAPIEnrollment,
		GrantsPut:         adminHandler.PutGrants,
		Token:             authHandler.Token,
		Spoofs:            spoofsHandler.GetFQDNSpoof,
		OverrideGet:       spoofsHandler.GetOverride,
		OverrideCreate:    spoofsHandler.CreateOverride,
		OverrideDelete:    spoofsHandler.DeleteOverride,
		ServicesGet:       serviceHandler.GetServices,
		ServiceGet:        serviceHandler.GetService,
		StatusGet:         serviceHandler.GetStatus,
		ServiceStatusGet:  serviceHandler.GetServiceStatus,
		MembersGet:        serviceHandler.GetMembers,
		MembersIdGet:      serviceHandler.GetMembersId,
		DNSDistConfigGet:  dnsdistHandler.GetConfig,
	}
}

// Mount wires routes onto g using whatever is in h, so callers can swap
// individual handlers (e.g. for v2) before calling this.
func Mount(g *router.RouteGroup, deps *routes.Dependencies, h Handlers) {
	authRouter := g.Group("/auth")
	authRouter.POST("/token", h.Token).Public()

	gslbServicesRouter := g.Group("/services").Use(deps.SecuredChain)
	gslbServicesRouter.GET("", h.ServicesGet)
	gslbServicesRouter.GET("/overrides", nil) // TODO
	gslbServicesRouter.GET("/status", h.StatusGet)

	gslbServicesMemberOfRouter := gslbServicesRouter.Group("/{memberOf}")
	gslbServicesMemberOfRouter.GET("", h.ServiceGet)

	gslbServicesMemberOfRouter.GET("/members", h.MembersGet)
	gslbServicesMemberOfRouter.GET("/members/{id}", h.MembersIdGet)

	gslbServicesMemberOfRouter.GET("/spoofs", h.Spoofs)

	gslbServicesMemberOfRouter.GET("/override", h.OverrideGet)
	gslbServicesMemberOfRouter.POST("/override", h.OverrideCreate)
	gslbServicesMemberOfRouter.DELETE("/override", h.OverrideDelete)

	if config.GSLB().Status().Enabled() {
		gslbServicesMemberOfRouter.GET("/status", h.ServiceStatusGet)
	}

	dnsdistRouter := g.Group("/dnsdist").Use(deps.SecuredChain)
	dnsdistRouter.GET("/config", h.DNSDistConfigGet)
}
