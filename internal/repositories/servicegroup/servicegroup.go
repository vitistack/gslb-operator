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

func (sr *ServiceGroupRepo) Mutate(memberOf string, mut func(*model.GSLBServiceGroup)) error {
	group, err := sr.Read(memberOf)
	if err != nil {
		return err
	}

	mut(&group)
	return sr.Update(memberOf, &group)
}

func (sr *ServiceGroupRepo) Update(memberOf string, group *model.GSLBServiceGroup) error {
	if err := sr.store.Save(memberOf, *group); err != nil {
		return fmt.Errorf("could not update group: %s: %w", memberOf, err)
	}
	return nil
}

func (sr *ServiceGroupRepo) UpdateMember(memberOf string, svc model.GSLBService) error {
	group, err := sr.store.Load(memberOf)
	if err != nil {
		return fmt.Errorf("failed to read from storage: %w", err)
	}
	if group.Members == nil {
		group.Members = make(map[string]model.GSLBService)
	}
	group.Members[svc.ID] = svc

	return sr.store.Save(memberOf, group)
}

func (sr *ServiceGroupRepo) Delete(memberOf string) error {
	if err := sr.store.Delete(memberOf); err != nil {
		return fmt.Errorf("failed to delete servicegroup: %s: %w", memberOf, err)
	}
	return nil
}

func (sr *ServiceGroupRepo) DeleteMember(memberOf string, member model.GSLBService) error {
	group, err := sr.Read(member.MemberOf)
	if err != nil {
		return fmt.Errorf("failed to fetch service group: %w", err)
	}

	delete(group.Members, member.ID)
	// delete entire service group when empty
	if len(group.Members) == 0 {
		return sr.Delete(member.MemberOf)
	}

	err = sr.store.Save(member.MemberOf, group)
	if err != nil {
		return fmt.Errorf("failed to delete member: %s in service group: %s: %w", member.ID, member.MemberOf, err)
	}

	return nil
}
