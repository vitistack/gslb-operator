package failover

import (
	"log/slog"
	"net/http"

	"github.com/vitistack/gslb-operator/internal/manager"
	"github.com/vitistack/gslb-operator/pkg/bslog"
	"github.com/vitistack/gslb-operator/pkg/models/failover"
	"github.com/vitistack/gslb-operator/pkg/rest/request"
	"github.com/vitistack/gslb-operator/pkg/rest/response"
)

type FailoverService struct {
	serviceManager manager.QueryManager
}

func NewFailoverService(mgr manager.QueryManager) *FailoverService {
	return &FailoverService{
		serviceManager: mgr,
	}
}

func (fs *FailoverService) FailoverService(w http.ResponseWriter, r *http.Request) {
	logger := bslog.With(slog.Any("request_id", r.Context().Value("id")))
	fqdn := r.PathValue("fqdn")

	if fqdn == "" {
		logger.Error("skipping request due to insufficient input parameters", slog.String("reason", "missing fqdn"))
		response.Err(w, response.ErrInvalidInput, "missing fqdn")
		return
	}

	failover := failover.Failover{}
	err := request.JSONDECODE(r.Body, &failover)
	if err != nil {
		logger.Error("could not decode response body", slog.String("reason", err.Error()))
		response.Err(w, response.ErrInvalidInput, "invalid request format")
		return
	}

	err = fs.serviceManager.Failover(fqdn, failover)
	if err != nil {
		logger.Error("could not perform failover action", slog.String("reason", err.Error()))
		response.Err(w, response.ErrInvalidInput, "unable to perform failover")
		return
	}

	w.WriteHeader(http.StatusCreated)
}
