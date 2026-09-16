package router

import (
    "net/http"

    "github.com/vitistack/gslb-operator/pkg/rest/middleware"
)

// RouteGroup is a path prefix that shares its middleware and public flag with
// every route and sub-group declared under it.
type RouteGroup struct {
    prefix   string
    public   bool
    mws      []middleware.MiddlewareFunc
    routes   []*Route
    children []*RouteGroup
}

func (g *RouteGroup) Group(prefix string) *RouteGroup {
    child := &RouteGroup{prefix: prefix}
    g.children = append(g.children, child)
    return child
}

func (g *RouteGroup) Use(mw ...middleware.MiddlewareFunc) *RouteGroup {
    g.mws = append(g.mws, mw...)
    return g
}

func (g *RouteGroup) Public() *RouteGroup { g.public = true; return g }

func (g *RouteGroup) GET(pattern string, h http.HandlerFunc) *Route {
    return g.handle(http.MethodGet, pattern, h)
}

func (g *RouteGroup) POST(pattern string, h http.HandlerFunc) *Route {
    return g.handle(http.MethodPost, pattern, h)
}

func (g *RouteGroup) PUT(pattern string, h http.HandlerFunc) *Route {
    return g.handle(http.MethodPut, pattern, h)
}

func (g *RouteGroup) PATCH(pattern string, h http.HandlerFunc) *Route {
    return g.handle(http.MethodPatch, pattern, h)
}

func (g *RouteGroup) DELETE(pattern string, h http.HandlerFunc) *Route {
    return g.handle(http.MethodDelete, pattern, h)
}

// Handle registers a full http.Handler (e.g. promhttp.Handler()) by adapting
// its ServeHTTP, so it goes through the same middleware and enforcement chain.
func (g *RouteGroup) Handle(method, pattern string, h http.Handler) *Route {
    return g.handle(method, pattern, h.ServeHTTP)
}

func (g *RouteGroup) handle(method, pattern string, h http.HandlerFunc) *Route {
    r := &Route{method: method, pattern: pattern, handler: h}
    g.routes = append(g.routes, r)
    return r
}