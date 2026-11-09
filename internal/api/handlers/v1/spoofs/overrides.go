package spoofs

/**
* NOTE: overrides are strictly used to manually update (from CLI) the spoofed ip adress for a service
* this is meant to only be used in an emergency. and is generally considered a disruptive action, due to it being no checking.
* be cautious when using this.
* for a more gracefull approach, see failover.
 */

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/vitistack/gslb-operator/internal/manager"
	"github.com/vitistack/gslb-operator/pkg/bslog"
	"github.com/vitistack/gslb-operator/pkg/models/spoofs"
	"github.com/vitistack/gslb-operator/pkg/rest/response"
	"github.com/vitistack/gslb-operator/pkg/rest/router"
)

func (s *SpoofsService) GetOverride(w http.ResponseWriter, r *router.Request) {
	memberOf := r.Param("memberOf")

	group, err := s.svcGroupRepo.Read(memberOf)
	if err != nil {
		r.Log().Error("could not fetch service group",
			slog.String("memberOf", memberOf),
			slog.String("reason", err.Error()),
		)
		response.Err(w, response.ErrInternalError, "something unexpected happened")
		return
	}

	if !group.HasOverride {
		r.Log().Info("no active override", slog.String("serviceGroup", memberOf))
		response.Err(w, response.ErrNotFound, "no active override for: "+memberOf)
		return
	}

	err = response.JSON(w, http.StatusOK, map[string]any{"override": group.Active})
	if err != nil {
		r.Log().Error("failed to create json response",
			slog.String("reason", err.Error()),
			slog.Group(
				"override",
				slog.Any("ip", group.Active),
			))
	}
}

func (s *SpoofsService) CreateOverride(w http.ResponseWriter, r *router.Request) {
	override := spoofs.Override{}
	if err := json.NewDecoder(r.Body).Decode(&override); err != nil {
		r.Log().Error("failed to decode request body",
			slog.String("reason", err.Error()),
		)
		response.Err(w, response.ErrInvalidInput, "invalid request body")
		return
	}

	err := s.overrideApplier.CreateOverride(override)
	if err != nil {
		if errors.Is(err, manager.ErrServiceGroupNotFound) {
			r.Log().Info("skipping request", slog.String("reason", err.Error()))
			response.Err(w, response.ErrNotFound, override.MemberOf+" not found")
			return
		}

		r.Log().Error("failed to create override",
			slog.String("reason", err.Error()),
			slog.Any("override", override),
		)
		response.Err(w, response.ErrInternalError, "something un-expected ocurred")
		return
	}

	w.WriteHeader(http.StatusCreated)
	bslog.Info("successfully created override",
		slog.Any("override", override),
	)
}

func (s *SpoofsService) DeleteOverride(w http.ResponseWriter, r *router.Request) {
	memberOf := r.Param("memberOf")

	err := s.overrideApplier.RemoveOverride(memberOf)
	if err != nil {
		r.Log().Error("failed to delete override",
			slog.String("reason", err.Error()),
			slog.String("memberOf", memberOf),
		)
		response.Err(w, response.ErrInternalError, "something unexpected ocurred")
		return
	}

	w.WriteHeader(http.StatusNoContent)
	bslog.Info("successfully deleted override", slog.String("memberOf", memberOf))
}
