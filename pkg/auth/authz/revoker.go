package authz

import (
	"context"
	"time"
)

// Revoker answers "is this identity/token killed right now?" on the hot path.
type Revoker interface {
	Revoke(ctx context.Context, subject string, ttl time.Duration) error
	IsRevoked(ctx context.Context, subject string) (bool, error)
	KeyVersion(ctx context.Context, subject string) (int, error)
}
