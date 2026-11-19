package spoofs

import (
	"log/slog"
	"net/http"

	"github.com/vitistack/gslb-operator/pkg/bslog"
	"github.com/vitistack/gslb-operator/pkg/models/pagination"
	"github.com/vitistack/gslb-operator/pkg/models/spoofs"
	"github.com/vitistack/gslb-operator/pkg/rest/response"
	"github.com/vitistack/gslb-operator/pkg/rest/router"
)

func (ss *SpoofsService) GetSpoofs(w http.ResponseWriter, r *router.Request) {
	params, err := pagination.NewPaginationParams(r.URL.Query())
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

	resp := pagination.NewPaginationResult(r.Request, params, total, items)
	response.JSON(w, http.StatusOK, resp)
}

func (ss *SpoofsService) GetFQDNSpoof(w http.ResponseWriter, r *router.Request) {
	memberOf := r.Param("memberOf")

	spoof, err := ss.spoofRepo.Read(memberOf)
	if err != nil {
		msg := "unable to fetch spoof with id: " + memberOf
		response.Err(w, response.ErrInternalError, msg)
		bslog.Error(msg, slog.String("reason", err.Error()))
		return
	}

	response.JSON(w, http.StatusOK, spoof)
}
