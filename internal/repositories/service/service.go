package service

import (
	"errors"
	"fmt"

	"github.com/vitistack/gslb-operator/internal/model"
	"github.com/vitistack/gslb-operator/pkg/persistence"
)

var (
	ErrServiceWithMemberOfNotFound = errors.New("service with member-of not found")
	ErrServiceInGroupNotFound      = errors.New("service in service-group not found")
)

// repository for services that are considered active in a service group
type ServiceRepo struct {
	store persistence.Store[model.GSLBServiceGroup]
}

func NewServiceRepo(store persistence.Store[model.GSLBServiceGroup]) *ServiceRepo {
	return &ServiceRepo{
		store: store,
	}
}

func (sr *ServiceRepo) Create(new *model.GSLBService) error {
	group, err := sr.Read(new.MemberOf)
	if err != nil {
		return fmt.Errorf("failed to read existing service group: %w", err)
	}

	if len(group.Members) == 0 {
		group.Members = make(map[string]model.GSLBService)
		group.Members[new.ID] = *new
		err := sr.store.Save(new.MemberOf, group)
		if err != nil {
			return fmt.Errorf("failed to store service: %w", err)
		}
		return nil
	}

	if _, ok := group.Members[new.ID]; ok {
		sr.Update(new)
	}

	group.Members[new.ID] = *new
	err = sr.store.Save(new.MemberOf, group)
	if err != nil {
		return fmt.Errorf("failed to store service: %w", err)
	}

	return nil
}

func (sr *ServiceRepo) Update(new *model.GSLBService) error {
	group, err := sr.Read(new.MemberOf)
	if err != nil {
		return fmt.Errorf("failed to check for existing service group: %w", err)
	}

	if len(group.Members) == 0 {
		return fmt.Errorf("failed to update service group: %s does not exist", new.MemberOf)
	}

	if _, ok := group.Members[new.ID]; ok {
		group.Members[new.ID] = *new
	} else {
		return fmt.Errorf("%w: %s id: %s", ErrServiceInGroupNotFound, new.MemberOf, new.ID)
	}

	if err := sr.store.Save(new.MemberOf, group); err != nil {
		return fmt.Errorf("failed to update entry with id: %s: %w", new.MemberOf, err)
	}

	return nil
}

func (sr *ServiceRepo) Delete(memberOf string, id string) error {
	group, err := sr.Read(memberOf)
	if err != nil {
		return err
	}

	// delete service with id
	delete(group.Members, id)
	if len(group.Members) == 0 { // delete service group if empty group
		err = sr.store.Delete(memberOf)
		if err != nil {
			return fmt.Errorf("failed to delete service group after empty result: %w", err)
		}
	}

	err = sr.store.Save(memberOf, group) // save the remaining services
	if err != nil {
		return fmt.Errorf("failed to delete entry with id: %s: %w", id, err)
	}

	return nil
}

func (sr *ServiceRepo) Read(id string) (model.GSLBServiceGroup, error) {
	group, err := sr.store.Load(id)
	if err != nil {
		return model.GSLBServiceGroup{}, fmt.Errorf("failed to read from storage: %w", err)
	}
	return group, nil
}

func (sr *ServiceRepo) ReadAll() ([]model.GSLBServiceGroup, error) {
	services, err := sr.store.LoadAll()
	if err != nil {
		return nil, fmt.Errorf("failed to read from storage: %w", err)
	}

	return services, nil
}

func (sr *ServiceRepo) GetActive(memberOf string) (model.GSLBService, error) {
	group, err := sr.Read(memberOf)
	if err != nil {
		return model.GSLBService{}, err
	}

	if group.Active != "" {
		return group.Members[group.Active], nil
	}

	return model.GSLBService{}, fmt.Errorf("%w: member-of %s", ErrServiceWithMemberOfNotFound, memberOf)
}

func (sr *ServiceRepo) GetMemberInGroup(memberOf, memberId string) (model.GSLBService, error) {
	group, err := sr.Read(memberOf)
	if err != nil {
		return model.GSLBService{}, err
	}

	if _, ok := group.Members[memberId]; ok {
		return group.Members[memberId], nil
	}

	return model.GSLBService{}, fmt.Errorf("%w: member-of: %s: member-id: %s", ErrServiceInGroupNotFound, memberOf, memberId)
}

//func (sr *ServiceRepo) HasOverride(group model.GSLBServiceGroup) bool
