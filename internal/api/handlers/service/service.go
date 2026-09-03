package service

import (
	"log/slog"
	"net/http"

	"github.com/vitistack/gslb-operator/internal/repositories/status"
	"github.com/vitistack/gslb-operator/pkg/bslog"
	"github.com/vitistack/gslb-operator/pkg/rest/response"
)

type GSLBServiceHandler struct {
	statusRepo *status.StatusRepo
}

func NewGSLBServiceHandler(repo *status.StatusRepo) *GSLBServiceHandler {
	return &GSLBServiceHandler{statusRepo: repo}
}

func (h *GSLBServiceHandler) GetServiceStatus(w http.ResponseWriter, r *http.Request) {
	logger := bslog.With(slog.Any("request_id", r.Context().Value("id")))
	fqdn := r.URL.Query().Get("fqdn")

	if fqdn == "" {
		logger.Info("skipping due to insufficient input parameter", slog.String("reason", "missing path-value memberOf"))
		response.Err(w, response.ErrInvalidInput, "missing path-value")
		return
	}

	status, err := h.statusRepo.Read(fqdn)
	if err != nil {
		logger.Error("failed to get status", slog.String("reason", err.Error()), slog.String("fqdn", fqdn))
		response.Err(w, response.ErrInternalError, "unexpected error")
		return
	}

	if status.MemberOf == "" {
		logger.Info("could not find status for service", slog.String("fqdn", fqdn))
		response.Err(w, response.ErrNotFound, "no status available yet for: "+fqdn)
		return
	}

	response.JSON(w, http.StatusOK, status)
}
