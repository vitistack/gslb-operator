package rabbitmq

import (
    "context"
    "fmt"
    "log/slog"
    "sync"

    "github.com/vitistack/gslb-operator/pkg/mq/rabbitmq/connection"
)

// channelHandle manages the lifecycle of a single channel on a connection,
// re-declaring it whenever the underlying connection or channel is lost.
type channelHandle struct {
    conn     *connection.Connection
    prefetch int
    topology connection.Topology
    logger   *slog.Logger
    retry    Retryer

    lock      sync.Mutex
    channel   *connection.Channel
    ready     chan struct{}
    chanReady chan *connection.Channel
}

func newChannelHandle(conn *connection.Connection, prefetch int, topology connection.Topology, logger *slog.Logger, retry Retryer) *channelHandle {
    return &channelHandle{
        conn:      conn,
        prefetch:  prefetch,
        topology:  topology,
        logger:    logger,
        retry:     retry,
        ready:     make(chan struct{}),
        chanReady: make(chan *connection.Channel),
    }
}

// start launches the lifecycle handler and requests a fresh channel whenever
// the underlying connection is (re)established.
func (h *channelHandle) start(ctx context.Context) {
    go h.run(ctx)

    h.conn.OnNewConnection(func() error {
        h.logger.Debug("mq: received new connection declaring topology")
        return h.retry.Retry(func() error {
            ch, err := h.conn.NewChannel(h.prefetch, h.topology)
            if err != nil {
                h.logger.Error("mq: failed to declare channel", slog.String("reason", err.Error()))
                return err
            }
            h.chanReady <- ch
            return nil
        })
    })
}

func (h *channelHandle) get(ctx context.Context) (*connection.Channel, error) {
    for {
        h.lock.Lock()
        ch := h.channel
        ready := h.ready
        h.lock.Unlock()

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

func (h *channelHandle) run(ctx context.Context) {
    var channel *connection.Channel

    setChannel := func(ch *connection.Channel) {
        channel = ch
        h.lock.Lock()
        h.channel = ch
        if h.ready != nil {
            close(h.ready)
            h.ready = nil
        }
        h.lock.Unlock()
    }

    clearChannel := func() {
        h.lock.Lock()
        h.channel = nil
        if h.ready == nil {
            h.ready = make(chan struct{})
        }
        h.lock.Unlock()
    }

    redeclare := func() *connection.Channel {
        var ch *connection.Channel
        h.retry.Retry(func() error {
            newCh, err := channel.GetConnection().NewChannel(h.prefetch, h.topology)
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
        case ch := <-h.chanReady:
            setChannel(ch)
        }
    }

    for {
        select {
        case <-ctx.Done():
            channel.Close()
            return

        case ch := <-h.chanReady:
            setChannel(ch)

        case chanClosed, ok := <-channel.ChannelClosed:
            if !ok {
                continue
            }
            h.logger.Warn("mq: channel closed unexpectedly",
                slog.String("reason", chanClosed.Reason),
                slog.String("error", chanClosed.Error()),
            )
            clearChannel()
            setChannel(redeclare())

        case reason, ok := <-channel.ChannelCancelled:
            if !ok {
                continue
            }
            h.logger.Warn("mq: channel cancelled unexpectedly",
                slog.String("reason", reason),
            )
            clearChannel()
            setChannel(redeclare())
        }
    }
}

func (h *channelHandle) close(ctx context.Context) error {
    ch, err := h.get(ctx)
    if err != nil {
        return fmt.Errorf("mq: failed to retrieve channel: %w", err)
    }
    return ch.Close()
}