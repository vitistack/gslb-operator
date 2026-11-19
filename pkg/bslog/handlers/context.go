package handlers

import (
	"context"
	"log/slog"
	"slices"
)

type attrsKey struct{}

type ContextHandler struct {
	slog.Handler
	ctx context.Context
}

func NewContextHandler(ctx context.Context, base slog.Handler) ContextHandler {
	return ContextHandler{
		Handler: base,
		ctx:     ctx,
	}
}

func CtxWithAttrs(ctx context.Context, attrs ...slog.Attr) context.Context {
	prev, _ := ctx.Value(attrsKey{}).([]slog.Attr)
	return context.WithValue(ctx, attrsKey{}, append(slices.Clip(prev), attrs...))
}

func (h ContextHandler) Handle(ctx context.Context, r slog.Record) error {
	src := ctx
	if h.ctx != nil {
		src = h.ctx
	}
	if attrs, ok := src.Value(attrsKey{}).([]slog.Attr); ok {
		r.AddAttrs(attrs...)
	}
	return h.Handler.Handle(ctx, r)
}

func (h ContextHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return ContextHandler{
		Handler: h.Handler.WithAttrs(attrs),
		ctx:     h.ctx,
	}
}

func (h ContextHandler) WithGroup(name string) slog.Handler {
	return ContextHandler{
		Handler: h.Handler.WithGroup(name),
		ctx:     h.ctx,
	}
}
