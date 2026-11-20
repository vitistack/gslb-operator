package router

import (
	"context"
	"fmt"
	"log/slog"
	"maps"
	"net/http"
	"slices"
	"strings"

	"github.com/vitistack/gslb-operator/pkg/auth/authz"
	"github.com/vitistack/gslb-operator/pkg/rest/middleware"
)

// Router is the root RouteGroup plus the wiring needed to build the handler.
type Router struct {
	*RouteGroup
	mux         *http.ServeMux
	logger      *slog.Logger
	global      []middleware.MiddlewareFunc
	objectTypes map[string]struct{}
}

func New(logger *slog.Logger, global ...middleware.MiddlewareFunc) *Router {
	return &Router{
		RouteGroup:  &RouteGroup{},
		mux:         http.NewServeMux(),
		logger:      logger,
		global:      global,
		objectTypes: map[string]struct{}{},
	}
}

// Build flattens the group tree, derives each secured route's object type
// (from its path) and action (from its method), wires middleware +
// enforcement, and returns the handler. It fails fast so a misconfigured route
// can never reach serving.
func (rt *Router) Build(enforcer *authz.Enforcer) (http.Handler, error) {
	logger := rt.logger
	if logger == nil {
		logger = slog.Default()
	}
	logRoutes := logger.Enabled(context.Background(), slog.LevelDebug)
	seenRoute := map[string]struct{}{}

	var walk func(g *RouteGroup, prefix, objectPrefix string, mws []middleware.MiddlewareFunc, public bool, globals []middleware.MiddlewareFunc) error
	walk = func(g *RouteGroup, prefix, objectPrefix string, mws []middleware.MiddlewareFunc, public bool, globals []middleware.MiddlewareFunc) error {
		gp := joinPattern(prefix, g.prefix)
		gop := objectPrefix
		switch {
		case g.objectRoot:
			gop = ""
		case !g.versionSegment:
			gop = joinPattern(objectPrefix, g.prefix)
		}

		gmws := append(append([]middleware.MiddlewareFunc{}, mws...), g.mws...)
		gpublic := public || g.public
		gglobals := globals
		if g.globalOverride {
			gglobals = g.globalMws
		}

		for _, r := range g.routes {
			full := joinPattern(gp, r.pattern)
			isPublic := gpublic || r.public

			key := r.method + " " + full
			if _, dup := seenRoute[key]; dup {
				return fmt.Errorf("duplicate route %q", key)
			}
			seenRoute[key] = struct{}{}

			rglobals := gglobals
			if r.globalOverride {
				rglobals = r.globalMws
			}

			chain := append([]middleware.MiddlewareFunc{}, rglobals...)
			chain = append(chain, gmws...)
			chain = append(chain, r.mws...)
			chain = append(chain, rt.requirePathValues(full))

			var objectType, action string
			if !isPublic {
				var ok bool
				if action, ok = authz.ActionFor(r.method); !ok {
					return fmt.Errorf("route %s: method has no action (mark Public to opt out)", key)
				}
				if objectType = authz.ObjectType(joinPattern(gop, r.pattern)); objectType == "" {
					return fmt.Errorf("route %s: path yields no object type (mark Public to opt out)", key)
				}
				rt.objectTypes[objectType] = struct{}{}
				if enforcer != nil {
					chain = append(chain, enforcer.Enforce(objectType, action, full, logger))
				}
			}

			if logRoutes {
				logger.Debug("registered route",
					slog.String("route", key),
					slog.Bool("public", isPublic),
					slog.String("object_type", objectType),
					slog.String("action", action),
				)
			}

			rt.mux.HandleFunc(key, middleware.Chain(chain...)(rt.adapt(full, r.handler)))
		}

		for _, c := range g.children {
			if err := walk(c, gp, gop, gmws, gpublic, gglobals); err != nil {
				return err
			}
		}
		return nil
	}

	if err := walk(rt.RouteGroup, "", "", nil, false, rt.global); err != nil {
		return nil, err
	}

	return rt.mux, nil
}

// ObjectTypes returns the object types of all secured routes so the policy
// loader can reject paths that don't exist. Valid after Build.
func (rt *Router) ObjectTypes() []string {
	return slices.Sorted(maps.Keys(rt.objectTypes))
}

func joinPattern(prefix, seg string) string {
	switch {
	case seg == "":
		return prefix
	case prefix == "":
		return seg
	}
	return strings.TrimSuffix(prefix, "/") + "/" + strings.TrimPrefix(seg, "/")
}
