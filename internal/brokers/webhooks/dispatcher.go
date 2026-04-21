package webhooks_broker

import (
	"log/slog"

	"github.com/vitistack/gslb-operator/internal/model"
	"github.com/vitistack/gslb-operator/internal/model/events/notifications"
	"github.com/vitistack/gslb-operator/pkg/bslog"
	"github.com/vitistack/gslb-operator/pkg/events"
)

type Dispatcher struct {
	webhook  model.WebHook
	notifier notifications.Notifier
}

func Dispatch(wh model.WebHook) error {
	dispatcher := &Dispatcher{
		webhook: wh,
	}

	dispatcher.notifier = notifications.NewNotifier(wh.Options.Format)

	err := wh.Apply(dispatcher)
	if err != nil {
		return err
	}

	return nil
}

func (d *Dispatcher) Handle(e *events.Event) {
	err := d.notifier.Publish(e, d.webhook)
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
