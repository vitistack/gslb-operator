package handler

import (
	"cmp"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"slices"

	"github.com/vitistack/gslb-operator/internal/model"
	"github.com/vitistack/gslb-operator/pkg/rest/request"
	"github.com/vitistack/gslb-operator/pkg/rest/response"
)

func (h *Handler) GetSpoofs(w http.ResponseWriter, r *http.Request) {
	spoofs, err := h.SpoofRepo.ReadAll()
	if err != nil {
		response.Err(w, response.ErrInternalError, "unable to fetch spoofs from storage")
		h.log.Sugar().Errorf("Unable to fetch all spoofs: %s", err.Error())
		return
	}

	params := request.NewPaginationParams()
	err = request.MarshallParams(r.URL.Query(), params)
	if err != nil {
		response.Err(w, response.ErrInvalidInput, "could not parse request parameters")
		h.log.Sugar().Errorf("unable to parse request parameters: %s", err.Error())
		return
	}

	resp := model.NewSpoofResponse(spoofs, params)
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
		h.log.Sugar().Errorf("%s: %s", msg, err.Error())
		return
	}

	response.JSON(w, http.StatusOK, spoof)
}

func (h *Handler) GetSpoofsHash(w http.ResponseWriter, r *http.Request) {
	spoofs, err := h.SpoofRepo.ReadAll()
	if err != nil {
		response.Err(w, response.ErrInternalError, "unable to fetch spoofs from storage")
		h.log.Sugar().Errorf("unable to read spoofs from storage: %s", err.Error())
		return
	}

	// IMPORTANT!! that spoofs are sorted alphabetically sorted on fqdn.
	// To get consistent hashes
	slices.SortFunc(spoofs, func(a, b model.Spoof) int {
		return cmp.Compare(a.FQDN, b.FQDN)
	})

	marshalledSpoofs, err := json.Marshal(spoofs)
	if err != nil {
		response.Err(w, response.ErrInternalError, "could not create spoofs-hash")
		h.log.Sugar().Errorf("unable to marshall spoofs: %s", err.Error())
		return
	}

	rawHash := sha256.Sum256(marshalledSpoofs) // creating bytes representation of spoofs
	hash := model.Hash{
		Hash: hex.EncodeToString(rawHash[:]),
	}

	if err = response.JSON(w, http.StatusOK, hash); err != nil {
		h.log.Sugar().Errorf("could not write response to client: %s", err.Error())
	}
}

func (h *Handler) CreateSpoof(w http.ResponseWriter, r *http.Request) {

}
