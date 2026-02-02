package bslog

import (
	"log/slog"

	"github.com/vitistack/gslb-operator/pkg/bslog/handlers"
)

var CustomLevelNames = map[slog.Level]string{
	LevelFatal: "FATAL",
}

type ReplaceAttrFunc func(groups []string, a slog.Attr) slog.Attr

func BaseReplaceAttr(groups []string, a slog.Attr) slog.Attr {
	if a.Key == slog.LevelKey {
		level := a.Value.Any().(slog.Level)
		levelLabel, exists := CustomLevelNames[level]

		if !exists {
			levelLabel = level.String()

		}
		a.Value = slog.StringValue(levelLabel)
	}

	if a.Value.Kind() == slog.KindString && a.Value.String() == "" { // if empty value in KEY:VALUE pair
		return slog.Attr{}
	}

	return a
}

type handlerOption func(base slog.Handler) slog.Handler

func InDevMode() handlerOption {
	return func(base slog.Handler) slog.Handler {
		return handlers.NewDevModeHandler(base)
	}
}
