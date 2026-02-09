package dns

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"sync"

	"codeberg.org/miekg/dns"
	"github.com/vitistack/gslb-operator/internal/manager"
	"github.com/vitistack/gslb-operator/internal/model"
	"github.com/vitistack/gslb-operator/internal/service"
	"github.com/vitistack/gslb-operator/pkg/bslog"
)

// Handles/Orchestrates DNS related things
type Handler struct {
	fetcher       *ZoneFetcher // fetch GSLB config from dns
	svcManager    *manager.ServicesManager
	updater       *Updater
	knownServices map[string]*service.Service // key: service.ID
	stopFetcher   chan struct{}
	cancel        func() // cancels zone fetches
	wg            sync.WaitGroup
}

func NewHandler(fetcher *ZoneFetcher, mgr *manager.ServicesManager, updater *Updater) *Handler {
	return &Handler{
		fetcher:       fetcher,
		svcManager:    mgr,
		updater:       updater,
		knownServices: make(map[string]*service.Service),
		stopFetcher:   make(chan struct{}),
		wg:            sync.WaitGroup{},
	}
}

func (h *Handler) Start(ctx context.Context, cancel func()) {
	h.cancel = cancel // context cancellation

	// function to update DNS
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
	bslog.Debug("Successfully stopped DNS - Handler")
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
					bslog.Info("service no longer exists in GSLB - config zone", slog.String("action", "removing"), slog.Any("service", oldSvc))
					err := h.svcManager.RemoveService(oldSvc)
					if err != nil {
						bslog.Error("failed to remove service", slog.Any("service", oldSvc), slog.String("reason", err.Error()))
					}
				}
			}

			h.knownServices = servicesInBatch

		case err, ok := <-pollErrors:
			if !ok {
				return
			}
			bslog.Error("zone transfer did not succeed", slog.String("reason", err.Error()))

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
		Script: `return status_code ~= 503`,
	}

	err := json.Unmarshal([]byte(data), &svcConfig)
	if err != nil {
		bslog.Error("failed to parse GSLB entry", slog.String("reason", err.Error()))
		return nil
	}

	svc, err := h.svcManager.RegisterService(svcConfig)
	if err != nil {
		bslog.Error("could not register service", slog.String("reason", err.Error()))
		return nil
	}

	return svc
}
