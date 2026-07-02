package dnsdist

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"regexp"
	"slices"
	"strings"
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
	"github.com/vitistack/gslb-operator/pkg/clients/dnsdist/rules"
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
		if config.SplitDNS().Enable() {
			if !dnsviews.Valid(srv.View) {
				return nil, fmt.Errorf("server %s: with unknown view: %s", srv.Name, srv.View)
			}
			selector = &dnsviews.SplitDNSSelector{View: srv.View}
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
			err := d.do(server.name,
				func() error {
					return server.Create(records...)
				},
			)
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
			return d.do(
				server.name,
				func() error {
					return server.Delete(id, views...)
				},
			)
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
	go func() {
		err := d.synchronizeServers()
		if err != nil {
			bslog.Error("failed to synchronize servers", slog.String("reason", err.Error()))
		}

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
				err := d.synchronizeServers()
				if err != nil {
					bslog.Error("unable to synchronize dnsdist - servers", slog.String("reason", err.Error()))
				}
			}
		}
	}()
}

func (d *DNSDISTUpdater) synchronizeServers() error {
	desiredHash, err := d.spoofRepo.Hash()
	if err != nil {
		return fmt.Errorf("unable to get hash representation of spoofs: %w", err)
	}

	wg := sync.WaitGroup{}
	syncErrors := make(chan error, len(d.servers))

	for _, srv := range d.servers {
		wg.Add(1)
		go func(server *server) {
			defer wg.Done()

			var rawRuleSet string
			err := d.do(server.name, func() error {
				var err error
				rawRuleSet, err = server.client.Rules().List(&rules.ListOptions{ShowUUIDs: new(true) /*, TruncateRuleWidth: new(5)*/})
				return err
			})

			if err != nil {
				bslog.Error("unable to fetch ruleset from dnsdist server", slog.String("reason", err.Error()))
				syncErrors <- fmt.Errorf("synchronization of %s failed: %w", server.name, err)
				return
			}

			spoofUUIDs, err := d.ParseRuleSet(rawRuleSet)
			if err != nil {
				bslog.Error("failed to parse dnsdist rule set", slog.String("reason", err.Error()), "server_name", server)
				syncErrors <- err
				return
			}

			joinedUUIDs := strings.Join(spoofUUIDs, ",")
			rawHash := sha256.Sum256([]byte(joinedUUIDs))
			hash := hex.EncodeToString(rawHash[:])

			if hash != desiredHash {
				err := d.reconcileServer(server.client, spoofUUIDs)
				if err != nil {
					bslog.Error("failed to reconcile dnsdist server", slog.String("server_name", server.name), slog.String("reason", err.Error()))
					syncErrors <- err
					events.Emit(&events.Event{
						Type:      domainEvents.EventTypeDNSDISTServerOutOfSync,
						Payload:   domainEvents.DNSDistServerOutOfSyncEvent{},
						Timestamp: time.Now(),
						ID:        events.ID(domainEvents.EventTypeDNSDISTServerOutOfSync, server.name),
					})
					return
				}
			}
		}(srv)
	}

	wg.Wait()
	close(syncErrors)

	for err := range syncErrors {
		if err != nil {
			events.Emit(&events.Event{
				Type: domainEvents.EventTypeDNSDISTSyncFailed,
				Payload: domainEvents.DNSDistSyncFailedEvent{
					Reason: err.Error(),
				},
				Timestamp: time.Now(), // TODO: what should the subject in the ID be?
				ID:        events.ID(domainEvents.EventTypeDNSDISTSyncFailed, "<server>?"),
			})
			return err
		}
	}

	return nil
}

func (d *DNSDISTUpdater) ParseRuleSet(ruleSet string) ([]string, error) {
	reader := strings.NewReader(ruleSet)
	lines := bufio.NewScanner(reader)

	pattern, err := regexp.Compile(`[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}|qname==[a-zA-Z0-9-_\.]+|spoof`)
	if err != nil {
		return nil, fmt.Errorf("unable to compile regex: %w", err)
	}

	spoofRules := make([]string, 0)
	for lines.Scan() {
		line := lines.Text()

		matches := pattern.FindAllString(line, -1)

		if len(matches) < 3 {
			continue
		}

		rule := rules.RuleLine{
			UUID:   matches[0],
			Rule:   matches[1],
			Action: matches[2],
		}

		if rule.Action != "spoof" && !strings.Contains(rule.Rule, "qname") {
			continue
		}

		spoofRules = append(spoofRules, rule.UUID)
	}
	slices.Sort(spoofRules)

	return spoofRules, nil
}

func (d *DNSDISTUpdater) reconcileServer(client dnsdist.Client, configuredSpoofUUIDs []string) error {
	gslbspoofs, err := d.spoofRepo.ReadAll()
	if err != nil {
		return fmt.Errorf("could not fetch spoofs: %w", err)
	}

	for _, spoof := range configuredSpoofUUIDs { // remove all spoofs that should not exist any more
		err := client.Rules().Remove(spoof)
		if err != nil {
			return fmt.Errorf("failed to remove configured spoofs: %w", err)
		}
	}

	for _, spoof := range gslbspoofs { // add all spoofs that does not exist but should
		err := client.Rules().Add(
			rules.QNameRule(spoof.FQDN),
			rules.SpoofAction(spoof.Address.Strings(), rules.SpoofActionOptions{TTL: new(30)}),
			rules.GlobalRuleOptions{
				Name: &spoof.Name,
				UUID: &spoof.UUID,
			},
		)
		if err != nil {
			return fmt.Errorf("failed to add spoofs: %w", err)
		}
	}

	return nil
}
