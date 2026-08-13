package dnsdist

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/vitistack/gslb-operator/internal/config"
	"github.com/vitistack/gslb-operator/internal/dns/update"
	dnsviews "github.com/vitistack/gslb-operator/internal/dns/views"
	"github.com/vitistack/gslb-operator/internal/model"
	domainEvents "github.com/vitistack/gslb-operator/internal/model/events"
	repo "github.com/vitistack/gslb-operator/internal/repositories/spoof"
	"github.com/vitistack/gslb-operator/pkg/bslog"
	"github.com/vitistack/gslb-operator/pkg/clients/dnsdist"
	"github.com/vitistack/gslb-operator/pkg/clients/dnsdist/transport/tcp"
	"github.com/vitistack/gslb-operator/pkg/events"
	"github.com/vitistack/gslb-operator/pkg/persistence"
	"golang.org/x/sync/errgroup"
)

const DEFAULT_SYNCHRONIZE_JOB = time.Minute

// contacts dnsdist servers to make update directly
type DNSDISTUpdater struct {
	servers   map[string]*server
	spoofRepo repo.SpoofRepo
}

func NewDNSDISTUpdater(store persistence.Store[model.GSLBServiceGroup]) (*DNSDISTUpdater, error) {
	updater := &DNSDISTUpdater{
		servers:   make(map[string]*server),
		spoofRepo: *repo.NewSpoofRepo(store),
	}

	file, err := os.ReadFile(config.GSLB().Servers())
	if err != nil {
		return nil, fmt.Errorf("could could not load dnsdist servers configuration: %w", err)
	}

	servers := []model.DNSDISTServer{}
	err = json.Unmarshal(file, &servers)
	if err != nil {
		return nil, fmt.Errorf("malformed dnsdist servers configuration: %w", err)
	}

	for _, srv := range servers {
		// initialize server connection to down
		serverUpMetric.WithLabelValues(srv.Name).Set(0)

		transport, err := tcp.NewTCPTransport(
			srv.Key,
			tcp.WithHost(srv.Host.String()),
			tcp.WithPort(srv.Port),
			tcp.WithTimeout(time.Second*5),
			tcp.WithNumRetriesOnCommandFailure(3),
		)
		if err != nil {
			return nil, fmt.Errorf("unable to create dnsdist client: %w", err)
		}

		var selector dnsviews.Selector = &dnsviews.AllSelector{}
		if config.DNS().Enable() {
			if !dnsviews.Valid(srv.View) {
				return nil, fmt.Errorf("server %s: with unknown view: %s", srv.Name, srv.View)
			}
			selector = dnsviews.NewSplitDNSSelector(srv.View)
		}

		updater.servers[srv.Name] = &server{
			name:     srv.Name,
			client:   dnsdist.NewClient(transport),
			selector: selector,
		}
	}

	return updater, nil
}

// wrapper function to handle the serverUpMetric with the different dnsdist client calls
func (d *DNSDISTUpdater) do(name string, fn func() error) error {
	if err := fn(); err != nil {
		// dnsdist server connection considered down
		serverUpMetric.WithLabelValues(name).Set(0)
		return err
	}
	// continue to present connection as up
	serverUpMetric.WithLabelValues(name).Set(1)
	return nil
}

func (d *DNSDISTUpdater) Create(records ...update.Record) error {
	wg := errgroup.Group{}

	for _, server := range d.servers {
		wg.Go(func() error {
			err := d.do(server.name, func() error { return server.Create(records...) })
			if err != nil {
				return err
			}

			return nil
		})
	}

	err := wg.Wait()
	if err != nil {
		if updateErr, ok := errors.AsType[update.UpdateError](err); ok {
			events.Emit(&events.Event{
				Type: domainEvents.EventTypeDNSDISTSpoofCreateFailed,
				Payload: domainEvents.DNSDistSpoofCreateFailedEvent{
					Server: updateErr.Server,
					Spoof:  updateErr.Spoof,
				},
				Timestamp: time.Now(),
				ID:        events.ID(domainEvents.EventTypeDNSDISTSpoofCreateFailed, updateErr.Server),
			})

			return updateErr
		}

		return err
	}

	return nil
}

