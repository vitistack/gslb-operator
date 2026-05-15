package bslog

import "log/slog"

const (
	LevelHealthCheck slog.Level = slog.LevelDebug - 4
	LevelFatal       slog.Level = slog.LevelError + 4
)
