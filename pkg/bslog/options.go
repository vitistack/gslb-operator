package bslog

import (
	"io"
	"log/slog"

	"github.com/vitistack/gslb-operator/pkg/bslog/handlers"
)

var CustomLevelNames = map[slog.Level]string{
	LevelHealthCheck: "HEALTHCHECK",
	LevelFatal: "FATAL",
}

type ReplaceAttrFunc func(groups []string, a slog.Attr) slog.Attr

type handlerFactory func(io.Writer) slog.Handler
type handlerOption func(handlerFactory) handlerFactory

func NewHandler(out io.Writer, factory handlerFactory, opts ...handlerOption) slog.Handler {
	for _, opt := range opts {
		factory = opt(factory)
	}

	return factory(out)
}

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

func InDevMode() handlerOption {
	return func(factory handlerFactory) handlerFactory {
		return func(w io.Writer) slog.Handler {
			return handlers.NewDevModeHandler(factory(w))
		}
	}
}

func WithColor() handlerOption {
	return func(factory handlerFactory) handlerFactory {
		return func(w io.Writer) slog.Handler {
			return handlers.NewColorHandler(w, factory)
		}
	}
}

func WithSplunkMultiHandler(secret, index string, threshold slog.Level) handlerOption {
	return func(factory handlerFactory) handlerFactory {
		return func(w io.Writer) slog.Handler {
			base := factory(w)
			return slog.NewMultiHandler(
				base,
				handlers.NewSplunkHandler(secret, index, threshold, base),
			)
		}
	}
}
