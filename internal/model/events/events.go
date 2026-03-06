package events

import (
	"slices"

	"github.com/vitistack/gslb-operator/pkg/events"
)

const (
	EventTypeGSLB                   events.EventType = "gslb"
	EventTypeGSLBFailover           events.EventType = "gslb:failover"
	EventTypeGSLBMember             events.EventType = "gslb:member"
	EventTypeGSLBMemberHealthChange events.EventType = "gslb:member:healthchange"
	EventTypeGSLBService            events.EventType = "gslb:service"
	EventTypeGSLBServiceUp          events.EventType = "gslb:service:up"
	EventTypeGSLBServiceDown        events.EventType = "gslb:service:down"
	EventTypeGSLBConfig             events.EventType = "gslb:config"
	EventTypeGSLBConfigCreate       events.EventType = "gslb:config:create"
	EventTypeGSLBConfigUpdate       events.EventType = "gslb:config:update"
	EventTypeGSLBConfigDelete       events.EventType = "gslb:config:delete"

	EventTypeDNSDIST           events.EventType = "dnsdist"
	EventTypeDNSDISTSpoof      events.EventType = "dnsdist:spoof"
	EvenTypeDNSDISTSpoofCreate events.EventType = "dnsdist:spoof:create"
	EvenTypeDNSDISTSpoofDelete events.EventType = "dnsdist:spoof:delete"
)

func init() {
	events.Register(EventTypeGSLBFailover, func() events.FilterOption {
		return &GSLBFailoverOption{}
	})
}


type GSLBFailoverOption struct {
	MemberOfs []string `json:"memberOfs"`
}

func (o *GSLBFailoverOption) Filter() events.EventFilter {
	return func(e *events.Event) bool {
		body, ok := e.Payload.(GSLBFailoverEvent)
		if !ok {
			return false
		}

		// returns wether this filter applies or not to the event
		return slices.Contains(o.MemberOfs, body.MemberOf)
	}
}
