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
	"github.com/vitistack/gslb-operator/internal/api/handlers/spoofs"
	"github.com/vitistack/gslb-operator/internal/api/routes"
	"github.com/vitistack/gslb-operator/internal/config"
	"github.com/vitistack/gslb-operator/internal/dns"
	"github.com/vitistack/gslb-operator/internal/manager"
	"github.com/vitistack/gslb-operator/internal/model"
	"github.com/vitistack/gslb-operator/internal/repositories/service"
	"github.com/vitistack/gslb-operator/pkg/auth"
	"github.com/vitistack/gslb-operator/pkg/auth/jwt"
	"github.com/vitistack/gslb-operator/pkg/bslog"
	"github.com/vitistack/gslb-operator/pkg/lua"
	"github.com/vitistack/gslb-operator/pkg/persistence/store/file"
	"github.com/vitistack/gslb-operator/pkg/rest/middleware"
)

func main() {
	cfg := config.GetInstance()

	// initialize lua execution environment
	if err := lua.LoadSandboxConfig(cfg.Server().LuaSandbox()); err != nil {
		bslog.Fatal("could not load lua configuration", slog.Any("reason", err))
	}

	// creating dns - handler objects
	zoneFetcher := dns.NewZoneFetcherWithAutoPoll()
	mgr := manager.NewManager(
		manager.WithMinRunningWorkers(100),
		manager.WithNonBlockingBufferSize(110),
	)

	serviceFileStore, err := file.NewStore[model.Service]("store.json")
	if err != nil {
		bslog.Fatal("could not create persistent storage", slog.String("reason", err.Error()))
	}

	svcRepo := service.NewServiceRepo(serviceFileStore)
	updater, err := dns.NewUpdater(
		dns.UpdaterWithSpoofRepo(svcRepo),
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

	// routes handlers
	spoofsApiService := spoofs.NewSpoofsService(serviceFileStore, mgr)

	failoverApiService := failover.NewFailoverService(mgr)

	// initializing the service jwt self signer
	jwt.InitServiceTokenManager(cfg.JWT().Secret(), cfg.JWT().User())
	fmt.Println(jwt.GetInstance().GetServiceToken())

	api.HandleFunc(routes.POST_FAILOVER, middleware.Chain(
		middleware.WithIncomingRequestLogging(slog.Default()),
	)(failoverApiService.FailoverService))

	api.HandleFunc(routes.GET_SPOOFS, middleware.Chain(
		middleware.WithIncomingRequestLogging(slog.Default()),
		auth.WithTokenValidation(slog.Default()),
	)(spoofsApiService.GetSpoofs))

	api.HandleFunc(routes.GET_SPOOFID, middleware.Chain(
		middleware.WithIncomingRequestLogging(slog.Default()),
		auth.WithTokenValidation(slog.Default()),
	)(spoofsApiService.GetFQDNSpoof))

	api.HandleFunc(routes.GET_SPOOFS_HASH, middleware.Chain(
		middleware.WithIncomingRequestLogging(slog.Default()),
		auth.WithTokenValidation(slog.Default()),
	)(spoofsApiService.GetSpoofsHash))

	// spoofs/override
	// TODO: add auth!
	api.HandleFunc(routes.GET_OVERRIDE, middleware.Chain(
		middleware.WithIncomingRequestLogging(slog.Default()),
	)(spoofsApiService.GetOverride))

	api.HandleFunc(routes.PUT_OVERRIDE, middleware.Chain(
		middleware.WithIncomingRequestLogging(slog.Default()),
	)(spoofsApiService.UpdateOverride))

	api.HandleFunc(routes.POST_OVERRIDE, middleware.Chain(
		middleware.WithIncomingRequestLogging(slog.Default()),
	)(spoofsApiService.CreateOverride))

	api.HandleFunc(routes.DELETE_OVERRIDE, middleware.Chain(
		middleware.WithIncomingRequestLogging(slog.Default()),
	)(spoofsApiService.DeleteOverride))

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
		bslog.Fatal("server crashed unexpectedly, no longer serving http", slog.String("reason", err.Error()))
	case <-quit:
		bslog.Info("gracefully shutting down...")
	}

	shutdown, cancel := context.WithTimeout(background, time.Second*5)
	defer cancel()

	dnsHandler.Stop(shutdown)
	if err := server.Shutdown(shutdown); err != nil {
		panic("error shutting down server: " + err.Error())
	}
}
