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
	case "prod", "PROD", "production", "PRODUCTION":
		switch format {
		case "slack":
			return NewSlackNotifier()
		default:
			return &MockNotifier{}
		}
	default:
		return &MockNotifier{}
	}
}
