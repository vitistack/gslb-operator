package rabbitmq

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

const connectionRetryBackoff = time.Second // backoff on connection failure

type connection struct {
	conn    *amqp.Connection
	channel *amqp.Channel
	amqpURL string

	connectionClosed chan *amqp.Error
	channelClosed    chan *amqp.Error
	channelCancelled chan string
	ready            chan struct{} // wait for connection reconnect

	logger                 *slog.Logger
	retry                  int // amount of times to retry on connection failure
	lock                   *sync.Mutex
	retryConnectionBackoff time.Duration

	// calls function on successfull creation of new connection
	onNewConnection func() error
}

func newConnection(ctx context.Context, amqpUrl string) *connection {
	c := &connection{
		amqpURL: amqpUrl,
		ready:   make(chan struct{}),
		logger:  slog.Default(),
		retry:   5,
		lock:    &sync.Mutex{},
	}

	go c.handleConnection(ctx)

	return c
}

func (c *connection) Close(_ context.Context) error {
	if c.conn != nil && c.channel != nil {
		return errors.Join(
			c.conn.Close(),
			c.channel.Close(),
		)
	}
	return nil
}

func (c *connection) reconnect(ctx context.Context) {
	if ctx.Err() != nil {
		return
	}
	err := c.connect(ctx)
	for err != nil {
		c.logger.Error("mq: failed to reconnect",
			slog.String("reason", err.Error()),
			slog.String("retry", c.retryConnectionBackoff.String()),
		)
		select {
		case <-ctx.Done():
			return
		case <-time.After(c.retryConnectionBackoff):
		}
		c.logger.Info("mq: retrying reconnection")
		err = c.connect(ctx)
	}
}

func (c *connection) connect(ctx context.Context) error {
	c.lock.Lock()
	if c.ready == nil {
		c.ready = make(chan struct{})
	}
	c.lock.Unlock()

	var conn *amqp.Connection
	var channel *amqp.Channel

	var err error = errors.New("nil") // init to atleast try once
	counter := 0

	for err != nil && counter < c.retry {
		err = nil
		conn, err = amqp.DialConfig(c.amqpURL,
			amqp.Config{
				Heartbeat:       time.Second * 5,
				TLSClientConfig: &tls.Config{},

				// do context aware dial
				Dial: func(network, addr string) (net.Conn, error) {
					return (&net.Dialer{}).DialContext(ctx, network, addr)
				},
			},
		)
		if err != nil {
			err = fmt.Errorf("failed to connect to message queue: %w", err)

			sleepFor := connectionRetryBackoff * time.Duration(counter+1)
			time.Sleep(sleepFor)
			counter++
			continue
		}

		channel, err = conn.Channel()
		if err != nil {
			conn.Close()
			err = fmt.Errorf("failed to open channel: %w", err)

			sleepFor := connectionRetryBackoff * time.Duration(counter+1)
			time.Sleep(sleepFor)
		}

		counter++
	}

	if err != nil {
		return err
	}

	c.conn = conn
	c.channel = channel

	// since amqp - lib closes the channels on close
	// we need to create them again for every new connection that is made
	c.lock.Lock()
	c.connectionClosed = make(chan *amqp.Error, 1)
	c.channelClosed = make(chan *amqp.Error, 1)
	c.channelCancelled = make(chan string, 1)
	c.lock.Unlock()

	c.conn.NotifyClose(c.connectionClosed)
	c.channel.NotifyClose(c.channelClosed)
	c.channel.NotifyCancel(c.channelCancelled)

	err = c.onNewConnection()
	if err != nil {
		return fmt.Errorf("failed to establish connection: %w", err)
	}

	close(c.ready) // broadcast ready
	c.ready = nil
	c.logger.Info("mq: successfully established connection")

	return nil
}

// handleConnection ensures that the amqp connection remains intact
// and reconnects when notified of connection issues
func (c *connection) handleConnection(ctx context.Context) {
	c.Wait(ctx)

	for {
		c.lock.Lock()
		connectionClosed := c.connectionClosed
		channelClosed := c.channelClosed
		channelCancelled := c.channelCancelled
		c.lock.Unlock()

		select {
		case <-ctx.Done():
			return

		case connClosed, ok := <-connectionClosed:
			if !ok {
				continue
			}

			c.logger.Warn("mq: connection closed unexpectedly",
				slog.String("reason", connClosed.Reason),
				slog.String("error", connClosed.Error()),
			)

			c.reconnect(ctx)

		case chanClosed, ok := <-channelClosed:
			if !ok {
				continue
			}

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

			c.reconnect(ctx)

		case reason, ok := <-channelCancelled:
			if !ok {
				continue
			}

			c.logger.Warn("mq: channel cancelled unexpectedly",
				slog.String("reason", reason),
			)

			if !c.conn.IsClosed() {
				err := c.conn.Close()
				if err != nil {
					c.logger.Error("mq: failed to close connection")
				}
			}

			c.reconnect(ctx)
		}
	}
}

// waits for connection to be ready/initialized
func (c *connection) Wait(ctx context.Context) {
	c.lock.Lock()
	ready := c.ready
	c.lock.Unlock()

	// connection already established
	if ready == nil {
		return
	}

	select {
	case <-ctx.Done():
		return
	case <-c.ready:
		return
	}
}
