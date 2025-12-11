package dns

import (
	"encoding/json"
	"strings"
	"sync"

	"codeberg.org/miekg/dns"
	"github.com/vitistack/gslb-operator/internal/manager"
	"github.com/vitistack/gslb-operator/internal/model"
	"github.com/vitistack/gslb-operator/internal/service"
	"go.uber.org/zap"
)

// Handles/Orchestrates DNS related things
type Handler struct {
	fetcher       *ZoneFetcher // fetch GSLB config from dns
	svcManager    *manager.ServicesManager
	updater       *Updater
	knownServices map[string]*service.Service // key: service.Fqdn:service.Datacenter
	log           *zap.SugaredLogger
	stopFetcher   chan struct{}
	wg            sync.WaitGroup
}

func NewHandler(logger *zap.Logger, fetcher *ZoneFetcher, mgr *manager.ServicesManager, updater *Updater) *Handler {
	return &Handler{
		fetcher:       fetcher,
		svcManager:    mgr,
		updater:       updater,
		knownServices: make(map[string]*service.Service),
		log:           logger.Sugar(),
		stopFetcher:   make(chan struct{}),
		wg:            sync.WaitGroup{},
	}
}

func (h *Handler) Start() error {
	h.svcManager.DNSUpdate = func(service *service.Service, healthy bool) {
		if healthy {
			h.onServiceUp(service)
		} else {
			h.onServiceDown(service)
		}
	}
	h.svcManager.Start()

	zoneBatches, pollErrors, err := h.fetcher.StartAutoPoll()
	if err != nil {
		return err
	}

	h.wg.Go(func() {
		h.handleZoneUpdates(zoneBatches, pollErrors)
	})

	return nil
}

func (h *Handler) Stop() {
	close(h.stopFetcher)
	h.wg.Wait()
	h.fetcher.StopPoll()
	h.svcManager.Stop()
	h.log.Debug("Successfully stopped DNS - Handler")
}

func (h *Handler) onServiceDown(svc *service.Service) {
	h.updater.ServiceDown(svc)
}

func (h *Handler) onServiceUp(svc *service.Service) {
	h.updater.ServiceUp(svc)
}

func (h *Handler) handleZoneUpdates(zoneBatch <-chan []dns.RR, pollErrors <-chan error) {
	for {
		select {
		case recordBatch, ok := <-zoneBatch:
			if !ok { // chan is closed
				return
			}
			servicesInBatch := make(map[string]*service.Service)
			for _, record := range recordBatch { // registers every service in the current batch
				svc := h.handleRecord(record)
				if svc != nil {
					key := svc.Fqdn + ":" + svc.Datacenter
					servicesInBatch[key] = svc
				}
			}

			for key, oldSvc := range h.knownServices { // remove any services that dont exist in the current batch
				if _, exists := servicesInBatch[key]; !exists {
					h.log.Infof("Service no longer exists in GSLB - config zone, removing: %s", key)
					err := h.svcManager.RemoveService(oldSvc, false)
					if err != nil {
						h.log.Errorf("failed to remove service: %s: %s", key, err.Error())
					}
				}
			}

			h.knownServices = servicesInBatch

		case err, ok := <-pollErrors:
			if !ok {
				return
			}
			h.log.Errorf("error while transferring zone: %s", err.Error())

		case <-h.stopFetcher:
			return
		}
	}

}

func (h *Handler) handleRecord(record dns.RR) *service.Service {
	txt, ok := record.(*dns.TXT)
	if !ok {
		return nil
	}

	rawData := txt.Txt[0]
	data := strings.ReplaceAll(rawData, "\\", "")
	svcConfig := model.GSLBConfig{Fqdn: txt.Hdr.Name}
	err := json.Unmarshal([]byte(data), &svcConfig)
	if err != nil {
		h.log.Errorf("failed to parse gslb config: %v", err.Error())
		return nil
	}

	svc, err := h.svcManager.RegisterService(svcConfig, false)
	if err != nil {
		h.log.Errorf("could not register service: %s", err.Error())
		return nil
	}

	return svc
}
