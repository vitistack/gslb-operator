package events

import (
	"github.com/slack-go/slack"
	"github.com/vitistack/gslb-operator/pkg/models/spoofs"
)

// dnsdist:spoof:create:failed
type DNSDistSpoofCreateFailedEvent struct {
	Server string
	Spoof  spoofs.Spoof
}

func (e DNSDistSpoofCreateFailedEvent) SlackValue() slack.MsgOption {
	return slack.MsgOptionBlocks()
}

// dnsdist:spoof:delete:failed
type DNSDistSpoofDeleteFailedEvent struct {
	ID     string
	Server string
}

func (e DNSDistSpoofDeleteFailedEvent) SlackValue() slack.MsgOption {
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
