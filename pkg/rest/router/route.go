package router

import (
	"net/http"

	"github.com/vitistack/gslb-operator/pkg/auth/authz"
	"github.com/vitistack/gslb-operator/pkg/rest/middleware"
)

// Route is a single endpoint; a RouteGroup verb method creates one and returns
// it so the action and per-route middleware can be chained on.
type Route struct {
	method  string
	pattern string
	action  authz.Action
	public  bool
	handler http.HandlerFunc
	mws     []middleware.MiddlewareFunc
}

func (r *Route) Action(a authz.Action) *Route { r.action = a; return r }

func (r *Route) Public() *Route { r.public = true; return r }

func (r *Route) Use(mw ...middleware.MiddlewareFunc) *Route {
	r.mws = append(r.mws, mw...)
	return r
}
