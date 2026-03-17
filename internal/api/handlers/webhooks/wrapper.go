package webhooks

import (
	"encoding/json"

	"github.com/vitistack/gslb-operator/pkg/bslog"
	"github.com/vitistack/gslb-operator/pkg/events"
)

type WebHookBodyWrapper interface {
	Wrap(*events.Event) ([]byte, error)
}

type SlackBodyWrapper struct{}

func (sw *SlackBodyWrapper) Wrap(e *events.Event) ([]byte, error) {
	bslog.Warn("un-implemented slack body wrapper interface")
	return json.Marshal(e)
}
