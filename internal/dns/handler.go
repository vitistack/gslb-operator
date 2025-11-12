package dns

import (
	"encoding/json"
	"log"
	"strings"
	"sync"

	"codeberg.org/miekg/dns"
	"github.com/vitistack/gslb-operator/internal/manager"
	"github.com/vitistack/gslb-operator/internal/model"
	"github.com/vitistack/gslb-operator/internal/service"
)

// wrapper object for ZoneFetcher
type Handler struct {
	fetcher     *ZoneFetcher
	manager     *manager.ServicesManager
	updater     *Updater
	stopFetcher chan struct{}
	wg          sync.WaitGroup
}

func NewHandler(fetcher *ZoneFetcher, mgr *manager.ServicesManager, updater *Updater) *Handler {
	return &Handler{
		fetcher:     fetcher,
		manager:     mgr,
		updater:     updater,
		stopFetcher: make(chan struct{}),
		wg:          sync.WaitGroup{},
	}
}

func (h *Handler) Start() error {
	h.manager.Start()

	records, pollErrors, err := h.fetcher.StartAutoPoll()
	if err != nil {
		return err
	}

	h.wg.Go(func() {
		h.handleZoneUpdates(records, pollErrors)
	})

	return nil
}

func (h *Handler) Stop() {
	close(h.stopFetcher)
	h.wg.Wait()
	h.fetcher.StopPoll()
	h.manager.Stop()
}

func (h *Handler) onServiceDown(svc *service.Service) {
	h.updater.ServiceDown(svc)
}

func (h *Handler) onServiceUp(svc *service.Service) {
	h.updater.ServiceUp(svc)
}

func (h *Handler) handleZoneUpdates(records <-chan dns.RR, pollErrors <-chan error) {
	for {
		select {
		case record, ok := <-records:
			if !ok {
				return
			}
			h.handleRecord(record)

		case err, ok := <-pollErrors:
			if !ok {
				return
			}
			log.Println(err)

		case <-h.stopFetcher:
			return
		}
	}
}

func (h *Handler) handleRecord(record dns.RR) {
	txt, ok := record.(*dns.TXT)
	if !ok {
		return
	}

	rawData := txt.Txt[0]
	data := strings.ReplaceAll(rawData, "\\", "")
	gslbConfig := model.GSLBConfig{}
	err := json.Unmarshal([]byte(data), &gslbConfig)
	if err != nil {
		log.Println("failed to parse gslb config: ", err.Error())
		return
	}

	svc := service.NewServiceFromGSLBConfig(gslbConfig)
	svc.SetHealthCheckCallback(func(healthy bool) {
		if !healthy {
			h.onServiceDown(svc)
		} else {
			h.onServiceUp(svc)
		}
	})

	h.manager.RegisterService(svc, false)
}
