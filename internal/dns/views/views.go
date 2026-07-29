package dnsviews

import (
	"slices"

	"github.com/vitistack/gslb-operator/internal/config"
)

// selector decides wether a record/spoof tagged with a view
// should be applied to a specific dnsdist-servers' view or not.
type Selector interface {
	Select(string) bool
	View() string
}

type AllSelector struct{}

func (*AllSelector) Select(string) bool { return true }
func (*AllSelector) View() string       { return "" }

type SplitDNSSelector struct {
	view string
}

func NewSplitDNSSelector(view string) *SplitDNSSelector {
	return &SplitDNSSelector{view: view}
}

func (s *SplitDNSSelector) Select(view string) bool {
	if s.view == "" || view == "" {
		return true
	}
	return s.view == view
}

func (s *SplitDNSSelector) View() string {
	return s.view
}

func Valid(view string) bool {
	if !config.SplitDNS().Enable() {
		return true
	}
	return slices.Contains(config.SplitDNS().DNSViews(), view)
}
