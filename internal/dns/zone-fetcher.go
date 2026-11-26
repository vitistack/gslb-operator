package dns

import (
	"context"
	"fmt"
	"log/slog"
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
	timeout  time.Duration
	client   *dns.Client
}

type fetcherOption func(fetcher *ZoneFetcher)

// auto fetches after a given duration
func NewZoneFetcherWithAutoPoll(opts ...fetcherOption) *ZoneFetcher {
	gslb := config.GSLB()
	fetcInterval, err := gslb.Poll()
	if err != nil {
		fetcInterval = timesutil.Duration(DEFAULT_POLL_INTERVAL)
	}

	fetcher := &ZoneFetcher{ // default values
		Zone:     gslb.Zone(),
		Server:   gslb.Nameserver(),
		wg:       sync.WaitGroup{},
		interval: fetcInterval,
		timeout:  time.Second * 5,
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

func WithTimeout(timeout time.Duration) fetcherOption {
	return func(fetcher *ZoneFetcher) {
		fetcher.timeout = timeout
	}
}

// starts the auto-fetch, and listen for errors and records on the returned channels
func (f *ZoneFetcher) StartAutoPoll(ctx context.Context) (zone chan []dns.RR, pollErrors chan error) {
	zone = make(chan []dns.RR, 1)
	pollErrors = make(chan error)

	bslog.Debug("polling config zone", slog.String("interval", f.interval.String()))
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

	transferID := fmt.Sprintf("%s:%d", f.Zone, time.Now().Unix())
	transferStart := time.Now()
	envelopeCount := 0

	bslog.Info("zone-transfer started",
		slog.String("zone", f.Zone),
		slog.String("server", f.Server),
		slog.String("transfer_id", transferID),
	)

	msg := dns.NewMsg(f.Zone, dns.TypeAXFR)

	envelopes, err := f.client.TransferIn(ctx, msg, f.Server, nil)
	if err != nil {
		transferErr := fmt.Errorf("could not transfer zone: %v from server: %v:%w", f.Zone, f.Server, err)
		bslog.Error("zone-transfer failed to initiate",
			slog.String("zone", f.Zone),
			slog.String("server", f.Server),
			slog.String("transfer_id", transferID),
			slog.String("error", err.Error()),
		)
		transferErrors <- transferErr
		return
	}

	records := make([]dns.RR, 0)
	for {
		select {
		case <-ctx.Done():
			bslog.Warn("zone-transfer cancelled before sending records",
				slog.String("zone", f.Zone),
				slog.String("transfer_id", transferID),
				slog.Int("envelopes_received", envelopeCount),
				slog.Int("records_accumulated", len(records)),
				slog.String("duration", time.Since(transferStart).String()),
			)
			return

		case envelope, ok := <-envelopes:
			if !ok { // transfer completed
				duration := time.Since(transferStart)
				bslog.Info("zone-transfer completed",
					slog.String("zone", f.Zone),
					slog.String("server", f.Server),
					slog.String("transfer_id", transferID),
					slog.Int("total_records", len(records)),
					slog.Int("total_envelopes", envelopeCount),
					slog.String("duration", duration.String()),
					slog.Int64("duration_ms", duration.Milliseconds()),
				)

				// safe publish to consumer
				select {
				case <-ctx.Done():
					bslog.Warn("zone-transfer cancelled before sending records to consumer",
						slog.String("zone", f.Zone),
						slog.String("transfer_id", transferID),
					)
					return
				case zone <- records:
					bslog.Debug("zone-transfer records delivered to consumer",
						slog.String("transfer_id", transferID),
						slog.Int("record_count", len(records)),
					)
					return
				case <-time.After(f.timeout): // dont block forever
					bslog.Warn("zone-transfer timed out while sending records to consumer",
						slog.String("zone", f.Zone),
						slog.String("transfer_id", transferID),
						slog.String("timeout", f.timeout.String()),
						slog.Int("records_pending", len(records)),
					)
					return
				}
			}

			if envelope.Error != nil {
				bslog.Error("zone-transfer envelope error",
					slog.String("zone", f.Zone),
					slog.String("transfer_id", transferID),
					slog.String("error", envelope.Error.Error()),
					slog.Int("envelopes_received", envelopeCount),
					slog.Int("records_accumulated", len(records)),
				)
				transferErrors <- envelope.Error
				return
			}

			envelopeCount++
			records = append(records, envelope.Answer...)

			if envelopeCount%2 == 0 {
				bslog.Debug("zone-transfer progress",
					slog.String("transfer_id", transferID),
					slog.Int("envelopes_received", envelopeCount),
					slog.Int("records_accumulated", len(records)),
					slog.Int64("elapsed_ms", time.Since(transferStart).Milliseconds()),
				)
			}

		case <-time.After(f.timeout):
			bslog.Warn("zone-transfer timed out receiving records",
				slog.String("zone", f.Zone),
				slog.String("server", f.Server),
				slog.String("transfer_id", transferID),
				slog.String("timeout", f.timeout.String()),
				slog.Int("envelopes_received", envelopeCount),
				slog.Int("records_accumulated", len(records)),
				slog.String("duration", time.Since(transferStart).String()),
			)
			return
		}
	}
}
