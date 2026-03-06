package webhooks

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/vitistack/gslb-operator/internal/model"
	"github.com/vitistack/gslb-operator/pkg/bslog"
	"github.com/vitistack/gslb-operator/pkg/events"
	"github.com/vitistack/gslb-operator/pkg/rest/request"
)

type Payload struct {
	events.Event
}

type Dispatcher struct {
	webhook model.WebHook
	client  *http.Client
}

func NewDispatcher(wh model.WebHook) *Dispatcher {
	return &Dispatcher{
		webhook: wh,
		client:  &http.Client{Timeout: time.Second * 5},
	}
}

func (d *Dispatcher) Handle(e *events.Event) {
	body, err := json.Marshal(e)
	if err != nil {
		bslog.Error("could not marshall event body", slog.String("reason", err.Error()))
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	builder := request.NewBuilder(d.webhook.URL).
		POST().
		Body(body).
		CTX(ctx)

	if d.webhook.Secret != nil {
		builder.SetHeader(*d.webhook.Options.SecretHeader, *d.webhook.Secret)
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
