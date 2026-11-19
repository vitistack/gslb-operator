package router

import (
	"log/slog"
	"net/http"
	"strings"

	"github.com/vitistack/gslb-operator/pkg/auth/authc"
	"github.com/vitistack/gslb-operator/pkg/auth/authz"
	"github.com/vitistack/gslb-operator/pkg/bslog"
	"github.com/vitistack/gslb-operator/pkg/rest/middleware"
	"github.com/vitistack/gslb-operator/pkg/rest/response"
)

type HandlerFunc func(w http.ResponseWriter, r *Request)

type Request struct {
	*http.Request
	logger *slog.Logger
	params map[string]string
}

func NewRequest(r *http.Request, logger *slog.Logger, pathValues ...string) *Request {
	req := &Request{
		Request: r,
		logger: &bslog.NewWithCtx(r.Context(), logger).Logger,
		params: make(map[string]string, len(pathValues)),
	}

	for _, name := range pathValues {
		req.params[name] = r.PathValue(name)
	}

	return req
}

func (r *Request) Param(name string) string { return r.params[name] }

func (r *Request) Principal() (authc.Principal, bool) { return authc.PrincipalFrom(r.Context()) }

func (r *Request) Grant() (authz.Grant, bool) { return authz.GrantFrom(r.Context()) }

func (r *Request) Log() *slog.Logger { return r.logger }

func (rt *Router) adapt(pattern string, h HandlerFunc) http.HandlerFunc {
	names, _ := pathValues(pattern)
	return func(w http.ResponseWriter, r *http.Request) {
		attrs := make([]slog.Attr, 0, len(names))
		for _, name := range names {
			attrs = append(attrs, slog.String(name, r.PathValue(name)))
		}
		h(w, NewRequest(r.WithContext(bslog.CtxWithAttrs(r.Context(), attrs...)), rt.logger, names...))
	}
}

func (rt *Router) requirePathValues(pattern string) middleware.MiddlewareFunc {
	_, required := pathValues(pattern)
	if len(required) == 0 {
		return middleware.Passthrough()
	}

	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			logger := rt.logger

			for _, name := range required {
				if r.PathValue(name) == "" {
					bslog.NewWithCtx(r.Context(), logger).Info("skipping due to insufficient input parameter",
						slog.String("reason", "missing path-value: "+name),
					)
					response.Err(w, response.ErrInvalidInput, "invalid path")
					return
				}
			}

			next(w, r)
		}
	}
}

func pathValues(pattern string) (all, required []string) {
	for segment := range strings.SplitSeq(pattern, "/") {
		inner, ok := strings.CutPrefix(segment, "{")
		if !ok {
			continue
		}

		inner, ok = strings.CutSuffix(inner, "}")
		if !ok || inner == "$" {
			continue
		}

		name, isRest := strings.CutSuffix(inner, "...")
		all = append(all, name)
		if !isRest {
			required = append(required, name)
		}
	}

	return all, required
}
