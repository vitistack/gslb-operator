package router

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/vitistack/gslb-operator/pkg/auth/authz"
	"github.com/vitistack/gslb-operator/pkg/rest/middleware"
)

// Router is the root RouteGroup plus the wiring needed to build the handler.
type Router struct {
	*RouteGroup
	mux      *http.ServeMux
	enforcer *authz.Enforcer
	logger   *slog.Logger
	global   []middleware.MiddlewareFunc
}

func New(enforcer *authz.Enforcer, logger *slog.Logger, global ...middleware.MiddlewareFunc) *Router {
	return &Router{
		RouteGroup: &RouteGroup{},
		mux:        http.NewServeMux(),
		enforcer:   enforcer,
		logger:     logger,
		global:     global,
	}
}

// endpoint identifies a route independent of which version mounted it, so
// Build() can tell "same action, same endpoint, different version" (allowed)
// apart from "same action reused on a genuinely different endpoint" (rejected).
type endpoint struct {
	method    string
	canonical string
}

// Build flattens the group tree, validates the invariants, wires middleware +
// enforcement, and returns the handler. It fails fast so a misconfigured route
// can never reach serving.
func (rt *Router) Build() (http.Handler, error) {
	seenAction := map[authz.Action]endpoint{}
	seenRoute := map[string]struct{}{}

	var walk func(g *RouteGroup, prefix, canonicalPrefix string, mws []middleware.MiddlewareFunc, public bool) error
	walk = func(g *RouteGroup, prefix, canonicalPrefix string, mws []middleware.MiddlewareFunc, public bool) error {
		gp := joinPattern(prefix, g.prefix)
		gc := canonicalPrefix
		if !g.versionSegment {
			gc = joinPattern(canonicalPrefix, g.prefix)
		}
		gmws := append(append([]middleware.MiddlewareFunc{}, mws...), g.mws...)
		gpublic := public || g.public

		for _, r := range g.routes {
			full := joinPattern(gp, r.pattern)
			isPublic := gpublic || r.public

			if !isPublic && r.action == "" {
				return fmt.Errorf("route %s %s: missing action (mark Public to opt out)", r.method, full)
			}

			if r.action != "" {
				ep := endpoint{method: r.method, canonical: joinPattern(gc, r.pattern)}
				if prev, dup := seenAction[r.action]; dup && prev != ep {
					return fmt.Errorf("action %q reused by %q and %q", r.action,
						prev.method+" "+prev.canonical, ep.method+" "+ep.canonical)
				}
				seenAction[r.action] = ep
			}

			key := r.method + " " + full
			if _, dup := seenRoute[key]; dup {
				return fmt.Errorf("duplicate route %q", key)
			}
			seenRoute[key] = struct{}{}

			chain := append([]middleware.MiddlewareFunc{}, rt.global...)
			chain = append(chain, gmws...)
			chain = append(chain, r.mws...)
			if !isPublic && rt.enforcer != nil {
				chain = append(chain, rt.enforcer.Enforce(r.action, full, rt.logger))
			}

			rt.mux.HandleFunc(key, middleware.Chain(chain...)(r.handler))
		}

		for _, c := range g.children {
			if err := walk(c, gp, gc, gmws, gpublic); err != nil {
				return err
			}
		}
		return nil
	}

	if err := walk(rt.RouteGroup, "", "", nil, false); err != nil {
		return nil, err
	}

	rt.DebugRoutes() // optionally print the registered routes
	return rt.mux, nil
}

// Actions returns the registered action inventory (whole tree, de-duplicated
// since the same action can legitimately appear once per version plus once
// for the "latest" alias), so the policy loader can reject any role
// referencing an action that doesn't exist.
func (rt *Router) Actions() []authz.Action {
	seen := map[authz.Action]struct{}{}
	var out []authz.Action
	var walk func(g *RouteGroup)
	walk = func(g *RouteGroup) {
		for _, r := range g.routes {
			if r.action == "" {
				continue
			}
			if _, dup := seen[r.action]; dup {
				continue
			}
			seen[r.action] = struct{}{}
			out = append(out, r.action)
		}
		for _, c := range g.children {
			walk(c)
		}
	}
	walk(rt.RouteGroup)
	return out
}

// DebugRoutes logs every registered route with its resolved pattern, effective
// public flag, and action ("NoAction" when unset).
func (rt *Router) DebugRoutes() {
	logger := rt.logger
	if logger == nil {
		logger = slog.Default()
	}

	if !logger.Enabled(context.Background(), slog.LevelDebug) {
		return
	}

	var walk func(g *RouteGroup, prefix string, public bool)
	walk = func(g *RouteGroup, prefix string, public bool) {
		gp := joinPattern(prefix, g.prefix)
		gpublic := public || g.public

		for _, r := range g.routes {
			full := joinPattern(gp, r.pattern)
			action := string(r.action)
			if action == "" {
				action = "NoAction"
			}
			logger.Debug("registered route",
				slog.String("route", r.method+" "+full),
				slog.Bool("public", gpublic || r.public),
				slog.String("action", action),
			)
		}

		for _, c := range g.children {
			walk(c, gp, gpublic)
		}
	}
	walk(rt.RouteGroup, "", false)
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
