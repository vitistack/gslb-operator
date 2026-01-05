package dns

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"codeberg.org/miekg/dns"
	"github.com/vitistack/gslb-operator/internal/utils/timesutil"
	"go.uber.org/zap"
)

// Fetches a DNS zone from a dedicated server via AXFR
type ZoneFetcher struct {
	Zone     string
	Server   string
	log      *zap.Logger
	stop     chan struct{}
	wg       sync.WaitGroup
	interval timesutil.Duration
}

type fetcherOption func(fetcher *ZoneFetcher)

// auto fetches after a given duration
func NewZoneFetcherWithAutoPoll(logger *zap.Logger, opts ...fetcherOption) *ZoneFetcher {
	fetcher := &ZoneFetcher{ // default values for testing
		Zone:     "gslb.test.dns.nhn.no.",
		Server:   "nsh1.nhn.no:53",
		interval: timesutil.Duration(DEFAULT_POLL_INTERVAL),
		stop:     make(chan struct{}),
		wg:       sync.WaitGroup{},
		log:      logger,
	}

	for _, opt := range opts { // set custom options
		opt(fetcher)
	}

	return fetcher
}

func WithZone(zone string) fetcherOption {
	return func(fetcher *ZoneFetcher) {
		fetcher.Zone = zone
	}
}

func WithServer(server string) fetcherOption {
	return func(fetcher *ZoneFetcher) {
		fetcher.Server = server
	}
}

func WithFetchInterval(interval time.Duration) fetcherOption {
	return func(fetcher *ZoneFetcher) {
		fetcher.interval = timesutil.Duration(interval)
	}
}

// starts the auto-fetch, and listen for errors and records on the returned channels
// WARNING: Returns immediatly if stop is not initialized. Call Upgrade(...) to start autopoll
func (f *ZoneFetcher) StartAutoPoll() (zoneBatch chan []dns.RR, pollErrors chan error, err error) {
	zoneBatch = make(chan []dns.RR)
	pollErrors = make(chan error)

	ticker := time.NewTicker(time.Duration(f.interval))

	if f.stop == nil { // needs to call upgrade first, or initialize with auto poll
		return nil, nil, errors.New("fetcher not configured for auto-poll")
	}

	f.wg.Go(func() {
		defer ticker.Stop()
		defer close(zoneBatch)
		defer close(pollErrors)

		records, err := f.AXFRTransfer() // initial transfer
		if err != nil {
			pollErrors <- err
		} else {
			zoneBatch <- records
		}

		for {
			select {
			case <-ticker.C:
				records, err := f.AXFRTransfer()
				if err != nil {
					pollErrors <- err
				} else {
					zoneBatch <- records // sends complete zone transfer
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
		f.log.Debug("closing zone-fetcher")
	}
}

func (f *ZoneFetcher) AXFRTransfer() ([]dns.RR, error) {
	client := dns.NewClient()
	client.Transfer = &dns.Transfer{}
	msg := dns.NewMsg(f.Zone, dns.TypeAXFR)

	env, err := client.TransferIn(context.TODO(), msg, "tcp", f.Server)
	if err != nil {
		return nil, fmt.Errorf("could not transfer zone: %v from server: %v:%w", f.Zone, f.Server, err)
	}

	records := make([]dns.RR, 0)
	for envelope := range env {
		if envelope.Error != nil {
			return nil, envelope.Error
		}
		records = append(records, envelope.Answer...)
	}
	f.log.Debug("Zone-Transfer Complete")

	return records, nil
}
