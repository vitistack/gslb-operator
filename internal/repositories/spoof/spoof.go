package spoof

import (
	"errors"
	"fmt"

	"github.com/vitistack/gslb-operator/internal/model"
	"github.com/vitistack/gslb-operator/pkg/models/spoofs"
	"github.com/vitistack/gslb-operator/pkg/persistence"
)

var (
	ErrSpoofInServiceGroupNotFound = errors.New("spoof in service group not found")
)

// read-only repo for spoofs
type SpoofRepo struct {
	store persistence.Store[model.GSLBServiceGroup]
}

func NewSpoofRepo(storage persistence.Store[model.GSLBServiceGroup]) *SpoofRepo {
	return &SpoofRepo{
		store: storage,
	}
}

func (r *SpoofRepo) Read(id string) (spoofs.Spoof, error) {
	group, err := r.store.Load(id)
	if err != nil {
		return spoofs.Spoof{}, fmt.Errorf("failed to read from storage: %w", err)
	}

	for _, svc := range group {
		if svc.IsActive {
			return svc.Spoof(), nil
		}
	}

	return spoofs.Spoof{}, nil
}

func (r *SpoofRepo) ReadMemberOf(memberOf string) (spoofs.Spoof, error) {
	group, err := r.store.Load(memberOf)
	if err != nil {
		return spoofs.Spoof{}, fmt.Errorf("failed to read from storage: %w", err)
	}

	for _, svc := range group {
		if svc.IsActive {
			return svc.Spoof(), nil
		}
	}

	return spoofs.Spoof{}, fmt.Errorf("%w: fqdn: %s", ErrSpoofInServiceGroupNotFound, memberOf)
}

func (r *SpoofRepo) ReadAll() ([]spoofs.Spoof, error) {
	groups, err := r.store.LoadAll()
	if err != nil {
		return nil, fmt.Errorf("failed to read from storage: %w", err)
	}

	spoofs := make([]spoofs.Spoof, 0)
	for _, group := range groups {
		for _, svc := range group {
			if svc.IsActive {
				spoofs = append(spoofs, svc.Spoof())
			}
		}
	}

	return spoofs, nil
}
