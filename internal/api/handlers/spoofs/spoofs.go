package spoofs

import (
	"log/slog"
	"net/http"

	"github.com/vitistack/gslb-operator/pkg/bslog"
	"github.com/vitistack/gslb-operator/pkg/models/pagination"
	"github.com/vitistack/gslb-operator/pkg/models/spoofs"
	"github.com/vitistack/gslb-operator/pkg/rest/request"
	"github.com/vitistack/gslb-operator/pkg/rest/response"
)

func (ss *SpoofsService) GetSpoofs(w http.ResponseWriter, r *http.Request) {
	params := pagination.NewPaginationParams()
	err := request.UnMarshallParams(r.URL.Query(), params)
	if err != nil {
		response.Err(w, response.ErrInvalidInput, "could not parse request parameters")
		bslog.Error("unable to parse request parameters", slog.String("reason", err.Error()))
		return
	}

	total := 0
	items := make([]spoofs.Spoof, 0, params.PageSize)
	startIdx := (params.Page - 1) * params.PageSize

	spoofsResult, finish := ss.spoofRepo.ReadAll()
	spoofsResult.Tap(func(s spoofs.Spoof) { total++ }).Skip(startIdx).Each(
		func(s spoofs.Spoof) {
			if len(items) < params.PageSize {
				items = append(items, s)
			}
		})

	if err := finish(); err != nil {
		response.Err(w, response.ErrInternalError, "unable to fetch spoofs")
		bslog.Error("Unable to fetch spoofs", slog.String("reason", err.Error()))
		return
	}

	resp := pagination.NewPaginationResult(params, total, items)
	response.JSON(w, http.StatusOK, resp)
}

func (ss *SpoofsService) GetFQDNSpoof(w http.ResponseWriter, r *http.Request) {
	fqdn := r.PathValue("fqdn")
	if fqdn == "" {
		response.Err(w, response.ErrInvalidInput, "empty id is not valid")
		return
	}

	spoof, err := ss.spoofRepo.Read(fqdn)
	if err != nil {
		msg := "unable to fetch spoof with id: " + fqdn
		response.Err(w, response.ErrInternalError, msg)
		bslog.Error(msg, slog.String("reason", err.Error()))
		return
	}

	response.JSON(w, http.StatusOK, spoof)
}

//func (ss *SpoofsService) GetSpoofsHash(w http.ResponseWriter, r *http.Request) {
//	data, err := ss.spoofRepo.ReadAll()
//	if err != nil {
//		response.Err(w, response.ErrInternalError, "unable to fetch spoofs from storage")
//		bslog.Error("unable to read spoofs from storage", slog.String("reason", err.Error()))
//		return
//	}
//
//	// IMPORTANT!! that spoofs are sorted alphabetically sorted on fqdn.
//	// To get consistent hashes
//	slices.SortFunc(data, func(a, b spoofs.Spoof) int {
//		return cmp.Compare(a.FQDN, b.FQDN)
//	})
//
//	marshalledSpoofs, err := json.Marshal(data)
//	if err != nil {
//		response.Err(w, response.ErrInternalError, "could not create spoofs-hash")
//		bslog.Error("unable to marshall spoofs", slog.String("reason", err.Error()))
//		return
//	}
//
//	rawHash := sha256.Sum256(marshalledSpoofs) // creating bytes representation of spoofs
//	hash := spoofs.Hash{
//		Hash: hex.EncodeToString(rawHash[:]),
//	}
//
//	if err = response.JSON(w, http.StatusOK, hash); err != nil {
//		bslog.Error("could not write response to client", slog.String("reason", err.Error()))
//	}
//}
