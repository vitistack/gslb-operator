package m2m

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/vitistack/gslb-operator/pkg/clients/gslb/auth"
	authmodels "github.com/vitistack/gslb-operator/pkg/models/auth"
)

// assertionTTL must stay under the 5s remaining-lifetime ceiling the server enforces.
const assertionTTL = 4 * time.Second

// refreshSkew re-authenticates slightly ahead of actual access-token expiry.
const refreshSkew = 10 * time.Second

// Credentials is the long-lived M2M identity used to obtain access tokens automatically.
type Credentials struct {
	ClientID   string
	PrivateKey ed25519.PrivateKey
	Audience   string // must match the server's configured client-assertion audience
}

type tokenSource struct {
	auth  auth.Client
	creds Credentials

	mu        sync.Mutex
	token     authmodels.TokenResponse
	expiresAt time.Time
}

func newTokenSource(authClient auth.Client, creds Credentials) *tokenSource {
	return &tokenSource{auth: authClient, creds: creds}
}

func (ts *tokenSource) Token(ctx context.Context) (authmodels.TokenResponse, error) {
	ts.mu.Lock()
	defer ts.mu.Unlock()

	if ts.token.AccessToken != "" && time.Now().Before(ts.expiresAt) {
		return ts.token, nil
	}

	resp, err := ts.authenticate(ctx)
	if err != nil {
		return authmodels.TokenResponse{}, err
	}

	ts.token = resp
	ts.expiresAt = time.Now().Add(time.Duration(resp.ExpiresIn)*time.Second - refreshSkew)
	return ts.token, nil
}

// authenticate proves possession of the private key; the very first call (not
// yet enrolled) falls back to binding the public key before retrying.
func (ts *tokenSource) authenticate(ctx context.Context) (authmodels.TokenResponse, error) {
	assertion, err := ts.signAssertion()
	if err != nil {
		return authmodels.TokenResponse{}, fmt.Errorf("sign client assertion: %w", err)
	}

	if resp, err := ts.auth.Token(ctx, authmodels.LoginPayload{ClientAssertion: assertion}); err == nil {
		return resp, nil
	} else {
		pub, _ := ts.creds.PrivateKey.Public().(ed25519.PublicKey)
		resp, bootstrapErr := ts.auth.Token(ctx, authmodels.LoginPayload{
			ClientID:  ts.creds.ClientID,
			PublicKey: base64.StdEncoding.EncodeToString(pub),
		})

		if bootstrapErr != nil {
			return authmodels.TokenResponse{}, errors.Join(err, bootstrapErr)
		}
		return resp, nil
	}
}

func (ts *tokenSource) signAssertion() (string, error) {
	jti, err := randomID()
	if err != nil {
		return "", err
	}

	now := time.Now()
	claims := jwt.RegisteredClaims{
		Subject:   ts.creds.ClientID,
		Audience:  jwt.ClaimStrings{ts.creds.Audience},
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(assertionTTL)),
		ID:        jti,
	}

	return jwt.NewWithClaims(jwt.SigningMethodEdDSA, claims).SignedString(ts.creds.PrivateKey)
}

func randomID() (string, error) {
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("generate jti: %w", err)
	}
	return hex.EncodeToString(raw), nil
}
