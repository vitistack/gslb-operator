package dns

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"codeberg.org/miekg/dns"
	"github.com/vitistack/gslb-operator/internal/utils/timesutil"
)

// Fetches a DNS zone from a dedicated server via AXFR
type ZoneFetcher struct {
	Zone     string
	Server   string
	stop     chan struct{}
	wg       sync.WaitGroup
	interval timesutil.Duration
}

// auto fetches after a given duration
func NewZoneFetcherWithAutoPoll(zone, server string, pollIntervall time.Duration) *ZoneFetcher {
	return &ZoneFetcher{
		Zone:     zone,
		Server:   server,
		stop:     make(chan struct{}),
		wg:       sync.WaitGroup{},
		interval: timesutil.Duration(pollIntervall),
	}
}

// starts the auto-fetch, and listen for errors and records on the returned channels
// WARNING: Returns immediatly if stop is not initialized. Call Upgrade(...) to start autopoll
func (f *ZoneFetcher) StartAutoPoll() (records chan dns.RR, pollErrors chan error, err error) {
	records = make(chan dns.RR)
	pollErrors = make(chan error)

	ticker := time.NewTicker(time.Duration(f.interval))

	if f.stop == nil { // needs to call upgrade first, or initialize with auto poll
		return nil, nil, errors.New("fetcher not configured for auto-poll")
	}

	f.wg.Go(func() {
		defer ticker.Stop()
		defer close(records)
		defer close(pollErrors)
		for {
			select {
			case <-ticker.C:
				dnsEnvelopeRecords, err := f.AXFRTransfer()
				if err != nil {
					pollErrors <- err
				}

				for _, record := range dnsEnvelopeRecords {
					records <- record
				}

			case <-f.stop:
				return
			}
		}
	})
	return
}

func (f *ZoneFetcher) StopPoll() {
	if f.stop != nil {
		close(f.stop)
		f.wg.Wait()
	}
}

func (f *ZoneFetcher) AXFRTransfer() ([]dns.RR, error) {
	client := dns.NewClient()
	client.Transfer = &dns.Transfer{}
	msg := dns.NewMsg(f.Zone, dns.TypeAXFR)

	env, err := client.TransferIn(context.TODO(), msg, "tcp", f.Server)
	if err != nil {
		return nil, fmt.Errorf("could not transfer zone: %v from server: %v%w", f.Zone, f.Server, err)
	}

	records := make([]dns.RR, 0)
	for envelope := range env {
		if envelope.Error != nil {
			return nil, envelope.Error
		}
		records = append(records, envelope.Answer...)
	}

	return records, nil
}
