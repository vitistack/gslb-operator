package webhooks

import (
	"log/slog"
	"net/http"

	"github.com/vitistack/gslb-operator/internal/model"
	"github.com/vitistack/gslb-operator/pkg/bslog"
	"github.com/vitistack/gslb-operator/pkg/events"
	"github.com/vitistack/gslb-operator/pkg/rest/request"
)

type Payload struct {
	events.Event
}

type Dispatcher struct {
	WebHookBodyWrapper
	webhook model.WebHook
	client  *http.Client
}

func Dispatch(wh model.WebHook) error {
	dispatcher := &Dispatcher{
		webhook: wh,
	}
	err := wh.Apply(dispatcher)
	if err != nil {
		return err
	}

	switch wh.Options.Format {
	default:
		dispatcher.WebHookBodyWrapper = &SlackBodyWrapper{}
	}

	return nil
}

func (d *Dispatcher) Handle(e *events.Event) {
	body, err := d.Wrap(e)
	if err != nil {
		bslog.Error("could not format event body", 
		slog.String("reason", err.Error()),
		slog.Any("event", e),
	)
		return
	}

	builder := request.NewBuilder(d.webhook.URL).
		POST().
		Body(body)

	if d.webhook.Secret != nil {
		builder.SetHeader(d.webhook.Options.SecretHeader, *d.webhook.Secret)
	}

	req, err := builder.Build()
	if err != nil {
		bslog.Error("failed to build webhook request", slog.String("reason", err.Error()))
		return
	}

	//nolint:errcheck
	resp, _ := d.client.Do(req)
	if resp != nil {
		resp.Body.Close()
	}
}

func (d *Dispatcher) GetID() string {
	return d.webhook.ID
}
