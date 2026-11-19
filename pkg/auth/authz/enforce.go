package authz

import (
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/vitistack/gslb-operator/pkg/auth/authc"
	"github.com/vitistack/gslb-operator/pkg/bslog"
	"github.com/vitistack/gslb-operator/pkg/rest/middleware"
	"github.com/vitistack/gslb-operator/pkg/rest/response"
)

type Enforcer struct {
	authorizer Authorizer
	revoker    Revoker
}

func NewEnforcer(a Authorizer, r Revoker) *Enforcer {
	return &Enforcer{authorizer: a, revoker: r}
}

func (e *Enforcer) Enforce(objectType, action, pattern string, logger *slog.Logger) middleware.MiddlewareFunc {
	return middleware.Chain(
		e.CheckPrincipal(logger),
		e.Decide(objectType, action, pattern, logger),
		BuildGrant(logger),
	)
}

// CheckPrincipal rejects revoked subjects and tokens minted under a superseded key.
func (e *Enforcer) CheckPrincipal(logger *slog.Logger) middleware.MiddlewareFunc {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			reqLogger := bslog.NewWithCtx(r.Context(), logger)
			principal, ok := authc.PrincipalFrom(r.Context())
			if !ok {
				reqLogger.Info("principal check rejected", slog.String("reason", "missing principal"))
				response.Err(w, response.ErrUnauthorized, "missing principal")
				return
			}

			revoked, err := e.revoker.IsRevoked(r.Context(), principal.Subject)
			if err != nil {
				reqLogger.Error("revocation check failed", slog.String("reason", err.Error()))
				response.Err(w, response.ErrInternalError, "authorization unavailable")
				return
			}
			if revoked {
				reqLogger.Warn("access revoked")
				response.Err(w, response.ErrForbidden, "Forbidden")
				return
			}

			// TODO: blacklisted

			if principal.Class != authc.C2M {
				currentKeyVersion, err := e.revoker.KeyVersion(r.Context(), principal.Subject)
				switch {
				case errors.Is(err, authc.ErrClientNotFound):
					reqLogger.Warn("client not found")
					response.Err(w, response.ErrUnauthorized, "UnAuthorized")
					return
				case err != nil:
					reqLogger.Error("key version check failed", slog.String("reason", err.Error()))
					response.Err(w, response.ErrInternalError, "authorization unavailable")
					return
				case principal.KeyVersion < currentKeyVersion:
					reqLogger.Warn("token superseded by key rotation")
					response.Err(w, response.ErrUnauthorized, "UnAuthorized")
					return
				}
			}

			next.ServeHTTP(w, r)
		}
	}
}

// Decide asks the policy engine and stores the Decision for BuildGrant.
func (e *Enforcer) Decide(objectType, action, pattern string, logger *slog.Logger) middleware.MiddlewareFunc {
	attributeKeys := wildcardNames(pattern)
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			r = r.WithContext(
				bslog.CtxWithAttrs(
					r.Context(),
					slog.String("objectType", objectType),
					slog.String("action", action),
				),
			)
			reqLogger := bslog.NewWithCtx(r.Context(), logger)
			principal, ok := authc.PrincipalFrom(r.Context())
			if !ok {
				reqLogger.Warn("policy decision rejected", slog.String("reason", "missing principal"))
				response.Err(w, response.ErrUnauthorized, "UnAuthorized")
				return
			}

			in := Input{
				Subject:  principal.Subject,
				Method:   principal.Method,
				Class:    string(principal.Class),
				Roles:    principal.Roles,
				Action:   action,
				Resource: objectType,
				Attrs:    pathAttrs(r, attributeKeys),
			}

			dec, err := e.authorizer.Authorize(r.Context(), in)
			if err != nil {
				reqLogger.Error("policy evaluation failed", slog.String("reason", err.Error()))
				response.Err(w, response.ErrInternalError, "authorization unavailable")
				return
			}

			if !dec.Allow {
				reqLogger.Warn("access denied",
					slog.String("reason", dec.Reason),
				)
				response.Err(w, response.ErrForbidden, "Forbidden")
				return
			}

			if len(dec.Obligations) > 0 {
				reqLogger.Error("unfulfilled policy obligations",
					slog.Any("obligations", dec.Obligations),
				)
				response.Err(w, response.ErrInternalError, "authorization unavailable")
				return
			}

			next.ServeHTTP(w, r.WithContext(withDecision(r.Context(), dec)))
		}
	}
}

// BuildGrant turns the request's Decision into the Grant handlers consult.
func BuildGrant(logger *slog.Logger) middleware.MiddlewareFunc {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			dec, ok := decisionFrom(r.Context())
			if !ok {
				logger.Error("grant build failed", slog.String("reason", "no decision in context"))
				response.Err(w, response.ErrInternalError, "authorization unavailable")
				return
			}

			next.ServeHTTP(w, r.WithContext(withGrant(r.Context(), Grant{allowed: dec.Allow, constraints: dec.Constraints})))
		}
	}
}

// pull route params (e.g. {fqdn}, {memberOf}) into policy attributes
func pathAttrs(r *http.Request, keys []string) map[string]string {
	attrs := map[string]string{}
	for _, k := range keys {
		if v := r.PathValue(k); v != "" {
			attrs[k] = v
		}
	}
	return attrs
}

// wildcardNames extracts {name} / {name...} params from a stdlib mux pattern.
func wildcardNames(pattern string) []string {
	var names []string
	for {
		open := strings.IndexByte(pattern, '{')
		if open < 0 {
			break
		}
		rest := pattern[open+1:]
		before, after, ok := strings.Cut(rest, "}")
		if !ok {
			break
		}
		name := strings.TrimSuffix(before, "...")
		if name != "" && name != "$" {
			names = append(names, name)
		}
		pattern = after
	}
	return names
}
