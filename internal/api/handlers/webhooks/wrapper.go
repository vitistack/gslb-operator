package webhooks

/*
import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/slack-go/slack"
	"github.com/vitistack/gslb-operator/internal/model/events/notifications"
	"github.com/vitistack/gslb-operator/pkg/bslog"
	"github.com/vitistack/gslb-operator/pkg/events"
)

type WebHookBodyWrapper interface {
	Wrap(*events.Event) ([]byte, error)
}

type SlackBodyWrapper struct{}

func (sw *SlackBodyWrapper) Wrap(e *events.Event) ([]byte, error) {
	bslog.Warn("un-implemented slack body wrapper interface")

	notification, ok := e.Payload.(notifications.SlackNotification)
	if !ok {
		return nil, fmt.Errorf("event payload: %T does not support slack notifications", e.Payload)
	}

	fields := make([]slack.AttachmentField, 0, len(notification.SlackFields()))
	for _, field := range notification.SlackFields() {
		fields = append(fields, slack.AttachmentField{
			Title: field.Title,
			Value: field.Value,
			Short: field.Short,
		})
	}

	msg := &slack.WebhookMessage{
		Attachments: []slack.Attachment{
			{
				Color:    notification.SlackColor(),
				Title:    notification.SlackTitle(),
				Fields:   fields,
				Footer:   fmt.Sprintf("gslb-operator • %s", e.Type),
				Ts:       json.Number(fmt.Sprintf("%d", e.Timestamp.UTC().Unix())),
				Fallback: fmt.Sprintf("[%s] %s at %s", e.Type, notification.SlackTitle(), e.Timestamp.UTC().Format(time.RFC3339)),
			},
		},
	}

	return json.Marshal(msg)
}
*/
