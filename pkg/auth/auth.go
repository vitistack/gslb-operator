package auth

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/vitistack/gslb-operator/pkg/auth/jwt"
	"github.com/vitistack/gslb-operator/pkg/rest/middleware"
)

func WithTokenValidation() middleware.MiddlewareFunc {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			ctx := context.WithValue(r.Context(), "request_method", r.Method)
			ctx = context.WithValue(ctx, "request_route", r.URL.String())
			
			resp, err := jwt.Validate(ctx, r.Header.Get("Authorization"))
			if err != nil {
				w.WriteHeader(resp.Code)
				json.NewEncoder(w).Encode(resp)
				return
			}
			next.ServeHTTP(w, r)
		}
	}
}
