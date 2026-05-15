package events

import (
	"encoding/json"

	"github.com/vitistack/gslb-operator/pkg/events"
)

func init() {
	events.Register(EventTypeDNSDIST, func() events.FilterOption {
		return &DNSDistEventOptions{}
	})
	events.Register(EventTypeDNSDISTSpoof, func() events.FilterOption {
		return &DNSDistSpoofEventOptions{}
	})
	events.Register(EventTypeDNSDISTSpoofCreate, func() events.FilterOption {
		return &DNSDistSpoofCreateEventOptions{}
	})
	events.Register(EventTypeDNSDISTSpoofDelete, func() events.FilterOption {
		return &DNSDistSpoofDeleteEventOptions{}
	})
	events.Register(EventTypeDNSDISTSync, func() events.FilterOption {
		return &DNSDistSynchEventOptions{}
	})
	events.Register(EventTypeDNSDISTSyncFailed, func() events.FilterOption {
		return &DNSDistSynchFailedEventOptions{}
	})
	events.Register(EventTypeDNSDISTSpoofCreateFailed, func() events.FilterOption {
		return &DNSDistSpoofCreateFailedEventOptions{}
	})
	events.Register(EventTypeDNSDISTServer, func() events.FilterOption {
		return &DNSDistServerEventOptions{}
	})
	events.Register(EventTypeDNSDISTServerOutOfSync, func() events.FilterOption {
		return &DNSDistServerOutOfSyncEventOptions{}
	})
}

type DNSDistWebHookOptions struct {
}

func (d *DNSDistWebHookOptions) matches() bool {
	return true
}

type DNSDistEventOptions struct {
	DNSDistWebHookOptions
}

func (d *DNSDistEventOptions) Filter() events.EventFilter {
	rawSelf, _ := json.Marshal(d)
	return func(e *events.Event) bool {
		child, err := events.ResolveOptions(e.Type, rawSelf)
		if err != nil {
			return true
		}

		return child.Filter()(e)
	}
}

type DNSDistSpoofEventOptions struct {
	DNSDistWebHookOptions
}

func (d *DNSDistSpoofEventOptions) Filter() events.EventFilter {
	rawSelf, _ := json.Marshal(d)
	return func(e *events.Event) bool {
		child, err := events.ResolveOptions(e.Type, rawSelf)
		if err != nil {
			return true
		}

		return child.Filter()(e)
	}
}

type DNSDistSpoofCreateEventOptions struct {
	DNSDistWebHookOptions
}

func (d *DNSDistSpoofCreateEventOptions) Filter() events.EventFilter {
	return func(e *events.Event) bool {
		return d.matches()
	}
}

type DNSDistSpoofDeleteEventOptions struct {
	DNSDistWebHookOptions
}

func (d *DNSDistSpoofDeleteEventOptions) Filter() events.EventFilter {
	return func(e *events.Event) bool {
		return d.matches()
	}
}

type DNSDistSynchEventOptions struct {
	DNSDistWebHookOptions
}

func (d *DNSDistSynchEventOptions) Filter() events.EventFilter {
	rawSelf, _ := json.Marshal(d)
	return func(e *events.Event) bool {
		child, err := events.ResolveOptions(e.Type, rawSelf)
		if err != nil {
			return true
		}

		return child.Filter()(e)
	}
}

type DNSDistSynchFailedEventOptions struct {
	DNSDistWebHookOptions
}

func (d *DNSDistSynchFailedEventOptions) Filter() events.EventFilter {
	return func(e *events.Event) bool {
		return d.matches()
	}
}

type DNSDistSpoofCreateFailedEventOptions struct {
	DNSDistWebHookOptions
}

func (d *DNSDistSpoofCreateFailedEventOptions) Filter() events.EventFilter {
	return func(e *events.Event) bool {
		return d.matches()
	}
}

type DNSDistServerEventOptions struct {
	DNSDistWebHookOptions
}

func (d *DNSDistServerEventOptions) Filter() events.EventFilter {
	rawSelf, _ := json.Marshal(d)
	return func(e *events.Event) bool {
		child, err := events.ResolveOptions(e.Type, rawSelf)
		if err != nil {
			return true
		}
		return child.Filter()(e)
	}
}

type DNSDistServerOutOfSyncEventOptions struct {
	DNSDistWebHookOptions
}

func (d *DNSDistServerOutOfSyncEventOptions) Filter() events.EventFilter {
	return func(e *events.Event) bool {
		return d.matches()
	}
}
