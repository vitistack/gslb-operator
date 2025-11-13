package dns

import (
	"log"

	"github.com/vitistack/gslb-operator/internal/service"
)

type Updater struct {
	Server string
	Zone   string
}

func (u *Updater) ServiceDown(svc *service.Service) {
	log.Printf("Service: %v:%v considered down", svc.Addr, svc.Datacenter)
}

func (u *Updater) ServiceUp(svc *service.Service) {
	log.Printf("Service: %v:%v considered up", svc.Addr, svc.Datacenter)
}
