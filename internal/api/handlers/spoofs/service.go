package spoofs

import (
	"github.com/vitistack/gslb-operator/internal/manager"
	"github.com/vitistack/gslb-operator/internal/model"
	"github.com/vitistack/gslb-operator/internal/repositories/servicegroup"
	"github.com/vitistack/gslb-operator/internal/repositories/spoof"
	"github.com/vitistack/gslb-operator/pkg/persistence"
)

type SpoofsService struct {
	svcGroupRepo   *servicegroup.ServiceGroupRepo
	spoofRepo      *spoof.SpoofRepo
	serviceManager manager.QueryManager
}

func NewSpoofsService(store persistence.Store[model.GSLBServiceGroup], svcManager manager.QueryManager) *SpoofsService {
	return &SpoofsService{
		svcGroupRepo:   servicegroup.NewServiceGroupRepo(store),
		spoofRepo:      spoof.NewSpoofRepo(store), // create read-only
		serviceManager: svcManager,
	}
}
