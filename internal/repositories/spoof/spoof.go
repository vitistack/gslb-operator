package spoof

import (
	"errors"
	"fmt"

	"github.com/vitistack/gslb-operator/internal/model"
	"github.com/vitistack/gslb-operator/pkg/models/spoofs"
	"github.com/vitistack/gslb-operator/pkg/persistence"
)

var (
	ErrSpoofWithFQDNNotFound = errors.New("spoof with fqdn not found")
)

// read-only repo for spoofs
type SpoofRepo struct {
	store persistence.Store[model.Service]
}

func NewSpoofRepo(storage persistence.Store[model.Service]) *SpoofRepo {
	return &SpoofRepo{
		store: storage,
	}
}

func (r *SpoofRepo) Read(id string) (spoofs.Spoof, error) {
	svc, err := r.store.Load(id)
	if err != nil {
		return spoofs.Spoof{}, fmt.Errorf("failed to read from storage: %w", err)
	}

	return svc.Spoof(), nil
}

func (r *SpoofRepo) ReadFQDN(fqdn string) (spoofs.Spoof, error) {
	allServices, err := r.store.LoadAll()
	if err != nil {
		return spoofs.Spoof{}, fmt.Errorf("failed to read from storage: %w", err)
	}

	for _, svc := range allServices {
		if svc.Fqdn == fqdn {
			return svc.Spoof(), nil
		}
	}

	return spoofs.Spoof{}, fmt.Errorf("%w: fqdn: %s", ErrSpoofWithFQDNNotFound, fqdn)
}

func (r *SpoofRepo) ReadAll() ([]spoofs.Spoof, error) {
	allServices, err := r.store.LoadAll()
	if err != nil {
		return nil, fmt.Errorf("failed to read from storage: %w", err)
	}

	spoofs := make([]spoofs.Spoof, 0)
	for _, svc := range allServices {
		spoofs = append(spoofs, svc.Spoof())
	}

	return spoofs, nil
}

func (r *SpoofRepo) HasOverride(fqdn string) (bool, error) {
	spoof, err := r.ReadFQDN(fqdn)
	if err != nil {
		if errors.Is(err, ErrSpoofWithFQDNNotFound) {
			return false, nil
		}
		return false, err
	}

	return spoof.DC == "OVERRIDE", nil
}
