package dns

import "github.com/vitistack/gslb-operator/internal/service"

type Updater struct {
	Server string
	Zone   string
}

func (u *Updater) ServiceDown(svc *service.Service) {
	
}

func (u *Updater) ServiceUp(svc *service.Service) {

}
