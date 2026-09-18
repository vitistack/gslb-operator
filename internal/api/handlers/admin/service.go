package admin

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/vitistack/gslb-operator/pkg/auth/authc"
	"github.com/vitistack/gslb-operator/pkg/bslog"
	"github.com/vitistack/gslb-operator/pkg/models/auth"
	"github.com/vitistack/gslb-operator/pkg/rest/response"
)

type ClientRegistry interface {
	SetAttribute(ctx context.Context, clientID, key string, values []string) error
	Attributes(ctx context.Context, clientID string) (map[string][]string, error)
	AssignRoles(ctx context.Context, clientID string, roles ...string) error
	Identity(ctx context.Context, clientID string) (roles []string, keyVersion string, err error)
}

type AdminService struct {
	registry ClientRegistry
}

func (a *AdminService) GrantAttribute(w http.ResponseWriter, r *http.Request) {
	logger := bslog.With(slog.Any("request_id", r.Context().Value("id")))
	clientID := r.PathValue("clientID")

	if clientID == "" {
		logger.Info("skipping attribute grant", slog.String("reason", "missing path-value clientID"))
		response.Err(w, response.ErrInvalidInput, "invalid request input")
		return
	}

	grantRequest := auth.AssignAttributes{}
	if err := json.NewDecoder(w).Decode(&grantRequest); err != nil {
		logger.Info("failed to decode request body", slog.String("reason", err.Error()))
		response.Err(w, response.ErrInvalidInput, "invalid request body")
		return
	}

	if grantRequest.Key == "" {
		logger.Info("skipping attribute grant", slog.String("reason", "attribute key required"))
		response.Err(w, response.ErrInvalidInput, "attribute key required")
		return
	}

	if err := a.registry.SetAttribute(r.Context(), clientID, grantRequest.Key, grantRequest.Values); err != nil {
		if errors.Is(err, authc.ErrClientNotFound) {
			logger.Info("skipping grant", slog.String("clientID", clientID), slog.String("reason", err.Error()))
			response.Err(w, response.ErrNotFound, "client not enrolled")
			return
		}

		logger.Error("failed to grant attribute",
			slog.String("clientID", clientID),
			slog.String("key", grantRequest.Key),
			slog.String("reason", err.Error()),
		)
		response.Err(w, response.ErrInternalError, "something unexpected occurred")
		return
	}

	logger.Info(
		"grant attribute request succeeded",
		slog.String("clientID", clientID),
		slog.String("key", grantRequest.Key),
		slog.Any("values", grantRequest.Values),
	)
	response.JSON(w, http.StatusOK, map[string]any{"clientID": clientID, grantRequest.Key: grantRequest.Values})
}

func (a *AdminService) GrantRole(w http.ResponseWriter, r *http.Request) {

}
