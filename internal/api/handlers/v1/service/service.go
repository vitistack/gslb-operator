package service

import (
	"fmt"
	"log/slog"
	"net/http"

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

	page := pagination.NewPaginationParams()
	err := request.UnMarshallParams(r.URL.Query(), page)
	if err != nil {
		logger.Error("failed to parse pagination parameters", slog.String("reason", err.Error()))
		response.Err(w, response.ErrInvalidInput, "invalid pagination parameters")
		return
	}

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

	defaults := pagination.NewPaginationParams()
	if page.Page < 1 {
		page.Page = defaults.Page
	}
	if page.PageSize < 1 {
		page.PageSize = defaults.PageSize
	}
	skip := (page.Page - 1) * page.PageSize
	items := make([]service.Member, 0, len(group.Members))
	iter.FromMap(group.Members).
		Skip(skip).
		Take(page.PageSize).
		Each(func(s string, g model.GSLBService) {
			items = append(
				items,
				service.Member{
					ID:        g.ID,
					Priority:  g.Priority,
					DC:        g.Datacenter,
					Views:     g.Views,
					Address:   g.Address,
					IsHealthy: g.IsHealthy,
				},
			)
		})

	resp := pagination.NewPaginationResult(page, len(group.Members), items)
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
