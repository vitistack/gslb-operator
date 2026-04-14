package webhooks_broker

import (
	"log/slog"

	"github.com/slack-go/slack"
	"github.com/vitistack/gslb-operator/internal/config"
	"github.com/vitistack/gslb-operator/internal/model"
	"github.com/vitistack/gslb-operator/internal/model/events/notifications"
	"github.com/vitistack/gslb-operator/pkg/bslog"
	"github.com/vitistack/gslb-operator/pkg/events"
)

type Dispatcher struct {
	webhook model.WebHook
	client  *slack.Client
}

func Dispatch(wh model.WebHook) error {
	dispatcher := &Dispatcher{
		webhook: wh,
		client: slack.New(
			config.GetInstance().Slack().BotToken(),
			slack.OptionAppLevelToken(config.GetInstance().Slack().AppToken()),
		),
	}

	/*
		TODO support different formats
			switch wh.Options.Format {
			default:
			}
	*/

	err := wh.Apply(dispatcher)
	if err != nil {
		return err
	}

	return nil
}

func (d *Dispatcher) Handle(e *events.Event) {
	notification, ok := e.Payload.(notifications.SlackNotification)
	if !ok {
		bslog.Debug("event payload is not of type SlackNotification", slog.Any("event", e))
		return
	}

	_, _, err := d.client.PostMessage(
		d.webhook.ID,
		notification.SlackValue(),
	)
	if err != nil {
		bslog.Error("failed to handle event",
			slog.String("reason", err.Error()),
			slog.Any("event", e),
		)
	}
}

func (d *Dispatcher) GetID() string {
	return d.webhook.ID
}
