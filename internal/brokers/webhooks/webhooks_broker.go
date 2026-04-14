package webhooks_broker

import (
	"context"
	"fmt"

	"github.com/vitistack/gslb-operator/internal/config"
	"github.com/vitistack/gslb-operator/internal/model"
	"github.com/vitistack/gslb-operator/internal/repositories/webhooks"
	"github.com/vitistack/gslb-operator/pkg/mq"
	"github.com/vitistack/gslb-operator/pkg/mq/rabbitmq"
	"github.com/vitistack/gslb-operator/pkg/persistence"
)

type WebhooksBroker struct {
	client mq.MessageBroker[model.WebHook]
	repo   *webhooks.WebHooksRepo
}

func New(ctx context.Context, store persistence.Store[model.WebHook]) *WebhooksBroker {
	mqCfg := config.GetInstance().MQ()
	return &WebhooksBroker{
		repo: webhooks.NewWebHooksRepo(store),
		client: rabbitmq.New(
			ctx,
			fmt.Sprintf(
				"amqp://%s:%s@%s:%s",
				mqCfg.User(),
				mqCfg.Pass(),
				mqCfg.Endpoint(),
				mqCfg.Port(),
			),
			rabbitmq.WithQueue[model.WebHook]("webhooks"),
		),
	}
}

func (w *WebhooksBroker) Subscribe(ctx context.Context) {
	go w.client.Subscribe(ctx, w.handle)
}

// handler function for webhooks registration
func (w *WebhooksBroker) handle(ctx context.Context, wh model.WebHook) error {
	err := Dispatch(wh)
	if err != nil {
		return fmt.Errorf("failed to dispatch webhook: %w", err)
	}

	return nil
}
