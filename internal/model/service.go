package model

import (
	"net"
	"net/netip"

	"github.com/google/uuid"
	"github.com/vitistack/gslb-operator/pkg/models/spoofs"
)

type GSLBServiceGroup struct {
	Active      string // when active override this is the overridden IP
	HasOverride bool
	UUID        uuid.UUID
	Members     map[string]GSLBService
}

func (g GSLBServiceGroup) Spoof() *spoofs.Spoof {
	if g.Active == "" {
		// no spoof exists for this group as there are no healthy members
		// or overrides
		return nil
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
	}

	active, ok := g.Members[g.Active]
	if ok {
		spoof.IP = active.IP.String()
	} else if g.HasOverride {
		spoof.IP = net.ParseIP(g.Active).String()
	}

	return &spoof
}

// storage representation of service
// services that are configured with gslb config end up as a service.Service
type GSLBService struct {
	ID           string     `json:"id"`
	MemberOf     string     `json:"memberOf"`
	Fqdn         string     `json:"fqdn"`
	Datacenter   string     `json:"datacenter"`
	IP           netip.Addr `json:"ip"`
	IsHealthy    bool       `json:"isHealthy"`
	FailureCount int        `json:"failureCount"`
}

func (s GSLBService) Key() string {
	return s.MemberOf
}
