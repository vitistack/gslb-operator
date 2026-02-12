package model

import (
	"github.com/vitistack/gslb-operator/pkg/models/spoofs"
)

// storage representation of service
// services that are configured with gslb config end up as a service.Service
type Service struct {
	ID           string `json:"id"`
	MemberOf     string `json:"memberOf"`
	Fqdn         string `json:"fqdn"`
	Datacenter   string `json:"datacenter"`
	IP           string `json:"ip"`
	IsHealthy    bool   `json:"isHealthy"`
	FailureCount int    `json:"failureCount"`
}

func (s Service) Key() string {
	return s.MemberOf + ":" + s.Datacenter
}

func (s Service) Spoof() spoofs.Spoof {
	return spoofs.Spoof{
		FQDN: s.Fqdn,
		IP:   s.IP,
		DC:   s.Datacenter,
	}
}
