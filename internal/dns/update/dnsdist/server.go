package dnsdist

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net/netip"
	"regexp"
	"slices"
	"strings"
	"sync"
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

var dnsdistRuleLineRegexPattern = regexp.MustCompile(`\S+`)

var dnsdistRuleUUIDRegexPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// matches candidate IPv4/IPv6 tokens in a rule action, validated below with netip
var dnsdistActionAddressRegexPattern = regexp.MustCompile(`[0-9a-fA-F.:]+`)

type server struct {
	name     string
	client   dnsdist.Client
	selector dnsviews.Selector
	lock     sync.RWMutex
}

func (s *server) Create(records ...update.Record) error {
	s.lock.Lock()
	defer s.lock.Unlock()
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
				rules.SpoofActionOptions{TTL: new(30), AA: new(true)},
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
	s.lock.Lock()
	defer s.lock.Unlock()

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
	s.lock.RLock()
	defer s.lock.RUnlock()

	rawRuleSet, err := s.client.Rules().List(&rules.ListOptions{ShowUUIDs: new(true)})
	if err != nil {
		return "", fmt.Errorf("failed to list rules: %w", err)
	}

	spoofEntries := make([]string, 0)
	IterateRuleSet(rawRuleSet).
		Filter(
			func(rl rules.RuleLine) bool {
				return strings.Contains(rl.Action, "spoof")
			},
		).
		Each(func(rl rules.RuleLine) {
			spoofEntries = append(spoofEntries, rl.UUID+":"+spoofActionAddress(rl.Action))
		})

	slices.Sort(spoofEntries)

	joinedUUIDs := strings.Join(spoofEntries, ",")
	rawHash := sha256.Sum256([]byte(joinedUUIDs))
	return hex.EncodeToString(rawHash[:]), nil
}

