package auth

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"

	"github.com/vitistack/gslb-operator/pkg/auth/jwt"
	"github.com/vitistack/gslb-operator/pkg/rest/middleware"
)

func WithTokenValidation(logger *slog.Logger) middleware.MiddlewareFunc {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			ctx := context.WithValue(r.Context(), "request_method", r.Method)
			ctx = context.WithValue(ctx, "request_route", r.URL.String())

			resp, err := jwt.Validate(ctx, strings.Split(r.Header.Get("Authorization"), "Bearer")[1])
			if err != nil {
				logger.Error("token-validation failed", slog.String("reason", err.Error()))
				w.WriteHeader(resp.Code)
				json.NewEncoder(w).Encode(resp)
				return
			}
			next.ServeHTTP(w, r)
		}
	}
}
