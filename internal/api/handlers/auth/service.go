package auth

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"

	"github.com/vitistack/gslb-operator/internal/config"
	"github.com/vitistack/gslb-operator/pkg/auth/authc"
	"github.com/vitistack/gslb-operator/pkg/auth/authz"
	"github.com/vitistack/gslb-operator/pkg/bslog"
	"github.com/vitistack/gslb-operator/pkg/models/auth"
	"github.com/vitistack/gslb-operator/pkg/rest/middleware"
	"github.com/vitistack/gslb-operator/pkg/rest/response"
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

	scopeGuard := authz.NewScopeGuard(
		authz.NewAttributeScopeGuard(registry, "memberOf"),
	)

	authorizer := authz.NewRBACAuthorizer(policy, scopeGuard)
	enforcer := authz.NewEnforcer(authorizer, registry)

	tokenIssuer := authc.NewTokenIssuer(
		config.Auth().JWT().Secret(),
		authc.WithAudience(config.Auth().JWT().Audience()),
		authc.WithIssuer(config.Auth().JWT().Issuer()),
		authc.WithTTL(config.Auth().JWT().TTL()),
	)

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
func (a *AuthService) Token(w http.ResponseWriter, r *http.Request) {
	logger := bslog.With("request_id", r.Context().Value("id"))

	principal, err := a.dispatcher.Authenticate(r)
	switch {
	case errors.Is(err, authc.ErrNoMethodDetected), errors.Is(err, authc.ErrUnknownMethod):
		bslog.Info("token exchange failed", slog.String("reason", "no usable authentication method"))
		response.Err(w, response.ErrInvalidInput, "no usable authentication method")
		return
	case errors.Is(err, authc.ErrKeyMismatch):
		bslog.Info("token exchange failed", slog.String("reason", "client_id registered to different key"))
		response.Err(w, response.ErrConflict, "client_id already registered to a different key")
		return
	case errors.Is(err, authc.ErrClientExists):
		bslog.Info("token exchange failed", slog.String("reason", "client enrolled"))
		response.Err(w, response.ErrConflict, "client already enrolled: use private-key-jwt")
		return
	case errors.Is(err, authc.ErrReplayed):
		response.Err(w, response.ErrUnauthorized, "client assertion already used")
	case errors.Is(err, authc.ErrTooLongClientAssertionTTL):
		response.Err(w, response.ErrForbidden, "too long TTL on client-assertion")
	case err != nil:
		logger.Error("authentication failed", slog.String("reason", err.Error()))
		response.Err(w, response.ErrInternalError, "authentication unavailable")
		return
	}

	token, expiresIn, err := a.issuer.Issue(principal)
	if err != nil {
		logger.Error("token issue failed", slog.String("reason", err.Error()))
		response.Err(w, response.ErrInternalError, "could not issue token")
		return
	}

	logger.Info(
		"authenticated",
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
			raw := bearerToken(r)
			if raw == "" {
				response.Err(w, response.ErrUnauthorized, "missing auth token")
				return
			}

			principal, err := a.issuer.Verify(raw)
			if err != nil {
				response.Err(w, response.ErrUnauthorized, "invalid access token")
				return
			}
			next.ServeHTTP(w, r.WithContext(authc.WithPrincipal(r.Context(), principal)))
		}
	}
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
