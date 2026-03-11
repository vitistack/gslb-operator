package model

import (
	"encoding/json"
	"log/slog"

	"github.com/vitistack/gslb-operator/pkg/events"
)

type WebHook struct {
	ID      string              `json:"id"`
	URL     string              `json:"url"`
	Secret  *string             `json:"secret,omitempty"`
	Events  []EventSubscription `json:"subscriptions"`
	Options WebHookOptions      `json:"options"`
}
type WebHookOptions struct {
	SecretHeader string `json:"secretHeader,omitempty"` // defaults to Authorization
}

type EventSubscription struct {
	Type    events.EventType `json:"event"`
	Options json.RawMessage  `json:"options,omitempty"`
}

func (wh *WebHook) Apply(dispatcher events.EventHandler) error {
	for _, sub := range wh.Events {
		opts, err := events.ResolveOptions(sub.Type, sub.Options)
		if err != nil {
			return err
		}

		events.On(sub.Type, dispatcher, opts.Filter())
	}

	return nil
}

func (wh *WebHook) LogValue() slog.Value {
	types := make([]string, len(wh.Events))
	for i, sub := range wh.Events {
		types[i] = string(sub.Type)
	}
	return slog.GroupValue(
		slog.String("id", wh.ID),
		slog.String("url", wh.URL),
		slog.Any("events", types),
	)
}
