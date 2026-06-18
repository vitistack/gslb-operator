package events

import (
	"github.com/vitistack/gslb-operator/pkg/events"
)

// GSLB event types
const (
	EventTypeGSLB events.EventType = "gslb"

	EventTypeGSLBService                   events.EventType = "gslb:service"
	EventTypeGSLBServiceUp                 events.EventType = "gslb:service:up"
	EventTypeGSLBServiceDown               events.EventType = "gslb:service:down"
	EventTypeGSLBServiceFailover           events.EventType = "gslb:service:failover"
	EventTypeGSLBServiceMember             events.EventType = "gslb:service:member"
	EventTypeGSLBServiceMemberHealthChange events.EventType = "gslb:service:member:healthchange"
	EventTypeGSLBServiceMemberAdd          events.EventType = "gslb:service:member:add"
	EventTypeGSLBServiceMemberRemove       events.EventType = "gslb:service:member:remove"

	EventTypeGSLBConfig       events.EventType = "gslb:config"
	EventTypeGSLBConfigCreate events.EventType = "gslb:config:create"
	EventTypeGSLBConfigUpdate events.EventType = "gslb:config:update"
	EventTypeGSLBConfigDelete events.EventType = "gslb:config:delete"
)

// DNSDIST event types
const (
	EventTypeDNSDIST events.EventType = "dnsdist"

	EventTypeDNSDISTSpoof events.EventType = "dnsdist:spoof"

	EventTypeDNSDISTSpoofCreateFailed events.EventType = "dnsdist:spoof:create_failed"
	EventTypeDNSDISTSpoofDeleteFailed events.EventType = "dnsdist:spoof:delete_failed"

	EventTypeDNSDISTSync       events.EventType = "dnsdist:sync"
	EventTypeDNSDISTSyncFailed events.EventType = "dnsdist:sync:failed"

	EventTypeDNSDISTServer          events.EventType = "dnsdist:server"
	EventTypeDNSDISTServerOutOfSync events.EventType = "dnsdist:server:out-of-sync"
)

// DNS - specific event types
const (
	EventTypeDNS events.EventType = "dns"

	EventTypeDNSZoneTransfer       events.EventType = "dns:zonetransfer"
	EventTypeDNSZoneTransferFailed events.EventType = "dns:zonetransfer:failed"
)
