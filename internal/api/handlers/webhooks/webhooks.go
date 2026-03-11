package webhooks

import (
	"log/slog"
	"net/http"

	"github.com/google/uuid"
	"github.com/vitistack/gslb-operator/internal/model"
	"github.com/vitistack/gslb-operator/internal/repositories/webhooks"
	"github.com/vitistack/gslb-operator/pkg/bslog"
	"github.com/vitistack/gslb-operator/pkg/events"
	"github.com/vitistack/gslb-operator/pkg/persistence"
	"github.com/vitistack/gslb-operator/pkg/rest/request"
	"github.com/vitistack/gslb-operator/pkg/rest/response"
)

type WebhooksService struct {
	webHooksRepo *webhooks.WebHooksRepo
}

func NewWebhookService(store persistence.Store[model.WebHook]) *WebhooksService {
	return &WebhooksService{
		webHooksRepo: webhooks.NewWebHooksRepo(store),
	}
}

func (ws *WebhooksService) GetWebhooks(w http.ResponseWriter, r *http.Request) {}

func (ws *WebhooksService) CreateWebHook(w http.ResponseWriter, r *http.Request) {
	logger := bslog.With(slog.Any("request_id", r.Context().Value("id")))
	wh := model.WebHook{
		// default SecretHeader
		Options: model.WebHookOptions{ 
			SecretHeader: "Authorization",
		},
	}

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
		response.Err(w, response.ErrInvalidInput, "malformed request body")
		return
	}

	err = ws.webHooksRepo.Create(wh)
	if err != nil {
		// delete all the event handlers since we could not store it
		for _, event := range wh.Events {
			events.Remove(event.Type, wh.ID)
		}

		bslog.Error(
			"unable to store webhook",
			slog.String("reason", err.Error()),
			slog.Any("webhook", wh),
		)
		response.Err(w, response.ErrInternalError, "could not store webhook")
	}
}

func (ws *WebhooksService) UpdateWebhook(w http.ResponseWriter, r *http.Request) {}
func (ws *WebhooksService) DeleteWebhook(w http.ResponseWriter, r *http.Request) {}
