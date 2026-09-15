package authc

import (
	"context"
	"fmt"
	"time"
)

const replayPrefix = "authc:jti:"

type Replay struct {
	store KVStore
}

func NewReplay(store KVStore) *Replay {
	return &Replay{store: store}
}

func (r *Replay) Once(ctx context.Context, jti string, ttl time.Duration) error {
	if jti == "" {
		return fmt.Errorf("empty jti")
	}
	if ttl < time.Second {
		ttl = time.Second
	}
	set, err := r.store.SetNX(ctx, replayPrefix+jti, "1", ttl)
	if err != nil {
		return fmt.Errorf("replay check failed: %w", err)
	}
	if !set {
		return ErrReplayed
	}
	return nil
}
