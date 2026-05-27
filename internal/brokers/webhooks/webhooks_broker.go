package webhooks_broker

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/vitistack/gslb-operator/internal/config"
	"github.com/vitistack/gslb-operator/internal/model"
	"github.com/vitistack/gslb-operator/internal/repositories/webhooks"
	"github.com/vitistack/gslb-operator/pkg/bslog"
	"github.com/vitistack/gslb-operator/pkg/events"
	"github.com/vitistack/gslb-operator/pkg/mq"
	"github.com/vitistack/gslb-operator/pkg/mq/rabbitmq"
	"github.com/vitistack/gslb-operator/pkg/persistence"
)

type WebhooksBroker struct {
	client mq.MessageBroker[model.WebHook]
	repo   *webhooks.WebHooksRepo
}

func Init(ctx context.Context, store persistence.Store[model.WebHook]) {
	if config.GetInstance().MQ().Enabled() {
		New(ctx, store).Subscribe(ctx)
	}
}

func New(ctx context.Context, store persistence.Store[model.WebHook]) *WebhooksBroker {
	mqCfg := config.GetInstance().MQ()
	broker := &WebhooksBroker{
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
			rabbitmq.WithRetryConnectionBackOff[model.WebHook](time.Second*10),
		),
	}

	webhooks, err := broker.repo.ReadAll()
	if err != nil {
		bslog.Error("failed read webhooks", slog.String("reason", err.Error()))
		return broker
	}

	for _, hook := range webhooks {
		err := Dispatch(hook)
		if err != nil {
			bslog.Error(
				"failed to dispatch stored webhook",
				slog.String("webhook_id", hook.ID),
				slog.String("reason", err.Error()),
			)
		}
	}

	return broker
}

func (w *WebhooksBroker) Subscribe(ctx context.Context) {
	go func() {
		const retryDelay = time.Second * 5

		for {
			err := w.client.Subscribe(ctx, w.handle)
			if err == nil {
				select {
				case <-ctx.Done():
					return
				default:
					bslog.Error("webhooks subscription stopped unexpectedly")
				}
			} else {
				select {
				case <-ctx.Done():
					return
				default:
					bslog.Error("webhooks subscription failed", slog.String("reason", err.Error()))
				}
			}
			select {
			case <-ctx.Done():
				return
			case <-time.After(retryDelay):
			}
		}
	}()
}

// handler function for webhooks registration
func (w *WebhooksBroker) handle(ctx context.Context, wh model.WebHook) error {
	bslog.Debug("received webhook new webhook", slog.Any("webhook", wh))

	err := Dispatch(wh)
	if err != nil {
		err := fmt.Errorf("failed to dispatch webhook: %w", err)
		bslog.Error("failed to process new webhooks registration", slog.String("reason", err.Error()))
		return err
	}

	err = w.repo.Create(wh)
	if err != nil {
		err := fmt.Errorf("failed to store webhook: %w", err)
		bslog.Error("failed to process new webhooks registration", slog.String("reason", err.Error()))

		// remove all registered event handlers for the webhook
		events.RemoveAll(wh.ID)
		return err
	}

	return nil
}
