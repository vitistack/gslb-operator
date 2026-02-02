package spoofs_service

import (
	"cmp"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"slices"

	"github.com/vitistack/gslb-operator/internal/repositories/spoof"
	"github.com/vitistack/gslb-operator/pkg/bslog"
	"github.com/vitistack/gslb-operator/pkg/models/hash"
	"github.com/vitistack/gslb-operator/pkg/models/pagination"
	"github.com/vitistack/gslb-operator/pkg/models/spoofs"
	"github.com/vitistack/gslb-operator/pkg/persistence"
	"github.com/vitistack/gslb-operator/pkg/persistence/store/file"
	"github.com/vitistack/gslb-operator/pkg/rest/request"
	"github.com/vitistack/gslb-operator/pkg/rest/response"
)

type SpoofsService struct {
	SpoofRepo persistence.Repository[spoofs.Spoof]
}

func NewSpoofsService() (*SpoofsService, error) {
	h := &SpoofsService{}

	store, err := file.NewStore[spoofs.Spoof]("store.json")
	if err != nil {
		return nil, fmt.Errorf("could not create filestore: %s", err.Error())
	}

	h.SpoofRepo = spoof.NewRepository(store)

	return h, nil
}

func (ss *SpoofsService) GetSpoofs(w http.ResponseWriter, r *http.Request) {
	data, err := ss.SpoofRepo.ReadAll()
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

func (ss *SpoofsService) GetFQDNSpoof(w http.ResponseWriter, r *http.Request) {
	fqdn := r.PathValue("fqdn")
	if fqdn == "" {
		response.Err(w, response.ErrInvalidInput, "empty id is not valid")
		return
	}

	spoof, err := ss.SpoofRepo.Read(fqdn)
	if err != nil {
		msg := "unable to fetch spoof with id: " + fqdn + " from storage"
		response.Err(w, response.ErrInternalError, msg)
		bslog.Error(msg, slog.String("reason", err.Error()))
		return
	}

	response.JSON(w, http.StatusOK, spoof)
}

func (ss *SpoofsService) GetSpoofsHash(w http.ResponseWriter, r *http.Request) {
	data, err := ss.SpoofRepo.ReadAll()
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

func (ss *SpoofsService) CreateSpoof(w http.ResponseWriter, r *http.Request) {

}
