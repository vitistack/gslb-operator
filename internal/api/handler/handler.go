package handler

import (
	"github.com/vitistack/gslb-operator/internal/model"
	"github.com/vitistack/gslb-operator/internal/repositories/spoof"
	"github.com/vitistack/gslb-operator/pkg/persistence"
)

type Handler struct {
	spoofRepo persistence.Repository[model.Spoof]
}

func NewHandler(storage persistence.Store[model.Spoof]) *Handler {
	return &Handler{
		spoofRepo: spoof.NewRepository(storage),
	}
}