func (d *DNSDISTUpdater) Delete(id string, views ...string) error {
	wg := errgroup.Group{}

	for _, server := range d.servers {
		wg.Go(func() error {
			return d.do(server.name, func() error { return server.Delete(id, views...) })
		})
	}

	err := wg.Wait()
	if err != nil {
		if updateErr, ok := errors.AsType[update.UpdateError](err); ok {
			events.Emit(&events.Event{
				Type:      domainEvents.EventTypeDNSDISTSpoofDeleteFailed,
				Payload:   domainEvents.DNSDistSpoofDeleteFailedEvent{Server: updateErr.Server, ID: id},
				Timestamp: time.Now(),
				ID:        events.ID(domainEvents.EventTypeDNSDISTSpoofDeleteFailed, updateErr.Server),
			})

			return updateErr
		}

		return err
	}

	return nil
}

func (d *DNSDISTUpdater) Synchronize(ctx context.Context) {
	err := d.synchronizeServers()
	if err != nil {
		bslog.Error("failed to synchronize dnsdist servers", slog.String("reason", err.Error()))
		events.Emit(&events.Event{
			Type:      domainEvents.EventTypeDNSDISTSyncFailed,
			Payload:   domainEvents.DNSDistSyncFailedEvent{Reason: err.Error()},
			Timestamp: time.Now(),
			ID:        events.ID(domainEvents.EventTypeDNSDISTSyncFailed, "???"),
		})
	}

	go func() {
		for {
			select {
			case <-ctx.Done():
				bslog.Info("stopping dnsdist - server synchronization")
				// close controll socket connections
				for _, server := range d.servers {
					if server.client != nil {
						bslog.Debug("closing dnsdist - server connection", slog.String("server", server.name))
						server.client.Close()
					}
				}

				return

			case <-time.After(DEFAULT_SYNCHRONIZE_JOB):
				bslog.Debug("starting dnsdist server synchronization")

				if err := d.synchronizeServers(); err != nil {
					bslog.Error("failed to synchronize dnsdist servers", slog.String("reason", err.Error()))
					events.Emit(&events.Event{
						Type:      domainEvents.EventTypeDNSDISTSyncFailed,
						Payload:   domainEvents.DNSDistSyncFailedEvent{Reason: err.Error()},
						Timestamp: time.Now(),
						ID:        events.ID(domainEvents.EventTypeDNSDISTSyncFailed, "???"),
					})
				}
			}
		}
	}()
}

func (d *DNSDISTUpdater) synchronizeServers() error {
	wg := &errgroup.Group{}

	hashesByView := make(map[string]string)
	lock := sync.Mutex{}
	for _, server := range d.servers {
		wg.Go(func() error {
			lock.Lock()
			hash, ok := hashesByView[server.selector.View()]
			lock.Unlock()
			if !ok {
				viewHash, err := d.spoofRepo.Hash(server.selector.View())
				if err != nil {
					return err
				}
				lock.Lock()
				hashesByView[server.selector.View()] = viewHash
				lock.Unlock()
				hash = viewHash
			}

			var serverHash string
			err := d.do(server.name,
				func() error {
					hash, err := server.Hash()
					if err != nil {
						return err
					}
					serverHash = hash
					return nil
				})

			if err != nil {
				return fmt.Errorf("failed to compute %s hash: %w", server.name, err)
			}

			if hash != serverHash {
				err = d.do(server.name, func() error { return server.Reconcile(d.spoofRepo.ReadAll()) })
				if err != nil {
					return fmt.Errorf("failed to reconcile %s: %w", server.name, err)
				}
				return nil
			}

			return nil
		})
	}

	if err := wg.Wait(); err != nil {
		return err
	}

	return nil
}
