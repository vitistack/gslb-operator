package events

import (
	"github.com/vitistack/gslb-operator/pkg/events"
)

const (
	EventTypeGSLB                          events.EventType = "gslb"
	EventTypeGSLBService                   events.EventType = "gslb:service"
	EventTypeGSLBServiceUp                 events.EventType = "gslb:service:up"
	EventTypeGSLBServiceDown               events.EventType = "gslb:service:down"
	EventTypeGSLBServiceFailover           events.EventType = "gslb:service:failover"
	EventTypeGSLBServiceMember             events.EventType = "gslb:service:member"
	EventTypeGSLBServiceMemberHealthChange events.EventType = "gslb:service:member:healthchange"
	EventTypeGSLBServiceMemberAdd          events.EventType = "gslb:service:member:add"
	EventTypeGSLBServiceMemberRemove       events.EventType = "gslb:service:member:remove"
	EventTypeGSLBConfig                    events.EventType = "gslb:config"
	EventTypeGSLBConfigCreate              events.EventType = "gslb:config:create"
	EventTypeGSLBConfigUpdate              events.EventType = "gslb:config:update"
	EventTypeGSLBConfigDelete              events.EventType = "gslb:config:delete"

	EventTypeDNSDIST            events.EventType = "dnsdist"
	EventTypeDNSDISTSpoof       events.EventType = "dnsdist:spoof"
	EventTypeDNSDISTSpoofCreate events.EventType = "dnsdist:spoof:create"
	EventTypeDNSDISTSpoofDelete events.EventType = "dnsdist:spoof:delete"
)
