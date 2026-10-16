package debug_broker

import (
	"log/slog"

	"github.com/vitistack/gslb-operator/internal/model"
	domainEvents "github.com/vitistack/gslb-operator/internal/model/events"
	"github.com/vitistack/gslb-operator/internal/model/events/notifications"
	"github.com/vitistack/gslb-operator/pkg/bslog"
	"github.com/vitistack/gslb-operator/pkg/events"
)

type EventLogger struct {
	notifier notifications.Notifier
}

func New() *EventLogger {
	return &EventLogger{notifier: &notifications.MockNotifier{}}
}

func Init() {
	logger := New()
	events.On(domainEvents.EventTypeDNS, logger)
	events.On(domainEvents.EventTypeDNSDIST, logger)
	events.On(domainEvents.EventTypeGSLB, logger)
}

func (l *EventLogger) Handle(e *events.Event) {
	if err := l.notifier.Publish(e, model.WebHook{}); err != nil {
		bslog.Error("failed to log event", slog.String("reason", err.Error()))
	}
}

func (l *EventLogger) GetID() string {
	return "debug:event-logger"
}
