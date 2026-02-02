package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	overrides "github.com/vitistack/gslb-operator/internal/api/handlers/overrides"
	spoofs "github.com/vitistack/gslb-operator/internal/api/handlers/spoofs"
	"github.com/vitistack/gslb-operator/internal/api/routes"
	"github.com/vitistack/gslb-operator/internal/config"
	"github.com/vitistack/gslb-operator/internal/dns"
	"github.com/vitistack/gslb-operator/internal/manager"
	spoofsrepo "github.com/vitistack/gslb-operator/internal/repositories/spoof"
	"github.com/vitistack/gslb-operator/pkg/auth"
	"github.com/vitistack/gslb-operator/pkg/auth/jwt"
	"github.com/vitistack/gslb-operator/pkg/bslog"
	"github.com/vitistack/gslb-operator/pkg/lua"
	"github.com/vitistack/gslb-operator/pkg/rest/middleware"
)

func main() {
	cfg := config.GetInstance()

	if err := lua.LoadSandboxConfig(cfg.Server().LuaSandbox()); err != nil {
		bslog.Fatal("could not load lua configuration", slog.Any("reason", err))
	}

	api := http.NewServeMux()

	spoofsApiService, err := spoofs.NewSpoofsService()
	if err != nil {
		bslog.Fatal("unable to start API service", slog.String("reason", err.Error()))
	}

	overridesApiService := overrides.NewOverrideService()

	jwt.InitServiceTokenManager(cfg.JWT().Secret(), cfg.JWT().User())

	api.HandleFunc(routes.GET_OVERRIDES, overridesApiService.GetOverrides)
	api.HandleFunc(routes.POST_OVERRIDE, overridesApiService.CreateOverride)
	api.HandleFunc(routes.DELETE_OVERRIDE, overridesApiService.DeleteOverride)

	api.HandleFunc(routes.GET_SPOOFS, middleware.Chain(
		middleware.WithContextRequestID(),
		middleware.WithIncomingRequestLogging(slog.Default()),
		auth.WithTokenValidation(slog.Default()),
	)(spoofsApiService.GetSpoofs))

	api.HandleFunc(routes.GET_SPOOFID, middleware.Chain(
		middleware.WithContextRequestID(),
		middleware.WithIncomingRequestLogging(slog.Default()),
		auth.WithTokenValidation(slog.Default()),
	)(spoofsApiService.GetFQDNSpoof))

	api.HandleFunc(routes.GET_SPOOFS_HASH, middleware.Chain(
		middleware.WithContextRequestID(),
		middleware.WithIncomingRequestLogging(slog.Default()),
		auth.WithTokenValidation(slog.Default()),
	)(spoofsApiService.GetSpoofsHash))

	api.HandleFunc(routes.POST_SPOOF, middleware.Chain(
		middleware.WithContextRequestID(),
		middleware.WithIncomingRequestLogging(slog.Default()),
		auth.WithTokenValidation(slog.Default()),
	)(spoofsApiService.CreateSpoof))

	server := http.Server{
		Addr:    cfg.API().Port(),
		Handler: api,
	}
	serverErr := make(chan error, 1)
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)

	bslog.Info("starting API service", slog.String("port", cfg.API().Port()))
	go func() {
		err := server.ListenAndServe()
		if err != nil {
			serverErr <- fmt.Errorf("server failed: %s", err.Error())
		}
	}()

	// creating dns - handler objects
	zoneFetcher := dns.NewZoneFetcherWithAutoPoll()
	mgr := manager.NewManager(
		manager.WithMinRunningWorkers(100),
		manager.WithNonBlockingBufferSize(110),
	)

	spoofRepo := spoofsApiService.SpoofRepo.(*spoofsrepo.Repository)
	updater, err := dns.NewUpdater(
		dns.UpdaterWithSpoofRepo(spoofRepo),
	)
	if err != nil {
		bslog.Fatal("unable to create updater", slog.String("error", err.Error()))
	}
	dnsHandler := dns.NewHandler(
		zoneFetcher,
		mgr,
		updater,
	)

	background := context.Background()
	dnsHandler.Start(context.WithCancel(background))

	select {
	case err := <-serverErr:
		dnsHandler.Stop()
		bslog.Fatal("server crashed unexpectedly, no longer serving http", slog.String("reason", err.Error()))
	case <-quit:
		bslog.Info("gracefully shutting down...")
		dnsHandler.Stop()
	}

	serverCtx, cancel := context.WithTimeout(background, time.Second*5)
	defer cancel()

	if err := server.Shutdown(serverCtx); err != nil {
		panic("error shutting down server: " + err.Error())
	}
}
