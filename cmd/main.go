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

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/valkey-io/valkey-go"
	"github.com/vitistack/gslb-operator/internal/api/handlers/auth"
	"github.com/vitistack/gslb-operator/internal/api/handlers/service"
	"github.com/vitistack/gslb-operator/internal/api/handlers/spoofs"
	"github.com/vitistack/gslb-operator/internal/brokers"
	"github.com/vitistack/gslb-operator/internal/config"
	"github.com/vitistack/gslb-operator/internal/dns"
	"github.com/vitistack/gslb-operator/internal/dns/update/dnsdist"
	"github.com/vitistack/gslb-operator/internal/manager"
	"github.com/vitistack/gslb-operator/internal/model"
	"github.com/vitistack/gslb-operator/internal/repositories/servicegroup"
	"github.com/vitistack/gslb-operator/internal/repositories/status"
	"github.com/vitistack/gslb-operator/pkg/auth/authc"
	"github.com/vitistack/gslb-operator/pkg/auth/authz"
	"github.com/vitistack/gslb-operator/pkg/bslog"
	"github.com/vitistack/gslb-operator/pkg/events"
	"github.com/vitistack/gslb-operator/pkg/lua"
	serviceModels "github.com/vitistack/gslb-operator/pkg/models/service"
	valkeyStore "github.com/vitistack/gslb-operator/pkg/persistence/store/valkey"
	"github.com/vitistack/gslb-operator/pkg/rest/middleware"
	"github.com/vitistack/gslb-operator/pkg/rest/router"
)

var ( // injected at buildtime
	version   string
	buildDate string
)

func main() {
	bslog.Info("Running GSLB - Operator",
		slog.String("version", version),
		slog.String("build-date", buildDate),
	)

	// initialize lua execution environment
	if err := lua.LoadSandboxConfig(config.Server().LuaSandbox()); err != nil {
		bslog.Fatal("could not load lua configuration", slog.Any("reason", err))
	}

	valkeyClient, err := valkeyStore.NewClient(
		valkey.ClientOption{
			InitAddress: []string{config.Valkey().Address()},
			Username:    config.Valkey().User(),
			Password:    config.Valkey().Password(),
		},
	)
	if err != nil {
		bslog.Fatal("failed to establish valkey connection", slog.String("reason", err.Error()))
	}

	servicesStore, err := valkeyStore.NewStore[model.GSLBServiceGroup](valkeyClient, "gslb:service_groups", time.Second*30)
	if err != nil {
		bslog.Fatal("failed to create valkey store for gslb service groups", slog.String("reason", err.Error()))
	}

	var statusRepo *status.StatusRepo
	if config.GSLB().StatusEnabled() {
		statusStore, err := valkeyStore.NewStore[serviceModels.GSLBServiceStatus](valkeyClient, "gslb:service_status", time.Minute*10)
		if err != nil {
			bslog.Fatal("failed to create valkey store for gslb service-status", slog.String("reason", err.Error()))
		}
		statusRepo = status.NewStatusRepo(statusStore)
	}

	svcGroupRepo := servicegroup.NewServiceGroupRepo(servicesStore)

	// creating dns - handler objects
	zoneFetcher := dns.NewZoneFetcherWithAutoPoll()
	mgr := manager.NewManager(
		manager.WithMinRunningWorkers(10),
		manager.WithNonBlockingBufferSize(15),
		manager.WithServiceGroupRepository(svcGroupRepo),
		//manager.WithDryRun(true),
	)

	updater, err := dnsdist.NewDNSDISTUpdater(servicesStore)
	if err != nil {
		if config.GSLB().StatusEnabled() {
			bslog.Fatal("fatal error: unable to create updater", slog.String("error", err.Error()))
		}
		bslog.Error("unable to create updater", slog.String("error", err.Error()))
	}

	dnsHandler := dns.NewHandler(
		zoneFetcher,
		mgr,
		updater,
	)

	background := context.Background()
	ctx, cancel := context.WithCancel(background)
	// mq brokers
	brokers.Init(ctx, valkeyClient, statusRepo, svcGroupRepo, updater)

	dnsHandler.Start(ctx, cancel)
	updater.Synchronize(ctx)

	authStore := authc.NewValkeyStore(valkeyClient)
	authService, err := auth.Init(authStore)
	if err != nil {
		bslog.Fatal("failed to init auth service", slog.String("reason", err.Error()))
	}

	// middleware chains
	securedChain := middleware.Chain(
		authService.Verify(),
	)

	router := router.New(
		authService.Enforcer(),
		slog.Default(),
		middleware.WithIncomingRequestLogging(slog.Default()),
	)

	rootRouter := router.Group("/")
	rootRouter.Handle( // static wiring of prometheus metrics handler
		http.MethodGet,
		"/metrics",
		promhttp.Handler(),
	).Public()

	apiRouter := rootRouter.Group("/api/v1")

	authRouter := apiRouter.Group("/auth")
	authRouter.POST("/token", authService.Token).Public()

	spoofsApiService := spoofs.NewSpoofsService(servicesStore, mgr)
	spoofsRouter := apiRouter.Group("/spoofs").Use(securedChain)
	spoofsRouter.GET("", spoofsApiService.GetSpoofs).Action(authz.SpoofsList)
	spoofsRouter.GET("/{memberOf}", spoofsApiService.GetFQDNSpoof).Action(authz.SpoofsRead)

	overrideRouter := spoofsRouter.Group("/override")
	overrideRouter.GET("", spoofsApiService.GetOverride).Action(authz.OverrideList)
	overrideRouter.GET("/{memberOf}", spoofsApiService.GetOverride).Action(authz.OverrideRead)
	overrideRouter.POST("{memberOf}", spoofsApiService.CreateOverride).Action(authz.OverrideCreate)
	overrideRouter.DELETE("/{memberOf}", spoofsApiService.DeleteOverride).Action(authz.OverrideDelete)

	if config.GSLB().StatusEnabled() {
		gslbServicesApiService := service.NewGSLBServiceHandler(statusRepo)
		gslbServiceRouter := apiRouter.Group("/service").Use(securedChain)

		statusRouter := gslbServiceRouter.Group("/status")
		statusRouter.GET("", gslbServicesApiService.GetServiceStatus).Action(authz.StatusList)
		statusRouter.GET("/{memberOf}", gslbServicesApiService.GetServiceStatus).Action(authz.StatusRead)
	}

	api, err := router.Build()
	if err != nil {
		bslog.Fatal("unable to build")
	}

	server := http.Server{
		Addr:    config.API().Port(),
		Handler: api,
	}
	serverErr := make(chan error, 1)
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)

	bslog.Info("starting API service", slog.String("port", config.API().Port()))
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

	shutdown, cancel := context.WithTimeout(background, time.Second*20)
	defer cancel()

	dnsHandler.Stop(shutdown)
	if err := server.Shutdown(shutdown); err != nil {
		panic("error shutting down server: " + err.Error())
	}

	// stop event handling
	events.Stop(shutdown)
}
