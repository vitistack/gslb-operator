package model

import (
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/vitistack/gslb-operator/internal/config"
	"github.com/vitistack/gslb-operator/internal/utils/ip"
	"github.com/vitistack/gslb-operator/pkg/models/service"
	"github.com/vitistack/gslb-operator/pkg/models/spoofs"
)

type GSLBServiceGroup struct {
	// when active override this is the overridden IP
	// key: view, val: id/address
	Active      map[string]string `json:"active"`
	HasOverride bool              `json:"hasOverride"`

	// common views for all members
	Views   []string               `json:"views"`
	UUID    uuid.UUID              `json:"uuid"`
	Members map[string]GSLBService `json:"members"`
}

func (g GSLBServiceGroup) Spoof(views ...string) *spoofs.Spoof {
	if len(g.Active) == 0 {
		// no spoof exists for this group as there are no healthy members
		// or overrides
		return nil
	}

	var view string
	if len(views) == 0 {
		view = config.DNS().DefaultView()
	} else {
		view = views[0]
	}

	// get member-of since it is unknown for the group
	var memberOf string
	for _, member := range g.Members {
		memberOf = member.MemberOf
		break
	}

	spoof := spoofs.Spoof{
		Name: memberOf,
		UUID: g.UUID.String(),
		FQDN: memberOf,
		View: view,
	}

	active, ok := g.Members[g.Active[view]]
	if ok {
		spoof.Address = active.Address
	} else if g.HasOverride {
		addr, err := ip.FromString(g.Active[view])
		if err != nil {
			spoof.Address = nil
		}
		spoof.Address = addr
	}

	return &spoof
}

// storage representation of service
// services that are configured with gslb config end up as a service.Service
type GSLBService struct {
	ID           string     `json:"id"`
	MemberOf     string     `json:"memberOf"`
	Fqdn         string     `json:"fqdn"`
	Port         string     `json:"port"`
	Datacenter   string     `json:"datacenter"`
	Views        []string   `json:"views"`
	Address      ip.Address `json:"address"`
	IsHealthy    bool       `json:"isHealthy"`
	FailureCount int        `json:"failureCount"`
}

func (g *GSLBService) GSLBServiceMemberStatus() service.ShortGSLBServiceMemberStatus {
	return service.ShortGSLBServiceMemberStatus{
		ID:      g.ID,
		Site:    g.Datacenter,
		Healthy: g.IsHealthy,
	}
}

func (s *GSLBService) UnmarshalJSON(b []byte) error {
	type Alias GSLBService
	aux := struct {
		Address json.RawMessage `json:"address"`
		*Alias
	}{
		Alias: (*Alias)(s),
	}

	if err := json.Unmarshal(b, &aux); err != nil {
		return err
	}

	addr, err := ip.ParseAddressJSON(aux.Address)
	if err != nil {
		return fmt.Errorf("address: %w", err)
	}
	s.Address = addr

	return nil
}
