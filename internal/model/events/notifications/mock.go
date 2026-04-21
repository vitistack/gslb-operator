package notifications

import (
	"log/slog"

	"github.com/vitistack/gslb-operator/internal/model"
	"github.com/vitistack/gslb-operator/pkg/bslog"
	"github.com/vitistack/gslb-operator/pkg/events"
)

type MockNotifier struct{}

func (m *MockNotifier) Publish(e *events.Event, wh model.WebHook) error {
	bslog.Info("received event", slog.Any("event", e))
	return nil
}
