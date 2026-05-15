package manager

import (
	"github.com/vitistack/gslb-operator/internal/service"
)

// interface for API handlers that needs specific functionality from the manager.
// without exposing all functionality
type QueryManager interface {
	GetActiveForMemberOf(memberOf string) *service.Service
}
