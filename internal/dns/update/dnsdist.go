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
	repo "github.com/vitistack/gslb-operator/internal/repositories/spoof"
	"github.com/vitistack/gslb-operator/internal/service"
	"github.com/vitistack/gslb-operator/pkg/bslog"
	"github.com/vitistack/gslb-operator/pkg/dnsdist"
	"github.com/vitistack/gslb-operator/pkg/models/spoofs"
	"github.com/vitistack/gslb-operator/pkg/persistence"
)

const DEFAULT_SYNCHRONIZE_JOB = time.Minute

// contacts dnsdist servers to make update directly
type DNSDISTUpdater struct {
	servers   map[string]*dnsdist.Client
	spoofRepo repo.SpoofRepo
}

func NewDNSDISTUpdater(store persistence.Store[model.GSLBServiceGroup]) (*DNSDISTUpdater, error) {
	updater := &DNSDISTUpdater{
		servers:   make(map[string]*dnsdist.Client),
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
		client, err := dnsdist.NewClient(
			server.Key,
			dnsdist.WithHost(server.Host.String()),
			dnsdist.WithPort(server.Port),
			dnsdist.WithTimeout(time.Second*5),
			dnsdist.WithNumRetriesOnCommandFailure(3),
		)

		if err != nil {
			return nil, fmt.Errorf("unable to create dnsdist client: %w", err)
		}

		updater.servers[server.Name] = client
	}

	err = updater.synchronizeServers()
	if err != nil {
		return updater, fmt.Errorf("failed synchronization on updater init: %w", err)
	}

	return updater, nil
}

func (d *DNSDISTUpdater) OnServiceUp(svc *service.Service) error {

	for _, client := range d.servers {
		err := client.AddDomainSpoof(svc.MemberOf+":"+svc.Datacenter, svc.MemberOf, svc.GetIP())
		if err != nil {
			return fmt.Errorf("could not create dnsdist-spoof: %w", err)
		}
	}

	return nil
}

func (d *DNSDISTUpdater) OnServiceDown(svc *service.Service) error {
	for _, client := range d.servers {
		err := client.RmRuleWithName(svc.MemberOf + ":" + svc.Datacenter)
		if err != nil {
			return fmt.Errorf("could not remove dnsdist-spoof: %w", err)
		}
	}
	return nil
}

func (d *DNSDISTUpdater) Synchronize(ctx context.Context) {
	go func() {
		for {
			select {
			case <-ctx.Done():
				bslog.Info("stopping dnsdist - server synchronization")

				// close controll socket connections
				for _, client := range d.servers {
					client.Disconnect()
				}

				return
			case <-time.After(DEFAULT_SYNCHRONIZE_JOB):
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

	for server, client := range d.servers {
		wg.Go(func() {
			rawRuleSet, err := client.ShowRules()
			if err != nil {
				bslog.Error("unable to fetch ruleset from dnsdist server", slog.String("reason", err.Error()))
				return
			}

			data, err := d.ParseRuleSet(rawRuleSet)
			if err != nil {
				bslog.Error("could not synchronize dnsdist server", slog.String("reason", err.Error()))
			}

			slices.SortFunc(data, func(a, b spoofs.Spoof) int {
				return cmp.Compare(fmt.Sprintf("%s:%s", a.FQDN, a.DC), fmt.Sprintf("%s:%s", b.FQDN, b.DC))
			})

			marshalledSpoofs, err := json.Marshal(data)
			if err != nil {
				bslog.Error("unable to marshall spoofs", slog.String("reason", err.Error()))
				return
			}

			rawHash := sha256.Sum256(marshalledSpoofs) // creating bytes representation of spoofs
			hash := hex.EncodeToString(rawHash[:])
			if hash != desiredHash {
				err := d.reconcileServer(client, data)
				if err != nil {
					bslog.Warn("failed to reconcile server", slog.String("server_name", server))
				}
			}
		})
	}

	wg.Wait()

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
		rule := dnsdist.Rule{
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

func (d *DNSDISTUpdater) reconcileServer(client *dnsdist.Client, configuredSpoofs []spoofs.Spoof) error {
	gslbspoofs, err := d.spoofRepo.ReadAll()
	if err != nil {
		return fmt.Errorf("could not fetch spoofs: %w", err)
	}

	for _, spoof := range configuredSpoofs { // remove all spoofs that should not exist any more
		if !slices.ContainsFunc(gslbspoofs, func(s spoofs.Spoof) bool {
			return s.FQDN+":"+s.DC == spoof.FQDN+":"+spoof.DC
		}) {
			err := client.RmRuleWithName(spoof.FQDN + ":" + spoof.DC)
			if err != nil {
				return fmt.Errorf("could not remove spoof: %w", err)
			}
		}
	}

	for _, spoof := range gslbspoofs { // add all spoofs that does not exist but should
		if !slices.ContainsFunc(configuredSpoofs, func(s spoofs.Spoof) bool {
			return s.FQDN+":"+s.DC == spoof.FQDN+":"+spoof.DC
		}) {
			err := client.AddDomainSpoof(spoof.FQDN+":"+spoof.DC, spoof.FQDN, spoof.IP)
			if err != nil {
				return fmt.Errorf("could not remove spoof: %w", err)
			}
		}
	}

	return nil
}
