package spoofs

import (
	"github.com/vitistack/gslb-operator/internal/model"
	"github.com/vitistack/gslb-operator/internal/repositories/servicegroup"
	"github.com/vitistack/gslb-operator/internal/repositories/spoof"
	"github.com/vitistack/gslb-operator/pkg/models/spoofs"
	"github.com/vitistack/gslb-operator/pkg/persistence"
)

type OverrideApplier interface {
	CreateOverride(spoofs.Override) error
	RemoveOverride(string, ...string) error
}

type SpoofsService struct {
	svcGroupRepo    *servicegroup.ServiceGroupRepo
	spoofRepo       *spoof.SpoofRepo
	overrideApplier OverrideApplier
}

func NewSpoofsService(store persistence.Store[model.GSLBServiceGroup], oa OverrideApplier) *SpoofsService {
	return &SpoofsService{
		svcGroupRepo:    servicegroup.NewServiceGroupRepo(store),
		spoofRepo:       spoof.NewSpoofRepo(store), // create read-only
		overrideApplier: oa,
	}
}
