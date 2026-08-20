package rabbitmq

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

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
	channel     *connection.Channel
	queue       string
	exchange    string
	dlx         string
	dlq         string
	consumerTag string
	prefetch    int
	logger      *slog.Logger

	lock      sync.Mutex
	ready     chan struct{}
	chanReady chan *connection.Channel

	retry Retryer
}

func New[T any](ctx context.Context, ampqURL string, opts ...brokerOption[T]) mq.MessageBroker[T] {
	conn := connection.NewConnection(ctx, ampqURL)

	broker := &Broker[T]{
		logger:    slog.Default(),
		chanReady: make(chan *connection.Channel),
		ready:     make(chan struct{}),
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

	conn.OnNewConnection(
		func() error {
			broker.logger.Debug("mq: received new connection declaring topology")
			return broker.retry.Retry(func() error {
				ch, err := conn.NewChannel(broker.prefetch, broker.declareTopology)
				if err != nil {
					broker.logger.Error("mq: broker failed to declare channel", slog.String("reason", err.Error()))
					return err
				}
				broker.chanReady <- ch
				return nil
			})
		},
	)

	go broker.handleChannel(ctx)

	return broker
}

func (b *Broker[T]) getChannel(ctx context.Context) (*connection.Channel, error) {
	for {
		b.lock.Lock()
		ch := b.channel
		ready := b.ready
		b.lock.Unlock()

		if ch != nil {
			return ch, nil
		}

		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("mq: waiting for channel: %w", ctx.Err())

		case <-ready:
		}
	}
}

func (b *Broker[T]) handleChannel(ctx context.Context) {
	var channel *connection.Channel

	setChannel := func(ch *connection.Channel) {
		channel = ch
		b.lock.Lock()
		b.channel = ch
		if b.ready != nil {
			close(b.ready)
			b.ready = nil
		}
		b.lock.Unlock()
	}

	clearChannel := func() {
		b.lock.Lock()
		b.channel = nil
		if b.ready == nil {
			b.ready = make(chan struct{})
		}
		b.lock.Unlock()
	}

	redeclare := func() *connection.Channel {
		var ch *connection.Channel
		b.retry.Retry(func() error {
			newCh, err := channel.GetConnection().NewChannel(b.prefetch, b.declareTopology)
			if err != nil {
				return err
			}
			ch = newCh
			return nil
		})
		return ch
	}

	for channel == nil {
		select {
		case <-ctx.Done():
			return
		case ch := <-b.chanReady:
			setChannel(ch)
		}
	}

	for {
		select {
		case <-ctx.Done():
			channel.Close()
			return

		case ch := <-b.chanReady:
			setChannel(ch)

		case chanClosed, ok := <-channel.ChannelClosed:
			if !ok {
				continue
			}
			b.logger.Warn("mq: channel closed unexpectedly",
				slog.String("reason", chanClosed.Reason),
				slog.String("error", chanClosed.Error()),
			)
			clearChannel()
			setChannel(redeclare())

		case reason, ok := <-channel.ChannelCancelled:
			if !ok {
				continue
			}
			b.logger.Warn("mq: channel cancelled unexpectedly",
				slog.String("reason", reason),
			)
			clearChannel()
			setChannel(redeclare())
		}
	}
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
		if err := channel.ExchangeDeclare(
			b.exchange,
			amqp.ExchangeDirect,
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

func (b *Broker[T]) Publish(ctx context.Context, msg T) error {
	body, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("could not marshall message body: %w", err)
	}

	channel, err := b.getChannel(ctx)
	if err != nil {
		return fmt.Errorf("mq: broker failed to retrieve channel: %w", err)
	}
	return channel.Publish(
		ctx,
		b.exchange,
		b.queue,
		amqp.Publishing{
			ContentType:  "application/json",
			DeliveryMode: amqp.Transient,
			Body:         body,
			Timestamp:    time.Now(),
		},
	)
}

func (b *Broker[T]) Subscribe(ctx context.Context, handler mq.MessageHandler[T]) error {
	channel, err := b.getChannel(ctx)
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
	channel, err := b.getChannel(ctx)
	if err != nil {
		return fmt.Errorf("mq: broker failed to retrieve channel: %w", err)
	}
	return channel.Close()
}
