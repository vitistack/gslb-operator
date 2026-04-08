package rabbitmq

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/vitistack/gslb-operator/pkg/mq"
)

// TODO: decide if generics is overkill or not, could just pass amqp.Publishing directly instead
type Broker[T any] struct {
	*connection
	queue       string
	exchange    string
	dlx         string
	dlq         string
	prefetch    int
	consumerTag string
	logger      *slog.Logger
}

func New[T any](ctx context.Context, ampqURL string, opts ...brokerOption[T]) mq.MessageBroker[T] {

	broker := &Broker[T]{
		connection: newConnection(ctx, ampqURL),
	}

	for _, opt := range opts {
		opt(broker)
	}

	return broker
}

func (b *Broker[T]) Publish(ctx context.Context, msg T) error {
	body, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("could not marshall message body: %w", err)
	}

	return b.channel.PublishWithContext(ctx,
		b.exchange,
		b.queue,
		false,
		false,
		amqp.Publishing{
			ContentType:  "application/json",
			DeliveryMode: amqp.Persistent,
			Body:         body,
			Timestamp:    time.Now(),
		},
	)
}

func (b *Broker[T]) Subscribe(ctx context.Context, handler mq.MessageHandler[T]) error {
	// Limit in-flight unACK'd messages — backpressure against slow handlers.
	if err := b.channel.Qos(b.prefetch, 0, false); err != nil {
		return fmt.Errorf("rabbitmq set QoS: %w", err)
	}

	messages, err := b.channel.ConsumeWithContext(ctx,
		b.queue,
		b.consumerTag,
		false,
		false,
		false,
		false,
		nil,
	)

	if err != nil {
		return fmt.Errorf("failed to start message consumption: %w", err)
	}

	for {
		select {
		case msg, ok := <-messages:
			if !ok {
				return errors.New("rabbitmq: channel closed unexpectedly")
			}

			b.handle(ctx, msg, handler)

		case <-ctx.Done():
			return nil
		}
	}
}

func (b *Broker[T]) handle(ctx context.Context, delivery amqp.Delivery, handler mq.MessageHandler[T]) {
	var msg T
	if err := json.Unmarshal(delivery.Body, &msg); err != nil {
		delivery.Nack(false, false)
		return
	}

	handler(ctx, msg)
}

func (b *Broker[T]) Close(ctx context.Context) error {
	return b.connection.Close(ctx)
}
