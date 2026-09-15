package auth

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/vitistack/gslb-operator/pkg/auth/authc"
	"github.com/vitistack/gslb-operator/pkg/models/auth"
)

const (
	testSecret   = "test-signing-secret-please-change"
	testIssuer   = "gslb-operator"
	testAudience = "gslb-operator"
)

func newTestService(t *testing.T) *AuthService {
	t.Helper()
	store := authc.NewMemStore()
	registry := authc.NewRegistry(store)
	replay := authc.NewReplay(store)
	issuer := authc.NewTokenIssuer(
		[]byte(testSecret),
		authc.WithAudience(testAudience),
		authc.WithIssuer(testIssuer),
		authc.WithTTL(time.Minute),
	)

	return NewAuthService(issuer,
		authc.NewBootstrapKey(registry),
		authc.NewClientAssertion(registry, replay, testAudience),
	)
}

func postToken(t *testing.T, svc *AuthService, body map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/auth/token", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	svc.Token(rec, req)
	return rec
}

func genKey(t *testing.T) (ed25519.PublicKey, ed25519.PrivateKey) {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	return pub, priv
}

func encodeKey(pub ed25519.PublicKey) string {
	return base64.StdEncoding.EncodeToString(pub)
}

func buildAssertion(t *testing.T, clientID string, priv ed25519.PrivateKey) string {
	t.Helper()
	now := time.Now()
	claims := jwt.RegisteredClaims{
		Issuer:    clientID,
		Subject:   clientID,
		Audience:  jwt.ClaimStrings{testAudience},
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(time.Minute)),
		ID:        newJTI(t), // unique per assertion (replay guard)
	}
	signed, err := jwt.NewWithClaims(jwt.SigningMethodEdDSA, claims).SignedString(priv)
	if err != nil {
		t.Fatalf("sign assertion: %v", err)
	}
	return signed
}

func newJTI(t *testing.T) string {
	t.Helper()
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		t.Fatalf("rand: %v", err)
	}
	return base64.RawURLEncoding.EncodeToString(b)
}

func decodeToken(t *testing.T, rec *httptest.ResponseRecorder) auth.TokenResponse {
	t.Helper()
	var resp auth.TokenResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode token response: %v", err)
	}
	return resp
}

func TestToken_FirstRegistration(t *testing.T) {
	svc := newTestService(t)
	pub, _ := genKey(t)

	rec := postToken(t, svc, map[string]string{
		"client_id":  "client_id",
		"public_key": encodeKey(pub),
	})

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	resp := decodeToken(t, rec)
	if resp.AccessToken == "" {
		t.Fatal("expected an access token")
	}
	if resp.TokenType != "Bearer" {
		t.Fatalf("expected token_type Bearer, got %q", resp.TokenType)
	}
}

func TestToken_ClientAssertion(t *testing.T) {
	svc := newTestService(t)
	pub, priv := genKey(t)
	const clientID = "client_id"

	// enrol on first contact (bootstrap)
	if rec := postToken(t, svc, map[string]string{
		"client_id":  clientID,
		"public_key": encodeKey(pub),
	}); rec.Code != http.StatusOK {
		t.Fatalf("bootstrap: expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	// authenticate against the enrolled key via a signed assertion
	rec := postToken(t, svc, map[string]string{
		"client_assertion": buildAssertion(t, clientID, priv),
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("assertion: expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if decodeToken(t, rec).AccessToken == "" {
		t.Fatal("expected an access token from the assertion flow")
	}
}

func TestToken_ClientAssertionReplay(t *testing.T) {
    svc := newTestService(t)
    pub, priv := genKey(t)
    const clientID = "client_id"

    // enrol on first contact (bootstrap)
    if rec := postToken(t, svc, map[string]string{
        "client_id":  clientID,
        "public_key": encodeKey(pub),
    }); rec.Code != http.StatusOK {
        t.Fatalf("bootstrap: expected 200, got %d: %s", rec.Code, rec.Body.String())
    }

    // same assertion (same jti) submitted twice
    assertion := buildAssertion(t, clientID, priv)

    if rec := postToken(t, svc, map[string]string{
        "client_assertion": assertion,
    }); rec.Code != http.StatusOK {
        t.Fatalf("first use: expected 200, got %d: %s", rec.Code, rec.Body.String())
    }

    rec := postToken(t, svc, map[string]string{
        "client_assertion": assertion,
    })
    if rec.Code != http.StatusUnauthorized {
        t.Fatalf("replay: expected 401, got %d: %s", rec.Code, rec.Body.String())
    }
}

func TestToken_DoubleRegistration(t *testing.T) {
	svc := newTestService(t)
	const clientID = "client_id"

	pub1, _ := genKey(t)
	if rec := postToken(t, svc, map[string]string{
		"client_id":  clientID,
		"public_key": encodeKey(pub1),
	}); rec.Code != http.StatusOK {
		t.Fatalf("first registration: expected 200, got %d", rec.Code)
	}

	t.Run("same key is idempotent", func(t *testing.T) {
		rec := postToken(t, svc, map[string]string{
			"client_id":  clientID,
			"public_key": encodeKey(pub1),
		})
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("different key conflicts", func(t *testing.T) {
		pub2, _ := genKey(t)
		rec := postToken(t, svc, map[string]string{
			"client_id":  clientID,
			"public_key": encodeKey(pub2),
		})
		if rec.Code != http.StatusConflict {
			t.Fatalf("expected 409, got %d: %s", rec.Code, rec.Body.String())
		}
	})
}
