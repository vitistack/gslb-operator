package authc

import (
	"context"
	"crypto/ed25519"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"
)

// enrollment is the per-client record: bound key plus server-side attributes.
// AllowedZones is coarse eligibility from the external source; Attributes holds
// materialized owned values (e.g. memberOf) claimed during config polling.
type enrollment struct {
	PublicKey  string              `json:"pub"`             // base64 Ed25519
	Roles      []string            `json:"roles"`           // assigned out-of-band, never self-claimed
	Attributes map[string][]string `json:"attrs,omitempty"` // owned values, materialized
	KeyVersion int                 `json:"kv"`
}

type Registry struct {
	store KVStore
}

func NewRegistry(store KVStore) *Registry {
	return &Registry{store: store}
}

func (r *Registry) Register(ctx context.Context, clientID string, pub ed25519.PublicKey, roles ...string) error {
	val, err := json.Marshal(enrollment{
		PublicKey:  base64.StdEncoding.EncodeToString(pub),
		Roles: []string{"default"},
		KeyVersion: 1,
	})
	if err != nil {
		return fmt.Errorf("enroll marshal: %w", err)
	}

	set, err := r.store.SetNX(ctx, registerPrefix+clientID, string(val), 0)
	if err != nil {
		return fmt.Errorf("enroll failed: %w", err)
	}

	if !set {
		return ErrClientExists
	}

	return nil
}

// Rotate re-binds the client's key and bumps KeyVersion; other fields preserved.
func (r *Registry) Rotate(ctx context.Context, clientID string, pub ed25519.PublicKey) error {
	rec, err := r.enrollmentOf(ctx, clientID)
	if err != nil {
		return err
	}

	rec.PublicKey = base64.StdEncoding.EncodeToString(pub)
	rec.KeyVersion++
	return r.save(ctx, clientID, rec)
}

// AssignRoles sets a client's roles; an admin operation, not self-service.
func (r *Registry) AssignRoles(ctx context.Context, clientID string, roles ...string) error {
	rec, err := r.enrollmentOf(ctx, clientID)
	if err != nil {
		return err
	}

	rec.Roles = roles
	return r.save(ctx, clientID, rec)
}

// SetOwned replaces a single owned attribute key, preserving the others — the
// poll materializes memberOf without touching zones or roles.
func (r *Registry) SetOwned(ctx context.Context, clientID, key string, values []string) error {
	rec, err := r.enrollmentOf(ctx, clientID)
	if err != nil {
		return err
	}

	if rec.Attributes == nil {
		rec.Attributes = map[string][]string{}
	}

	rec.Attributes[key] = values
	return r.save(ctx, clientID, rec)
}

// SetAttribute replaces one attribute key, preserving the others — the coarse
// grant and the materialized set are independent keys, written by different callers.
func (r *Registry) SetAttribute(ctx context.Context, clientID, key string, values []string) error {
	rec, err := r.enrollmentOf(ctx, clientID)
	if err != nil {
		return err
	}

	if rec.Attributes == nil {
		rec.Attributes = map[string][]string{}
	}

	rec.Attributes[key] = values
	return r.save(ctx, clientID, rec)
}

// Attributes returns owned values, read live so an ownership change takes effect
// without re-issuing the client's token.
func (r *Registry) Attributes(ctx context.Context, clientID string) (map[string][]string, error) {
	rec, err := r.enrollmentOf(ctx, clientID)
	if err != nil {
		return nil, err
	}

	return rec.Attributes, nil
}

func (r *Registry) Revoke(ctx context.Context, subject string, ttl time.Duration) error {
	if _, err := r.store.SetNX(ctx, revokePrefix+subject, "1", ttl); err != nil {
		return fmt.Errorf("revoke: %w", err)
	}

	return nil
}

func (r *Registry) IsRevoked(ctx context.Context, subject string) (bool, error) {
	_, found, err := r.store.Get(ctx, revokePrefix+subject)
	return found, err
}

// KeyVersion is the current key version for a subject; satisfies authz.Revoker.
func (r *Registry) KeyVersion(ctx context.Context, clientID string) (int, error) {
	rec, err := r.enrollmentOf(ctx, clientID)
	if err != nil {
		return 0, err
	}

	return rec.KeyVersion, nil
}

// Identity returns the attributes stamped onto a Principal at auth time.
func (r *Registry) Identity(ctx context.Context, clientID string) (roles []string, keyVersion int, err error) {
	rec, err := r.enrollmentOf(ctx, clientID)
	if err != nil {
		return nil, 0, err
	}

	return rec.Roles, rec.KeyVersion, nil
}

func (r *Registry) PublicKey(ctx context.Context, clientID string) (ed25519.PublicKey, error) {
	rec, err := r.enrollmentOf(ctx, clientID)
	if err != nil {
		return nil, err
	}

	decoded, err := base64.StdEncoding.DecodeString(rec.PublicKey)
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

func (r *Registry) enrollmentOf(ctx context.Context, clientID string) (enrollment, error) {
	raw, found, err := r.store.Get(ctx, registerPrefix+clientID)
	if err != nil {
		return enrollment{}, fmt.Errorf("lookup failed: %w", err)
	}

	if !found {
		return enrollment{}, ErrClientNotFound
	}

	var rec enrollment
	if err := json.Unmarshal([]byte(raw), &rec); err != nil {
		return enrollment{}, fmt.Errorf("decode enrollment: %w", err)
	}

	return rec, nil
}

func (r *Registry) save(ctx context.Context, clientID string, rec enrollment) error {
	val, err := json.Marshal(rec)
	if err != nil {
		return fmt.Errorf("enroll marshal: %w", err)
	}

	return r.store.Set(ctx, registerPrefix+clientID, string(val))
}
