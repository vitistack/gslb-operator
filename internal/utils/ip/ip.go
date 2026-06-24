package ip

import (
	"encoding/json"
	"fmt"
	"net/netip"
	"strings"
)

type Address interface {
	fmt.Stringer
	Strings() []string
	IPFamily() IPFamily

	PrimaryTCPAddr(port string) string
	TCPAddrs(port string) []string
}

type IPFamily int

const (
	SingleStack IPFamily = iota // IPv4
	DualStack                   // IPv4 and IPv6
)

func (f *IPFamily) UnmarshalJSON(b []byte) error {
	var s string

	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}

	switch s {
	case "SingleStack", "singlestack", "ss":
		*f = SingleStack
	case "DualStack", "dualstack", "ds":
		*f = DualStack
	default:
		return fmt.Errorf("unknown ip-family: %s", s)
	}

	return nil
}

func (f *IPFamily) MarshalJSON() ([]byte, error) {
	switch *f {
	case SingleStack:
		return json.Marshal("SingleStack")
	case DualStack:
		return json.Marshal("DualStack")
	default:
		return json.Marshal("SingleStack")
	}
}

func ParseAddressJSON(b []byte) (Address, error) {
	if len(b) == 0 {
		return nil, nil
	}

	var probe struct {
		Family *IPFamily `json:"ipFamily"`
	}
	if err := json.Unmarshal(b, &probe); err != nil {
		return nil, err
	}
	if probe.Family == nil {
		return nil, nil
	}

	switch *probe.Family {
	case SingleStack:
		var a SingleStackAddr
		if err := json.Unmarshal(b, &a); err != nil {
			return nil, err
		}
		return &a, nil

	case DualStack:
		var a DualStackAddr
		if err := json.Unmarshal(b, &a); err != nil {
			return nil, err
		}
		return &a, nil

	default:
		return nil, fmt.Errorf("unknown ipFamily: %d", *probe.Family)
	}
}

// comma seperated list of addresses
func FromString(addressesStr string) (Address, error) {
	addresses := strings.Split(addressesStr, ",")

	if len(addresses) > 1 {
		ip, err := netip.ParseAddr(addresses[0])
		if err != nil {
			return nil, fmt.Errorf("unable to parse address: %w", err)
		}
		return &SingleStackAddr{Family: SingleStack, IPv4: &ip}, nil
	}

	dualstack := &DualStackAddr{Family: DualStack}
	for ipStr := range strings.SplitSeq(addressesStr, ",") {
		ip, err := netip.ParseAddr(ipStr)
		if err != nil {
			return nil, fmt.Errorf("unable to parse address: %w", err)
		}

		if ip.Is4() {
			dualstack.IPv4 = &ip
		} else if ip.Is6() {
			dualstack.IPv6 = &ip
		}
	}

	return dualstack, nil
}
