package events

import (
	"github.com/slack-go/slack"
	"github.com/vitistack/gslb-operator/pkg/models/spoofs"
)

// dnsdist:spoof:create
type DNSDistSpoofCreateEvent struct {
	Spoof spoofs.Spoof
}

func (e DNSDistSpoofCreateEvent) SlackValue() slack.MsgOption {
	return slack.MsgOptionBlocks()
}

// dnsdist:spoof:delete
type DNSDistSpoofDeleteEvent struct {
	Spoof spoofs.Spoof
}

func (e DNSDistSpoofDeleteEvent) SlackValue() slack.MsgOption {
	return slack.MsgOptionBlocks()
}

type DNSDistSyncFailedEvent struct {
	Reason string
}

func (e DNSDistSyncFailedEvent) SlackValue() slack.MsgOption {
	return slack.MsgOptionBlocks()
}

type DNSDistServerOutOfSyncEvent struct {
}

func (e DNSDistServerOutOfSyncEvent) SlackValue() slack.MsgOption {
	return slack.MsgOptionBlocks()
}
