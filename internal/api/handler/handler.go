package handler

import (
	"fmt"

	"github.com/vitistack/gslb-operator/internal/model"
	"github.com/vitistack/gslb-operator/internal/repositories/spoof"
	"github.com/vitistack/gslb-operator/pkg/persistence"
	"github.com/vitistack/gslb-operator/pkg/persistence/store/file"
)

type Handler struct {
	SpoofRepo persistence.Repository[model.Spoof]
}

func NewHandler() (*Handler, error) {
	h := &Handler{}

	store, err := file.NewStore[model.Spoof]("store.json")
	if err != nil {
		return nil, fmt.Errorf("could not create filestore: %s", err.Error())
	}

	h.SpoofRepo = spoof.NewRepository(store)

	return h, nil
}
