package routes

import (
	"github.com/vitistack/gslb-operator/internal/api/handlers/v1/auth"
	"github.com/vitistack/gslb-operator/internal/manager"
	"github.com/vitistack/gslb-operator/internal/model"
	"github.com/vitistack/gslb-operator/internal/repositories/status"
	"github.com/vitistack/gslb-operator/pkg/persistence"
	"github.com/vitistack/gslb-operator/pkg/rest/middleware"
)

// Deps are the dependencies shared by every API version's Register function.
// Add fields here rather than widening each version package's signature.
type Dependencies struct {
	AuthService   *auth.AuthService
	SecuredChain  middleware.MiddlewareFunc
	ServicesStore persistence.Store[model.GSLBServiceGroup]
	Manager       *manager.ServicesManager
	StatusRepo    *status.StatusRepo
}
