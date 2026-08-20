package notifications

import (
	"fmt"

	"github.com/vitistack/gslb-operator/internal/config"
	"github.com/vitistack/gslb-operator/internal/model"
	"github.com/vitistack/gslb-operator/pkg/events"
)

type Notifier interface {
	Publish(*events.Event, model.WebHook) error
}

func NewNotifier(format string) (Notifier, error) {
	if config.Webhooks().Enabled() {
		switch format {
		case "slack":
			if config.Webhooks().Notifications().Slack().Enabled() {
				return NewSlackNotifier(), nil
			}
			return nil, fmt.Errorf("slack notifications are not enabled")

		default:
			return nil, fmt.Errorf("unknown notifier format: %s", format)
		}
	}
	return nil, fmt.Errorf("webhooks not enabled")
}
