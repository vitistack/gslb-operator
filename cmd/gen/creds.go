//go:build ignore

// test-only: stable Ed25519 client credential + a fresh signed client_assertion
// for the token-exchange flow. DO NOT use in production.
package main

import (
    "crypto/ed25519"
    "crypto/rand"
    "encoding/base64"
    "encoding/hex"
    "flag"
    "fmt"
    "time"

    "github.com/golang-jwt/jwt/v5"
)

const audience = "gslb_operator" // must equal jwt.issuerName in the operator config

func main() {
    clientID := flag.String("client", "test-client", "client_id to enroll and assert")
    flag.Parse()

    seed := make([]byte, ed25519.SeedSize) // deterministic → stable keypair (TEST ONLY)
    copy(seed, []byte("gslb-operator-local-test-seed-01"))
    priv := ed25519.NewKeyFromSeed(seed)
    pub := priv.Public().(ed25519.PublicKey)

    jti := make([]byte, 16)
    _, _ = rand.Read(jti)

    now := time.Now()
    assertion, err := jwt.NewWithClaims(jwt.SigningMethodEdDSA, jwt.RegisteredClaims{
        Subject:   *clientID,
        Audience:  jwt.ClaimStrings{audience},
        ID:        hex.EncodeToString(jti),
        IssuedAt:  jwt.NewNumericDate(now),
        ExpiresAt: jwt.NewNumericDate(now.Add(time.Hour)),
    }).SignedString(priv)
    if err != nil {
        panic(err)
    }

    fmt.Println("client_id:        ", *clientID)
    fmt.Println("private_seed_b64: ", base64.StdEncoding.EncodeToString(seed))
    fmt.Println("public_key_b64:   ", base64.StdEncoding.EncodeToString(pub))
    fmt.Println("client_assertion: ", assertion)
}