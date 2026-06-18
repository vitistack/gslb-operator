package spoofs

import (
	"net/netip"
)

type Override struct {
	MemberOf string     `json:"memberOf"`
	IP       netip.Addr `json:"ip"`
}
