package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/vitistack/gslb-operator/internal/api/handler"
	"github.com/vitistack/gslb-operator/internal/config"
	"github.com/vitistack/gslb-operator/internal/dns"
	"github.com/vitistack/gslb-operator/internal/manager"
	"github.com/vitistack/gslb-operator/internal/repositories/spoof"
)

func main() {
	cfg := config.GetInstance()

	api := http.NewServeMux()

	hc, log, err := handler.NewHandler()
	if err != nil {
		fmt.Printf("unable to start service: %s", err.Error())
		os.Exit(1)
	}

	api.HandleFunc(handler.GET_OVERRIDES, hc.GetOverrides)
	api.HandleFunc(handler.POST_OVERRIDE, hc.CreateOverride)
	api.HandleFunc(handler.DELETE_OVERRIDE, hc.DeleteOverride)

	api.HandleFunc(handler.GET_SPOOFS, hc.GetSpoofs)
	api.HandleFunc(handler.GET_SPOOFID, hc.GetFQDNSpoof)
	api.HandleFunc(handler.GET_SPOOFS_HASH, hc.GetSpoofsHash)
	api.HandleFunc(handler.POST_SPOOF, hc.CreateSpoof)

	server := http.Server{
		Addr:    cfg.API().Port(),
		Handler: api,
	}
	serverErr := make(chan error, 1)
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)

	log.Sugar().Infof("starting service on port%s", cfg.API().Port())
	go func() {
		err := server.ListenAndServe()
		if err != nil {
			serverErr <- fmt.Errorf("server failed: %s", err.Error())
		}
	}()

	// creating dns - handler objects
	zoneFetcher := dns.NewZoneFetcherWithAutoPoll(
		log,
		dns.WithServer(cfg.GSLB().NameServer()),
		dns.WithZone(cfg.GSLB().Zone()),
	)
	mgr := manager.NewManager(
		log,
		manager.WithMinRunningWorkers(100),
		manager.WithNonBlockingBufferSize(105),
		//manager.WithDryRun(true),
	)
	/*

		mgr.RegisterService(model.GSLBConfig{
			Fqdn:       "test.nhn.no",
			Ip:         "127.0.0.1",
			Port:       "80",
			Datacenter: "Abels1",
			Interval:   timesutil.FromDuration(time.Second * 5),
			Priority:   1,
			Type:       "TCP-FULL",
		}, false)

			mgr.RegisterService(model.GSLBConfig{
				Fqdn:       "test.nhn.no",
				Ip:         "127.0.0.1",
				Port:       "90",
				Datacenter: "Abels2",
				Interval:   timesutil.FromDuration(time.Second * 5),
				Priority:   2,
				Type:       "TCP-FULL",
			}, false)
	*/

	spoofRepo := hc.SpoofRepo.(*spoof.Repository)
	updater, err := dns.NewUpdater(
		log,
		dns.UpdaterWithSpoofRepo(spoofRepo),
	)
	if err != nil {
		log.Sugar().Fatalf("unable to create updater: %s", err.Error())
	}
	dnsHandler := dns.NewHandler(
		log,
		zoneFetcher,
		mgr,
		updater,
	)

	background := context.Background()
	dnsHandler.Start(context.WithCancel(background))

	select {
	case err := <-serverErr:
		dnsHandler.Stop()
		log.Sugar().Fatalf("server crashed unexpectedly, no longer serving http: %s", err.Error())
	case <-quit:
		log.Info("gracefully shutting down...")
		dnsHandler.Stop()
	}

	serverCtx, cancel := context.WithTimeout(background, time.Second*5)
	defer cancel()

	if err := server.Shutdown(serverCtx); err != nil {
		panic("error shutting down server: " + err.Error())
	}
}
