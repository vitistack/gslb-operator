package notifications

import (
	"fmt"

	"github.com/slack-go/slack"
	"github.com/vitistack/gslb-operator/internal/config"
	"github.com/vitistack/gslb-operator/internal/model"
	"github.com/vitistack/gslb-operator/pkg/events"
)

type SlackNotification interface {
	SlackValue() slack.MsgOption
}

type SlackNotifier struct {
	client *slack.Client
}

func NewSlackNotifier() *SlackNotifier {
	return &SlackNotifier{
		client: slack.New(
			config.Webhooks().Notifications().Slack().BotToken(),
			slack.OptionAppLevelToken(config.Webhooks().Notifications().Slack().AppToken()),
		),
	}
}

func (s *SlackNotifier) Publish(e *events.Event, wh model.WebHook) error {
	notification, ok := e.Payload.(SlackNotification)
	if !ok {
		return fmt.Errorf("event payload is not of type SlackNotification")
	}

	_, _, err := s.client.PostMessage(
		wh.ID,
		notification.SlackValue(),
	)
	if err != nil {
		return fmt.Errorf("failed to create slack message: %w", err)
	}

	return nil
}
