package dnsviews

import (
	"slices"

	"github.com/vitistack/gslb-operator/internal/config"
)

// selector decides wether a record/spoof tagged with a view
// should be applied to a specific dnsdist-servers' view or not.
type Selector interface {
	Select(...string) bool
	View() string
}

type AllSelector struct{}

func (*AllSelector) Select(...string) bool { return true }
func (*AllSelector) View() string          { return config.DNS().DefaultView() }

type SplitDNSSelector struct {
	view string
}

func NewSplitDNSSelector(view string) *SplitDNSSelector {
	return &SplitDNSSelector{view: view}
}

func (s *SplitDNSSelector) Select(views ...string) bool {
	if s.view == "" || len(views) == 0 {
		return true
	}

	return slices.Contains(views, s.view)
}

func (s *SplitDNSSelector) View() string {
	return s.view
}

func Valid(view string) bool {
	if !config.DNS().Enable() {
		return true
	}

	return slices.Contains(config.DNS().DNSViews(), view)
}
