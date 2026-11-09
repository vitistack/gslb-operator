package service

import (
	"fmt"
	"log/slog"
	"maps"
	"net/http"
	"slices"
	"strings"

	"github.com/vitistack/gslb-operator/internal/config"
	"github.com/vitistack/gslb-operator/internal/model"
	"github.com/vitistack/gslb-operator/internal/repositories/servicegroup"
	"github.com/vitistack/gslb-operator/internal/repositories/status"
	"github.com/vitistack/gslb-operator/pkg/bslog"
	"github.com/vitistack/gslb-operator/pkg/iter"
	"github.com/vitistack/gslb-operator/pkg/models/pagination"
	"github.com/vitistack/gslb-operator/pkg/models/service"
	"github.com/vitistack/gslb-operator/pkg/persistence"
	"github.com/vitistack/gslb-operator/pkg/rest/request"
	"github.com/vitistack/gslb-operator/pkg/rest/response"
)

type GSLBServiceHandler struct {
	statusRepo   *status.StatusRepo
	svcGroupRepo *servicegroup.ServiceGroupRepo
}

func NewGSLBServiceHandler(statusRepo *status.StatusRepo, serviceStore persistence.Store[model.GSLBServiceGroup]) *GSLBServiceHandler {
	return &GSLBServiceHandler{
		statusRepo:   statusRepo,
		svcGroupRepo: servicegroup.NewServiceGroupRepo(serviceStore),
	}
}

func (h *GSLBServiceHandler) GetServiceStatus(w http.ResponseWriter, r *http.Request) {
	logger := bslog.With(slog.Any("request_id", r.Context().Value("id")))
	fqdn := r.PathValue("memberOf")

	if fqdn == "" {
		logger.Info("skipping due to insufficient input parameter", slog.String("reason", "missing path-value memberOf"))
		response.Err(w, response.ErrInvalidInput, "missing path-value")
		return
	}

	status, err := h.statusRepo.Read(fqdn)
	if err != nil {
		logger.Error("failed to get status", slog.String("reason", err.Error()), slog.String("fqdn", fqdn))
		response.Err(w, response.ErrInternalError, "unexpected error")
		return
	}

	if status.MemberOf == "" {
		logger.Info("could not find status for service", slog.String("fqdn", fqdn))
		response.Err(w, response.ErrNotFound, "no status available yet for: "+fqdn)
		return
	}

	response.JSON(w, http.StatusOK, status)
}

func (h *GSLBServiceHandler) GetMembers(w http.ResponseWriter, r *http.Request) {
	logger := bslog.With(slog.Any("request_id", r.Context().Value("id")))
	memberOf := r.PathValue("memberOf")

	if memberOf == "" {
		logger.Info("skipping due to insufficient input parameter", slog.String("reason", "missing path-value memberOf"))
		response.Err(w, response.ErrInvalidInput, "missing path-value")
		return
	}

	queryParameters := service.GetMembersParams{
		Views: strings.Join(config.DNS().DNSViews(), ","), // defaults to all views
	}
	if err := request.UnmarshalParams(r.URL.Query(), &queryParameters); err != nil {
		logger.Error("failed to parse request parameters", slog.String("reason", err.Error()))
		response.Err(w, response.ErrInvalidInput, "invalid query parameters")
		return
	}
	page := pagination.EnsureDefaults(&queryParameters.PaginationParams)

	group, err := h.svcGroupRepo.Read(memberOf)
	if err != nil {
		logger.Error("failed to get service group", slog.String("reason", err.Error()), slog.String("memberOf", memberOf))
		response.Err(w, response.ErrInternalError, "unexpected error")
		return
	}

	if group.Members == nil {
		response.Err(w, response.ErrNotFound, fmt.Sprintf("%s: not found", memberOf))
		return
	}

	views := strings.Split(queryParameters.Views, ",")
	matches := iter.FromSlice(slices.Sorted(maps.Keys(group.Members))).
		Map(func(id string) service.Member {
			g := group.Members[id]
			return service.Member{
				ID:        g.ID,
				Priority:  g.Priority,
				DC:        g.Datacenter,
				Views:     g.Views,
				Address:   g.Address,
				IsHealthy: g.IsHealthy,
			}
		}).
		Filter(func(m service.Member) bool {
			if !slices.ContainsFunc(m.Views, func(v string) bool { return slices.Contains(views, v) }) {
				return false
			}
			return queryParameters.IsHealthy == nil || m.IsHealthy == *queryParameters.IsHealthy
		}).
		Collect()

	skip := (page.Page - 1) * page.PageSize
	items := iter.FromSlice(matches).Skip(skip).Take(page.PageSize).Collect()

	resp := pagination.NewPaginationResult(page, len(matches), items)
	response.JSON(w, http.StatusOK, resp)
}

func (h *GSLBServiceHandler) GetMembersId(w http.ResponseWriter, r *http.Request) {
	logger := bslog.With(slog.Any("request_id", r.Context().Value("id")))
	memberOf := r.PathValue("memberOf")
	memberId := r.PathValue("id")

	if memberOf == "" || memberId == "" {
		logger.Info("skipping due to insufficient input parameters", slog.String("reason", "missing path-value"))
		response.Err(w, response.ErrInvalidInput, "missing path-value")
		return
	}

	group, err := h.svcGroupRepo.Read(memberOf)
	if err != nil {
		logger.Error("failed to get service group", slog.String("reason", err.Error()), slog.String("memberOf", memberOf))
		response.Err(w, response.ErrInternalError, "unexpected error")
		return
	}

	if group.Members == nil {
		response.Err(w, response.ErrNotFound, fmt.Sprintf("memberOf %s: not found", memberOf))
		return
	}

	var groupMember *service.Member
	for id, member := range group.Members {
		if id == memberId {
			groupMember = &service.Member{
				ID:        member.ID,
				Priority:  member.Priority,
				DC:        member.Datacenter,
				Views:     member.Views,
				Address:   member.Address,
				IsHealthy: member.IsHealthy,
			}
		}
	}

	if groupMember == nil {
		response.Err(w, response.ErrNotFound, fmt.Sprintf("member-id %s: not found", memberId))
		return
	}

	response.JSON(w, http.StatusOK, groupMember)
}
