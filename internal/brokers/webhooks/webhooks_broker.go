package webhooks

import (
	"context"

	"github.com/vitistack/gslb-operator/internal/model"
	"github.com/vitistack/gslb-operator/internal/repositories/webhooks"
	"github.com/vitistack/gslb-operator/pkg/mq"
	"github.com/vitistack/gslb-operator/pkg/persistence"
)

type WebhooksBroker struct {
	client mq.MessageBroker[model.WebHook]
	repo   *webhooks.WebHooksRepo
}

func New(store persistence.Store[model.WebHook]) *WebhooksBroker {
	return &WebhooksBroker{
		repo: webhooks.NewWebHooksRepo(store),
	}
}

func (w *WebhooksBroker) Subscribe(ctx context.Context) error {
	return w.client.Subscribe(ctx, w.handle)
}

func (w *WebhooksBroker) handle(ctx context.Context, wh model.WebHook) {
	
}
