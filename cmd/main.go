package main

import (
	"fmt"
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

	fetcher := dns.NewZoneFetcherWithAutoPoll("gslb.test.dns.nhn.no.", "nsh1.nhn.no:53", dns.DEFAULT_POLL_INTERVAL, logger)
	mgr := manager.NewManager(10, 10, logger)
	config := model.GSLBConfig{
		Address:    "localhost:80",
		Datacenter: "Abels",
		Interval:   timesutil.FromDuration(time.Second * 5),
		Priority:   1,
	}

	svc := service.NewServiceFromGSLBConfig(config, logger.Sugar())
	svc.SetHealthCheckCallback(func(health bool) {
		if health {
			logger.Sugar().Infof("Service: %v:%v is considered up", svc.Addr, svc.Datacenter)
		} else {
			logger.Sugar().Infof("Service: %v:%v is considered down", svc.Addr, svc.Datacenter)
		}
	})
	mgr.RegisterService(svc, false)
	mgr.Start()

	handler := dns.NewHandler(fetcher, mgr, &dns.Updater{}, logger)
	err = handler.Start()
	if err != nil {
		msg := fmt.Sprintf("error starting dns handler: %v", err)
		logger.Error(msg)
	}

	time.Sleep(time.Second * 60)
	handler.Stop()
}
