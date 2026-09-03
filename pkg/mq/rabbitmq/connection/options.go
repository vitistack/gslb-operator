package connection

import (
	"log/slog"
	"time"
)

func WithLogger(l *slog.Logger) connectionOption {
	return func(c *Connection) { c.logger = l }
}

func WithRetryBackoff(d time.Duration) connectionOption {
	return func(c *Connection) { c.retryConnectionBackoff = d }
}
