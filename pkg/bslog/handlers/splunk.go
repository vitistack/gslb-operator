package handlers

import (
	"context"
	"log"
	"log/slog"
)

// sending all log messages to splunk that are above a given loglevel threshold
type SplunkHandler struct {
	LevelThreshold slog.Level
	secret         string
	index          string
	base           slog.Handler
}

func NewSplunkHandler(secret, index string, threshold slog.Level, base slog.Handler) *SplunkHandler {
	handler := &SplunkHandler{
		LevelThreshold: threshold,
		secret:         secret,
		index:          index,
		base:           base,
	}

	return handler
}

func (h SplunkHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return level > h.LevelThreshold
}

// sends record to splunk
func (h SplunkHandler) Handle(ctx context.Context, record slog.Record) error {
	log.Println("UN-IMPLEMENTED SPLUNK HANDLE")
	return nil
}

func (h SplunkHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &SplunkHandler{
		LevelThreshold: h.LevelThreshold,
		secret:         h.secret,
		base:           h.base.WithAttrs(attrs),
	}
}

func (h SplunkHandler) WithGroup(name string) slog.Handler {
	return &SplunkHandler{
		LevelThreshold: h.LevelThreshold,
		secret:         h.secret,
		base:           h.base.WithGroup(name),
	}
}
