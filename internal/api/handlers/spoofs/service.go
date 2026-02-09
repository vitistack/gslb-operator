package spoofs

import (
	"github.com/vitistack/gslb-operator/internal/repositories/spoof"
	"github.com/vitistack/gslb-operator/internal/service"
)

type SpoofsService struct {
	SpoofRepo        *spoof.Repository
	GetCurrentActiveForFQDN func(fqdn string) *service.Service
}

func NewSpoofsService(repo *spoof.Repository) *SpoofsService {
	return &SpoofsService{
		SpoofRepo: repo,
	}
}
