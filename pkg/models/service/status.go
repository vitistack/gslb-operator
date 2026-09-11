package service

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/vitistack/gslb-operator/internal/utils/ip"
)

type GSLBServiceSiteHealth string

const (
	HEALTHY  GSLBServiceSiteHealth = "HEALTHY"
	DEGRADED GSLBServiceSiteHealth = "DEGRADED"
	DOWN     GSLBServiceSiteHealth = "DOWN"
	UNKNOWN  GSLBServiceSiteHealth = "UNKNOWN"
)

// GSLBServiceStatus aggregates a best effort current status for a gslb-service
// between the different sites that the GSLB-operator is running from
type GSLBServiceStatus struct {
	MemberOf string       `json:"memberOf"`
	Sites    []SiteStatus `json:"sites"`
}

type SiteStatus struct {
	Site   string                `json:"site"`
	Health GSLBServiceSiteHealth `json:"health"` // TODO: should this be by view?
	LocalGSLBServiceStatus
}

type SiteGSLBServiceStatus struct {
	MemberOf string `json:"memberOf"`
	SiteStatus
}

type LocalGSLBServiceStatus struct {
	Members   []ShortGSLBServiceMemberStatus `json:"members"`
	DNSStatus DNSStatusForService            `json:"dnsdistStatus"`
	LastSeen  time.Time                      `json:"lastSeen"`
}

type DNSStatusForService struct {
	Resolvers []DNSServerStatusForService `json:"resolvers"`
}

type DNSServerStatusForService struct {
	Programmed bool       `json:"programmed"`
	Host       string     `json:"host"`
	View       string     `json:"view"`
	Address    ip.Address `json:"resolvesTo"`
}

func (d *DNSServerStatusForService) UnmarshalJSON(b []byte) error {
	type Alias DNSServerStatusForService
	aux := struct {
		Address json.RawMessage `json:"resolvesTo"`
		*Alias
	}{
		Alias: (*Alias)(d),
	}

	if err := json.Unmarshal(b, &aux); err != nil {
		return err
	}

	addr, err := ip.ParseAddressJSON(aux.Address)
	if err != nil {
		return fmt.Errorf("address: %w", err)
	}

	// we allow nil address here because a resolver might not resolve the address
	d.Address = addr

	return nil
}

type ShortGSLBServiceMemberStatus struct {
	ID      string `json:"id"`
	Site    string `json:"site"`
	Healthy bool   `json:"healthy"`
}
