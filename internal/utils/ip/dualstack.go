package ip

import (
	"encoding/json"
	"fmt"
	"net/netip"
)

type DualStackAddr struct {
	Family IPFamily    `json:"ipFamily"`
	IPv4   *netip.Addr `json:"ipv4,omitempty"`
	IPv6   *netip.Addr `json:"ipv6,omitempty"`
}

func (a *DualStackAddr) String() string {
	return fmt.Sprintf("%s,%s", a.IPv4.String(), a.IPv6.String())
}

func (a *DualStackAddr) Strings() []string {
	return []string{
		a.IPv4.String(),
		a.IPv6.String(),
	}
}

func (a *DualStackAddr) IPFamily() IPFamily {
	return DualStack
}

func (a *DualStackAddr) PrimaryTCPAddr(port string) string {
	return a.IPv4.String() + port
}

func (a *DualStackAddr) TCPAddrs(port string) []string {
	return []string{
		a.IPv4.String() + port,
		a.IPv6.String() + port,
	}
}

func (a *DualStackAddr) UnmarshalJSON(b []byte) error {
	type Alias DualStackAddr
	aux := &struct {
		*Alias
	}{
		Alias: (*Alias)(a),
	}
	if err := json.Unmarshal(b, aux); err != nil {
		return err
	}

	if a.IPv4 == nil && a.IPv6 == nil {
		return fmt.Errorf("needs atleast one ip-address")
	}

	if a.IPv4 != nil && !a.IPv4.Is4() {
		return fmt.Errorf("invalid ipv4 address: %v", a.IPv4)
	}

	if a.IPv6 != nil && !a.IPv6.Is6() {
		return fmt.Errorf("invalid ipv6 address: %v", a.IPv6)
	}

	return nil
}