func (s *server) Reconcile(gslbSpoofs iter.Iterator[spoofs.Spoof], finish func() error) (err error) {
	s.lock.Lock()
	defer s.lock.Unlock()

	defer func() {
		if err != nil { // reads named returned err value
			events.Emit(&events.Event{
				Type:      domainEvents.EventTypeDNSDISTServerOutOfSync,
				Payload:   domainEvents.DNSDistServerOutOfSyncEvent{},
				Timestamp: time.Now(),
				ID:        events.ID(domainEvents.EventTypeDNSDISTServerOutOfSync, s.name),
			})
		}
	}()

	rawRuleSet, err := s.client.Rules().List(&rules.ListOptions{ShowUUIDs: new(true)})
	if err != nil {
		return fmt.Errorf("failed to list rules: %w", err)
	}

	for rule := range IterateRuleSet(rawRuleSet).
		Filter(func(rl rules.RuleLine) bool { return strings.Contains(rl.Action, "spoof") }) {
		err := s.client.Rules().Remove(rule.UUID)
		if err != nil {
			return fmt.Errorf("%s failed to remove configured spoofs: %w", s.name, err)
		}
	}

	for spoof := range gslbSpoofs.Filter(func(spoof spoofs.Spoof) bool { return spoof.View == s.selector.View() }) { // add all spoofs that does not exist but should
		err = s.client.Rules().Add(
			rules.QNameRule(spoof.FQDN),
			rules.SpoofAction(spoof.Address.Strings(), rules.SpoofActionOptions{TTL: new(30), AA: new(true)}),
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

//func (s *server) BulkStatus() (map[uuid.UUID]service.DNSServerStatusForService, error) {
//	s.lock.RLock()
//	defer s.lock.RUnlock()
//
//	baseStatus := service.DNSServerStatusForService{
//		Programmed: true,
//		Host:       s.name,
//		View:       s.selector.View(),
//		Address:    nil,
//	}
//
//	statuses := make(map[uuid.UUID]service.DNSServerStatusForService)
//
//	rawRuleSet, err := s.client.Rules().List(&rules.ListOptions{ShowUUIDs: new(true)})
//	if err != nil {
//		return nil, fmt.Errorf("%s in view %s: failed to fetch rule-set: %w", s.name, s.selector.View(), err)
//	}
//
//	IterateRuleSet(rawRuleSet).
//		Filter(func(rl rules.RuleLine) bool { return strings.Contains(rl.Action, "spoof") }).
//		Each(
//			func(rl rules.RuleLine) {
//				rawAddress := strings.TrimPrefix(rl.Action, "spoof in ")
//				status := baseStatus
//				status.Address, err = ip.FromString(rawAddress)
//				if err != nil {
//					bslog.Error(
//						"failed to generate status",
//						slog.Group("server",
//							slog.String("name", s.name),
//							slog.String("view", s.selector.View()),
//						),
//						slog.String("reason", fmt.Errorf("failed to parse spoof-address from RuleAction: %w", err).Error()),
//					)
//					return
//				}
//
//				id, err := uuid.Parse(rl.UUID)
//				if err != nil {
//					bslog.Error(
//						"failed to generate status",
//						slog.Group("server",
//							slog.String("name", s.name),
//							slog.String("view", s.selector.View()),
//						),
//						slog.String("reason", fmt.Errorf("failed to parse rule-line uuid: %w", err).Error()),
//					)
//					return
//				}
//
//				statuses[id] = status
//			},
//		)
//
//	return statuses, nil
//}

func (s *server) Status(uuid string) (service.DNSServerStatusForService, error) {
	s.lock.RLock()
	defer s.lock.RUnlock()

	status := service.DNSServerStatusForService{
		Programmed: true, // assume true
		Host:       s.name,
		View:       s.selector.View(),
		Address:    nil,
	}

	rawRuleSet, err := s.client.Rules().List(&rules.ListOptions{ShowUUIDs: new(true)})
	if err != nil {
		return service.DNSServerStatusForService{}, fmt.Errorf("failed to fetch rules: %w", err)
	}

	rule, ok := IterateRuleSet(rawRuleSet).Find(func(rl rules.RuleLine) bool { return rl.UUID == uuid })
	if !ok {
		return status, nil // no status for uuid
	}

	status.Address, err = ip.FromString(spoofActionAddress(rule.Action))
	if err != nil {
		return status, fmt.Errorf("failed to parse ip address: %w", err)
	}

	return status, nil
}

func MapStringToDNSDistRule(ruleLine string) rules.RuleLine {
	matches := dnsdistRuleLineRegexPattern.FindAllString(ruleLine, -1)
	if len(matches) == 0 {
		return rules.RuleLine{}
	}

	rule := rules.RuleLine{ID: matches[0]}

	uuidPos := slices.IndexFunc(matches, func(s string) bool { return dnsdistRuleUUIDRegexPattern.MatchString(s) })
	if uuidPos == -1 || uuidPos+2 >= len(matches) {
		return rule
	}

	rule.Name = strings.Join(matches[1:uuidPos], " ")
	rule.UUID = matches[uuidPos]
	rule.Matches = matches[uuidPos+1]
	rule.Rule = matches[uuidPos+2]
	rule.Action = strings.Join(matches[uuidPos+3:], " ")

	return rule
}

func IterateRuleSet(ruleSet string) iter.Iterator[rules.RuleLine] {
	reader := strings.NewReader(ruleSet)
	lines := bufio.NewScanner(reader)

	baseIter := iter.FromSeq(
		func(yield func(string) bool) {
			for lines.Scan() {
				if !yield(lines.Text()) {
					return
				}
			}
		},
	)

	// 1 skip because the first line is always the header line for rules
	return baseIter.Skip(1).Map(MapStringToDNSDistRule)
}

func spoofActionAddress(action string) string {
	addresses := make([]string, 0, 2)
	for _, candidate := range dnsdistActionAddressRegexPattern.FindAllString(action, -1) {
		if _, err := netip.ParseAddr(candidate); err == nil {
			addresses = append(addresses, candidate)
		}
	}
	return strings.Join(addresses, ",")
}
