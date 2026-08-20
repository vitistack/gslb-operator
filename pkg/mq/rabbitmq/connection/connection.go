package connection

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
	"golang.org/x/sync/errgroup"
)

const ConnectionRetryBackoff = time.Second // backoff on Connection failure

var mqConn *Connection // global MQ connection
var lock sync.Mutex
var handleOnce sync.Once
var connOpts []connectionOption

type connectionOption func(*Connection)

type Connection struct {
	amqpURL string

	conn *amqp.Connection

	connectionClosed chan *amqp.Error

	ready chan struct{} // wait for Connection reconnect

	logger                 *slog.Logger
	retry                  int // amount of times to retry on Connection failure
	lock                   sync.Mutex
	retryConnectionBackoff time.Duration

	// calls function on successfull creation of new Connection
	postNewConnectionActions []func() error
}

func Configure(opts ...connectionOption) {
	lock.Lock()
	connOpts = opts
	lock.Unlock()
}

func NewConnection(ctx context.Context, amqpUrl string) *Connection {
	lock.Lock()
	if mqConn == nil {
		mqConn = &Connection{
			amqpURL: amqpUrl,
			ready:   make(chan struct{}),
			logger:  slog.Default(),
			retry:   5,
			lock:    sync.Mutex{},
		}

		for _, opt := range connOpts {
			opt(mqConn)
		}
	}

	handleOnce.Do(func() {
		go func() {
			err := mqConn.connect(ctx)
			for err != nil {
				mqConn.logger.Error("mq: failed to connect",
					slog.String("reason", err.Error()),
					slog.String("retry", mqConn.retryConnectionBackoff.String()),
				)
				time.Sleep(mqConn.retryConnectionBackoff)
				err = mqConn.connect(ctx)
			}
		}()
		go mqConn.handleConnection(ctx)
	})
	lock.Unlock()

	return mqConn
}

func (c *Connection) NewChannel(prefetch int, fn Topology) (*Channel, error) {
	ch, err := c.conn.Channel()
	if err != nil {
		return nil, fmt.Errorf("failed to declare new channel: %w", err)
	}
	channel := &Channel{
		conn:             c,
		Channel:          ch,
		prefetch:         prefetch,
		Topology:         fn,
		ChannelClosed:    make(chan *amqp.Error),
		ChannelCancelled: make(chan string),
	}
	channel.NotifyClose(channel.ChannelClosed)
	channel.NotifyCancel(channel.ChannelCancelled)

	if err := channel.Topology(channel); err != nil {
		return nil, fmt.Errorf("mq: channel failed to declare topology: %w", err)
	}

	return channel, nil
}

func (c *Connection) OnNewConnection(fn func() error) {
	c.lock.Lock()
	defer c.lock.Unlock()
	// connection already ready
	// execute the action immediatly
	if c.ready == nil {
		fn()
		c.postNewConnectionActions = append(c.postNewConnectionActions, fn)
		return
	}
	c.postNewConnectionActions = append(c.postNewConnectionActions, fn)

}

func (c *Connection) Close(_ context.Context) error {
	return c.conn.Close()
}

func (c *Connection) reconnect(ctx context.Context) {
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

func (c *Connection) connect(ctx context.Context) error {
	c.lock.Lock()
	if c.ready == nil {
		c.ready = make(chan struct{})
	}
	c.lock.Unlock()

	var conn *amqp.Connection

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

			sleepFor := ConnectionRetryBackoff * time.Duration(counter+1)
			time.Sleep(sleepFor)
			counter++
			continue
		}

		counter++
	}

	if err != nil {
		return err
	}

	c.conn = conn

	// since amqp - lib closes the channels on close
	// we need to create them again for every new Connection that is made
	c.lock.Lock()
	c.connectionClosed = make(chan *amqp.Error, 1)
	c.lock.Unlock()

	c.conn.NotifyClose(c.connectionClosed)

	c.OnSuccessFullConnection()

	close(c.ready) // broadcast ready
	c.ready = nil
	c.logger.Info("mq: successfully established Connection")

	return nil
}

func (c *Connection) OnSuccessFullConnection() {
	go func() {
		wg := errgroup.Group{}

		for _, job := range c.postNewConnectionActions {
			wg.Go(job)
		}

		if err := wg.Wait(); err != nil {
			c.logger.Error("")
		}
	}()
}

// handleConnection ensures that the amqp Connection remains intact
// and reconnects when notified of Connection issues
func (c *Connection) handleConnection(ctx context.Context) {
	c.Wait(ctx)

	for {
		c.lock.Lock()
		ConnectionClosed := c.connectionClosed
		c.lock.Unlock()

		select {
		case <-ctx.Done():
			c.logger.Debug("closing mq: connection")
			if closeErr := c.conn.Close(); closeErr != nil {
				c.logger.Info("mq: successfully closed connection")
			}
			return

		case connClosed, ok := <-ConnectionClosed:
			if !ok {
				continue
			}

			c.logger.Warn("mq: Connection closed unexpectedly",
				slog.String("reason", connClosed.Reason),
				slog.String("error", connClosed.Error()),
			)

			c.reconnect(ctx)
		}
	}
}

// waits for Connection to be ready/initialized
func (c *Connection) Wait(ctx context.Context) {
	c.lock.Lock()
	ready := c.ready
	c.lock.Unlock()

	// Connection already established
	if ready == nil {
		return
	}

	select {
	case <-ctx.Done():
		return
	case <-ready:
		return
	}
}
