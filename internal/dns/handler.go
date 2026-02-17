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
	knownServices map[string]struct{} // service.ID: makes it easier to look up using map, but dont need a real value!
	stop          chan struct{}
	cancel        func() // cancels context
	wg            sync.WaitGroup
}

func NewHandler(fetcher *ZoneFetcher, mgr *manager.ServicesManager, updater *Updater) *Handler {
	return &Handler{
		fetcher:       fetcher,
		svcManager:    mgr,
		updater:       updater,
		knownServices: make(map[string]struct{}),
		stop:          make(chan struct{}),
		wg:            sync.WaitGroup{},
	}
}

func (h *Handler) Start(ctx context.Context, cancel func()) {
	h.cancel = func() {
		cancel() // context cancellation
		close(h.stop)
	}

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

func (h *Handler) Stop(ctx context.Context) {
	done := make(chan struct{})
	go func() {
		h.cancel() // cancel zone-updates
		h.wg.Wait()
		h.svcManager.Stop()
		close(done)
	}()

	select {
	case <-done:
		bslog.Debug("successfully stopped DNS - Handler")
	case <-ctx.Done():
		bslog.Error("timed out during stop sequence", slog.String("reason", ctx.Err().Error()))
	}
}

func (h *Handler) onServiceDown(svc *service.Service) {
	err := h.updater.ServiceDown(svc)
	if err != nil {
		bslog.Warn("error while updating service on service down", slog.String("error", err.Error()))
	}
}

func (h *Handler) onServiceUp(svc *service.Service) {
	err := h.updater.ServiceUp(svc)
	if err != nil {
		bslog.Warn("error while updating service state on service up", slog.String("error", err.Error()))
	}
}

func (h *Handler) handleZoneUpdates(zone <-chan []dns.RR, pollErrors <-chan error) {
	for {
		select {
		case records, ok := <-zone:
			if !ok { // chan is closed
				return
			}
			servicesInBatch := make(map[string]struct{}, 0)
			for _, record := range records { // registers every service in the current batch
				svc := h.handleRecord(record)
				if svc != nil {
					servicesInBatch[svc.GetID()] = struct{}{}
				}
			}

			for key := range h.knownServices { // remove any services that dont exist in the current batch
				if _, exists := servicesInBatch[key]; !exists {
					bslog.Info("service no longer exists in GSLB - config zone", slog.String("action", "removing"), slog.String("serviceID", key))
					err := h.svcManager.RemoveService(key)
					if err != nil {
						bslog.Error("failed to remove service", slog.Any("serviceID", key), slog.String("reason", err.Error()))
					}
				}
			}

			h.knownServices = servicesInBatch

		case err, ok := <-pollErrors:
			if !ok {
				return
			}
			bslog.Error("zone transfer did not succeed", slog.String("reason", err.Error()))

		case <-h.stop:
			bslog.Debug("no longer handling zone transfers")
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
		bslog.Error("failed to parse GSLB entry", slog.String("reason", err.Error()))
		return nil
	}

	bslog.Debug("registering new GSLB - config", slog.Any("config", svcConfig))
	svc, err := h.svcManager.RegisterService(svcConfig)
	if err != nil {
		bslog.Error("could not register service", slog.String("reason", err.Error()))
		return nil
	}

	return svc
}
