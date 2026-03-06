package webhooks

import (
	"log/slog"
	"net/http"

	"github.com/google/uuid"
	"github.com/vitistack/gslb-operator/internal/model"
	"github.com/vitistack/gslb-operator/pkg/bslog"
	"github.com/vitistack/gslb-operator/pkg/rest/request"
	"github.com/vitistack/gslb-operator/pkg/rest/response"
)

type WebhooksService struct {
}

func NewWebhookService() *WebhooksService {
	return &WebhooksService{}
}

func (ws *WebhooksService) GetWebhooks(w http.ResponseWriter, r *http.Request) {}

func (ws *WebhooksService) CreateWebHook(w http.ResponseWriter, r *http.Request) {
	logger := bslog.With(slog.Any("request_id", r.Context().Value("id")))
	wh := model.WebHook{}
	err := request.JSONDECODE(r.Body, &wh)
	if err != nil {
		logger.Error("could not parse request body", slog.String("reason", err.Error()))
		response.Err(w, response.ErrInvalidInput, "invalid body")
		return
	}

	id, err := uuid.NewV7()
	if err != nil {
		logger.Error("could not generate webhook id", slog.String("reason", err.Error()))
		response.Err(w, response.ErrInternalError, "unable to generate webhook id")
		return
	}
	wh.ID = id.String()

	dispatcher := NewDispatcher(wh)
	if err := wh.Apply(dispatcher); err != nil {

	}

}
func (ws *WebhooksService) UpdateWebhook(w http.ResponseWriter, r *http.Request) {}
func (ws *WebhooksService) DeleteWebhook(w http.ResponseWriter, r *http.Request) {}
