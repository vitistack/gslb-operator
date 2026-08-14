package rules

import (
	"fmt"
	"strings"
)

// optional flags for SpoofAction
type SpoofActionOptions struct {
	AA  *bool
	AD  *bool
	RA  *bool
	TTL *int
}

type spoofAction struct {
	ips  []string
	opts *SpoofActionOptions
}

// SpoofAction spoofs replies with one or more IP addresses.
// A single IP is emitted as a string; multiple IPs are emitted as a Lua table.
func SpoofAction(ips []string, opts ...SpoofActionOptions) Action {
	action := spoofAction{ips: ips}
	if len(opts) > 0 {
		action.opts = &opts[0]
	}
	return action
}

func (a spoofAction) luaAction() string {
	var ipArg string
	if len(a.ips) == 1 {
		ipArg = fmt.Sprintf("'%s'", a.ips[0])
	} else {
		quoted := make([]string, len(a.ips))
		for i, ip := range a.ips {
			quoted[i] = fmt.Sprintf("'%s'", ip)
		}
		ipArg = "{" + strings.Join(quoted, ", ") + "}"
	}

	optStr := a.opts.luaTable()
	if optStr != "" {
		return fmt.Sprintf("SpoofAction(%s, %s)", ipArg, optStr)
	}
	return fmt.Sprintf("SpoofAction(%s)", ipArg)
}

func (o *SpoofActionOptions) luaTable() string {
	if o == nil {
		return ""
	}
	var parts []string
	if o.AA != nil {
		parts = append(parts, fmt.Sprintf("aa=%v", *o.AA))
	}
	if o.AD != nil {
		parts = append(parts, fmt.Sprintf("ad=%v", *o.AD))
	}
	if o.RA != nil {
		parts = append(parts, fmt.Sprintf("ra=%v", *o.RA))
	}
	if o.TTL != nil {
		parts = append(parts, fmt.Sprintf("ttl=%d", *o.TTL))
	}
	if len(parts) == 0 {
		return ""
	}
	return "{" + strings.Join(parts, ", ") + "}"
}
