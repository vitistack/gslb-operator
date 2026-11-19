package auth

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"slices"
	"strings"

	"github.com/vitistack/gslb-operator/internal/config"
	"github.com/vitistack/gslb-operator/pkg/auth/authc"
	"github.com/vitistack/gslb-operator/pkg/auth/authz"
	"github.com/vitistack/gslb-operator/pkg/bslog"
	"github.com/vitistack/gslb-operator/pkg/models/auth"
	"github.com/vitistack/gslb-operator/pkg/rest/middleware"
	"github.com/vitistack/gslb-operator/pkg/rest/response"
	"github.com/vitistack/gslb-operator/pkg/rest/router"
)

type AuthService struct {
	dispatcher *authc.Dispatcher
	issuer     *authc.TokenIssuer
	enforcer   *authz.Enforcer
}

func Init(store authc.KVStore) (*AuthService, error) {
	registry := authc.NewRegistry(store)
	authReplay := authc.NewReplay(store)

	rawPolicy, err := os.ReadFile(config.Auth().Policy())
	if err != nil {
		return nil, fmt.Errorf("failed to read policy file: %w", err)
	}

	policy, err := authz.ParsePolicy(rawPolicy)
	if err != nil {
		return nil, fmt.Errorf("failed to parse policy: %w", err)
	}

	if err := policy.Validate(); err != nil {
		return nil, fmt.Errorf("invalid policy: %w", err)
	}

	authorizer := authz.NewOwnershipAuthorizer(authz.NewRBACAuthorizer(policy), registry, "memberOf")
	enforcer := authz.NewEnforcer(authorizer, registry)

	tokenIssuerOpts := []authc.TokenIssuerOption{
		authc.WithAudience(config.Auth().JWT().Audience()),
		authc.WithIssuer(config.Auth().JWT().Issuer()),
	}

	if ttl := config.Auth().JWT().TTL(); ttl > 0 {
		tokenIssuerOpts = append(tokenIssuerOpts, authc.WithTTL(ttl))
	}

	tokenIssuer, err := authc.NewTokenIssuer(
		config.Auth().JWT().Secret(),
		tokenIssuerOpts...,
	)

	if err != nil {
		return nil, err
	}

	return newAuthService(
		tokenIssuer,
		enforcer,
		authc.NewBootstrapKey(registry),
		authc.NewClientAssertion(registry, authReplay, config.Auth().JWT().Issuer()),
	), nil
}

func newAuthService(issuer *authc.TokenIssuer, enforcer *authz.Enforcer, authenticators ...authc.Authenticator) *AuthService {
	return &AuthService{
		dispatcher: authc.NewDispatcher(authenticators...),
		issuer:     issuer,
		enforcer:   enforcer,
	}
}

// verify a credential and issue a short-lived access token
func (a *AuthService) Token(w http.ResponseWriter, r *router.Request) {
	principal, err := a.dispatcher.Authenticate(r.Request)
	switch {
	case errors.Is(err, authc.ErrNoMethodDetected), errors.Is(err, authc.ErrUnknownMethod):
		r.Log().Warn("token exchange failed", slog.String("reason", "no usable authentication method"))
		response.Err(w, response.ErrInvalidInput, "no usable authentication method")
		return
	case errors.Is(err, authc.ErrKeyMismatch):
		r.Log().Warn("token exchange failed", slog.String("reason", "client_id registered to different key"))
		response.Err(w, response.ErrConflict, "Conflict")
		return
	case errors.Is(err, authc.ErrClientExists):
		r.Log().Warn("token exchange failed", slog.String("reason", "client already enrolled"))
		response.Err(w, response.ErrConflict, "Conflict")
		return
	case errors.Is(err, authc.ErrReplayed):
		r.Log().Warn("token exchange failed", slog.String("reason", "client assertion replay"))
		response.Err(w, response.ErrUnauthorized, "UnAuthorized")
		return
	case errors.Is(err, authc.ErrTooLongClientAssertionTTL):
		r.Log().Warn("token exchange failed", slog.String("reason", "client-assertion ttl exceeds max ttl value"))
		response.Err(w, response.ErrForbidden, "Forbidden")
		return
	case errors.Is(err, authc.ErrUnauthorized):
		var errorAttributes slog.Attr
		if verify, ok := errors.AsType[*authc.VerifyError](err); ok {
			errorAttributes = slog.Any("token", verify)
		} else {
			errorAttributes = slog.String("reason", err.Error())
		}
		r.Log().Warn("token exchange failed", errorAttributes)
		response.Err(w, response.ErrUnauthorized, "UnAuthorized")
		return
	case err != nil:
		r.Log().Error("authentication failed", slog.String("reason", err.Error()))
		response.Err(w, response.ErrInternalError, "authentication unavailable")
		return
	}

	token, expiresIn, err := a.issuer.Issue(principal)
	if err != nil {
		r.Log().Error("token issue failed", slog.String("reason", err.Error()))
		response.Err(w, response.ErrInternalError, "Unexpected error ocurred")
		return
	}

	r.Log().Info(
		"issuing token",
		slog.String("subject", principal.Subject),
		slog.String("method", principal.Method),
		slog.String("class", string(principal.Class)),
	)

	response.JSON(w, http.StatusOK, auth.TokenResponse{
		AccessToken: token,
		TokenType:   "Bearer",
		ExpiresIn:   expiresIn,
	})
}

func (a *AuthService) Verify() middleware.MiddlewareFunc {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			logger := bslog.NewWithCtx(r.Context(), nil)
			raw := bearerToken(r)
			if raw == "" {
				logger.Warn("token verification failed",
					slog.String("reason", "missing Bearer token"),
					slog.String("authScheme", authScheme(r)),
				)
				response.Err(w, response.ErrUnauthorized, "UnAuthorized")
				return
			}

			principal, err := a.issuer.Verify(raw)
			if err != nil {
				logger.Warn("token verification failed",
					slog.Any("token", err),
				)
				response.Err(w, response.ErrUnauthorized, "UnAuthorized")
				return
			}

			ctx := authc.WithPrincipal(r.Context(), principal)
			ctx = bslog.CtxWithAttrs(ctx,
				slog.Group("auth",
					slog.String("subject", principal.Subject),
					slog.String("method", principal.Method),
					slog.String("class", string(principal.Class)),
				),
			)
			next.ServeHTTP(w, r.WithContext(ctx))
		}
	}
}

var knownSchemes = []string{"basic", "bearer", "digest", "dpop", "negotiate"}

// authScheme never returns the credential itself, only a recognised scheme.
func authScheme(r *http.Request) string {
	h := r.Header.Get("Authorization")
	if h == "" {
		return "none"
	}
	scheme, _, ok := strings.Cut(h, " ")
	if !ok || !slices.Contains(knownSchemes, strings.ToLower(scheme)) {
		return "unrecognised"
	}
	return scheme
}

func bearerToken(r *http.Request) string {
	parts := strings.SplitN(r.Header.Get("Authorization"), " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return ""
	}
	return strings.TrimSpace(parts[1])
}

func (a *AuthService) Enforcer() *authz.Enforcer {
	return a.enforcer
}
