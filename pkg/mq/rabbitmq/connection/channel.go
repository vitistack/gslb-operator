package connection

import (
	"context"
	"fmt"

	amqp "github.com/rabbitmq/amqp091-go"
)

type Topology func(ch *Channel) error

type Channel struct {
	conn *Connection
	*amqp.Channel

	prefetch int

	// used to declare topology on a new channel
	Topology Topology

	ChannelClosed    chan *amqp.Error
	ChannelCancelled chan string
}

func (c *Channel) GetConnection() *Connection {
	return c.conn
}

func (c *Channel) Publish(ctx context.Context, exchange, queue string, publishing amqp.Publishing) error {
	c.conn.Wait(ctx)
	return c.Channel.PublishWithContext(
		ctx,
		exchange,
		queue,
		false,
		false,
		publishing,
	)
}

func (c *Channel) Subscribe(ctx context.Context, queue, consumerTag string) (<-chan amqp.Delivery, error) {
	c.conn.Wait(ctx)
	if ctx.Err() != nil {
		return nil, fmt.Errorf("mq: failed to subscribe: %w", ctx.Err())
	}

	// Limit in-flight unACK'd messages — backpressure against slow handlers.
	if err := c.Channel.Qos(c.prefetch, 0, false); err != nil {
		return nil, fmt.Errorf("mq set QoS: %w", err)
	}

	return c.Channel.ConsumeWithContext(ctx,
		queue,
		consumerTag,
		false,
		false,
		false,
		false,
		nil,
	)
}

func (c *Channel) Close() error {
	if !c.Channel.IsClosed() {
		if err := c.Channel.Close(); err != nil {
			return fmt.Errorf("failed to close channel: %w", err)
		}
	}
	return nil
}
