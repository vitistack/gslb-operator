package service

import (
	"errors"
	"fmt"

	"github.com/vitistack/gslb-operator/internal/model"
	"github.com/vitistack/gslb-operator/pkg/persistence"
)

var (
	ErrServiceWithMemberOfNotFound = errors.New("service with member-of not found")
)

// repository for services that are considered active in a service group
type ServiceRepo struct {
	store persistence.Store[model.Service]
}

func NewServiceRepo(store persistence.Store[model.Service]) *ServiceRepo {
	return &ServiceRepo{
		store: store,
	}
}

func (sr *ServiceRepo) Create(new *model.Service) error {
	err := sr.store.Save(new.Key(), *new)
	if err != nil {
		return fmt.Errorf("failed to store service: %w", err)
	}
	return nil
}

func (sr *ServiceRepo) Update(id string, new *model.Service) error {
	err := sr.store.Save(id, *new)
	if err != nil {
		return fmt.Errorf("failed to update entry with id: %s: %w", id, err)
	}
	return nil
}

func (sr *ServiceRepo) Delete(id string) error {
	err := sr.store.Delete(id)
	if err != nil {
		return fmt.Errorf("failed to delete entry with id: %s: %w", id, err)
	}
	return nil
}

func (sr *ServiceRepo) Read(id string) (model.Service, error) {
	svc, err := sr.store.Load(id)
	if err != nil {
		return model.Service{}, fmt.Errorf("failed to read from storage: %w", err)
	}
	return svc, nil
}
func (sr *ServiceRepo) ReadAll() ([]model.Service, error) {
	services, err := sr.store.LoadAll()
	if err != nil {
		return nil, fmt.Errorf("failed to read from storage: %w", err)
	}

	return services, nil
}

func (sr *ServiceRepo) FetchServiceMemberOf(memberOf string) (model.Service, error) {
	allServices, err := sr.ReadAll()
	if err != nil {
		return model.Service{}, err
	}

	for _, svc := range allServices {
		if svc.MemberOf == memberOf {
			return svc, nil
		}
	}

	return model.Service{}, fmt.Errorf("%w: member-of %s", ErrServiceWithMemberOfNotFound, memberOf)
}

func (sr *ServiceRepo) HasOverride(memberOf string) (bool, error) {
	svc, err := sr.FetchServiceMemberOf(memberOf)
	if err != nil {
		if errors.Is(err, ErrServiceWithMemberOfNotFound) {
			return false, nil
		}
		return false, err
	}

	return svc.Datacenter == "OVERRIDE", nil
}
