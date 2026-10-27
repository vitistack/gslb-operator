package authz

import (
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/vitistack/gslb-operator/pkg/auth/authc"
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

// Enforce builds the policy input from the verified Principal + request, runs
// the instant kill-switch and key-rotation checks, then asks the policy engine.
func (e *Enforcer) Enforce(action Action, pattern string, logger *slog.Logger) middleware.MiddlewareFunc {
	attributeKeys := wildcardNames(pattern)
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			principal, ok := authc.PrincipalFrom(r.Context())
			if !ok {
				logger.Info("skipping authz enforce", slog.String("reason", "missing principal"))
				response.Err(w, response.ErrUnauthorized, "missing principal")
				return
			}

			// 1) instant kill-switch
			revoked, err := e.revoker.IsRevoked(r.Context(), principal.Subject)
			if err != nil {
				logger.Error("revocation check failed", slog.String("reason", err.Error()))
				response.Err(w, response.ErrInternalError, "authorization unavailable")
				return
			}

			if revoked {
				response.Err(w, response.ErrForbidden, "access revoked")
				return
			}
			
			// TODO: blacklisted

			// 2) key rotation: reject tokens minted under a superseded key
			if principal.Class != authc.C2M {
				cur, err := e.revoker.KeyVersion(r.Context(), principal.Subject)
				switch {
				case errors.Is(err, authc.ErrClientNotFound):
					response.Err(w, response.ErrUnauthorized, "client no longer enrolled")
					return
				case err != nil:
					logger.Error("key version check failed", slog.String("reason", err.Error()))
					response.Err(w, response.ErrInternalError, "authorization unavailable")
					return
				case principal.KeyVersion < cur:
					response.Err(w, response.ErrUnauthorized, "token superseded by key rotation")
					return
				}
			}

			// 3) policy decision
			in := Input{
				Subject: principal.Subject,
				Method:  principal.Method,
				Class:   string(principal.Class),
				Roles:   principal.Roles,
				Action:  string(action),
				//Resource: resource,
				Attrs: pathAttrs(r, attributeKeys),
			}

			dec, err := e.authorizer.Authorize(r.Context(), in)
			if err != nil {
				logger.Error("policy evaluation failed", slog.String("reason", err.Error()))
				response.Err(w, response.ErrInternalError, "authorization unavailable")
				return
			}

			if !dec.Allow {
				logger.Warn("access denied",
					slog.String("subject", principal.Subject),
					slog.String("action", in.Action),
					slog.String("resource", in.Resource),
					slog.String("reason", dec.Reason),
				)
				response.Err(w, response.ErrForbidden, "insufficient permissions")
				return
			}

			next.ServeHTTP(w, r)
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
		end := strings.IndexByte(rest, '}')
		if end < 0 {
			break
		}
		name := strings.TrimSuffix(rest[:end], "...")
		if name != "" && name != "$" {
			names = append(names, name)
		}
		pattern = rest[end+1:]
	}
	return names
}
