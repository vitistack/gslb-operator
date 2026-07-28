package status

import (
	serviceModels "github.com/vitistack/gslb-operator/pkg/models/service"
	"github.com/vitistack/gslb-operator/pkg/persistence"
)

type StatusRepo struct {
	store persistence.Store[serviceModels.GSLBServiceStatus]
}

func NewStatusRepo(store persistence.Store[serviceModels.GSLBServiceStatus]) *StatusRepo {
	return &StatusRepo{
		store: store,
	}
}

func (sr *StatusRepo) Create(memberOf string, status serviceModels.GSLBServiceStatus) error {
	return sr.store.Save(memberOf, status)
}

func (sr *StatusRepo) Read(memberOf string) (serviceModels.GSLBServiceStatus, error) {
	return sr.store.Load(memberOf)
}

func (sr *StatusRepo) ReadAll() ([]serviceModels.GSLBServiceStatus, error) {
	return sr.store.LoadAll()
}

func (sr *StatusRepo) Update(memberOf string, status serviceModels.GSLBServiceStatus) error {
	return sr.store.Save(memberOf, status)
}

func (sr *StatusRepo) Delete(memberOf string) error {
	return sr.store.Delete(memberOf)
}
