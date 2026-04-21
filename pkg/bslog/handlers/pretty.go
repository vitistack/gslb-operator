package handlers

import (
	"context"
	"log/slog"
)

type PrettyHandler struct {
	base slog.Handler
}

func (h PrettyHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.base.Enabled(ctx, level)
}

func (h PrettyHandler) Handle(ctx context.Context, record slog.Record) error {
	return h.base.Handle(ctx, record)
}

func (h PrettyHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &PrettyHandler{
		base: h.base.WithAttrs(attrs),
	}
}

func (h PrettyHandler) WithGroup(name string) slog.Handler {
	return &PrettyHandler{
		base: h.base.WithGroup(name),
	}
}
