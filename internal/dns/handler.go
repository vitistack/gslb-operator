package dns

import (
	"context"
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
	knownServices map[string]*service.Service // key: service.ID
	log           *zap.SugaredLogger
	stopFetcher   chan struct{}
	cancel        func() // cancels zone fetches
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

func (h *Handler) Start(ctx context.Context, cancel func()) {
	h.cancel = cancel
	h.svcManager.DNSUpdate = func(service *service.Service, healthy bool) {
		if healthy {
			h.onServiceUp(service)
		} else {
			h.onServiceDown(service)
		}
	}
	h.svcManager.Start()

	zoneBatches, pollErrors := h.fetcher.StartAutoPoll(ctx)

	h.wg.Go(func() {
		h.handleZoneUpdates(zoneBatches, pollErrors)
	})
}

func (h *Handler) Stop() {
	h.cancel()
	close(h.stopFetcher)
	h.wg.Wait()
	h.svcManager.Stop()
	h.log.Debug("Successfully stopped DNS - Handler")
}

func (h *Handler) onServiceDown(svc *service.Service) {
	h.updater.ServiceDown(svc)
}

func (h *Handler) onServiceUp(svc *service.Service) {
	h.updater.ServiceUp(svc)
}

func (h *Handler) handleZoneUpdates(zone <-chan []dns.RR, pollErrors <-chan error) {
	for {
		select {
		case records, ok := <-zone:
			if !ok { // chan is closed
				return
			}
			servicesInBatch := make(map[string]*service.Service)
			for _, record := range records { // registers every service in the current batch
				svc := h.handleRecord(record)
				if svc != nil {
					servicesInBatch[svc.GetID()] = svc
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
	svcConfig := model.GSLBConfig{
		MemberOf:         txt.Hdr.Name,
		FailureThreshold: service.DEFAULT_FAILURE_THRESHOLD,
	}
	err := json.Unmarshal([]byte(data), &svcConfig)
	if err != nil {
		h.log.Errorf("failed to parse gslb config: %v", err.Error())
		return nil
	}

	svc, err := h.svcManager.RegisterService(svcConfig)
	if err != nil {
		h.log.Errorf("could not register service: %s", err.Error())
		return nil
	}

	return svc
}
