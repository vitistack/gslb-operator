package authc

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"log/slog"
	"time"

	"github.com/golang-jwt/jwt/v5"
	models "github.com/vitistack/gslb-operator/pkg/models/auth"
)

var (
	ErrActiveToken               = errors.New("authc: client already has an active token")
	ErrUnauthorized              = errors.New("authc: unauthorized")
	ErrClientExists              = errors.New("authc: client_id already enrolled")
	ErrClientNotFound            = errors.New("authc: client not enrolled")
	ErrKeyMismatch               = errors.New("authc: public key mismatch")
	ErrReplayed                  = errors.New("authc: assertion replayed")
	ErrUnknownMethod             = errors.New("authc: unknown authentication method")
	ErrNoMethodDetected          = errors.New("authc: could not detect an authentication method")
	ErrTooLongClientAssertionTTL = errors.New("authc: client-assertion token exceeds tolerated TTL")
)

type VerifyError struct {
	cause       error
	fingerprint string
	parsed      bool
	alg         string
	kid         string
	claims      accessClaims
}

func newVerifyError(raw string, cause error) *VerifyError {
	sum := sha256.Sum256([]byte(raw))
	e := &VerifyError{cause: cause, fingerprint: hex.EncodeToString(sum[:8])}

	token, _, err := jwt.NewParser().ParseUnverified(raw, &e.claims)
	if err == nil {
		e.parsed = true
		e.alg, _ = token.Header["alg"].(string)
		e.kid, _ = token.Header["kid"].(string)
	}
	return e
}

func (e *VerifyError) Error() string { return "access token rejected: " + e.cause.Error() }

func (e *VerifyError) Unwrap() []error { return []error{ErrUnauthorized, e.cause} }

// Claimed returns the principal the token asserts, unverified.
func (e *VerifyError) Claimed() Principal {
	return Principal{
		Subject:    e.claims.Subject,
		Method:     e.claims.Method,
		Class:      e.claims.Class,
		Roles:      e.claims.Roles,
		KeyVersion: e.claims.KeyVersion,
	}
}

func (e *VerifyError) LogValue() slog.Value {
	c := e.claims
	attrs := []slog.Attr{
		slog.String("reason", e.cause.Error()),
		slog.String("sha256", e.fingerprint),
		slog.Bool("parsed", e.parsed),
		slog.Bool("verified", false),
		slog.String("alg", clip(e.alg)),
		slog.String("kid", clip(e.kid)),
		slog.String("sub", clip(c.Subject)),
		slog.String("method", clip(c.Method)),
		slog.String("class", clip(string(c.Class))),
		slog.Any("roles", clipAll(c.Roles)),
		slog.Int("kv", c.KeyVersion),
		slog.String("iss", clip(c.Issuer)),
		slog.Any("aud", clipAll(c.Audience)),
		slog.String("jti", clip(c.ID)),
	}

	if c.IssuedAt != nil {
		attrs = append(attrs, slog.Time("iat", c.IssuedAt.Time))
	}

	if c.NotBefore != nil {
		attrs = append(attrs, slog.Time("nbf", c.NotBefore.Time))
	}

	if c.ExpiresAt != nil {
		attrs = append(attrs, slog.Time("exp", c.ExpiresAt.Time))
		if since := time.Since(c.ExpiresAt.Time); since > 0 {
			attrs = append(attrs, slog.Duration("expiredFor", since))
		}
	}

	return slog.GroupValue(attrs...)
}

const (
	maxClaimLen   = 128
	maxClaimItems = 16
)

func clip(s string) string {
	if len(s) > maxClaimLen {
		return s[:maxClaimLen] + "..."
	}
	return s
}

func clipAll(ss []string) []string {
	out := make([]string, 0, min(len(ss), maxClaimItems))
	for _, s := range ss[:min(len(ss), maxClaimItems)] {
		out = append(out, clip(s))
	}
	return out
}

// LoginError is a failed token exchange; every claimed field is UNVERIFIED.
type LoginError struct {
	cause     error
	method    string
	class     AuthClass
	payload   models.LoginPayload
	decodeErr error
}

func (e *LoginError) Error() string { return "login failed (" + e.method + "): " + e.cause.Error() }

func (e *LoginError) Unwrap() error { return e.cause }

func (e *LoginError) LogValue() slog.Value {
	p := e.payload
	attrs := []slog.Attr{
		slog.String("reason", e.cause.Error()),
		slog.String("method", clip(e.method)),
		slog.String("class", string(e.class)),
		slog.Bool("verified", false),
		slog.String("client_id", clip(p.ClientID)),
		slog.Group("fields_present",
			slog.Bool("clientId", p.ClientID != ""),
			slog.Bool("publicKey", p.PublicKey != ""),
			slog.Bool("clientAssertion", p.ClientAssertion != ""),
		),
	}
	if e.decodeErr != nil && e.decodeErr != io.EOF {
		attrs = append(attrs, slog.String("body_decode_error", clip(e.decodeErr.Error())))
	}
	if p.PublicKey != "" {
		attrs = append(attrs, slog.String("public_key_sha256", fingerprint(p.PublicKey)))
	}
	if p.ClientAssertion != "" {
		attrs = append(attrs, slog.Attr{Key: "assertion", Value: slog.GroupValue(assertionAttrs(p.ClientAssertion)...)})
	}
	return slog.GroupValue(attrs...)
}

func assertionAttrs(raw string) []slog.Attr {
	attrs := []slog.Attr{slog.String("sha256", fingerprint(raw))}

	claims := &jwt.RegisteredClaims{}
	token, _, err := jwt.NewParser().ParseUnverified(raw, claims)
	attrs = append(attrs, slog.Bool("parsed", err == nil))
	if err != nil {
		return attrs
	}

	alg, _ := token.Header["alg"].(string)
	kid, _ := token.Header["kid"].(string)
	attrs = append(attrs, slog.String("alg", clip(alg)), slog.String("kid", clip(kid)))
	return append(attrs, registeredClaimAttrs(claims)...)
}

func registeredClaimAttrs(c *jwt.RegisteredClaims) []slog.Attr {
	attrs := []slog.Attr{
		slog.String("sub", clip(c.Subject)),
		slog.String("iss", clip(c.Issuer)),
		slog.Any("aud", clipAll(c.Audience)),
		slog.String("jti", clip(c.ID)),
	}
	if c.IssuedAt != nil {
		attrs = append(attrs, slog.Time("iat", c.IssuedAt.Time))
	}
	if c.NotBefore != nil {
		attrs = append(attrs, slog.Time("nbf", c.NotBefore.Time))
	}
	if c.ExpiresAt != nil {
		attrs = append(attrs, slog.Time("exp", c.ExpiresAt.Time))
		if c.IssuedAt != nil {
			attrs = append(attrs, slog.Duration("ttl", c.ExpiresAt.Sub(c.IssuedAt.Time)))
		}
	}
	return attrs
}

func fingerprint(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:8])
}
