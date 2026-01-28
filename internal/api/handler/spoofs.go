package handler

import (
	"cmp"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"net/http"
	"slices"

	"github.com/vitistack/gslb-operator/pkg/bslog"
	"github.com/vitistack/gslb-operator/pkg/models/hash"
	"github.com/vitistack/gslb-operator/pkg/models/pagination"
	"github.com/vitistack/gslb-operator/pkg/models/spoofs"
	"github.com/vitistack/gslb-operator/pkg/rest/request"
	"github.com/vitistack/gslb-operator/pkg/rest/response"
)

func (h *Handler) GetSpoofs(w http.ResponseWriter, r *http.Request) {
	data, err := h.SpoofRepo.ReadAll()
	if err != nil {
		response.Err(w, response.ErrInternalError, "unable to fetch spoofs from storage")
		bslog.Error("Unable to fetch spoofs", slog.String("reason", err.Error()))
		return
	}

	params := pagination.NewPaginationParams()
	err = request.MarshallParams(r.URL.Query(), params)
	if err != nil {
		response.Err(w, response.ErrInvalidInput, "could not parse request parameters")
		bslog.Error("unable to parse request parameters", slog.String("reason", err.Error()))
		return
	}

	resp := spoofs.NewSpoofResponse(data, params)
	response.JSON(w, http.StatusOK, resp)
}

func (h *Handler) GetFQDNSpoof(w http.ResponseWriter, r *http.Request) {
	fqdn := r.PathValue("fqdn")
	if fqdn == "" {
		response.Err(w, response.ErrInvalidInput, "empty id is not valid")
		return
	}

	spoof, err := h.SpoofRepo.Read(fqdn)
	if err != nil {
		msg := "unable to fetch spoof with id: " + fqdn + " from storage"
		response.Err(w, response.ErrInternalError, msg)
		bslog.Error(msg, slog.String("reason", err.Error()))
		return
	}

	response.JSON(w, http.StatusOK, spoof)
}

func (h *Handler) GetSpoofsHash(w http.ResponseWriter, r *http.Request) {
	data, err := h.SpoofRepo.ReadAll()
	if err != nil {
		response.Err(w, response.ErrInternalError, "unable to fetch spoofs from storage")
		bslog.Error("unable to read spoofs from storage", slog.String("reason", err.Error()))
		return
	}

	// IMPORTANT!! that spoofs are sorted alphabetically sorted on fqdn.
	// To get consistent hashes
	slices.SortFunc(data, func(a, b spoofs.Spoof) int {
		return cmp.Compare(a.FQDN, b.FQDN)
	})

	marshalledSpoofs, err := json.Marshal(data)
	if err != nil {
		response.Err(w, response.ErrInternalError, "could not create spoofs-hash")
		bslog.Error("unable to marshall spoofs", slog.String("reason", err.Error()))
		return
	}

	rawHash := sha256.Sum256(marshalledSpoofs) // creating bytes representation of spoofs
	hash := hash.Hash{
		Hash: hex.EncodeToString(rawHash[:]),
	}

	if err = response.JSON(w, http.StatusOK, hash); err != nil {
		bslog.Error("could not write response to client", slog.String("reason", err.Error()))
	}
}

func (h *Handler) CreateSpoof(w http.ResponseWriter, r *http.Request) {

}
