package ip

import (
	"encoding/json"
	"fmt"
	"net/netip"
)

type SingleStackAddr struct {
	Family IPFamily    `json:"ipFamily"`
	IPv4   *netip.Addr `json:"ipv4"`
}

func (a *SingleStackAddr) IPFamily() IPFamily {
	return SingleStack
}

func (a *SingleStackAddr) String() string {
	return a.IPv4.String()
}

func (a *SingleStackAddr) Strings() []string {
	return []string{
		a.IPv4.String(),
	}
}

func (a *SingleStackAddr) PrimaryTCPAddr(port string) string {
	return a.IPv4.String() + port
}

func (a *SingleStackAddr) TCPAddrs(port string) []string {
	return []string{
		a.IPv4.String() + port,
	}
}

func (a *SingleStackAddr) UnmarshalJSON(b []byte) error {
	type Alias SingleStackAddr
	aux := &struct {
		*Alias
	}{
		Alias: (*Alias)(a),
	}
	if err := json.Unmarshal(b, aux); err != nil {
		return err
	}

	if a.IPv4 == nil {
		return fmt.Errorf("empty ipv4 address")
	}

	if !a.IPv4.Is4() {
		return fmt.Errorf("invalid ipv4 address: %v", a.IPv4)
	}

	return nil
}
