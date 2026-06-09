package spoofs

/**
* NOTE: overrides are strictly used to manually update (from CLI) the spoofed ip adress for a service
* this is meant to only be used in an emergency. and is generally considered a disruptive action, due to it being no checking.
* be cautious when using this.
* for a more gracefull approach, see failover.
 */

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/vitistack/gslb-operator/internal/api/routes"
	"github.com/vitistack/gslb-operator/pkg/bslog"
	"github.com/vitistack/gslb-operator/pkg/models/spoofs"
	"github.com/vitistack/gslb-operator/pkg/rest/response"
)

func (s *SpoofsService) GetOverride(w http.ResponseWriter, r *http.Request) {
	logger := bslog.With(slog.Any("request_id", r.Context().Value("id")))
	memberOf := r.PathValue(routes.MemberOf)

	if memberOf == "" {
		logger.Info("skipping due to insufficient input parameter", slog.String("reason", "missing path-value memberOf"))
		response.Err(w, response.ErrInvalidInput, "missing path-value")
		return
	}

	group, err := s.svcGroupRepo.Read(memberOf)
	if err != nil {
		logger.Error("could not fetch service group",
			slog.String("memberOf", memberOf),
			slog.String("reason", err.Error()),
		)
		response.Err(w, response.ErrInternalError, "something unexpected happened")
		return
	}

	if !group.HasOverride {
		logger.Info("no active override", slog.String("serviceGroup", memberOf))
		response.Err(w, response.ErrNotFound, "no active override for: "+memberOf)
		return
	}

	err = response.JSON(w, http.StatusOK, map[string]string{"ip": group.Active})
	if err != nil {
		logger.Error("failed to create json response",
			slog.String("reason", err.Error()),
			slog.Group(
				"override",
				slog.String("ip", group.Active),
			))
	}
}

func (s *SpoofsService) CreateOverride(w http.ResponseWriter, r *http.Request) {
	logger := bslog.With(slog.Any("request_id", r.Context().Value("id")))
	override := spoofs.Override{}

	if err := json.NewDecoder(r.Body).Decode(&override); err != nil {
		logger.Error("failed to decode request body",
			slog.String("reason", err.Error()),
		)
		response.Err(w, response.ErrInvalidInput, "invalid request body")
		return
	}

	err := s.overrideApplier.CreateOverride(override.MemberOf, override.IP)
	if err != nil {
		logger.Error("failed to create override",
			slog.String("reason", err.Error()),
			slog.Group(
				"override",
				slog.String("memberOf", override.MemberOf),
				slog.String("ip", override.IP.String()),
			),
		)
		response.Err(w, response.ErrInternalError, "something un-expected ocurred")
		return
	}

	w.WriteHeader(http.StatusCreated)
}

func (s *SpoofsService) DeleteOverride(w http.ResponseWriter, r *http.Request) {
	logger := bslog.With(slog.Any("request_id", r.Context().Value("id")))
	memberOf := r.PathValue(routes.MemberOf)

	if memberOf == "" {
		logger.Info("skipping due to insufficient input parameter",
			slog.String("reason", "missing path-value memberOf"),
		)
		response.Err(w, response.ErrInvalidInput, "missing path-value")
		return
	}

	err := s.overrideApplier.RemoveOverride(memberOf)
	if err != nil {
		logger.Error("failed to delete override",
			slog.String("reason", err.Error()),
			slog.String("memberOf", memberOf),
		)
		response.Err(w, response.ErrInternalError, "something unexpected ocurred")
		return
	}

	w.WriteHeader(http.StatusOK)
}
