package model

import (
	"encoding/json"

	"github.com/vitistack/gslb-operator/pkg/events"
)

type WebHook struct {
	ID      string              `json:"id"`
	URL     string              `json:"url"`
	Secret  *string             `json:"secret,omitempty"`
	Events  []EventSubscription `json:"subscriptions"`
	Options WebHookOptions      `json:"options"`
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

type WebHookOptions struct {
	SecretHeader *string `json:"secretHeader,omitempty"` // defaults to Authorization
}

type EventSubscription struct {
	Type    events.EventType `json:"event"`
	Options json.RawMessage  `json:"options,omitempty"`
}
