package events

import (
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
	EventTypeDNSDISTSpoofCreate events.EventType = "dnsdist:spoof:create"
	EventTypeDNSDISTSpoofDelete events.EventType = "dnsdist:spoof:delete"
)
