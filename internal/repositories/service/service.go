package service

import (
	"errors"
	"fmt"
	"slices"

	"github.com/vitistack/gslb-operator/internal/model"
	"github.com/vitistack/gslb-operator/pkg/persistence"
)

var (
	ErrServiceWithMemberOfNotFound = errors.New("service with member-of not found")
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
	override, err := sr.HasOverride(new.MemberOf)
	if err != nil {
		return err
	}

	if override {
		return nil
	}

	group, err := sr.Read(new.MemberOf)
	if err != nil {
		return fmt.Errorf("failed to check for existing service group: %w", err)
	}

	if group == nil {
		group = make(model.GSLBServiceGroup, 0)
		group = append(group, *new)
		err := sr.store.Save(new.MemberOf, group)
		if err != nil {
			return fmt.Errorf("failed to store service: %w", err)
		}
		return nil
	}

	if slices.ContainsFunc(
		group,
		func(s model.GSLBService) bool {
			return s.ID == new.ID
		}) {
		return fmt.Errorf("failed to store service: service already exists")
	}

	group = append(group, *new)
	err = sr.store.Save(new.Key(), group)
	if err != nil {
		return fmt.Errorf("failed to store service: %w", err)
	}

	return nil
}

func (sr *ServiceRepo) Update(new *model.GSLBService) error {
	override, err := sr.HasOverride(new.MemberOf)
	if err != nil {
		return err
	}

	group, err := sr.Read(new.MemberOf)
	if err != nil {
		return fmt.Errorf("failed to check for existing service group: %w", err)
	}

	if group == nil {
		return fmt.Errorf("failed to update service: service group for: %s does not exist", new.MemberOf)
	}

	for idx, svc := range group {
		if svc.ID == new.ID {
			if svc.IsActive {
				if override {
					new.IP = svc.IP // preserve ip on override, but only for active
					new.HasOverride = true
				}
			}
			group[idx] = *new
			err = sr.store.Save(new.MemberOf, group)

			if err != nil {
				return fmt.Errorf("failed to update entry with id: %s: %w", new.MemberOf, err)
			}
			break
		}
	}

	return nil
}

func (sr *ServiceRepo) UpdateOverride(ip string, service *model.GSLBService) error {
	service.IP = ip

	group, err := sr.Read(service.MemberOf)
	if err != nil {
		return fmt.Errorf("failed to retrieve service group: %w", err)
	}

	if group == nil {
		return fmt.Errorf("failed to update service: service group for: %s does not exist", service.MemberOf)
	}

	for idx, svc := range group {
		if svc.ID == service.ID {
			group[idx] = *service
			err = sr.store.Save(service.MemberOf, group)

			if err != nil {
				return fmt.Errorf("failed to update override: %w", err)
			}
			break
		}
	}
	return nil
}

func (sr *ServiceRepo) RemoveOverrideFlag(memberOf string) error {
	group, err := sr.Read(memberOf)
	if err != nil {
		return err
	}

	for idx := range group {
		group[idx].HasOverride = false // update flag for every service in group
	}

	return sr.store.Save(memberOf, group)
}

func (sr *ServiceRepo) Delete(memberOf string, id string) error {
	group, err := sr.Read(memberOf)
	if err != nil {
		return err
	}

	override, err := sr.HasOverride(memberOf)
	if err != nil {
		return err
	}

	if override {
		return nil
	}

	group = slices.DeleteFunc(group, func(s model.GSLBService) bool { // delete service with id
		return s.ID == id
	})
	if len(group) == 0 { // delete service group if empty group
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
		return nil, fmt.Errorf("failed to read from storage: %w", err)
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

	for _, svc := range group {
		if svc.IsActive {
			return svc, nil
		}
	}

	return model.GSLBService{}, fmt.Errorf("%w: member-of %s", ErrServiceWithMemberOfNotFound, memberOf)
}

func (sr *ServiceRepo) HasOverride(memberOf string) (bool, error) {
	svc, err := sr.GetActive(memberOf)
	if err != nil {
		if errors.Is(err, ErrServiceWithMemberOfNotFound) {
			return false, nil
		}
		return false, err
	}

	return svc.HasOverride, nil
}
