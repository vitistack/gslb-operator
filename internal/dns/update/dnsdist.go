package update

import (
	"bufio"
	"cmp"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/vitistack/gslb-operator/internal/config"
	"github.com/vitistack/gslb-operator/internal/model"
	domainEvents "github.com/vitistack/gslb-operator/internal/model/events"
	repo "github.com/vitistack/gslb-operator/internal/repositories/spoof"
	"github.com/vitistack/gslb-operator/internal/service"
	"github.com/vitistack/gslb-operator/pkg/bslog"
	"github.com/vitistack/gslb-operator/pkg/clients/dnsdist"
	"github.com/vitistack/gslb-operator/pkg/clients/dnsdist/rules"
	"github.com/vitistack/gslb-operator/pkg/clients/dnsdist/transport/tcp"
	"github.com/vitistack/gslb-operator/pkg/events"
	"github.com/vitistack/gslb-operator/pkg/models/spoofs"
	"github.com/vitistack/gslb-operator/pkg/persistence"
)

const DEFAULT_SYNCHRONIZE_JOB = time.Minute

// contacts dnsdist servers to make update directly
type DNSDISTUpdater struct {
	servers   map[string]dnsdist.Client
	spoofRepo repo.SpoofRepo
}

func NewDNSDISTUpdater(store persistence.Store[model.GSLBServiceGroup]) (*DNSDISTUpdater, error) {
	updater := &DNSDISTUpdater{
		servers:   make(map[string]dnsdist.Client),
		spoofRepo: *repo.NewSpoofRepo(store),
	}

	file, err := os.ReadFile(config.GetInstance().GSLB().Servers())
	if err != nil {
		return nil, fmt.Errorf("could could not load dnsdist servers configuration: %w", err)
	}

	servers := []model.DNSDISTServer{}
	err = json.Unmarshal(file, &servers)
	if err != nil {
		return nil, fmt.Errorf("malformed dnsdist servers configuration: %w", err)
	}

	for _, server := range servers {
		// initialize server connection to down
		serverUpMetric.WithLabelValues(server.Name).Set(0)

		transport, err := tcp.NewTCPTransport(
			server.Key,
			tcp.WithHost(server.Host.String()),
			tcp.WithPort(server.Port),
			tcp.WithTimeout(time.Second*5),
			tcp.WithNumRetriesOnCommandFailure(3),
		)
		if err != nil {
			return nil, fmt.Errorf("unable to create dnsdist client: %w", err)
		}

		updater.servers[server.Name] = dnsdist.NewClient(transport)
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

func (d *DNSDISTUpdater) OnServiceUp(svc *service.Service) error {
	for name, client := range d.servers {
		err := d.do(
			name,
			func() error {
				err := client.Rules().Add(
					svc.MemberOf+":"+svc.Datacenter,
					rules.QNameRule(svc.MemberOf),
					rules.SpoofAction([]string{svc.GetIP()}, rules.SpoofActionOptions{TTL: new(30)}),
				)
				if err != nil {
					return fmt.Errorf("could not create dnsdist-spoof: %w", err)
				}

				return nil
			},
		)

		if err != nil {
			return err
		}
	}

	events.Emit(&events.Event{
		Type: domainEvents.EventTypeDNSDISTSpoofCreate,
		Payload: domainEvents.DNSDistSpoofCreateEvent{
			Spoof: svc.GSLBService().Spoof(),
		},
		Timestamp: time.Now(),
		ID:        events.ID(domainEvents.EventTypeDNSDISTSpoofCreate, svc.MemberOf),
	})

	return nil
}

func (d *DNSDISTUpdater) OnServiceDown(svc *service.Service) error {
	for name, client := range d.servers {
		err := d.do(
			name,
			func() error {
				err := client.Rules().Remove(svc.MemberOf + ":" + svc.Datacenter)
				if err != nil {
					return fmt.Errorf("could not remove dnsdist-spoof: %w", err)
				}
				return nil
			},
		)

		if err != nil {
			return err
		}
	}

	events.Emit(&events.Event{
		Type: domainEvents.EventTypeDNSDISTSpoofDelete,
		Payload: domainEvents.DNSDistSpoofDeleteEvent{
			Spoof: svc.GSLBService().Spoof(),
		},
		Timestamp: time.Now(),
		ID:        events.ID(domainEvents.EventTypeDNSDISTSpoofDelete, svc.MemberOf),
	})

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
				for _, client := range d.servers {
					if client != nil {
						client.Close()
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

	for server, client := range d.servers {
		wg.Add(1)
		go func(server string) {
			defer wg.Done()

			var rawRuleSet string
			err := d.do(server, func() error {
				var err error
				rawRuleSet, err = client.Rules().List()
				return err
			})

			if err != nil {
				bslog.Error("unable to fetch ruleset from dnsdist server", slog.String("reason", err.Error()))
				syncErrors <- fmt.Errorf("synchronization of %s failed: %w", server, err)
				return
			}

			data, err := d.ParseRuleSet(rawRuleSet)
			if err != nil {
				bslog.Error("could not synchronize dnsdist server", slog.String("reason", err.Error()))
				syncErrors <- fmt.Errorf("synchronization of %s failed: %w", server, err)
				return
			}

			slices.SortFunc(data, func(a, b spoofs.Spoof) int {
				return cmp.Compare(fmt.Sprintf("%s:%s", a.FQDN, a.DC), fmt.Sprintf("%s:%s", b.FQDN, b.DC))
			})

			marshalledSpoofs, err := json.Marshal(data)
			if err != nil {
				bslog.Error("unable to marshall spoofs", slog.String("reason", err.Error()))
				syncErrors <- fmt.Errorf("synchronization of %s failed: %w", server, err)
				return
			}

			// hash representation of all spoofs
			rawHash := sha256.Sum256(marshalledSpoofs)
			hash := hex.EncodeToString(rawHash[:])
			if hash != desiredHash {
				err := d.reconcileServer(client, data)
				if err != nil {
					bslog.Warn("failed to reconcile server", slog.String("server_name", server), slog.String("reason", err.Error()))
					syncErrors <- err
					events.Emit(&events.Event{
						Type:      domainEvents.EventTypeDNSDISTServerOutOfSync,
						Payload:   domainEvents.DNSDistServerOutOfSyncEvent{},
						Timestamp: time.Now(),
						ID:        events.ID(domainEvents.EventTypeDNSDISTServerOutOfSync, server),
					})
					return
				}
			}
		}(server)
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

func (d *DNSDISTUpdater) ParseRuleSet(ruleSet string) ([]spoofs.Spoof, error) {
	reader := strings.NewReader(ruleSet)
	lines := bufio.NewScanner(reader)

	pattern, err := regexp.Compile(`[a-zA-Z0-9._-]+:[A-Z0-9]+|spoof|\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}`)
	if err != nil {
		return nil, fmt.Errorf("unable to compile regex: %w", err)
	}

	spoofRules := make([]spoofs.Spoof, 0)
	for lines.Scan() {
		line := lines.Text()
		matches := pattern.FindAllString(line, -1)
		if len(matches) < 3 {
			continue
		}
		rule := rules.RuleLine{
			Name:   matches[0],
			Action: matches[1],
		}

		if rule.Action != "spoof" {
			continue
		}

		spoofRules = append(spoofRules,
			spoofs.Spoof{
				FQDN: strings.Split(rule.Name, ":")[0],
				DC:   strings.Split(rule.Name, ":")[1],
				IP:   matches[2],
			})
	}

	return spoofRules, nil
}

func (d *DNSDISTUpdater) reconcileServer(client dnsdist.Client, configuredSpoofs []spoofs.Spoof) error {
	gslbspoofs, err := d.spoofRepo.ReadAll()
	if err != nil {
		return fmt.Errorf("could not fetch spoofs: %w", err)
	}

	for _, spoof := range configuredSpoofs { // remove all spoofs that should not exist any more
		if !slices.ContainsFunc(gslbspoofs, func(s spoofs.Spoof) bool {
			return s.FQDN+":"+s.DC == spoof.FQDN+":"+spoof.DC
		}) {
			err := client.Rules().Remove(spoof.FQDN + ":" + spoof.DC)
			if err != nil {
				return fmt.Errorf("could not remove spoof: %w", err)
			}
		}
	}

	for _, spoof := range gslbspoofs { // add all spoofs that does not exist but should
		if !slices.ContainsFunc(configuredSpoofs, func(s spoofs.Spoof) bool {
			return s.FQDN+":"+s.DC == spoof.FQDN+":"+spoof.DC
		}) {
			err := client.Rules().Add(
				spoof.FQDN+":"+spoof.DC,
				rules.QNameRule(spoof.FQDN),
				rules.SpoofAction([]string{spoof.IP}, rules.SpoofActionOptions{TTL: new(30)}),
			)
			if err != nil {
				return fmt.Errorf("could not remove spoof: %w", err)
			}
		}
	}

	return nil
}
