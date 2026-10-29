package router

import (
	"net/http"

	"github.com/vitistack/gslb-operator/pkg/auth/authz"
	"github.com/vitistack/gslb-operator/pkg/rest/middleware"
)

// Route is a single endpoint; a RouteGroup verb method creates one and returns
// it so the action and per-route middleware can be chained on.
type Route struct {
	method         string
	pattern        string
	action         authz.Action
	public         bool
	handler        http.HandlerFunc
	mws            []middleware.MiddlewareFunc
	globalMws      []middleware.MiddlewareFunc
	globalOverride bool
}

func (r *Route) Action(a authz.Action) *Route { r.action = a; return r }

func (r *Route) Public() *Route { r.public = true; return r }

func (r *Route) UseGlobals(mws ...middleware.MiddlewareFunc) *Route {
	r.globalMws = append(r.globalMws, mws...)
	r.globalOverride = true
	return r
}

func (r *Route) Use(mws ...middleware.MiddlewareFunc) *Route {
	r.mws = append(r.mws, mws...)
	return r
}
