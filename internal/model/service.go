package model

import (
	"github.com/vitistack/gslb-operator/pkg/models/spoofs"
)

type GSLBServiceGroup struct {
	Active      string
	HasOverride bool
	Members     map[string]GSLBService
}

// storage representation of service
// services that are configured with gslb config end up as a service.Service
type GSLBService struct {
	ID           string `json:"id"`
	MemberOf     string `json:"memberOf"`
	Fqdn         string `json:"fqdn"`
	Datacenter   string `json:"datacenter"`
	IP           string `json:"ip"`
	IsHealthy    bool   `json:"isHealthy"`
	FailureCount int    `json:"failureCount"`
}

func (s GSLBService) Key() string {
	return s.MemberOf
}

// returns spoof representation of GSLBService
func (s GSLBService) Spoof() spoofs.Spoof {
	return spoofs.Spoof{
		FQDN: s.MemberOf,
		IP:   s.IP,
		DC:   s.Datacenter,
	}
}
