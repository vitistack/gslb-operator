package dnsdist

import (
	"log/slog"
	"net/http"
	"slices"
	"strings"

	"github.com/vitistack/gslb-operator/internal/config"
	"github.com/vitistack/gslb-operator/internal/model"
	"github.com/vitistack/gslb-operator/internal/repositories/spoof"
	"github.com/vitistack/gslb-operator/pkg/bslog"
	"github.com/vitistack/gslb-operator/pkg/models/dnsdist"
	"github.com/vitistack/gslb-operator/pkg/models/spoofs"
	"github.com/vitistack/gslb-operator/pkg/persistence"
	"github.com/vitistack/gslb-operator/pkg/rest/request"
	"github.com/vitistack/gslb-operator/pkg/rest/response"
)

type DNSDistService struct {
	spoofRepo *spoof.SpoofRepo
}

func NewDNSDistService(store persistence.Store[model.GSLBServiceGroup]) *DNSDistService {
	return &DNSDistService{
		spoofRepo: spoof.NewSpoofRepo(store),
	}
}

func (s *DNSDistService) GetConfig(w http.ResponseWriter, r *http.Request) {
	logger := bslog.With(slog.Any("request_id", r.Context().Value("id")))

	params := dnsdist.GetConfigParams{}
	err := request.UnMarshallParams(r.URL.Query(), &params)
	if err != nil {
		logger.Error("failed to unmarshall request query-parameters", slog.String("reason", err.Error()))
		response.Err(w, response.ErrInvalidInput, "query-parameters not valid")
		return
	}

	encoder, err := newOutputEncoder(params.Format)
	if err != nil {
		logger.Error("unsupported output format requested", slog.String("reason", err.Error()))
		response.Err(w, response.ErrInvalidInput, err.Error())
		return
	}

	views := strings.Split(params.Views, ",")
	for i := range views {
		views[i] = strings.TrimSpace(views[i])
	}
	if len(views) == 1 && views[0] == "" {
		views = []string{config.DNS().DefaultView()}
	}

	allSpoofs, finish := s.spoofRepo.ReadAll()
	result := allSpoofs.Filter(
		func(s spoofs.Spoof) bool { return slices.Contains(views, s.View) },
	).Collect()

	if err := finish(); err != nil {
		logger.Error("failed to iterate spoofs", slog.String("reason", err.Error()))
		response.Err(w, response.ErrInternalError, "unexpected error ocurred")
		return
	}

	if err := encoder.encode(w, result); err != nil {
		logger.Error("failed to write config response", slog.String("reason", err.Error()))
	}
}
