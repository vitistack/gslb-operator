package events

import (
	"fmt"

	"github.com/slack-go/slack"
	"github.com/vitistack/gslb-operator/internal/model"
)

// gslb:service:up
type GSLBServiceUpEvent struct {
	NewActive model.GSLBService
}

func (e GSLBServiceUpEvent) SlackValue() slack.MsgOption {
	return slack.MsgOptionBlocks(
		slack.NewHeaderBlock(slack.NewTextBlockObject(slack.PlainTextType, ":white_check_mark: Service Up", true, false)),
		slack.NewSectionBlock(
			slack.NewTextBlockObject(slack.MarkdownType, "*Service:* "+e.NewActive.MemberOf, false, false),
			nil, nil,
		),
		serviceFields(e.NewActive),
	)
}

// gslb:service:down
type GSLBServiceDownEvent struct {
	MemberOf string
}

func (e GSLBServiceDownEvent) SlackValue() slack.MsgOption {
	return slack.MsgOptionBlocks(
		slack.NewHeaderBlock(slack.NewTextBlockObject(slack.PlainTextType, ":skull: Service Down", true, false)),
		slack.NewSectionBlock(
			slack.NewTextBlockObject(slack.MarkdownType, "*Service:* "+e.MemberOf+" is no longer active.", false, false),
			nil, nil,
		),
	)
}

// gslb:failover
type GSLBServiceFailoverEvent struct {
	MemberOf   string
	LastActive model.GSLBService
	NewActive  model.GSLBService
}

func (e GSLBServiceFailoverEvent) SlackValue() slack.MsgOption {
	return slack.MsgOptionBlocks(
		slack.NewHeaderBlock(slack.NewTextBlockObject(slack.PlainTextType, ":arrows_counterclockwise: GSLB Failover", true, false)),
		slack.NewSectionBlock(
			slack.NewTextBlockObject(slack.MarkdownType, "*Service:* "+e.MemberOf, false, false),
			nil, nil,
		),
		slack.NewSectionBlock(
			slack.NewTextBlockObject(slack.MarkdownType, "*Previous Active*", false, false),
			nil, nil,
		),
		serviceFields(e.LastActive),
		slack.NewSectionBlock(
			slack.NewTextBlockObject(slack.MarkdownType, "*New Active*", false, false),
			nil, nil,
		),
		serviceFields(e.NewActive),
	)
}

// gslb:service:member:healthchange
type GSLBServiceMemberHealthChangeEvent struct {
	Member model.GSLBService
}

func (e GSLBServiceMemberHealthChangeEvent) SlackValue() slack.MsgOption {
	status := ":large_green_circle: Healthy"
	if !e.Member.IsHealthy {
		status = ":red_circle: Unhealthy"
	}
	return slack.MsgOptionBlocks(
		slack.NewHeaderBlock(slack.NewTextBlockObject(slack.PlainTextType, "Member Health Changed", true, false)),
		slack.NewSectionBlock(
			slack.NewTextBlockObject(slack.MarkdownType, "*Service:* "+e.Member.MemberOf+"\n*Status:* "+status, false, false),
			nil, nil,
		),
		serviceFields(e.Member),
	)
}

// gslb:service:member:add
type GSLBServiceMemberAddEvent struct {
	Service   string
	NewMember model.GSLBService
}

func (e GSLBServiceMemberAddEvent) SlackValue() slack.MsgOption {
	return slack.MsgOptionBlocks(
		slack.NewHeaderBlock(slack.NewTextBlockObject(slack.PlainTextType, "Member Added to Service", true, false)),
		slack.NewSectionBlock(
			slack.NewTextBlockObject(slack.MarkdownType, "*Service:* "+e.Service, false, false),
			nil, nil,
		),
		serviceFields(e.NewMember),
	)
}

// gslb:service:member:remove
type GSLBServiceMemberRemoveEvent struct {
	Service string
	Removed model.GSLBService
}

func (e GSLBServiceMemberRemoveEvent) SlackValue() slack.MsgOption {
	return slack.MsgOptionBlocks(
		slack.NewHeaderBlock(slack.NewTextBlockObject(slack.PlainTextType, "Member Removed from Service", true, false)),
		slack.NewSectionBlock(
			slack.NewTextBlockObject(slack.MarkdownType, "*Service:* "+e.Service, false, false),
			nil, nil,
		),
		serviceFields(e.Removed),
	)
}

// gslb:config:create
type GSLBConfigCreateEvent struct {
	Config model.GSLBConfig
}

func (e GSLBConfigCreateEvent) SlackValue() slack.MsgOption {
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

func (e GSLBConfigUpdateEvent) SlackValue() slack.MsgOption {
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

func (e GSLBConfigDeleteEvent) SlackValue() slack.MsgOption {
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
		field("MemberOf", c.MemberOf),
		field("FQDN", c.Fqdn),
		//field("IP", c.Ip),
		field("Port", c.Port),
		field("Datacenter", c.Datacenter),
		field("Check Type", c.CheckType),
		field("Interval", c.Interval.String()),
		field("Priority", fmt.Sprintf("%d", c.Priority)),
		field("Failure Threshold", fmt.Sprintf("%d", c.FailureThreshold)),
	}, nil)
}

func serviceFields(s model.GSLBService) *slack.SectionBlock {
	field := func(title, value string) *slack.TextBlockObject {
		return slack.NewTextBlockObject(slack.MarkdownType, "*"+title+":*\n"+value, false, false)
	}
	healthy := "No"
	if s.IsHealthy {
		healthy = "Yes"
	}
	return slack.NewSectionBlock(nil, []*slack.TextBlockObject{
		field("FQDN", s.Fqdn),
		//field("IP", s.IP.String()),
		field("Datacenter", s.Datacenter),
		field("Healthy", healthy),
	}, nil)
}
