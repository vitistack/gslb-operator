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

type connection struct {
	conn             *amqp.Connection
	channel          *amqp.Channel
	connectionClosed chan *amqp.Error
	channelClosed    chan *amqp.Error
	channelCancelled chan string
	ready            chan struct{} // wait for connection reconnect
	logger           *slog.Logger
	amqpURL          string
	retry            int // amount of times to retry on connection failure
	lock             *sync.Mutex

	// calls function on successfull creation of new connection
	onNewConnection func() error
}

func newConnection(ctx context.Context, amqpUrl string) *connection {
	c := &connection{
		ready:   make(chan struct{}),
		logger:  slog.Default(),
		retry:   5,
		amqpURL: amqpUrl,
		lock:    &sync.Mutex{},
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

func (c *connection) reconnect() {
	err := c.connect()
	for err != nil {
		c.logger.Error("mq: failed to reconnect", slog.String("reason", err.Error()))
		time.Sleep(time.Second * 30)
		c.logger.Info("mq: retrying reconnection")
		err = c.connect()
	}
}

func (c *connection) connect() error {
	c.lock.Lock()
	c.ready = make(chan struct{})
	c.lock.Unlock()

	var err error = errors.New("nil") // init to atleast try once
	counter := 0

	for err != nil && counter < c.retry {
		err = nil
		c.conn, err = amqp.DialConfig(c.amqpURL,
			amqp.Config{
				Heartbeat:       time.Second * 10,
				TLSClientConfig: &tls.Config{},
			},
		)
		if err != nil {
			err = fmt.Errorf("failed to connect to message queue: %w", err)

			sleepFor := connectionRetryBackoff * time.Duration(counter+1)
			time.Sleep(sleepFor)
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

	if err != nil {
		return err
	}

	// since amqp - lib closes the channels on close
	// we need to create them again for every new connection that is made
	c.connectionClosed = make(chan *amqp.Error, 1)
	c.channelClosed = make(chan *amqp.Error, 1)
	c.channelCancelled = make(chan string, 1)

	c.conn.NotifyClose(c.connectionClosed)
	c.channel.NotifyClose(c.channelClosed)
	c.channel.NotifyCancel(c.channelCancelled)

	err = c.onNewConnection()
	if err != nil {
		return fmt.Errorf("failed to establish connection: %w", err)
	}

	close(c.ready) // broadcast ready
	c.logger.Info("mq: successfully established connection")
	return nil
}

// handleConnection ensures that the amqp connection remains intact
// and reconnects when notified of connection issues
func (c *connection) handleConnection(ctx context.Context) {
	for {
		c.Wait(ctx)

		c.lock.Lock()
		connectionClosed := c.connectionClosed
		channelClosed := c.channelClosed
		channelCancelled := c.channelCancelled
		c.lock.Unlock()

		select {
		case <-ctx.Done():
			return

		case connClosed := <-connectionClosed:
			c.logger.Warn("mq: connection closed unexpectedly",
				slog.String("reason", connClosed.Reason),
				slog.String("error", connClosed.Error()),
			)

			c.reconnect()

		case chanClosed := <-channelClosed:
			c.logger.Warn("mq: channel closed unexpectedly",
				slog.String("reason", chanClosed.Reason),
				slog.String("error", chanClosed.Error()),
			)

			if !c.conn.IsClosed() {
				err := c.conn.Close()
				if err != nil {
					c.logger.Error("mq: failed to close connection")
				}
			}

			if !c.channel.IsClosed() {
				err := c.channel.Close()
				if err != nil {
					c.logger.Error("mq: failed to close channel")
				}
			}

			c.reconnect()

		case reason := <-channelCancelled:

			c.logger.Warn("mq: channel cancelled unexpectedly",
				slog.String("reason", reason),
			)

			if !c.conn.IsClosed() {
				err := c.conn.Close()
				if err != nil {
					c.logger.Error("mq: failed to close connection")
				}
			}

			c.reconnect()
		}
	}
}

// waits for connection to be ready/initialized
func (c *connection) Wait(ctx context.Context) {
	select {
	case <-ctx.Done():
		return
	case <-c.ready:
		return
	}
}
