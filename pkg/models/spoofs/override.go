package spoofs

import "net"

type Override struct {
	FQDN string `json:"fqdn"`
	IP   net.IP `json:"ip,omitempty"`
}
