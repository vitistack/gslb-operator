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

type DNSDistSynchStartedEvent struct {
}

func (e DNSDistSynchStartedEvent) SlackValue() slack.MsgOption {
	return slack.MsgOptionBlocks()
}

type DNSDistSynchCompletedEvent struct {
}

func (e DNSDistSynchCompletedEvent) SlackValue() slack.MsgOption {
	return slack.MsgOptionBlocks()
}

type DNSDistSynchFailedEvent struct {
	Reason string
}

func (e DNSDistSynchFailedEvent) SlackValue() slack.MsgOption {
	return slack.MsgOptionBlocks()
}
