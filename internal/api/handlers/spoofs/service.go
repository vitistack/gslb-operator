package spoofs

import (
	"github.com/vitistack/gslb-operator/internal/manager"
	"github.com/vitistack/gslb-operator/internal/model"
	"github.com/vitistack/gslb-operator/internal/repositories/service"
	"github.com/vitistack/gslb-operator/internal/repositories/spoof"
	"github.com/vitistack/gslb-operator/pkg/persistence"
)

type SpoofsService struct {
	svcRepo        *service.ServiceRepo
	spoofRepo      *spoof.SpoofRepo
	serviceManager manager.QueryManager
}

func NewSpoofsService(store persistence.Store[model.Service], svcManager manager.QueryManager) *SpoofsService {
	return &SpoofsService{
		svcRepo:        service.NewServiceRepo(store),
		spoofRepo:      spoof.NewSpoofRepo(store), // create read-only
		serviceManager: svcManager,
	}
}
