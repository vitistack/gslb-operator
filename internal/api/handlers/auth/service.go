package auth

import (
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/vitistack/gslb-operator/pkg/auth/authc"
	"github.com/vitistack/gslb-operator/pkg/bslog"
	"github.com/vitistack/gslb-operator/pkg/models/auth"
	"github.com/vitistack/gslb-operator/pkg/rest/middleware"
	"github.com/vitistack/gslb-operator/pkg/rest/response"
)

type AuthService struct {
	dispatcher *authc.Dispatcher
	issuer     *authc.TokenIssuer
}

func NewAuthService(issuer *authc.TokenIssuer, authenticators ...authc.Authenticator) *AuthService {
	return &AuthService{
		dispatcher: authc.NewDispatcher(authenticators...),
		issuer:     issuer,
	}
}

// verify a credential and issue a short-lived access token
func (a *AuthService) Token(w http.ResponseWriter, r *http.Request) {
	logger := bslog.With("request_id", r.Context().Value("id"))

	principal, err := a.dispatcher.Authenticate(r)
	switch {
	case errors.Is(err, authc.ErrNoMethodDetected), errors.Is(err, authc.ErrUnknownMethod):
		response.Err(w, response.ErrInvalidInput, "no usable authentication method")
		return
	case errors.Is(err, authc.ErrKeyMismatch):
		response.Err(w, response.ErrConflict, "client_id already registered to a different key")
		return
	case err != nil:
		logger.Warn("authentication failed", slog.String("reason", err.Error()))
		response.Err(w, response.ErrUnauthorized, "authentication failed")
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
