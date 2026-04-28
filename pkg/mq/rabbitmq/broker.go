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
	retry       int
	consumerTag string
	logger      *slog.Logger
}

func New[T any](ctx context.Context, ampqURL string, opts ...brokerOption[T]) mq.MessageBroker[T] {
	broker := &Broker[T]{
		connection: newConnection(ctx, ampqURL),
		logger:     slog.Default(),
		retry:      3,
	}
	broker.retryConnectionBackoff = connectionRetryBackoff * 15

	broker.connection.onNewConnection = broker.declareTopology

	for _, opt := range opts {
		opt(broker)
	}

	go func() {
		err := broker.connection.connect(ctx)
		for err != nil {
			broker.logger.Error("mq: failed to connect",
				slog.String("reason", err.Error()),
				slog.String("retry", broker.retryConnectionBackoff.String()),
			)
			time.Sleep(broker.retryConnectionBackoff)
			err = broker.connection.connect(ctx)
		}
	}()

	return broker
}

// TODO: implement full version
func (b *Broker[T]) declareTopology() error {
	if b.exchange != "" {
		// declare exchange with queue
		err := b.connection.channel.ExchangeDeclare(
			b.exchange,
			amqp.ExchangeDirect,
			true,
			false,
			false,
			false,
			nil,
		)

		if err != nil {

		}

		_, err = b.connection.channel.QueueDeclare(
			b.queue,
			true,
			false,
			false,
			false,
			nil,
		)

		if err != nil {

		}

		b.channel.QueueBind(
			"",
			"",
			"",
			false,
			nil,
		)

	} else {
		_, err := b.connection.channel.QueueDeclare(
			b.queue,
			true,
			false,
			false,
			false,
			nil,
		)

		if err != nil {

		}
	}

	if b.dlx != "" {
		// declare dead-letter exchange with queue
	} else {
		// only declare dead-letter queue
	}

	return nil
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
	b.connection.Wait(ctx)
	if ctx.Err() != nil {
		return fmt.Errorf("mq: failed to subscribe: %w", ctx.Err())
	}

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
				return errors.New("mq: channel closed unexpectedly")
			}
			b.handle(ctx, msg, handler)

		case <-ctx.Done():
			return b.connection.Close(ctx)
		}
	}
}

func (b *Broker[T]) handle(ctx context.Context, delivery amqp.Delivery, handler mq.MessageHandler[T]) {
	var msg T
	if err := json.Unmarshal(delivery.Body, &msg); err != nil {
		delivery.Nack(false, false)
		return
	}

	err := handler(ctx, msg)
	if err != nil {
		delivery.Reject(false)
	}

	err = delivery.Ack(true)
	if err != nil {
		b.logger.Error("mq: could not acknowledge delivery",
			slog.String("reason", err.Error()),
			slog.String("id", delivery.MessageId),
		)
	}
}

func (b *Broker[T]) Close(ctx context.Context) error {
	return b.connection.Close(ctx)
}
