package rabbitmq

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"
	"uuid"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/vitistack/gslb-operator/pkg/bslog"
	"github.com/vitistack/gslb-operator/pkg/mq"
	"github.com/vitistack/gslb-operator/pkg/mq/rabbitmq/connection"
)

type Retryer interface {
	Retry(func() error) error
}

type RetryFunc func(func() error) error

func (rf RetryFunc) Retry(fn func() error) error {
	return rf(fn)
}

// TODO: decide if generics is overkill or not, could just pass amqp.Publishing directly instead
type Broker[T any] struct {
	pub         *channelHandle
	sub         *channelHandle
	queue       string
	exchange    string
	dlx         string
	dlq         string
	fanout      bool
	consumerTag string
	prefetch    int
	logger      *slog.Logger

	lock      sync.Mutex
	ready     chan struct{}
	chanReady chan *connection.Channel

	retry Retryer
}

func New[T any](ctx context.Context, ampqURL string, opts ...brokerOption[T]) mq.MessageBroker[T] {
	broker := &Broker[T]{
		logger: slog.Default(),
		retry: RetryFunc(func(errFunc func() error) error {
			err := errFunc()
			for err != nil {
				time.Sleep(connection.ConnectionRetryBackoff * 30)
				err = errFunc()
			}
			return nil
		}),
	}

	for _, opt := range opts {
		opt(broker)
	}

	// Separate connections so a slow/blocked consumer connection can never
	// stall publishing (and vice versa).
	pubConn := connection.NewConnection(ctx, connection.Publish, ampqURL)
	subConn := connection.NewConnection(ctx, connection.Subscribe, ampqURL)

	broker.pub = newChannelHandle(pubConn, broker.prefetch, broker.declareTopology, broker.logger, broker.retry)
	broker.sub = newChannelHandle(subConn, broker.prefetch, broker.declareTopology, broker.logger, broker.retry)

	broker.pub.start(ctx)
	broker.sub.start(ctx)

	return broker
}

func (b *Broker[T]) declareTopology(channel *connection.Channel) error {
	// Build dead-letter arguments for the main queue if DL is configured.
	var mainQueueArgs amqp.Table
	if b.dlx != "" {
		mainQueueArgs = amqp.Table{
			"x-dead-letter-exchange": b.dlx,
		}
		if b.dlq != "" {
			mainQueueArgs["x-dead-letter-routing-key"] = b.dlq
		}
	} else if b.dlq != "" {
		// No DLX: use default exchange and route directly to the named DLQ.
		mainQueueArgs = amqp.Table{
			"x-dead-letter-exchange":    "",
			"x-dead-letter-routing-key": b.dlq,
		}
	}

	if b.exchange != "" {
		kind := amqp.ExchangeDirect
		if b.fanout {
			kind = amqp.ExchangeFanout
		}
		if err := channel.ExchangeDeclare(
			b.exchange,
			kind,
			true,
			false,
			false,
			false,
			nil,
		); err != nil {
			return fmt.Errorf("mq: failed to declare exchange %q: %w", b.exchange, err)
		}

		if _, err := channel.QueueDeclare(
			b.queue,
			true,
			false,
			false,
			false,
			mainQueueArgs,
		); err != nil {
			return fmt.Errorf("mq: failed to declare queue %q: %w", b.queue, err)
		}

		if err := channel.QueueBind(
			b.queue,
			b.queue, // routing key equals queue name for a direct exchange
			b.exchange,
			false,
			nil,
		); err != nil {
			return fmt.Errorf("mq: failed to bind queue %q to exchange %q: %w", b.queue, b.exchange, err)
		}
	} else {
		if _, err := channel.QueueDeclare(
			b.queue,
			true,
			false,
			false,
			false,
			mainQueueArgs,
		); err != nil {
			return fmt.Errorf("mq: failed to declare queue %q: %w", b.queue, err)
		}
	}

	if b.dlx != "" {
		if err := channel.ExchangeDeclare(
			b.dlx,
			amqp.ExchangeDirect,
			true,
			false,
			false,
			false,
			nil,
		); err != nil {
			return fmt.Errorf("mq: failed to declare dead-letter exchange %q: %w", b.dlx, err)
		}

		// Derive DLQ name: use configured name or fall back to main queue + ".dlq".
		dlq := b.dlq
		if dlq == "" {
			dlq = b.queue + ".dlq"
		}

		if _, err := channel.QueueDeclare(
			dlq,
			true,
			false,
			false,
			false,
			nil,
		); err != nil {
			return fmt.Errorf("mq: failed to declare dead-letter queue %q: %w", dlq, err)
		}

		if err := channel.QueueBind(
			dlq,
			dlq, // routing key matches x-dead-letter-routing-key on the main queue
			b.dlx,
			false,
			nil,
		); err != nil {
			return fmt.Errorf("mq: failed to bind dead-letter queue %q to exchange %q: %w", dlq, b.dlx, err)
		}
	} else if b.dlq != "" {
		// No DLX: just ensure the dead-letter queue exists on the default exchange.
		if _, err := channel.QueueDeclare(
			b.dlq,
			true,
			false,
			false,
			false,
			nil,
		); err != nil {
			return fmt.Errorf("mq: failed to declare dead-letter queue %q: %w", b.dlq, err)
		}
	}
	bslog.Debug("declared topology",
		slog.Group(
			"topology",
			slog.String("dlx", b.dlx),
			slog.String("dlq", b.dlq),
			slog.String("exchange", b.exchange),
			slog.String("queue", b.queue),
			slog.String("consumerTag", b.consumerTag),
		),
	)
	return nil
}

func (b *Broker[T]) Publish(ctx context.Context, payload T) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("mq: failed to marshall message: %w", err)
	}

	msg := amqp.Publishing{
		MessageId:   uuid.NewV7().String(),
		ContentType: "application/json",
		Body:        body,
	}

	channel, err := b.pub.get(ctx)
	if err != nil {
		return fmt.Errorf("mq: broker failed to retrieve channel: %w", err)
	}

	b.logger.Info("mq: publishing message", slog.String("message_id", msg.MessageId))

	return channel.Publish(
		ctx,
		b.exchange,
		b.queue,
		msg,
	)
}

func (b *Broker[T]) Subscribe(ctx context.Context, handler mq.MessageHandler[T]) error {
	channel, err := b.sub.get(ctx)
	if err != nil {
		return fmt.Errorf("mq: broker failed to retrieve channel: %w", err)
	}
	messages, err := channel.Subscribe(ctx, b.queue, b.consumerTag)
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
			return b.Close(ctx)
		}
	}
}

func (b *Broker[T]) handle(ctx context.Context, delivery amqp.Delivery, handler mq.MessageHandler[T]) {
	var msg T
	if err := json.Unmarshal(delivery.Body, &msg); err != nil {
		delivery.Nack(false, false)
		return
	}

	b.logger.Info("mq: received new message", slog.String("message_id", delivery.MessageId))

	err := handler(ctx, msg)
	if err != nil {
		delivery.Reject(false)
		return
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
	return errors.Join(
		b.pub.close(ctx),
		b.sub.close(ctx),
	)
}
