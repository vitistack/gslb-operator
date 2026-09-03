package dnsdist

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/vitistack/gslb-operator/internal/dns/update"
	dnsviews "github.com/vitistack/gslb-operator/internal/dns/views"
	domainEvents "github.com/vitistack/gslb-operator/internal/model/events"
	"github.com/vitistack/gslb-operator/internal/utils/ip"
	"github.com/vitistack/gslb-operator/pkg/bslog"
	"github.com/vitistack/gslb-operator/pkg/clients/dnsdist"
	"github.com/vitistack/gslb-operator/pkg/clients/dnsdist/rules"
	"github.com/vitistack/gslb-operator/pkg/events"
	"github.com/vitistack/gslb-operator/pkg/iter"
	"github.com/vitistack/gslb-operator/pkg/models/service"
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
			continue
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
			err := s.client.Rules().Remove(rec.UUID)
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
		bslog.Debug(
			"successfully created dnsdist spoof record",
			slog.Any("record", rec),
			slog.String("server", s.name),
			slog.String("view", s.selector.View()),
		)
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
	bslog.Debug(
		"successfully deleted dnsdist spoof record",
		slog.String("recordId", id),
		slog.String("server", s.name),
		slog.String("view", s.selector.View()),
	)

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
	var err error
	defer func(err error) {
		if err != nil {
			events.Emit(&events.Event{
				Type:      domainEvents.EventTypeDNSDISTServerOutOfSync,
				Payload:   domainEvents.DNSDistServerOutOfSyncEvent{},
				Timestamp: time.Now(),
				ID:        events.ID(domainEvents.EventTypeDNSDISTServerOutOfSync, s.name),
			})
		}
	}(err)

	rawRuleSet, err := s.client.Rules().List(&rules.ListOptions{ShowUUIDs: new(true)})
	if err != nil {
		return fmt.Errorf("failed to list rules: %w", err)
	}

	pattern, err := regexp.Compile(`\S+`)
	if err != nil {
		return fmt.Errorf("failed to compile pattern: %w", err)
	}

	baseIter := IterateRuleSet(rawRuleSet).Skip(1)
	rulesIter := iter.Map(
		baseIter,
		func(line string) rules.RuleLine {
			return MapStringToDNSDistRule(line, pattern)
		}).
		Filter(func(rl rules.RuleLine) bool { return strings.Contains(rl.Action, "spoof") })

	for rule := range rulesIter {
		err := s.client.Rules().Remove(rule.UUID)
		if err != nil {
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
			return fmt.Errorf("failed to add spoofs: %w", err)
		}
	}

	if err = finish(); err != nil {
		return fmt.Errorf("failed to fetch spoof: %w", err)
	}

	return nil
}

func (s *server) Status(uuid string) (service.DNSDISTServerStatusForService, error) {
	rawRuleSet, err := s.client.Rules().List(&rules.ListOptions{ShowUUIDs: new(true)})
	if err != nil {
		return service.DNSDISTServerStatusForService{}, fmt.Errorf("failed to list rules: %w", err)
	}

	pattern, err := regexp.Compile(`\S+`)
	if err != nil {
		return service.DNSDISTServerStatusForService{}, fmt.Errorf("failed to compile pattern: %w", err)
	}

	status := service.DNSDISTServerStatusForService{
		Host:    s.name,
		View:    s.selector.View(),
		Address: nil,
	}

	line, ok := IterateRuleSet(rawRuleSet).Skip(1).Find(func(s string) bool { return strings.Contains(s, uuid) })
	if !ok {
		return status, nil
	}

	rule := MapStringToDNSDistRule(line, pattern)
	rawAddresses := strings.Trim(rule.Action, "spoof in")

	status.Address, err = ip.FromString(rawAddresses)
	if err != nil {
		return status, fmt.Errorf("failed to parse ip address: %w", err)
	}

	return status, nil
}

func MapStringToDNSDistRule(ruleLine string, pattern *regexp.Regexp) rules.RuleLine {
	matches := pattern.FindAllString(ruleLine, -1)
	if len(matches) > 0 {
		rule := rules.RuleLine{
			ID: matches[0],
		}
		if len(matches) == 8 {
			rule.Name = ""
			rule.UUID = matches[1]
			rule.Matches = matches[3]
			rule.Rule = matches[4]
			rule.Action = strings.Join(matches[5:], " ")
		} else if len(matches) == 9 {
			rule.Name = matches[1]
			rule.UUID = matches[2]
			rule.Matches = matches[4]
			rule.Rule = matches[5]
			rule.Action = strings.Join(matches[6:], " ")
		}
		return rule
	}
	return rules.RuleLine{}
}

func IterateRuleSet(ruleSet string) iter.Iterator[string] {
	reader := strings.NewReader(ruleSet)
	lines := bufio.NewScanner(reader)

	return func(yield func(string) bool) {
		for lines.Scan() {
			if !yield(lines.Text()) {
				return
			}
		}
	}
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
