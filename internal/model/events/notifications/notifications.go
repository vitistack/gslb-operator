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
	if config.Webhooks().Notifications().Slack().Enable() {
		switch format {
		case "slack":
			return NewSlackNotifier(), nil
		default:
			return nil, fmt.Errorf("un-supported notifier format: %s", format)
		}

	}
	return nil, fmt.Errorf("webhooks not enabled")
}
