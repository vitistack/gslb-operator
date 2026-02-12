package spoofs

/**
* NOTE: overrides are strictly used to manually update (from CLI) the spoofed ip adress for a service
* this is meant to only be used in an emergency. and is generally considered a disruptive action, due to it being no checking.
* be cautious when using this.
* for a more gracefull approach, see failover.
 */

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	spoofRepo "github.com/vitistack/gslb-operator/internal/repositories/spoof"
	"github.com/vitistack/gslb-operator/pkg/bslog"
	"github.com/vitistack/gslb-operator/pkg/models/spoofs"
	"github.com/vitistack/gslb-operator/pkg/rest/request"
	"github.com/vitistack/gslb-operator/pkg/rest/response"
)

func (ss *SpoofsService) GetOverride(w http.ResponseWriter, r *http.Request) {
	logger := bslog.With(slog.Any("request_id", r.Context().Value("id")))
	memberOf := r.PathValue("memberOf")

	if memberOf == "" {
		logger.Error("skipping request due to insufficient input parameters", slog.String("reason", "missing fqdn"))
		response.Err(w, response.ErrInvalidInput, "missing fqdn")
		return
	}

	exist, err := ss.spoofRepo.ReadFQDN(memberOf)
	if err != nil {
		logger.Error("could not read spoofs", slog.String("reason", err.Error()))
		response.Err(w, response.ErrInternalError, "")
		return
	}

	if exist.DC != "OVERRIDE" {
		logger.Error("service does not have an active override", slog.String("fqdn", exist.FQDN))
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
		if errors.Is(err, spoofRepo.ErrSpoofWithFQDNNotFound) {
			response.Err(w, response.ErrNotFound, "fqdn not found: "+override.MemberOf)
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
		if errors.Is(err, spoofRepo.ErrSpoofWithFQDNNotFound) {
			response.Err(w, response.ErrNotFound, "member-of not found: "+override.MemberOf)
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
	exist, err := ss.svcRepo.FetchServiceMemberOf(override.MemberOf)
	if err != nil {
		return fmt.Errorf("unable to read spoofs from storage: %w", err)
	}

	if exist.Datacenter == "OVERRIDE" {
		return fmt.Errorf("service already has active override: %s", exist.MemberOf)
	}

	err = ss.svcRepo.Delete(exist.Key())
	if err != nil {
		return fmt.Errorf("could not delete old spoof: %w", err)
	}

	exist.Datacenter = "OVERRIDE"
	exist.IP = override.IP.String()

	err = ss.svcRepo.Create(&exist)
	if err != nil {
		return fmt.Errorf("could not create spoof: %w", err)
	}

	return nil
}

func (ss *SpoofsService) updateOverride(override spoofs.Override) error {
	exist, err := ss.svcRepo.FetchServiceMemberOf(override.MemberOf)
	if err != nil {
		return fmt.Errorf("unable to read spoofs from storage: %w", err)
	}

	if exist.Datacenter != "OVERRIDE" {
		return fmt.Errorf("%s does not have an active override", override.MemberOf)
	}

	exist.IP = override.IP.String()

	err = ss.svcRepo.Update(exist.Key(), &exist)
	if err != nil {
		return fmt.Errorf("could not update spoof: %w", err)
	}

	return nil
}

func (ss *SpoofsService) deleteOverride(override spoofs.Override) error {
	exist, err := ss.svcRepo.FetchServiceMemberOf(override.MemberOf)
	if err != nil {
		return fmt.Errorf("unable to read spoofs from storage: %w", err)
	}

	if exist.Datacenter != "OVERRIDE" {
		return fmt.Errorf("%s does not have an override currently set", override.MemberOf)
	}

	spoof := ss.restoreSpoof(override)

	err = ss.svcRepo.Delete(exist.Key())
	if err != nil {
		return fmt.Errorf("could not update spoof: %w", err)
	}

	if spoof == nil { // if not possible to create new spoof, we return with NO spoof for the fqdn
		return nil
	}

	exist.Datacenter = spoof.DC
	exist.Fqdn = spoof.FQDN
	exist.IP = spoof.IP

	err = ss.svcRepo.Create(&exist)
	if err != nil {
		return fmt.Errorf("could not create spoof for active service: %w", err)
	}

	return nil
}

func (ss *SpoofsService) restoreSpoof(override spoofs.Override) *spoofs.Spoof {
	svc := ss.serviceManager.GetActiveForMemberOf(override.MemberOf)
	if svc == nil { // no active service: e.g. no spoof should be there
		return nil
	}

	ip, err := svc.GetIP()
	if err != nil {
		return nil
	}

	return &spoofs.Spoof{
		FQDN: svc.Fqdn,
		DC:   svc.Datacenter,
		IP:   ip,
	}
}
