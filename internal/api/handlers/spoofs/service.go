package spoofs

import (
	"github.com/vitistack/gslb-operator/internal/repositories/spoof"
	"github.com/vitistack/gslb-operator/pkg/models/spoofs"
)

type SpoofsService struct {
	SpoofRepo    *spoof.Repository
	restoreSpoof func(spoofs.Override) spoofs.Spoof
}

func NewSpoofsService(repo *spoof.Repository) *SpoofsService {
	return &SpoofsService{
		SpoofRepo: repo,
	}
}
