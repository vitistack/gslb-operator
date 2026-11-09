package middleware

import (
	"context"
	"log/slog"
	"net/http"
	"net/netip"
	"slices"
	"strings"
	"time"
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

type ResponseWriter struct {
	http.ResponseWriter
	StatusCode int
	Bytes      int
}

func (rw *ResponseWriter) WriteHeader(statusCode int) {
	rw.StatusCode = statusCode
	rw.ResponseWriter.WriteHeader(statusCode)
}

func (rw *ResponseWriter) Write(b []byte) (int, error) {
	n, err := rw.ResponseWriter.Write(b)
	rw.Bytes += n
	return n, err
}

func WithRequestLogging(logger *slog.Logger, level slog.Level, trustedPrefixes []netip.Prefix) MiddlewareFunc {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			parent := r.Context() // re-use request context

			r = r.WithContext(context.WithValue(parent, "id", uuid.NewV7().String()))
			requestID := slog.Any("request_id", r.Context().Value("id"))

			logger.Log(context.Background(), level, "incoming request",
				slog.GroupAttrs(
					"meta_data",
					requestID,
					slog.String("method", r.Method),
					slog.String("remote_host", Addr(r.RemoteAddr, r.Header, trustedPrefixes)),
					slog.String("route", r.URL.String()),
					slog.String("query", r.URL.RawQuery),
					slog.String("user_agent", r.UserAgent()),
				),
			)

			rw := &ResponseWriter{ResponseWriter: w, StatusCode: http.StatusOK}
			next.ServeHTTP(rw, r)

			logger.Log(r.Context(), level, "response",
				slog.GroupAttrs(
					"meta_data",
					requestID,
					slog.String("method", r.Method),
					slog.String("route", r.URL.Path),
					slog.Int("status_code", rw.StatusCode),
					slog.Int("bytes", rw.Bytes),
					slog.String("content_type", rw.Header().Get("Content-Type")),
					slog.String("took", time.Since(start).String()),
				),
			)
		}
	}
}

func Addr(remoteAddr string, headers http.Header, trustedPrefixes []netip.Prefix) string {
	ip, err := netip.ParseAddrPort(remoteAddr)
	if err != nil {
		return remoteAddr
	}

	for _, prefix := range trustedPrefixes {
		if !prefix.Contains(ip.Addr()) {
			continue
		}

		if xForwardedFor := headers.Get("X-Forwarded-For"); xForwardedFor != "" {
			if addr, ok := xForwardedForUntrustedAddress(xForwardedFor, trustedPrefixes); ok {
				return addr
			}
		}

		if xRealIp := headers.Get("X-Real-Ip"); xRealIp != "" {
			return xRealIp
		}
	}

	return ip.Addr().String()
}

func xForwardedForUntrustedAddress(chain string, trustedPrefixes []netip.Prefix) (string, bool) {
	hops := strings.Split(chain, ",")

	for _, hop := range slices.Backward(hops) {
		candidate, err := netip.ParseAddr(strings.TrimSpace(hop))
		if err != nil {
			continue // malformed hop
		}

		trusted := false
		for _, prefix := range trustedPrefixes {
			if prefix.Contains(candidate) {
				trusted = true
				break
			}
		}

		if !trusted {
			return candidate.String(), true
		}
	}

	return "", false
}

func Passthrough() MiddlewareFunc {
	return func(next http.HandlerFunc) http.HandlerFunc { return next }
}
