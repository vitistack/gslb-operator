package middleware

import (
	"log/slog"
	"net/http"
	"net/netip"
	"slices"
	"strings"
	"time"
	"uuid"

	"github.com/vitistack/gslb-operator/pkg/bslog"
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

// Unwrap lets http.ResponseController reach Flush/Hijack on the underlying writer.
func (rw *ResponseWriter) Unwrap() http.ResponseWriter { return rw.ResponseWriter }

func WithRequestLogging(logger *slog.Logger, level slog.Level, trustedPrefixes []netip.Prefix) MiddlewareFunc {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			attrs := []slog.Attr{
				slog.String("requestId", uuid.NewV7().String()),
				slog.GroupAttrs(
					"requestMetaData",
					slog.String("http", r.Proto),
					slog.String("method", r.Method),
					slog.String("remoteHost", Addr(r.RemoteAddr, r.Header, trustedPrefixes)),
					slog.String("route", r.URL.Path),
					slog.String("query", r.URL.Query().Encode()),
					slog.String("userAgent", r.UserAgent()),
				),
			}

			parent := r.Context()
			r = r.WithContext(bslog.CtxWithAttrs(parent, attrs...))
			reqLogger := bslog.NewWithCtx(r.Context(), logger)

			rw := &ResponseWriter{ResponseWriter: w, StatusCode: http.StatusOK}
			next.ServeHTTP(rw, r)

			reqLogger.Log(r.Context(), level,
				"incoming request",
				slog.Int("status_code", rw.StatusCode),
				slog.Int("bytes", rw.Bytes),
				slog.String("content_type", rw.Header().Get("Content-Type")),
				slog.Int64("tookMs", time.Since(start).Milliseconds()),
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
