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
	"github.com/vitistack/gslb-operator/internal/api/handlers/failover"
	"github.com/vitistack/gslb-operator/internal/api/handlers/spoofs"
	"github.com/vitistack/gslb-operator/internal/api/handlers/webhooks"
	"github.com/vitistack/gslb-operator/internal/api/routes"
	whBroker "github.com/vitistack/gslb-operator/internal/brokers/webhooks"
	"github.com/vitistack/gslb-operator/internal/config"
	"github.com/vitistack/gslb-operator/internal/dns"
	"github.com/vitistack/gslb-operator/internal/dns/update"
	"github.com/vitistack/gslb-operator/internal/manager"
	"github.com/vitistack/gslb-operator/internal/model"
	"github.com/vitistack/gslb-operator/internal/repositories/servicegroup"
	"github.com/vitistack/gslb-operator/pkg/auth"
	"github.com/vitistack/gslb-operator/pkg/auth/jwt"
	"github.com/vitistack/gslb-operator/pkg/bslog"
	"github.com/vitistack/gslb-operator/pkg/events"
	"github.com/vitistack/gslb-operator/pkg/lua"
	valkeyStore "github.com/vitistack/gslb-operator/pkg/persistence/store/valkey"
	"github.com/vitistack/gslb-operator/pkg/rest/middleware"
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
	cfg := config.GetInstance()
	bslog.Info("loaded environment", slog.Any("env", cfg))

	// initialize lua execution environment
	if err := lua.LoadSandboxConfig(cfg.Server().LuaSandbox()); err != nil {
		bslog.Fatal("could not load lua configuration", slog.Any("reason", err))
	}

	valkeyClient, err := valkeyStore.NewClient(
		valkey.ClientOption{
			InitAddress: []string{"localhost:6379"},
		},
	)
	if err != nil {
		bslog.Fatal("failed to establish valkey connection", slog.String("reason", err.Error()))
	}
	servicesStore := valkeyStore.NewStore[model.GSLBServiceGroup](valkeyClient, "gslb:service_groups", time.Second*30)
	webhooksStore := valkeyStore.NewStore[model.WebHook](valkeyClient, "gslb:webhooks", time.Minute*30)

	//serviceFileStore, err := file.NewStore[model.GSLBServiceGroup]("./data/store.json")
	//if err != nil {
	//	bslog.Fatal("could not create persistent storage", slog.String("reason", err.Error()))
	//}
	svcGroupRepo := servicegroup.NewServiceGroupRepo(servicesStore)

	//webhooksFileStore, err := file.NewStore[model.WebHook]("./data/webhooks.json")
	//if err != nil {
	//	bslog.Fatal("could not create persistent storage", slog.String("reason", err.Error()))
	//}

	// creating dns - handler objects
	zoneFetcher := dns.NewZoneFetcherWithAutoPoll()
	mgr := manager.NewManager(
		manager.WithMinRunningWorkers(10),
		manager.WithNonBlockingBufferSize(15),
		manager.WithServiceGroupRepository(svcGroupRepo),
		//manager.WithDryRun(true),
	)

	updater, err := update.NewDNSDISTUpdater(servicesStore)
	if err != nil {
		bslog.Fatal("unable to create updater", slog.String("error", err.Error()))
	}

	dnsHandler := dns.NewHandler(
		zoneFetcher,
		mgr,
		updater,
	)

	background := context.Background()
	ctx, cancel := context.WithCancel(background)

	// mq brokers
	webhooksBroker := whBroker.New(ctx, webhooksStore)
	webhooksBroker.Subscribe(ctx)

	dnsHandler.Start(ctx, cancel)
	updater.Synchronize(ctx)

	//configs := getRandomGSLBConfig()
	//for _, cfg := range configs {
	//	_, err := mgr.RegisterService(cfg)
	//	if err != nil {
	//		bslog.Fatal("could not create service", slog.String("reason", err.Error()))
	//	}
	//}

	api := http.NewServeMux()

	// routes handlers
	spoofsApiService := spoofs.NewSpoofsService(servicesStore, mgr)
	failoverApiService := failover.NewFailoverService(mgr)
	webhooksApiService := webhooks.NewWebhookService(webhooksStore)

	// initializing the service jwt self signer
	jwt.InitServiceTokenManager(cfg.JWT().Secret(), cfg.JWT().User())

	// failover
	api.HandleFunc(routes.POST_FAILOVER, middleware.Chain(
		middleware.WithIncomingRequestLogging(slog.Default()),
	)(failoverApiService.FailoverService))

	// spoofs
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

	/*
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
	*/

	// webhooks
	api.HandleFunc(routes.GET_WEBHOOKS, middleware.Chain(
		middleware.WithIncomingRequestLogging(slog.Default()),
	)(webhooksApiService.GetWebhooks))

	api.HandleFunc(routes.POST_WEBHOOKS, middleware.Chain(
		middleware.WithIncomingRequestLogging(slog.Default()),
	)(webhooksApiService.CreateWebHook))

	/*
		api.HandleFunc(routes.PUT_WEBHOOKS, middleware.Chain(
			middleware.WithIncomingRequestLogging(slog.Default()),
		)(webhooksApiService.UpdateWebhook))
	*/

	api.HandleFunc(routes.DELETE_WEBHOOKS, middleware.Chain(
		middleware.WithIncomingRequestLogging(slog.Default()),
	)(webhooksApiService.DeleteWebhook))

	// metrics
	api.Handle(routes.METRICS, promhttp.Handler())

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

	shutdown, cancel := context.WithTimeout(background, time.Second*20)
	defer cancel()

	dnsHandler.Stop(shutdown)
	if err := server.Shutdown(shutdown); err != nil {
		panic("error shutting down server: " + err.Error())
	}

	// stop event handling
	events.Stop(shutdown)
}

//func getRandomGSLBConfig() []model.GSLBConfig {
//	configs := make([]model.GSLBConfig, 0, 500)
//
//	cfg := model.GSLBConfig{
//		Fqdn:             "test.example.com",
//		Ip:               "10.10.0.1",
//		Port:             "80",
//		Datacenter:       "DC1",
//		Interval:         timesutil.FromDuration(time.Second * 5),
//		Priority:         1,
//		FailureThreshold: 3,
//		CheckType:        checks.TCP_FULL,
//	}
//
//	for idx := range cap(configs) {
//
//		cfg.ServiceID = fmt.Sprintf("%d", idx)
//		cfg.MemberOf = fmt.Sprintf("%s.%s", cfg.ServiceID, cfg.Fqdn)
//
//		configs = append(configs, cfg)
//	}
//
//	return configs
//}
