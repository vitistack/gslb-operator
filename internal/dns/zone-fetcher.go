package dns

import (
	"context"
	"fmt"
	"sync"
	"time"

	"codeberg.org/miekg/dns"
	"github.com/vitistack/gslb-operator/internal/config"
	"github.com/vitistack/gslb-operator/internal/utils/timesutil"
	"github.com/vitistack/gslb-operator/pkg/bslog"
)

// Fetches a DNS zone from a dedicated server via AXFR
type ZoneFetcher struct {
	Zone     string
	Server   string
	wg       sync.WaitGroup
	interval timesutil.Duration
	client   *dns.Client
}

type fetcherOption func(fetcher *ZoneFetcher)

// auto fetches after a given duration
func NewZoneFetcherWithAutoPoll(opts ...fetcherOption) *ZoneFetcher {
	gslb := config.GetInstance().GSLB()
	fetcInterval, err := gslb.PollInterval()
	if err != nil {
		fetcInterval = timesutil.Duration(DEFAULT_POLL_INTERVAL)
	}

	fetcher := &ZoneFetcher{ // default values
		Zone:     gslb.Zone(),
		Server:   gslb.NameServer(),
		wg:       sync.WaitGroup{},
		interval: fetcInterval,
		client:   dns.NewClient(),
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
func (f *ZoneFetcher) StartAutoPoll(ctx context.Context) (zone chan []dns.RR, pollErrors chan error) {
	zone = make(chan []dns.RR, 10)
	pollErrors = make(chan error, 10)

	ticker := time.NewTicker(time.Duration(f.interval))
	f.wg.Go(func() {
		defer close(zone)
		defer close(pollErrors)
		defer bslog.Debug("closing zone-fetcher")

		f.AXFRTransfer(ctx, zone, pollErrors)

		for {
			select {
			case <-ctx.Done():
				ticker.Stop()
				return

			case <-ticker.C:
				f.AXFRTransfer(ctx, zone, pollErrors)
			}
		}
	})
	return
}

func (f *ZoneFetcher) StopPoll() {
	f.wg.Wait()

}

func (f *ZoneFetcher) AXFRTransfer(ctx context.Context, zone chan []dns.RR, transferErrors chan error) {
	if ctx.Err() != nil {
		return // context is cancelled
	}

	bslog.Debug("starting zone-transfer")
	f.client.Transfer = &dns.Transfer{}
	msg := dns.NewMsg(f.Zone, dns.TypeAXFR)

	envelopes, err := f.client.TransferIn(ctx, msg, "tcp", f.Server)
	if err != nil {
		transferErrors <- fmt.Errorf("could not transfer zone: %v from server: %v:%w", f.Zone, f.Server, err)
	}
	records := make([]dns.RR, 0)
	for {
		select {
		case <-ctx.Done():
			return

		case envelope, ok := <-envelopes:
			if !ok {
				zone <- records
				bslog.Debug("zone-transfer completed")
				return // zone complete
			}
			if envelope.Error != nil {
				transferErrors <- envelope.Error
				return
			}
			records = append(records, envelope.Answer...)
		}
	}
}
