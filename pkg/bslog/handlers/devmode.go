package handlers

import (
	"context"
	"log/slog"
	"runtime"
)

type DevModeOptions struct {
	slog.HandlerOptions
}

type Devmode struct {
	base slog.Handler
}

func NewDevModeHandler(base slog.Handler) slog.Handler {
	h := Devmode{
		base: base,
	}

	return h
}

func (dm Devmode) Enabled(ctx context.Context, level slog.Level) bool {
	return dm.base.Enabled(ctx, level)
}

func (dm Devmode) Handle(ctx context.Context, record slog.Record) error {
	record.AddAttrs(
		slog.String("env", "dev"),
	)

	if pc := record.PC; pc != 0 {
		pc, _, _, ok := runtime.Caller(4)
		if !ok {
			return dm.base.Handle(ctx, record)
		}

		fs := runtime.CallersFrames([]uintptr{pc})
		f, _ := fs.Next()
		record.AddAttrs(
			slog.Group(
				"caller_meta_data",
				slog.String("func", f.Function),
				slog.String("file", f.File),
				slog.Int("line", f.Line),
			),
		)
	}

	return dm.base.Handle(ctx, record)
}

func (dm Devmode) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &Devmode{
		base: dm.base.WithAttrs(attrs),
	}
}

func (dm Devmode) WithGroup(name string) slog.Handler {
	return &Devmode{
		base: dm.base.WithGroup(name),
	}
}
