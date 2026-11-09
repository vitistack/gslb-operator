package router

import (
	"log/slog"
	"net/http"
	"strings"

	"github.com/vitistack/gslb-operator/pkg/rest/middleware"
	"github.com/vitistack/gslb-operator/pkg/rest/response"
)

type HandlerFunc func(w http.ResponseWriter, r *Request)

type Request struct {
	*http.Request
	logger *slog.Logger
	params map[string]string
}

func NewRequest(r *http.Request, logger *slog.Logger, pathParams ...string) *Request {
	if logger == nil {
		logger = slog.Default()
	}

	req := &Request{
		Request: r,
		logger:  logger,
		params:  make(map[string]string, len(pathParams)),
	}

	attrs := make([]any, 0, len(pathParams)+1)
	attrs = append(attrs, slog.Any("request_id", r.Context().Value("id")))
	for _, name := range pathParams {
		v := r.PathValue(name)
		req.params[name] = v
		attrs = append(attrs, slog.String(name, v))
	}

	req.logger = logger.With(attrs...)

	return req
}

func (r *Request) Param(name string) string { return r.params[name] }

func (r *Request) Log() *slog.Logger { return r.logger }

func (rt *Router) adapt(pattern string, h HandlerFunc) http.HandlerFunc {
	names, _ := pathValues(pattern)
	return func(w http.ResponseWriter, r *http.Request) {
		h(w, NewRequest(r, rt.logger, names...))
	}
}

func (rt *Router) requirePathValues(pattern string) middleware.MiddlewareFunc {
	_, required := pathValues(pattern)
	if len(required) == 0 {
		return middleware.Passthrough()
	}

	logger := rt.logger
	if logger == nil {
		logger = slog.Default()
	}

	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			for _, name := range required {
				if r.PathValue(name) == "" {
					logger.Info("skipping due to insufficient input parameter",
						slog.Any("request_id", r.Context().Value("id")),
						slog.String("reason", "missing path-value "+name),
					)
					response.Err(w, response.ErrInvalidInput, "missing path-value: "+name)
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
