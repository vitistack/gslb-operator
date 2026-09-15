package authc

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type accessClaims struct {
	Method string    `json:"method"`
	Class  AuthClass `json:"class"`
	jwt.RegisteredClaims
}

// TokenIssuer mints and verifies short-lived operator-signed access tokens.
// Ed25519 keeps verification asymmetric (verifiers need only the public key).
type TokenIssuer struct {
	signingSecret []byte
	issuer        string
	audience      string
	ttl           time.Duration
}

type TokenIssuerOption func(*TokenIssuer)

func WithTTL(ttl time.Duration) TokenIssuerOption {
	return func(ti *TokenIssuer) { ti.ttl = ttl }
}

func WithAudience(aud string) TokenIssuerOption {
	return func(ti *TokenIssuer) {
		ti.audience = aud
	}
}

func WithIssuer(iss string) TokenIssuerOption {
	return func(ti *TokenIssuer) {
		ti.issuer = iss
	}
}

func NewTokenIssuer(signingSecret []byte, opts ...TokenIssuerOption) *TokenIssuer {
	ti := &TokenIssuer{
		signingSecret: signingSecret,
		ttl:           10 * time.Minute,
	}

	for _, opt := range opts {
		opt(ti)
	}

	return ti
}

// Issue mints an access token for the principal; returns token + seconds-to-live.
func (ti *TokenIssuer) Issue(p Principal) (string, int, error) {
	now := time.Now()
	claims := accessClaims{
		Method:    p.Method,
		Class:     p.Class,
		Issuer:    ti.issuer,
		Subject:   p.Subject,
		Audience:  jwt.ClaimStrings{ti.audience},
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(ti.ttl)),
	}
	signed, err := jwt.NewWithClaims(jwt.SigningMethodHS512, claims).SignedString(ti.signingSecret)
	if err != nil {
		return "", 0, fmt.Errorf("sign access token: %w", err)
	}
	return signed, int(ti.ttl.Seconds()), nil
}

// Verify validates an access token and reconstructs the Principal.
func (ti *TokenIssuer) Verify(tokenString string) (Principal, error) {
	claims := &accessClaims{}
	token, err := jwt.ParseWithClaims(tokenString, claims,
		func(t *jwt.Token) (any, error) { return ti.signingSecret, nil },
		jwt.WithValidMethods([]string{"EdDSA"}),
		jwt.WithIssuer(ti.issuer),
		jwt.WithAudience(ti.audience),
		jwt.WithExpirationRequired(),
	)
	if err != nil || !token.Valid {
		return Principal{}, ErrUnauthorized
	}
	return Principal{Subject: claims.Subject, Method: claims.Method, Class: claims.Class}, nil
}
