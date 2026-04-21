package handlers

import (
	"context"
	"log/slog"
)

type SplunkHandlerOptions struct {
	slog.HandlerOptions
}

type SplunkHandler struct {
	LevelThreshold slog.Level
	secret         string
	base           slog.Handler
}

func NewSplunkHandler(secret string) *SplunkHandler {
	return &SplunkHandler{}
}

func (h SplunkHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return level > h.LevelThreshold
}

// sends record to splunk
func (h SplunkHandler) Handle(ctx context.Context, record slog.Record) error {
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
