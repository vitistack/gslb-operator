package dnsdist

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/vitistack/gslb-operator/internal/dns/update"
	dnsviews "github.com/vitistack/gslb-operator/internal/dns/views"
	domainEvents "github.com/vitistack/gslb-operator/internal/model/events"
	"github.com/vitistack/gslb-operator/pkg/clients/dnsdist"
	"github.com/vitistack/gslb-operator/pkg/clients/dnsdist/rules"
	"github.com/vitistack/gslb-operator/pkg/events"
	"github.com/vitistack/gslb-operator/pkg/iter"
	"github.com/vitistack/gslb-operator/pkg/models/spoofs"
)

type server struct {
	name     string
	client   dnsdist.Client
	selector dnsviews.Selector
}

func (s *server) Create(records ...update.Record) error {
	for _, rec := range records {
		if !s.selector.Select(rec.Views...) {
			return nil
		}

		exist, err := s.client.Rules().Exist(rec.UUID)
		if err != nil {
			return update.UpdateError{
				Err:    fmt.Errorf("%s: unable to check existing rules: %w", s.name, err),
				Server: s.name,
				Spoof:  spoofs.Spoof{FQDN: rec.Name, Address: rec.Address, UUID: rec.UUID},
			}
		}

		if exist {
			err := s.client.Rules().Remove(rec.Name)
			if err != nil {
				return update.UpdateError{
					Err:    fmt.Errorf("%s: failed to delete old record: %w", s.name, err),
					Server: s.name,
					Spoof:  spoofs.Spoof{FQDN: rec.Name, Address: rec.Address, UUID: rec.UUID},
				}
			}
		}

		err = s.client.Rules().Add(
			rules.QNameRule(rec.Name),
			rules.SpoofAction(
				rec.Address.Strings(),
				rules.SpoofActionOptions{TTL: new(30)},
			),
			rules.GlobalRuleOptions{
				Name: &rec.Name,
				UUID: &rec.UUID,
			},
		)

		if err != nil {
			return update.UpdateError{
				Err:    fmt.Errorf("failed to create spoof: %w", err),
				Server: s.name,
				Spoof:  spoofs.Spoof{FQDN: rec.Name, Address: rec.Address, UUID: rec.UUID},
			}
		}
	}

	return nil
}

func (s *server) Delete(id string, views ...string) error {
	if !s.selector.Select(views...) {
		return nil
	}

	exist, err := s.client.Rules().Exist(id)
	if err != nil {
		return update.UpdateError{
			Err:    fmt.Errorf("%s: unable to check existing rules: %w", s.name, err),
			Server: s.name,
		}
	}

	if !exist {
		return nil
	}

	err = s.client.Rules().Remove(id)
	if err != nil {
		return update.UpdateError{
			Err:    fmt.Errorf("%s: failed to delete record: %w", s.name, err),
			Server: s.name,
		}
	}

	return nil
}

func (s *server) Hash() (string, error) {
	rawRuleSet, err := s.client.Rules().List(&rules.ListOptions{ShowUUIDs: new(true)})
	if err != nil {
		return "", fmt.Errorf("failed to list rules: %w", err)
	}

	spoofUUIDs, err := ParseRuleSet(rawRuleSet)
	if err != nil {
		return "", fmt.Errorf("%s could not parse rules: %w", s.name, err)
	}

	joinedUUIDs := strings.Join(spoofUUIDs, ",")
	rawHash := sha256.Sum256([]byte(joinedUUIDs))
	return hex.EncodeToString(rawHash[:]), nil
}

func (s *server) Reconcile(gslbSpoofs iter.Iterator[spoofs.Spoof], finish func() error) error {
	rawRuleSet, err := s.client.Rules().List(&rules.ListOptions{ShowUUIDs: new(true)})
	if err != nil {
		events.Emit(&events.Event{
			Type:      domainEvents.EventTypeDNSDISTServerOutOfSync,
			Payload:   domainEvents.DNSDistServerOutOfSyncEvent{},
			Timestamp: time.Now(),
			ID:        events.ID(domainEvents.EventTypeDNSDISTServerOutOfSync, s.name),
		})
		return fmt.Errorf("failed to list rules: %w", err)
	}

	spoofUUIDs, err := ParseRuleSet(rawRuleSet)
	if err != nil {
		events.Emit(&events.Event{
			Type:      domainEvents.EventTypeDNSDISTServerOutOfSync,
			Payload:   domainEvents.DNSDistServerOutOfSyncEvent{},
			Timestamp: time.Now(),
			ID:        events.ID(domainEvents.EventTypeDNSDISTServerOutOfSync, s.name),
		})
		return fmt.Errorf("%s could not parse rules: %w", s.name, err)
	}

	for _, configuredSpoof := range spoofUUIDs {
		err := s.client.Rules().Remove(configuredSpoof)
		if err != nil {
			events.Emit(&events.Event{
				Type:      domainEvents.EventTypeDNSDISTServerOutOfSync,
				Payload:   domainEvents.DNSDistServerOutOfSyncEvent{},
				Timestamp: time.Now(),
				ID:        events.ID(domainEvents.EventTypeDNSDISTServerOutOfSync, s.name),
			})
			return fmt.Errorf("%s failed to remove configured spoofs: %w", s.name, err)
		}
	}

	for spoof := range gslbSpoofs.Filter(func(spoof spoofs.Spoof) bool { return spoof.View == s.selector.View() }) { // add all spoofs that does not exist but should
		err = s.client.Rules().Add(
			rules.QNameRule(spoof.FQDN),
			rules.SpoofAction(spoof.Address.Strings(), rules.SpoofActionOptions{TTL: new(30)}),
			rules.GlobalRuleOptions{
				Name: &spoof.Name,
				UUID: &spoof.UUID,
			},
		)
		if err != nil {
			events.Emit(&events.Event{
				Type:      domainEvents.EventTypeDNSDISTServerOutOfSync,
				Payload:   domainEvents.DNSDistServerOutOfSyncEvent{},
				Timestamp: time.Now(),
				ID:        events.ID(domainEvents.EventTypeDNSDISTServerOutOfSync, s.name),
			})
			return fmt.Errorf("failed to add spoofs: %w", err)
		}
	}

	if err := finish(); err != nil {
		return fmt.Errorf("failed to fetch spoof: %w", err)
	}

	return nil
}

func ParseRuleSet(ruleSet string) ([]string, error) {
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
