package authc

import (
    "context"
    "crypto/ed25519"
    "crypto/subtle"
    "encoding/base64"
    "fmt"
)

const enrollmentPrefix = "authc:client:"

type Registry struct {
    store KVStore
}

func NewRegistry(store KVStore) *Registry {
    return &Registry{store: store}
}

func (r *Registry) Register(ctx context.Context, clientID string, pub ed25519.PublicKey) error {
    val := base64.StdEncoding.EncodeToString(pub)
    set, err := r.store.SetNX(ctx, enrollmentPrefix+clientID, val, 0)
    if err != nil {
        return fmt.Errorf("enroll failed: %w", err)
    }
    if !set {
        return ErrClientExists
    }
    return nil
}

func (r *Registry) Rotate(ctx context.Context, clientID string, pub ed25519.PublicKey) error {
    return r.store.Set(ctx, enrollmentPrefix+clientID, base64.StdEncoding.EncodeToString(pub))
}

func (r *Registry) Revoke(ctx context.Context, clientID string) error {
    return r.store.Del(ctx, enrollmentPrefix+clientID)
}

func (r *Registry) PublicKey(ctx context.Context, clientID string) (ed25519.PublicKey, error) {
    raw, found, err := r.store.Get(ctx, enrollmentPrefix+clientID)
    if err != nil {
        return nil, fmt.Errorf("lookup failed: %w", err)
    }
    if !found {
        return nil, ErrClientNotFound
    }
    decoded, err := base64.StdEncoding.DecodeString(raw)
    if err != nil {
        return nil, fmt.Errorf("decode key: %w", err)
    }
    return ed25519.PublicKey(decoded), nil
}

func (r *Registry) Verify(ctx context.Context, clientID string, pub ed25519.PublicKey) error {
    stored, err := r.PublicKey(ctx, clientID)
    if err != nil {
        return err
    }
    if subtle.ConstantTimeCompare(stored, pub) != 1 {
        return ErrKeyMismatch
    }
    return nil
}