package dnsdist

import (
	"fmt"

	"github.com/vitistack/gslb-operator/internal/dns/update"
	dnsviews "github.com/vitistack/gslb-operator/internal/dns/views"
	"github.com/vitistack/gslb-operator/pkg/clients/dnsdist"
	"github.com/vitistack/gslb-operator/pkg/clients/dnsdist/rules"
	"github.com/vitistack/gslb-operator/pkg/models/spoofs"
)

type server struct {
	name     string
	client   dnsdist.Client
	selector dnsviews.Selector
}

func (s *server) Create(records ...update.Record) error {
	for _, rec := range records {
		if !s.selector.Select(rec.View) {
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
	selectView := false
	if len(views) == 0 {
		selectView = true
	}

	for _, view := range views {
		if s.selector.Select(view) {
			selectView = true
		}
	}

	if !selectView {
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

func (s *server) Reconcile()
