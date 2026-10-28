package middleware

import (
	"slices"
	"context"
	"log/slog"
	"net/http"
	"uuid"
)

type MiddlewareFunc func(next http.HandlerFunc) http.HandlerFunc

func Chain(mws ...MiddlewareFunc) MiddlewareFunc {
	return func(next http.HandlerFunc) http.HandlerFunc {
		for _, mw := range slices.Backward(mws) {
			next = mw(next)
		}
		return next
	}
}

func WithIncomingRequestLogging(logger *slog.Logger) MiddlewareFunc {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			parent := r.Context() // re-use request context

			r = r.WithContext(context.WithValue(parent, "id", uuid.NewV7().String()))

			logger.Info("incoming request",
				slog.GroupAttrs(
					"meta_data",
					slog.Any("request_id", r.Context().Value("id")),
					slog.String("method", r.Method),
					slog.String("remote_host", r.RemoteAddr),
					slog.String("route", r.URL.String()),
					slog.String("query", r.URL.RawQuery),
					slog.String("user_agent", r.UserAgent()),
				),
			)

			next.ServeHTTP(w, r)
		}
	}
}

func Passthrough() MiddlewareFunc {
	return func(next http.HandlerFunc) http.HandlerFunc { return next }
}
