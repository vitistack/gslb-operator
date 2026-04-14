package notifications

import "github.com/slack-go/slack"

type SlackNotification interface {
	SlackValue() slack.MsgOption
}
