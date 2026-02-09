package spoofs

/**
* NOTE: overrides are strictly used to manually update (from CLI) the spoofed ip adress for a service
* this is meant to only be used in an emergency. and is generally considered a disruptive action, due to it being no checking.
* be cautious when using this.
* for a more gracefull approach, see failover.
 */

import (
	"fmt"
	"log/slog"
	"net/http"

	"github.com/vitistack/gslb-operator/pkg/bslog"
	"github.com/vitistack/gslb-operator/pkg/models/spoofs"
	"github.com/vitistack/gslb-operator/pkg/rest/request"
	"github.com/vitistack/gslb-operator/pkg/rest/response"
)

/*
* {
*	"fqdn": "example.com",
* 	"ip": "10.10.0.1",
* }
 */

func (ss *SpoofsService) GetOverride(w http.ResponseWriter, r *http.Request) {

}

func (ss *SpoofsService) CreateOverride(w http.ResponseWriter, r *http.Request) {
	override := spoofs.Override{}

	err := request.JSONDECODE(r.Body, &override)
	if err != nil {
		bslog.Error("could not decode request body", slog.String("reason", err.Error()), slog.Any("request_id", r.Context().Value("id")))
		response.Err(w, response.ErrInvalidInput, "unable to decode request body")
		return
	}

	err = ss.newOverride(override)
	if err != nil {
		bslog.Error("could not override spoof", slog.String("reason", err.Error()), slog.Any("request_id", r.Context().Value("id")))
		response.Err(w, response.ErrInvalidInput, "unable to create spoof")
		return
	}

	w.WriteHeader(http.StatusCreated)
}

func (ss *SpoofsService) DeleteOverride(w http.ResponseWriter, r *http.Request) {
	override := spoofs.Override{}

	err := request.JSONDECODE(r.Body, &override)
	if err != nil {
		bslog.Error("could not decode request body", slog.String("reason", err.Error()), slog.Any("request_id", r.Context().Value("id")))
		response.Err(w, response.ErrInvalidInput, "unable to decode request body")
		return
	}

	err = ss.deleteOverride(override)
	if err != nil {
		bslog.Error("could not delete overridden spoof", slog.String("reason", err.Error()), slog.Any("request_id", r.Context().Value("id")))
		response.Err(w, response.ErrInvalidInput, "unable to delete override")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (ss *SpoofsService) newOverride(override spoofs.Override) error {
	exist, err := ss.SpoofRepo.ReadFQDN(override.FQDN)
	if err != nil {
		return fmt.Errorf("unable to read spoofs from storage: %w", err)
	}

	err = ss.SpoofRepo.Delete(exist.Key())
	if err != nil {
		return fmt.Errorf("could not delete old spoof: %w", err)
	}

	exist.DC = "OVERRIDE"
	exist.IP = override.IP.String()

	err = ss.SpoofRepo.Create(exist.Key(), &exist)
	if err != nil {
		return fmt.Errorf("could not update spoof: %w", err)
	}

	return nil
}

func (ss *SpoofsService) deleteOverride(override spoofs.Override) error {
	exist, err := ss.SpoofRepo.ReadFQDN(override.FQDN)
	if err != nil {
		return fmt.Errorf("unable to read spoofs from storage: %w", err)
	}

	if exist.DC != "OVERRIDE" {
		return fmt.Errorf("%s does not have an override currently set", override.FQDN)
	}

	spoof := ss.restoreSpoof(override)
	err = ss.SpoofRepo.Delete(exist.Key())
	if err != nil {
		return fmt.Errorf("could not updat spoof: %w", err)
	}

	err = ss.SpoofRepo.Create(spoof.Key(), &spoof)
	if err != nil {
		return fmt.Errorf("could not create spoof for active service: %w", err)
	}

	return nil
}
