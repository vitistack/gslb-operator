package manager

import (
	"github.com/vitistack/gslb-operator/internal/service"
	"github.com/vitistack/gslb-operator/pkg/models/failover"
)

// interface for API handlers that needs specific functionality from the manager.
// without exposing all functionality
type QueryManager interface {
	GetActiveForMemberOf(memberOf string) *service.Service

	//write operations
	Failover(fqdn string, failover failover.Failover) error
}
