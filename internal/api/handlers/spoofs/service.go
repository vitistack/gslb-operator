package spoofs

import (
	"github.com/vitistack/gslb-operator/pkg/models/spoofs"
	"github.com/vitistack/gslb-operator/pkg/persistence"
)

type SpoofsService struct {
	SpoofRepo persistence.Repository[spoofs.Spoof]
	OverrideSpoof func(spoofs.Spoof) error
}

func NewSpoofsService(repo persistence.Repository[spoofs.Spoof]) *SpoofsService {
	return &SpoofsService{
		SpoofRepo: repo,
	}
}
