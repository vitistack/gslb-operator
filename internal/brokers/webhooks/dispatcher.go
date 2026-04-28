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

func Dispatch(wh model.WebHook) error {
	// remove old references for the webhook
	// re-registration replaces old version
	events.RemoveAll(wh.ID)

	dispatcher := &Dispatcher{
		webhook: wh,
		notifier: notifications.NewNotifier(wh.Options.Format),
	}

	err := wh.Apply(dispatcher)
	if err != nil {
		return err
	}

	bslog.Info("new webhook registered", slog.Any("webhook", wh))

	return nil
}
