package jwt

import (
	jwt "github.com/golang-jwt/jwt/v5"
)

type tokenIssuerOption func(issuer *TokenIssuer)

type TokenIssuer struct {
	secret        []byte
	signingMethod jwt.SigningMethod
}

func NewTokenIssuer(secret []byte, opts ...tokenIssuerOption) *TokenIssuer {
	issuer := &TokenIssuer{
		secret:        secret,
		signingMethod: jwt.SigningMethodHS512,
	}

	for _, opt := range opts {
		opt(issuer)
	}

	return issuer
}

func WithSigningMethod(method jwt.SigningMethod) tokenIssuerOption {
	return func(issuer *TokenIssuer) {
		issuer.signingMethod = method
	}
}

func (ti *TokenIssuer) New(claims jwt.Claims) (string, error) {
	return jwt.NewWithClaims(ti.signingMethod, claims).SignedString(ti.secret)
}
