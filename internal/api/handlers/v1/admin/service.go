package admin

import (
	"context"
	"encoding/json/v2"
	"errors"
	"log/slog"
	"net/http"

	"github.com/vitistack/gslb-operator/pkg/auth/authc"
	"github.com/vitistack/gslb-operator/pkg/iter"
	"github.com/vitistack/gslb-operator/pkg/models/auth"
	"github.com/vitistack/gslb-operator/pkg/models/pagination"
	"github.com/vitistack/gslb-operator/pkg/rest/response"
	"github.com/vitistack/gslb-operator/pkg/rest/router"
)

type ClientRegistry interface {
	Enrollments(ctx context.Context) ([]authc.Enrollment, error)
	EnrollmentOf(ctx context.Context, id string) (authc.Enrollment, error)
	SetAttributes(ctx context.Context, clientID string, attributes map[string][]string) error
	Attributes(ctx context.Context, clientID string) (map[string][]string, error)
	AssignRoles(ctx context.Context, clientID string, roles ...string) error
	Identity(ctx context.Context, clientID string) (roles []string, keyVersion int, err error)
}

type AdminService struct {
	registry ClientRegistry
}

func NewAdminService(registry ClientRegistry) *AdminService {
	return &AdminService{
		registry: registry,
	}
}

func (a *AdminService) GetAPIEnrollments(w http.ResponseWriter, r *router.Request) {
	queryParams, err := pagination.NewPaginationParams(r.URL.Query())
	if err != nil {
		r.Log().Error("failed to parse pagination parameters", slog.String("reason", err.Error()))
		response.Err(w, response.ErrInvalidInput, "invalid input")
		return
	}

	enrollments, err := a.registry.Enrollments(r.Context())
	if err != nil {
		r.Log().Error("failed to list API enrollments", slog.String("reason", err.Error()))
		response.Err(w, response.ErrInternalError, "unexpected error")
		return
	}
	skip := (queryParams.Page - 1) * queryParams.PageSize
	result := iter.FromSlice(enrollments).
		Skip(skip).
		Map(
			func(e authc.Enrollment) auth.Enrollment {
				return auth.Enrollment{
					ID:         "", // TODO!
					Roles:      e.Roles,
					Attributes: e.Attributes,
				}
			},
		).
		Take(queryParams.PageSize).
		Collect()

	resp := pagination.NewPaginationResult(r.Request, queryParams, len(enrollments), result)
	response.JSON(w, http.StatusOK, resp)
}

func (a *AdminService) GetAPIEnrollment(w http.ResponseWriter, r *router.Request) {
	id := r.Param("id")

	enrollment, err := a.registry.EnrollmentOf(r.Context(), id)
	if err != nil {
		if errors.Is(err, authc.ErrClientNotFound) {
			response.Err(w, response.ErrNotFound, "not found")
			return
		}
		r.Log().Error("failed to get enrollment", slog.String("reason", err.Error()))
		response.Err(w, response.ErrInternalError, "unexpected error")
		return
	}

	response.JSON(w, http.StatusOK,
		auth.Enrollment{
			ID:         id,
			Roles:      enrollment.Roles,
			Attributes: enrollment.Attributes,
		},
	)
}

func (a *AdminService) PutGrants(w http.ResponseWriter, r *router.Request) {
	id := r.Param("id")

	grants := auth.Grant{}
	if err := json.UnmarshalRead(r.Body, &grants,
		json.RejectUnknownMembers(true),
		json.RejectUnknownMembers(true),
	); err != nil {
		r.Log().Error("failed to unmarshall request body", slog.String("reason", err.Error()))
		response.Err(w, response.ErrInvalidInput, "invalid input")
		return
	}

	err := a.registry.SetAttributes(r.Context(), id, *grants.Attributes)
	if err != nil {
		if errors.Is(err, authc.ErrClientNotFound) {
			r.Log().Info("skipping grant request", slog.String("reason", err.Error()))
			response.Err(w, response.ErrNotFound, "not found")
			return
		}
		r.Log().Error("failed to grant attributes", slog.String("reason", err.Error()))
		response.Err(w, response.ErrInternalError, "unexpected error")
		return
	}

	err = a.registry.AssignRoles(r.Context(), id, *grants.Roles...)
	if err != nil {
		r.Log().Error("failed to grant roles", slog.String("reason", err.Error()))
		response.Err(w, response.ErrInternalError, "unexpected error")
		return
	}

	w.WriteHeader(http.StatusCreated)
}
