package spoofs

import "net"

type Override struct {
	MemberOf string `json:"memberOf"`
	IP       net.IP `json:"ip,omitempty"`
}
