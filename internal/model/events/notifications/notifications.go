package notifications

import (
	"github.com/vitistack/gslb-operator/internal/config"
	"github.com/vitistack/gslb-operator/internal/model"
	"github.com/vitistack/gslb-operator/pkg/events"
)

type Notifier interface {
	Publish(*events.Event, model.WebHook) error
}

func NewNotifier(format string) Notifier {
	env := config.GetInstance().Server().Env()

	switch env {
	case "dev", "development", "DEV", "DEVELOPMENT", "local":
		return &MockNotifier{}
	default:
		switch format {
		case "slack":
			return NewSlackNotifier()
		}
	}

	return &MockNotifier{}
}
