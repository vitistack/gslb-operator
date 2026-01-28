package spoof

import (
	"fmt"

	"github.com/vitistack/gslb-operator/pkg/models/spoofs"
	"github.com/vitistack/gslb-operator/pkg/persistence"
)

type Repository struct {
	storage persistence.Store[spoofs.Spoof]
}

func NewRepository(storage persistence.Store[spoofs.Spoof]) *Repository {
	return &Repository{
		storage: storage,
	}
}

func (r *Repository) Create(key string, new *spoofs.Spoof) error {
	err := r.storage.Save(key, *new)
	if err != nil {
		return fmt.Errorf("unable to store entry: %s", err.Error())
	}
	return nil
}

func (r *Repository) Update(id string, new *spoofs.Spoof) error {
	err := r.storage.Save(id, *new)
	if err != nil {
		return fmt.Errorf("unable to update entry with id: %s: %s", id, err.Error())
	}
	return nil
}

func (r *Repository) Delete(id string) error {
	err := r.storage.Delete(id)
	if err != nil {
		return fmt.Errorf("unable to delete entry with id: %s: %s", id, err.Error())
	}
	return nil
}

func (r *Repository) Read(id string) (spoofs.Spoof, error) {
	spoof, err := r.storage.Load(id)
	if err != nil {
		return spoofs.Spoof{}, fmt.Errorf("unable to read resource with id: %s", err.Error())
	}

	return spoof, nil
}

func (r *Repository) ReadAll() ([]spoofs.Spoof, error) {
	return r.storage.LoadAll()
}
