package service

import (
	"time"

	"github.com/vitistack/gslb-operator/internal/utils/ip"
)

// GSLBServiceStatus aggregates a best effort current status for a gslb-service
// between the different sites that the GSLB-operator is running from
type GSLBServiceStatus struct {
	MemberOf string                  `json:"memberOf"`
	Sites    []SiteGSLBServiceStatus `json:"sites"`
}

type SiteGSLBServiceStatus struct {
	Service string `json:"service"`
	Site    string `json:"site"`
	Healthy bool   `json:"healthy"` // TODO: should this be by view?
	LocalGSLBServiceStatus
}

type LocalGSLBServiceStatus struct {
	Members       []ShortGSLBServiceMemberStatus `json:"members"`
	DNSDISTStatus DNSDISTStatusForService        `json:"dnsdistStatus"`
	LastSeen      time.Time                      `json:"lastSeen"`
}

type DNSDISTStatusForService struct {
	Programmed bool                            `json:"programmed"`
	Resolvers  []DNSDISTServerStatusForService `json:"resolvers"`
}

type DNSDISTServerStatusForService struct {
	Host    string     `json:"host"`
	View    string     `json:"view"`
	Address ip.Address `json:"resolvesTo"`
}

type ShortGSLBServiceMemberStatus struct {
	ID      string `json:"id"`
	Site    string `json:"site"`
	Healthy bool   `json:"healthy"`
}
