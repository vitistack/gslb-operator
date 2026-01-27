package bslog

import (
	"context"
	"log/slog"
	"os"
)

func NewHandler(base slog.Handler, opts ...handlerOption) slog.Handler {
	for _, opt := range opts {
		base = opt(base)
	}

	return base
}

func Debug(msg string, args ...any) {
	slog.Debug(msg, args...)
}

func DebugContext(ctx context.Context, msg string, args ...any) {
	slog.DebugContext(ctx, msg, args...)
}

func Info(msg string, args ...any) {
	slog.Info(msg, args...)
}

func InfoContext(ctx context.Context, msg string, args ...any) {
	slog.InfoContext(ctx, msg, args...)
}

func Warn(msg string, args ...any) {
	slog.Warn(msg, args...)
}

func WarnContext(ctx context.Context, msg string, args ...any) {
	slog.WarnContext(ctx, msg, args...)
}

func Error(msg string, args ...any) {
	slog.Error(msg, args...)
}

func ErrorContext(ctx context.Context, msg string, args ...any) {
	slog.ErrorContext(ctx, msg, args...)
}

func Fatal(msg string, args ...any) {
	slog.Log(context.Background(), LevelFatal, msg, args...)
	os.Exit(1)
}

func FatalContext(ctx context.Context, msg string, args ...any) {
	slog.Log(ctx, LevelFatal, msg, args...)
	panic(msg)
}
