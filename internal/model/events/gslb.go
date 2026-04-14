package events

import (
	"fmt"

	"github.com/slack-go/slack"
	"github.com/vitistack/gslb-operator/internal/model"
)

// gslb:failover
type GSLBFailoverEvent struct {
	MemberOf   string
	LastActive model.GSLBService
	NewActive  model.GSLBService
}

func (e *GSLBFailoverEvent) SlackValue() slack.MsgOption {
	return slack.MsgOptionBlocks()
}

// gslb:service:up
type GSLBServiceUpEvent struct {
	NewActive model.GSLBService
}

func (e *GSLBServiceUpEvent) SlackValue() slack.MsgOption {
	return slack.MsgOptionBlocks()
}

// gslb:service:down
type GSLBServiceDownEvent struct {
	MemberOf string
}

func (e *GSLBServiceDownEvent) SlackValue() slack.MsgOption {
	return slack.MsgOptionBlocks()
}

// gslb:member:healthchange
type GSLBServiceMemberHealthChangeEvent struct {
	Member model.GSLBService
}

func (e *GSLBServiceMemberHealthChangeEvent) SlackValue() slack.MsgOption {
	return slack.MsgOptionBlocks()
}

// gslb:config:create
type GSLBConfigCreateEvent struct {
	Config model.GSLBConfig
}

func (e *GSLBConfigCreateEvent) SlackValue() slack.MsgOption {
	return slack.MsgOptionBlocks(
		slack.NewHeaderBlock(slack.NewTextBlockObject(slack.PlainTextType, ":new: GSLB Config Created", true, false)),
		configFields(e.Config),
	)
}

// gslb:config:update
type GSLBConfigUpdateEvent struct {
	LastConfig    model.GSLBConfig
	CurrentConfig model.GSLBConfig
}

func (e *GSLBConfigUpdateEvent) SlackValue() slack.MsgOption {
	return slack.MsgOptionBlocks(
		slack.NewHeaderBlock(slack.NewTextBlockObject(slack.PlainTextType, ":pencil2: GSLB Config Updated", true, false)),
		slack.NewSectionBlock(
			slack.NewTextBlockObject(slack.MarkdownType, "*Service:* "+e.CurrentConfig.MemberOf, false, false),
			nil, nil,
		),
		slack.NewSectionBlock(
			slack.NewTextBlockObject(slack.MarkdownType, "*Before*", false, false),
			nil, nil,
		),
		configFields(e.LastConfig),
		slack.NewSectionBlock(
			slack.NewTextBlockObject(slack.MarkdownType, "*After*", false, false),
			nil, nil,
		),
		configFields(e.CurrentConfig),
	)
}

// gslb:config:delete
type GSLBConfigDeleteEvent struct {
	LastConfig model.GSLBConfig
}

func (e *GSLBConfigDeleteEvent) SlackValue() slack.MsgOption {
	return slack.MsgOptionBlocks(
		slack.NewHeaderBlock(slack.NewTextBlockObject(slack.PlainTextType, ":wastebasket: GSLB Config Deleted", true, false)),
		configFields(e.LastConfig),
	)
}

func configFields(c model.GSLBConfig) *slack.SectionBlock {
	field := func(title, value string) *slack.TextBlockObject {
		return slack.NewTextBlockObject(slack.MarkdownType, "*"+title+":*\n"+value, false, false)
	}
	return slack.NewSectionBlock(nil, []*slack.TextBlockObject{
		field("Service", c.MemberOf),
		field("FQDN", c.Fqdn),
		field("IP", c.Ip),
		field("Port", c.Port),
		field("Datacenter", c.Datacenter),
		field("Check Type", c.CheckType),
		field("Interval", c.Interval.String()),
		field("Priority", fmt.Sprintf("%d", c.Priority)),
		field("Failure Threshold", fmt.Sprintf("%d", c.FailureThreshold)),
	}, nil)
}
