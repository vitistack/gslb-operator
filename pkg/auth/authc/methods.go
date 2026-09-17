package authc

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"fmt"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v5"
	models "github.com/vitistack/gslb-operator/pkg/models/auth"
)

type payloadCtxKey struct{}

func WithLoginPayload(parent context.Context, payload models.LoginPayload) context.Context {
	return context.WithValue(parent, payloadCtxKey{}, payload)
}

func LoginPayloadFrom(ctx context.Context) (models.LoginPayload, bool) {
	p, ok := ctx.Value(payloadCtxKey{}).(models.LoginPayload)
	return p, ok
}

// --- M2M: bootstrap-key (enrol-on-first-use) --------------------------------

type BootstrapKey struct {
	registry *Registry
}

func NewBootstrapKey(reg *Registry) *BootstrapKey {
	return &BootstrapKey{registry: reg}
}

func (b *BootstrapKey) Method() string   { return "bootstrap-key" }
func (b *BootstrapKey) Class() AuthClass { return M2M }

func (b *BootstrapKey) Detect(r *http.Request) bool {
	payload, ok := LoginPayloadFrom(r.Context())
	return ok && payload.PublicKey != "" && payload.ClientAssertion == ""
}

func (b *BootstrapKey) Authenticate(r *http.Request) (Principal, error) {
	payload, ok := LoginPayloadFrom(r.Context())
	if !ok || payload.ClientID == "" || payload.PublicKey == "" {
		return Principal{}, ErrUnauthorized
	}

	pub, err := base64.StdEncoding.DecodeString(payload.PublicKey)
	if err != nil || len(pub) != ed25519.PublicKeySize {
		return Principal{}, ErrUnauthorized
	}
	// FCFS: bind the key now; later private-key-jwt assertions verify against it.
	if err := b.registry.Register(r.Context(), payload.ClientID, ed25519.PublicKey(pub)); err != nil {
		return Principal{}, err
	}

	roles, kv, err := b.registry.Identity(r.Context(), payload.ClientID)
	if err != nil {
		return Principal{}, err
	}

	return Principal{Subject: payload.ClientID, Method: b.Method(), Class: M2M, Roles: roles, KeyVersion: kv}, nil
}

// --- M2M: private-key-jwt (proof of possession) -----------------------------

type ClientAssertion struct {
	registry *Registry
	replay   *Replay
	audience string
	leeway   time.Duration
}

func NewClientAssertion(reg *Registry, replay *Replay, audience string) *ClientAssertion {
	return &ClientAssertion{registry: reg, replay: replay, audience: audience, leeway: 5 * time.Second}
}

func (c *ClientAssertion) Method() string   { return "private-key-jwt" }
func (c *ClientAssertion) Class() AuthClass { return M2M }

// The assertion lives in the body, so this method is selected via the
// explicit X-Auth-Method header (handled by Dispatcher.Resolve) rather than sniffing.
func (c *ClientAssertion) Detect(r *http.Request) bool {
	p, ok := LoginPayloadFrom(r.Context())
	return ok && p.ClientAssertion != ""
}

func (c *ClientAssertion) Authenticate(r *http.Request) (Principal, error) {
	payload, ok := LoginPayloadFrom(r.Context())
	if !ok || payload.ClientAssertion == "" {
		return Principal{}, ErrUnauthorized
	}

	claims := &jwt.RegisteredClaims{}
	token, err := jwt.ParseWithClaims(payload.ClientAssertion, claims,
		func(t *jwt.Token) (any, error) {
			if claims.Subject == "" {
				return nil, ErrUnauthorized
			}
			return c.registry.PublicKey(r.Context(), claims.Subject)
		},
		jwt.WithValidMethods([]string{"EdDSA"}),
		jwt.WithAudience(c.audience),
		jwt.WithExpirationRequired(),
		jwt.WithLeeway(c.leeway),
	)
	if err != nil || !token.Valid || claims.ID == "" {
		return Principal{}, fmt.Errorf("%w:%w", ErrUnauthorized, err)
	}

	ttl := time.Until(claims.ExpiresAt.Time) + c.leeway

	if err := c.replay.Once(r.Context(), claims.ID, ttl); err != nil {
		return Principal{}, err
	}

	if ttl > time.Second*5 {
		return Principal{}, ErrTooLongClientAssertionTTL
	}

	roles, kv, err := c.registry.Identity(r.Context(), claims.Subject)
	if err != nil {
		return Principal{}, err
	}

	return Principal{Subject: claims.Subject, Method: c.Method(), Class: M2M, Roles: roles, KeyVersion: kv}, nil
}

// --- C2M: oidc-session (human via external IdP) ------------------------------

// SessionStore validates a browser/OIDC session and returns the user identity.
type SessionStore interface {
	User(r *http.Request) (string, error)
}

type OIDCSession struct {
	sessions SessionStore
}

func NewOIDCSession(s SessionStore) *OIDCSession { return &OIDCSession{sessions: s} }

func (o *OIDCSession) Method() string   { return "oidc-session" }
func (o *OIDCSession) Class() AuthClass { return C2M }

func (o *OIDCSession) Detect(r *http.Request) bool {
	_, err := r.Cookie("session")
	return err == nil
}

func (o *OIDCSession) Authenticate(r *http.Request) (Principal, error) {
	user, err := o.sessions.User(r)
	if err != nil {
		return Principal{}, ErrUnauthorized
	}
	return Principal{Subject: user, Method: o.Method(), Class: C2M}, nil
}
