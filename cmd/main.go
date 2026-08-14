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
	"github.com/vitistack/gslb-operator/internal/api/handlers/spoofs"
	"github.com/vitistack/gslb-operator/internal/api/routes"
	whBroker "github.com/vitistack/gslb-operator/internal/brokers/webhooks"
	"github.com/vitistack/gslb-operator/internal/config"
	"github.com/vitistack/gslb-operator/internal/dns"
	"github.com/vitistack/gslb-operator/internal/dns/update/dnsdist"
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

	servicesStore, err := valkeyStore.NewStore[model.GSLBServiceGroup](
		valkeyClient,
		"gslb:service_groups",
		time.Second*30,
		//valkeyStore.WithMigrations[model.GSLBServiceGroup](servicegroup.MigrateActiveToMap(config.SplitDNS().DefaultView())),
	)
	if err != nil {
		bslog.Fatal("failed to create valkey store for gslb service groups", slog.String("reason", err.Error()))
	}

	webhooksStore, err := valkeyStore.NewStore[model.WebHook](valkeyClient, "gslb:webhooks", time.Minute*30)
	if err != nil {
		bslog.Fatal("failed to create valkey store for gslb webhooks", slog.String("reason", err.Error()))
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
	whBroker.Init(ctx, webhooksStore)

	dnsHandler.Start(ctx, cancel)
	updater.Synchronize(ctx)

	//configs := getRandomGSLBConfig()
	//for _, config := range configs {
	//	_, err := mgr.RegisterService(config)
	//	if err != nil {
	//		bslog.Fatal("could not create service", slog.String("reason", err.Error()))
	//	}
	//}

	api := http.NewServeMux()

	// routes handlers
	spoofsApiService := spoofs.NewSpoofsService(servicesStore, mgr)

	// initializing the service jwt self signer
	jwt.InitServiceTokenManager(config.JWT().Secret(), config.JWT().User())

	// spoofs
	api.HandleFunc(routes.GET_SPOOFS, middleware.Chain(
		middleware.WithIncomingRequestLogging(slog.Default()),
		auth.WithTokenValidation(slog.Default()),
	)(spoofsApiService.GetSpoofs))

	api.HandleFunc(routes.GET_SPOOFID, middleware.Chain(
		middleware.WithIncomingRequestLogging(slog.Default()),
		auth.WithTokenValidation(slog.Default()),
	)(spoofsApiService.GetFQDNSpoof))

	//api.HandleFunc(routes.GET_SPOOFS_HASH, middleware.Chain(
	//	middleware.WithIncomingRequestLogging(slog.Default()),
	//	auth.WithTokenValidation(slog.Default()),
	//)(spoofsApiService.GetSpoofsHash))

	// spoofs/override
	// TODO: add auth!
	api.HandleFunc(routes.GET_OVERRIDE, middleware.Chain(
		middleware.WithIncomingRequestLogging(slog.Default()),
	)(spoofsApiService.GetOverride))

	api.HandleFunc(routes.POST_OVERRIDE, middleware.Chain(
		middleware.WithIncomingRequestLogging(slog.Default()),
	)(spoofsApiService.CreateOverride))

	api.HandleFunc(routes.DELETE_OVERRIDE, middleware.Chain(
		middleware.WithIncomingRequestLogging(slog.Default()),
	)(spoofsApiService.DeleteOverride))

	// metrics
	api.Handle(routes.METRICS, promhttp.Handler())

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

//func getRandomGSLBConfig() []model.GSLBConfig {
//	configs := make([]model.GSLBConfig, 0, 500)
//
//	config := model.GSLBConfig{
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
//		config.ServiceID = fmt.Sprintf("%d", idx)
//		config.MemberOf = fmt.Sprintf("%s.%s", config.ServiceID, config.Fqdn)
//
//		configs = append(configs, config)
//	}
//
//	return configs
//}
