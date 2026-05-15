package handlers

import (
	"context"
	"log/slog"
	"path/filepath"
	"runtime"
	"strings"
)

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
	record.AddAttrs(slog.String("env", "dev"))

	if record.PC != 0 {
		if pc, _, _, ok := runtime.Caller(5); ok {
			f, _ := runtime.CallersFrames([]uintptr{pc}).Next()
			record.AddAttrs(
				slog.Group("caller_meta_data",
					slog.String("func", shortFunc(f.Function)),
					slog.String("file", filepath.Base(f.File)),
					slog.Int("line", f.Line),
				),
			)
		}
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

// shortFunc strips the package path from a fully qualified function name,
// e.g. "github.com/foo/bar/pkg.(*T).Method" -> "pkg.(*T).Method".
func shortFunc(fn string) string {
	if i := strings.LastIndex(fn, "/"); i >= 0 {
		return fn[i+1:]
	}
	return fn
}
