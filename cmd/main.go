package main

import (
	"log"
	"time"

	"github.com/vitistack/gslb-operator/internal/dns"
	"github.com/vitistack/gslb-operator/internal/manager"
	"github.com/vitistack/gslb-operator/internal/model"
	"github.com/vitistack/gslb-operator/internal/service"
	"github.com/vitistack/gslb-operator/internal/utils/timesutil"
	"go.uber.org/zap"
)

func main() {
	logger, err := zap.NewDevelopment()
	if err != nil {
		log.Fatalf("could not create logger")
	}

	//fetcher := dns.NewZoneFetcherWithAutoPoll("gslb.test.dns.nhn.no.", "nsh1.nhn.no:53", dns.DEFAULT_POLL_INTERVAL, logger)
	mgr := manager.NewManager(10, 10, logger)
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
	/*
		handler := dns.NewHandler(fetcher, mgr, &dns.Updater{}, logger)
		err = handler.Start()
		if err != nil {
			msg := fmt.Sprintf("error starting dns handler: %v", err)
			logger.Error(msg)
			}
			handler.Stop()
	*/
	time.Sleep(dns.DEFAULT_POLL_INTERVAL)
}
