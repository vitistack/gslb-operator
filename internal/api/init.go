package api

import (
	"fmt"
	"log/slog"
	"net/http"

	"github.com/vitistack/gslb-operator/internal/api/routes"
	"github.com/vitistack/gslb-operator/internal/api/routes/debug"
	"github.com/vitistack/gslb-operator/internal/api/routes/metrics"
	testRoutes "github.com/vitistack/gslb-operator/internal/api/routes/test"
	v1Routes "github.com/vitistack/gslb-operator/internal/api/routes/v1"
	"github.com/vitistack/gslb-operator/internal/config"
	"github.com/vitistack/gslb-operator/pkg/rest/middleware"
	"github.com/vitistack/gslb-operator/pkg/rest/router"
)

func Init(deps routes.Dependencies) (*http.Server, error) {
	router := router.New(
		deps.AuthService.Enforcer(),
		slog.Default(),
		middleware.WithIncomingRequestLogging(slog.Default()),
	)

	rootRouter := router.Group("/")

	debug.Register(rootRouter)
	metrics.Register(rootRouter)

	apiRouter := rootRouter.Group("/api")

	v1Routes.Register(apiRouter, deps)
	v1Routes.RegisterLatest(apiRouter, deps)
	testRoutes.Register(apiRouter, deps)

	api, err := router.Build()
	if err != nil {
		return nil, fmt.Errorf("API scheme build failed: %w", err)
	}

	return &http.Server{
		Handler: api,
		Addr:    config.API().Port(),
	}, nil
}
