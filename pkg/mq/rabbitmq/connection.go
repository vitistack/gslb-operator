package rabbitmq

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

const connectionRetryBackoff = time.Second // backoff on connection failure

type Connection interface {
	Close(context.Context) error
	Healthy() bool
}

type connection struct {
	conn             *amqp.Connection
	channel          *amqp.Channel
	connectionClosed chan *amqp.Error
	channelClosed    chan *amqp.Error
	channelCancelled chan *amqp.Error
	connection       *sync.WaitGroup // wait for connection reconnect
	logger           *slog.Logger
	amqpURL          string
	retry            int // amount of times to retry on connection failure
}

func newConnection(ctx context.Context, amqpUrl string) *connection {
	c := &connection{
		logger:  slog.Default(),
		retry:   1, // at least one try
		amqpURL: amqpUrl,
	}

	go c.handleConnection(ctx)
	return c
}

func (c *connection) Close(_ context.Context) error {
	return errors.Join(
		c.conn.Close(),
		c.channel.Close(),
	)
}

func (c *connection) Healthy() bool {
	if c.conn == nil || c.channel == nil {
		return false
	}

	return true
}

func (c *connection) connect() error {
	var err error
	counter := 0

	for err != nil && counter < c.retry {
		err = nil
		c.conn, err = amqp.DialTLS(c.amqpURL, &tls.Config{})
		if err != nil {
			err = fmt.Errorf("failed to connect to message queue: %w", err)

			counter++
			continue
		}

		c.channel, err = c.conn.Channel()
		if err != nil {
			c.conn.Close()
			err = fmt.Errorf("failed to open channel: %w", err)

			sleepFor := connectionRetryBackoff * time.Duration(counter+1)
			time.Sleep(sleepFor)
		}

		counter++
	}

	return err
}

// handleConnection ensures that the amqp connection remains intact
// and reconnects when notified of connection issues
func (c *connection) handleConnection(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return

		case connClosed := <-c.connectionClosed:
			c.connection.Add(1)
			c.logger.Info("mq: connection closed unexpectedly",
				slog.String("reason", connClosed.Reason),
				slog.String("error", connClosed.Error()),
			)

			err := c.connect()
			if err != nil {
				c.logger.Error("failed to reconnect", slog.String("reason", err.Error()))
			}
			c.connection.Done()

		case chanClosed := <-c.channelClosed:
			c.connection.Add(1)
			c.logger.Info("mq: channel closed unexpectedly",
				slog.String("reason", chanClosed.Reason),
				slog.String("error", chanClosed.Error()),
			)

			err := c.conn.Close()
			if err != nil {
				c.logger.Error("mq: failed to close connection")
			}

			err = c.connect()
			if err != nil {
				c.logger.Error("failed to reconnect", slog.String("reason", err.Error()))
			}
			c.connection.Done()

		case chanCancelled := <-c.channelCancelled:
			c.connection.Add(1)
			c.logger.Info("mq: channel cancelled unexpectedly",
				slog.String("reason", chanCancelled.Reason),
				slog.String("error", chanCancelled.Error()),
			)

			err := c.channel.Close()
			if err != nil {
				c.logger.Error("mq: failed to close channel")
			}

			err = c.conn.Close()
			if err != nil {
				c.logger.Error("mq: failed to close connection")
			}

			err = c.connect()
			if err != nil {
				c.logger.Error("failed to reconnect", slog.String("reason", err.Error()))
			}
			c.connection.Done()
		}
	}
}
