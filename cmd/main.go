package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/vitistack/gslb-operator/internal/api/handler"
	"github.com/vitistack/gslb-operator/internal/config"
	"github.com/vitistack/gslb-operator/internal/dns"
	"github.com/vitistack/gslb-operator/internal/manager"
)

func main() {
	/*

		//fetcher := dns.NewZoneFetcherWithAutoPoll("gslb.test.dns.nhn.no.", "nsh1.nhn.no:53", dns.DEFAULT_POLL_INTERVAL, logger)
		mgr := manager.NewManager(logger)
		mgr.DNSUpdate = func(s *service.Service, b bool) {
			if b {
				logger.Sugar().Infof("service %v:%v considered UP", s.Fqdn, s.Datacenter)
			} else {
				logger.Sugar().Infof("service %v:%v considered DOWN", s.Fqdn, s.Datacenter)
			}
		}
		configActive := model.GSLBConfig{
			Fqdn:       "localhost",
			Ip:         "127.0.0.1",
			Port:       "80",
			Datacenter: "Abels1",
			Interval:   timesutil.FromDuration(time.Second * 5),
			Priority:   1,
		}

		configPassive := model.GSLBConfig{
			Fqdn:       "localhost",
			Ip:         "127.0.0.1",
			Port:       "90",
			Datacenter: "Abels2",
			Interval:   timesutil.FromDuration(time.Second * 5),
			Priority:   2,
		}

		svcA, err := service.NewServiceFromGSLBConfig(configActive, logger.Sugar())
		if err != nil {
			logger.Fatal(err.Error())
		}
		svcB, err := service.NewServiceFromGSLBConfig(configPassive, logger.Sugar())
		if err != nil {
			logger.Fatal(err.Error())
		}
		mgr.RegisterService(svcA, false)
		mgr.RegisterService(svcB, false)
		mgr.Start()

			handler := dns.NewHandler(fetcher, mgr, &dns.Updater{}, logger)
			err = handler.Start()
			if err != nil {
				msg := fmt.Sprintf("error starting dns handler: %v", err)
				logger.Error(msg)
				}
				handler.Stop()
				time.Sleep(dns.DEFAULT_POLL_INTERVAL)
	*/
	cfg, err := config.NewConfig()
	if err != nil {
		log.Fatalf("error loading config: %s", err.Error())
	}

	api := http.NewServeMux()

	hc, log, err := handler.NewHandler(cfg)
	if err != nil {
		fmt.Printf("unable to start service: %s", err.Error())
		os.Exit(1)
	}

	api.HandleFunc(handler.GET_OVERRIDES, hc.GetOverrides)
	api.HandleFunc(handler.POST_OVERRIDE, hc.CreateOverride)
	api.HandleFunc(handler.DELETE_OVERRIDE, hc.DeleteOverride)

	api.HandleFunc(handler.GET_SPOOFS, hc.GetSpoofs)
	api.HandleFunc(handler.GET_SPOOFID, hc.GetFQDNSpoof)
	api.HandleFunc(handler.POST_SPOOF, hc.CreateSpoof)

	server := http.Server{
		Addr:    cfg.API.Port,
		Handler: api,
	}
	serverErr := make(chan error, 1)
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)

	log.Sugar().Infof("starting service on port%s", cfg.API.Port)
	go func() {
		err := server.ListenAndServe()
		if err != nil {
			serverErr <- fmt.Errorf("server failed: %s", err.Error())
		}
	}()

	zoneFetcher := dns.NewZoneFetcherWithAutoPoll(
		log,
		dns.WithServer(cfg.GSLB.NameServer),
		dns.WithZone(cfg.GSLB.Zone),
	)
	mgr := manager.NewManager(
		log,
		manager.WithMinRunningWorkers(100),
		manager.WithNonBlockingBufferSize(105),
		manager.WithDryRun(true),
	)

	dnsHandler := dns.NewHandler(
		log,
		zoneFetcher,
		mgr,
		&dns.Updater{},
	)

	dnsHandler.Start()

	select {
	case err := <-serverErr:
		dnsHandler.Stop()
		log.Sugar().Fatalf("server crashed unexpectedly, no longer serving http: %s", err.Error())
	case <-quit:
		dnsHandler.Stop()
		log.Info("gracefully shutting down...")
	}

	ctx := context.Background()
	serverCtx, cancel := context.WithTimeout(ctx, time.Second*5)
	defer cancel()

	if err := server.Shutdown(serverCtx); err != nil {
		panic("error shutting down server: " + err.Error())
	}
}
