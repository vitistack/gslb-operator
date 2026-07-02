package dnsviews

import (
	"slices"

	"github.com/vitistack/gslb-operator/internal/config"
)

// selector decides wether a record/spoof tagged with a view
// should be applied to a specific dnsdist-servers' view or not.
type Selector interface {
	Select(string) bool
}

type AllSelector struct{}

func (*AllSelector) Select(string) bool { return true }

type SplitDNSSelector struct {
	View string
}

func (s *SplitDNSSelector) Select(view string) bool {
	if s.View == "" || view == "" {
		return true
	}
	return s.View == view
}

func Valid(view string) bool {
	if !config.SplitDNS().Enable() {
		return true
	}
	return slices.Contains(config.SplitDNS().DNSViews(), view)
}
