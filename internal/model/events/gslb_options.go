package events

import (
	"encoding/json"
	"slices"

	"github.com/vitistack/gslb-operator/pkg/events"
)

func init() {
	// registering each event type to resolve its event options on the fly when filtering events for emit
	events.Register(EventTypeGSLB, func() events.FilterOption {
		return &GSLBEventOptions{}
	})
	events.Register(EventTypeGSLBService, func() events.FilterOption {
		return &GSLBServiceEventOptions{}
	})
	events.Register(EventTypeGSLBServiceUp, func() events.FilterOption {
		return &GSLBServiceUpEventOptions{}
	})
	events.Register(EventTypeGSLBServiceDown, func() events.FilterOption {
		return &GSLBServiceDownEventOptions{}
	})
	events.Register(EventTypeGSLBServiceFailover, func() events.FilterOption {
		return &GSLBServiceFailoverEventOptions{}
	})
	events.Register(EventTypeGSLBServiceMember, func() events.FilterOption {
		return &GSLBServiceMemberEventOptions{}
	})
	events.Register(EventTypeGSLBServiceMemberHealthChange, func() events.FilterOption {
		return &GSLBServiceMemberHealthChangeEventOptions{}
	})
	events.Register(EventTypeGSLBServiceMemberAdd, func() events.FilterOption {
		return &GSLBServiceMemberAddEventOptions{}
	})
	events.Register(EventTypeGSLBServiceMemberRemove, func() events.FilterOption {
		return &GSLBServiceMemberRemoveEventOptions{}
	})
	events.Register(EventTypeGSLBConfig, func() events.FilterOption {
		return &GSLBConfigEventOptions{}
	})
	events.Register(EventTypeGSLBConfigCreate, func() events.FilterOption {
		return &GSLBConfigCreateEventOptions{}
	})
	events.Register(EventTypeGSLBConfigUpdate, func() events.FilterOption {
		return &GSLBConfigUpdateEventOptions{}
	})
	events.Register(EventTypeGSLBConfigDelete, func() events.FilterOption {
		return &GSLBConfigDeleteEventOptions{}
	})
}

type GSLBWebhookOptions struct {
	MemberOfs []string `json:"memberOfs"`
}

func (g *GSLBWebhookOptions) matches(memberOfs ...string) bool {
	if len(g.MemberOfs) > 0 {
		for _, memberOf := range memberOfs {
			if slices.Contains(g.MemberOfs, memberOf) {
				return true
			}
		}
		return false
	}
	return true
}

type GSLBEventOptions struct {
	GSLBWebhookOptions
}

func (g *GSLBEventOptions) Filter() events.EventFilter {
	rawSelf, _ := json.Marshal(g)
	return func(e *events.Event) bool {
		child, err := events.ResolveOptions(e.Type, rawSelf)
		if err != nil {
			return true
		}

		return child.Filter()(e)
	}
}

type GSLBServiceEventOptions struct {
	GSLBWebhookOptions
}

func (g *GSLBServiceEventOptions) Filter() events.EventFilter {
	rawSelf, _ := json.Marshal(g)
	return func(e *events.Event) bool {
		child, err := events.ResolveOptions(e.Type, rawSelf)
		if err != nil {
			return true
		}

		return child.Filter()(e)
	}
}

type GSLBServiceUpEventOptions struct {
	GSLBWebhookOptions
}

func (g *GSLBServiceUpEventOptions) Filter() events.EventFilter {
	return func(e *events.Event) bool {
		body, ok := e.Payload.(GSLBServiceUpEvent)
		if !ok {
			return false
		}

		return g.matches(body.NewActive.MemberOf)
	}
}

type GSLBServiceDownEventOptions struct {
	GSLBWebhookOptions
}

func (g *GSLBServiceDownEventOptions) Filter() events.EventFilter {
	return func(e *events.Event) bool {
		body, ok := e.Payload.(GSLBServiceDownEvent)
		if !ok {
			return false
		}

		return g.matches(body.MemberOf)
	}
}

type GSLBServiceFailoverEventOptions struct {
	GSLBWebhookOptions
}

func (g *GSLBServiceFailoverEventOptions) Filter() events.EventFilter {
	return func(e *events.Event) bool {
		body, ok := e.Payload.(GSLBServiceFailoverEvent)
		if !ok {
			return false
		}
		return g.matches(body.MemberOf)
	}
}

type GSLBServiceMemberEventOptions struct {
	GSLBWebhookOptions
}

func (g *GSLBServiceMemberEventOptions) Filter() events.EventFilter {
	rawSelf, _ := json.Marshal(g)
	return func(e *events.Event) bool {
		child, err := events.ResolveOptions(e.Type, rawSelf)
		if err != nil {
			return true
		}

		return child.Filter()(e)
	}
}

type GSLBServiceMemberHealthChangeEventOptions struct {
	GSLBWebhookOptions
}

func (g *GSLBServiceMemberHealthChangeEventOptions) Filter() events.EventFilter {
	return func(e *events.Event) bool {
		body, ok := e.Payload.(GSLBServiceMemberHealthChangeEvent)
		if !ok {
			return false
		}
		return g.matches(body.Member.MemberOf)
	}
}

type GSLBServiceMemberAddEventOptions struct {
	GSLBWebhookOptions
}

func (g *GSLBServiceMemberAddEventOptions) Filter() events.EventFilter {
	return func(e *events.Event) bool {
		body, ok := e.Payload.(GSLBServiceMemberAddEvent)
		if !ok {
			return false
		}
		return g.matches(body.Service, body.NewMember.MemberOf)
	}
}

type GSLBServiceMemberRemoveEventOptions struct {
	GSLBWebhookOptions
}

func (g *GSLBServiceMemberRemoveEventOptions) Filter() events.EventFilter {
	return func(e *events.Event) bool {
		body, ok := e.Payload.(GSLBServiceMemberRemoveEvent)
		if !ok {
			return false
		}
		return g.matches(body.Service, body.Removed.MemberOf)
	}
}

type GSLBConfigEventOptions struct {
	GSLBWebhookOptions
}

func (g *GSLBConfigEventOptions) Filter() events.EventFilter {
	rawSelf, _ := json.Marshal(g)
	return func(e *events.Event) bool {
		child, err := events.ResolveOptions(e.Type, rawSelf)
		if err != nil {
			return true
		}

		return child.Filter()(e)
	}
}

type GSLBConfigCreateEventOptions struct {
	GSLBWebhookOptions
}

func (g *GSLBConfigCreateEventOptions) Filter() events.EventFilter {
	return func(e *events.Event) bool {
		body, ok := e.Payload.(GSLBConfigCreateEvent)
		if !ok {
			return false
		}

		return g.matches(body.Config.MemberOf)
	}
}

type GSLBConfigUpdateEventOptions struct {
	GSLBWebhookOptions
}

func (g *GSLBConfigUpdateEventOptions) Filter() events.EventFilter {
	return func(e *events.Event) bool {
		body, ok := e.Payload.(GSLBConfigUpdateEvent)
		if !ok {
			return false
		}

		return g.matches(body.LastConfig.MemberOf, body.CurrentConfig.MemberOf)
	}
}

type GSLBConfigDeleteEventOptions struct {
	GSLBWebhookOptions
}

func (g *GSLBConfigDeleteEventOptions) Filter() events.EventFilter {
	return func(e *events.Event) bool {
		body, ok := e.Payload.(GSLBConfigDeleteEvent)
		if !ok {
			return false
		}

		return g.matches(body.LastConfig.MemberOf)
	}
}
