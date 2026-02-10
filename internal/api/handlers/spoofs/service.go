package spoofs

import (
	"github.com/vitistack/gslb-operator/internal/manager"
	"github.com/vitistack/gslb-operator/internal/repositories/spoof"
)

type SpoofsService struct {
	SpoofRepo      *spoof.Repository
	serviceManager manager.QueryManager
}

func NewSpoofsService(repo *spoof.Repository, svcManager manager.QueryManager) *SpoofsService {
	return &SpoofsService{
		SpoofRepo:      repo,
		serviceManager: svcManager,
	}
}
