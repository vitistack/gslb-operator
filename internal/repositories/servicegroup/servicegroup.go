package servicegroup

import (
	"fmt"

	"github.com/vitistack/gslb-operator/internal/model"
	"github.com/vitistack/gslb-operator/pkg/persistence"
)

type ServiceGroupRepo struct {
	store persistence.Store[model.GSLBServiceGroup]
}

func NewServiceGroupRepo(store persistence.Store[model.GSLBServiceGroup]) *ServiceGroupRepo {
	return &ServiceGroupRepo{
		store: store,
	}
}

func (sr *ServiceGroupRepo) Create(memberOf string, group *model.GSLBServiceGroup) error {
	if err := sr.store.Save(memberOf, *group); err != nil {
		return fmt.Errorf("failed to create service group: %w", err)
	}
	return nil
}

func (sr *ServiceGroupRepo) Read(memberOf string) (model.GSLBServiceGroup, error) {
	if group, err := sr.store.Load(memberOf); err != nil {
		return model.GSLBServiceGroup{}, fmt.Errorf("failed to read from storage: %w", err)
	} else {
		return group, nil
	}
}

func (sr *ServiceGroupRepo) Update(memberOf string, group *model.GSLBServiceGroup) error {
	if err := sr.store.Save(memberOf, *group); err != nil {
		return fmt.Errorf("could not update group: %s: %w", memberOf, err)
	}
	return nil
}

func (sr *ServiceGroupRepo) Delete(memberOf string) error {
	if err := sr.store.Delete(memberOf); err != nil {
		return fmt.Errorf("failed to delete servicegroup: %s: %w", memberOf, err)
	}
	return nil
}
