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

	"github.com/vitistack/gslb-operator/internal/api/handlers/failover"
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
	apiContractSpoof "github.com/vitistack/gslb-operator/pkg/models/spoofs"
	"github.com/vitistack/gslb-operator/pkg/persistence/store/file"
	"github.com/vitistack/gslb-operator/pkg/rest/middleware"
)

func main() {
	cfg := config.GetInstance()

	if err := lua.LoadSandboxConfig(cfg.Server().LuaSandbox()); err != nil {
		bslog.Fatal("could not load lua configuration", slog.Any("reason", err))
	}

	// creating dns - handler objects
	zoneFetcher := dns.NewZoneFetcherWithAutoPoll()
	mgr := manager.NewManager(
		manager.WithMinRunningWorkers(100),
		manager.WithNonBlockingBufferSize(110),
	)

	spoofsFileStore, err := file.NewStore[apiContractSpoof.Spoof]("store.json")
	if err != nil {
		bslog.Fatal("could not create persistent storage", slog.String("reason", err.Error()))
	}

	spoofRepo := spoofsrepo.NewRepository(spoofsFileStore)
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

	api := http.NewServeMux()

	// different routes handlers
	spoofsApiService := spoofs.NewSpoofsService(spoofRepo)
	failoverApiService := failover.NewFailoverService()

	// initializing the service jwt self signer
	jwt.InitServiceTokenManager(cfg.JWT().Secret(), cfg.JWT().User())

	api.HandleFunc(routes.POST_FAILOVER, failoverApiService.FailoverService)

	api.HandleFunc(routes.GET_OVERRIDE, spoofsApiService.GetOverride)
	api.HandleFunc(routes.POST_OVERRIDE, spoofsApiService.CreateOverride)
	api.HandleFunc(routes.DELETE_OVERRIDE, spoofsApiService.DeleteOverride)

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

	/*
		TODO: Does this need to be here?
		api.HandleFunc(routes.POST_SPOOF, middleware.Chain(
			middleware.WithContextRequestID(),
			middleware.WithIncomingRequestLogging(slog.Default()),
			auth.WithTokenValidation(slog.Default()),
			)(spoofsApiService.CreateSpoof))
	*/

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
