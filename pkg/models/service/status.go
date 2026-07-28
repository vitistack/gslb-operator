package service

import "time"

// GSLBServiceStatus aggregates a best effort current status for a gslb-service
// between the different sites that the GSLB-operator is running from
type GSLBServiceStatus struct {
	MemberOf string `json:"memberOf"`

	// key: gslb-operator instance
	Sites []SiteGSLBServiceStatus `json:"sites"`
}

type SiteGSLBServiceStatus struct {
	Site string `json:"site"`
	LocalGSLBServiceStatus
}

type LocalGSLBServiceStatus struct {
	// TODO: members
	DNSDISTStatus DNSDISTStatusForService `json:"dnsdistStatus"`
	LastSeen      time.Time               `json:"lastSeen"`
}

type DNSDISTStatusForService struct {
	Programmed bool `json:"programmed"`
	/*
		need something more here
	*/
}
