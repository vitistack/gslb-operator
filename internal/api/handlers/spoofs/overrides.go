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

	group, err := s.svcGroupRepo.Read(override.MemberOf)
	if err != nil {
		logger.Error("could not fetch service group",
			slog.String("memberOf", override.MemberOf),
			slog.String("reason", err.Error()),
		)
		response.Err(w, response.ErrInternalError, "something unexpected happened")
		return
	}

	if group.HasOverride {
		logger.Info("skipping override creation",
			slog.String("reason", "override already exists"),
		)
		response.Err(w, response.ErrInvalidInput, "group already has override")
		return
	}

	group.HasOverride = true
	group.Active = override.IP.String()

	err = s.svcGroupRepo.Update(override.MemberOf, &group)
	if err != nil {
		logger.Error(
			"failed to update group with override",
			slog.String("reason", err.Error()),
			slog.Group(
				"override",
				slog.String("memberOf", override.MemberOf),
				slog.String("ip", override.IP.String()),
			),
		)
		response.Err(w, response.ErrInternalError, "something unexpected happened")
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
		// TODO: should i log something here?
		w.WriteHeader(http.StatusOK)
	}

	group.HasOverride = false
	// TODO: how to find active svc id?
	group.Active = "Hello World!"

	err = s.svcGroupRepo.Update(memberOf, &group)
	if err != nil {
		logger.Error("failed to delete group override",
			slog.String("memberOf", memberOf),
			slog.String("reason", err.Error()),
		)
		response.Err(w, response.ErrInternalError, "something unexpected happened")
		return
	}

	w.WriteHeader(http.StatusOK)
}

/*
func (ss *SpoofsService) GetOverride(w http.ResponseWriter, r *http.Request) {
	logger := bslog.With(slog.Any("request_id", r.Context().Value("id")))
	memberOf := r.PathValue(routes.MemberOf)

	if memberOf == "" {
		logger.Error("skipping request due to insufficient input parameters", slog.String("reason", "missing member-of"))
		response.Err(w, response.ErrInvalidInput, "missing member-of")
		return
	}

	exist, err := ss.svcRepo.GetActive(memberOf)
	if err != nil {
		logger.Error("could not read spoofs", slog.String("reason", err.Error()))
		response.Err(w, response.ErrInternalError, "")
		return
	}

	if !exist.HasOverride {
		logger.Error("service does not have an active override", slog.String("memberOf", exist.MemberOf))
		response.Err(w, response.ErrNotFound, "not an active override")
		return
	}

	err = response.JSON(w, http.StatusOK, exist)
	if err != nil {
		logger.Error("unable to create json response", slog.String("reason", err.Error()))
	}
}

func (ss *SpoofsService) CreateOverride(w http.ResponseWriter, r *http.Request) {
	logger := bslog.With(slog.Any("request_id", r.Context().Value("id")))
	override := spoofs.Override{}

	err := request.JSONDECODE(r.Body, &override)
	if err != nil {
		logger.Error("could not decode request body", slog.String("reason", err.Error()))
		response.Err(w, response.ErrInvalidInput, "invalid request format")
		return
	}

	err = ss.newOverride(override)
	if err != nil {
		logger.Error("could not override spoof", slog.String("reason", err.Error()))
		if errors.Is(err, spoofRepo.ErrSpoofInServiceGroupNotFound) {
			response.Err(w, response.ErrNotFound, "group: "+override.MemberOf)
			return
		}

		response.Err(w, response.ErrInvalidInput, "unable to create spoof")
		return
	}

	w.WriteHeader(http.StatusCreated)
}

func (ss *SpoofsService) UpdateOverride(w http.ResponseWriter, r *http.Request) {
	logger := bslog.With(slog.Any("request_id", r.Context().Value("id")))
	override := spoofs.Override{}

	err := request.JSONDECODE(r.Body, &override)
	if err != nil {
		logger.Error("could not decode request body", slog.String("reason", err.Error()))
		response.Err(w, response.ErrInvalidInput, "invalid request format")
		return
	}

	err = ss.updateOverride(override)
	if err != nil {
		logger.Error("could not update spoof", slog.String("reason", err.Error()))
		if errors.Is(err, spoofRepo.ErrSpoofInServiceGroupNotFound) {
			response.Err(w, response.ErrNotFound, "group: "+override.MemberOf)
			return
		}

		response.Err(w, response.ErrInvalidInput, "unable to update spoof")
		return
	}

	w.WriteHeader(http.StatusCreated)
}

func (ss *SpoofsService) DeleteOverride(w http.ResponseWriter, r *http.Request) {
	logger := bslog.With(slog.Any("request_id", r.Context().Value("id")))
	override := spoofs.Override{}

	err := request.JSONDECODE(r.Body, &override)
	if err != nil {
		logger.Error("could not decode request body", slog.String("reason", err.Error()))
		response.Err(w, response.ErrInvalidInput, "invalid request format")
		return
	}

	err = ss.deleteOverride(override)
	if err != nil {
		logger.Error("could not delete overridden spoof", slog.String("reason", err.Error()))
		response.Err(w, response.ErrInvalidInput, "unable to delete override")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (ss *SpoofsService) newOverride(override spoofs.Override) error {
	exist, err := ss.svcRepo.GetActive(override.MemberOf)
	if err != nil {
		return fmt.Errorf("unable to get active service for group: %s: %w", override.MemberOf, err)
	}

	if exist.HasOverride {
		return fmt.Errorf("service already has active override: %s", exist.MemberOf)
	}

	exist.IP = override.IP.String()
	exist.HasOverride = true

	err = ss.svcRepo.Update(&exist)
	if err != nil {
		return fmt.Errorf("failed to update GSLB service with override flag: %w", err)
	}

	return nil
}

func (ss *SpoofsService) updateOverride(override spoofs.Override) error {
	active, err := ss.svcRepo.GetActive(override.MemberOf)
	if err != nil {
		return fmt.Errorf("unable to get active service for group: %s: %w", override.MemberOf, err)
	}

	if active.HasOverride {
		return fmt.Errorf("service already has active override: %s", active.MemberOf)
	}

	active.IP = override.IP.String()

	err = ss.svcRepo.UpdateOverride(override.IP.String(), &active)
	if err != nil {
		return fmt.Errorf("failed to update GSLB service with override flag: %w", err)
	}

	return nil
}

func (ss *SpoofsService) deleteOverride(override spoofs.Override) error {
	exist, err := ss.svcRepo.GetActive(override.MemberOf)
	if err != nil {
		return fmt.Errorf("unable to get active service for group: %s: %w", override.MemberOf, err)
	}

	if !exist.HasOverride {
		return fmt.Errorf("%s does not have an override currently set", override.MemberOf)
	}

	err = ss.svcRepo.RemoveOverrideFlag(override.MemberOf)
	if err != nil {
		return fmt.Errorf("failed to remove override flag: %w", err)
	}

	active := ss.restoreActive(override)
	err = ss.svcRepo.Update(active)
	if err != nil {
		return fmt.Errorf("could not restore active service in group after override flag has been removed: %w", err)
	}

	return nil
}

func (ss *SpoofsService) restoreActive(override spoofs.Override) *model.GSLBService {
	svc := ss.serviceManager.GetActiveForMemberOf(override.MemberOf)
	if svc == nil { // no active service: e.g. no spoof should be there
		return nil
	}

	return svc.GSLBService()
}
*/
