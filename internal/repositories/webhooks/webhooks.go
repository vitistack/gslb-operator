package webhooks

import (
	"github.com/vitistack/gslb-operator/internal/model"
	"github.com/vitistack/gslb-operator/pkg/persistence"
)

type WebHooksRepo struct {
	store persistence.Store[model.WebHook]
}

func NewWebHooksRepo(store persistence.Store[model.WebHook]) *WebHooksRepo {
	return &WebHooksRepo{
		store: store,
	}
}

func (wr *WebHooksRepo) Create(new model.WebHook) error {
	return wr.store.Save(new.ID, new)
}

func (wr *WebHooksRepo) Read(id string) (model.WebHook, error) {
	return wr.store.Load(id)
}

func (wr *WebHooksRepo) ReadAll() ([]model.WebHook, error) {
	return wr.store.LoadAll()
}

func (wr *WebHooksRepo) Update(id string, new model.WebHook) error {
	return wr.store.Save(id, new)
}

func (wr *WebHooksRepo) Delete(id string) error {
	return wr.store.Delete(id)
}
