package update

import "github.com/vitistack/gslb-operator/internal/service"

type Updater interface {
	OnServiceUp(*service.Service) error
	OnServiceDown(*service.Service) error
}
