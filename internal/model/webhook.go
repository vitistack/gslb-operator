package model

import (
	"encoding/json"
	"log/slog"

	"github.com/vitistack/gslb-operator/pkg/events"
)

type WebHook struct {
	ID           string            `json:"id"`
	URL          *string           `json:"url,omitempty"`
	Secret       *string           `json:"secret,omitempty"`
	Subscription EventSubscription `json:"subscription"`
	Options      WebHookOptions    `json:"options"`
}
type WebHookOptions struct {
	SecretHeader string `json:"secretHeader,omitempty"` // defaults to Authorization
	Format       string `json:"format"`
}

type EventSubscription struct {
	Events  []events.EventType `json:"events"`
	Options json.RawMessage    `json:"options,omitempty"`
}

func (wh *WebHook) Apply(dispatcher events.EventHandler) error {
	for _, event := range wh.Subscription.Events {
		opts, err := events.ResolveOptions(event, wh.Subscription.Options)
		if err != nil {
			return err
		}

		events.On(event, dispatcher, opts.Filter())
	}

	return nil
}

func (wh *WebHook) LogValue() slog.Value {
	types := make([]string, len(wh.Subscription.Events))
	for i, sub := range wh.Subscription.Events {
		types[i] = string(sub)
	}
	return slog.GroupValue(
		slog.String("id", wh.ID),
		slog.Any("url", wh.URL),
		slog.Any("events", types),
	)
}
