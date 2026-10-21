package authc

import (
	"context"
	"errors"
	"time"

	"github.com/valkey-io/valkey-go"
)

// KVStore is the minimal key-value surface Registry and Replay need.
type KVStore interface {
	// SetNX sets key=value only if absent; ttl==0 means no expiry.
	// Returns false if the key already existed.
	SetNX(ctx context.Context, key, value string, ttl time.Duration) (bool, error)
	Set(ctx context.Context, key, value string) error
	Get(ctx context.Context, key string) (value string, found bool, err error)
	Del(ctx context.Context, key string) error
}

type ValkeyStore struct {
	client valkey.Client
}

func NewValkeyStore(client valkey.Client) *ValkeyStore {
	return &ValkeyStore{client: client}
}

func (s *ValkeyStore) SetNX(ctx context.Context, key, value string, ttl time.Duration) (bool, error) {
	var cmd valkey.Completed
	if ttl > 0 {
		cmd = s.client.B().Set().Key(key).Value(value).Nx().ExSeconds(int64(ttl.Seconds())).Build()
	} else {
		cmd = s.client.B().Set().Key(key).Value(value).Nx().Build()
	}
	err := s.client.Do(ctx, cmd).Error()
	if errors.Is(err, valkey.Nil) { // NX not applied → already existed
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func (s *ValkeyStore) Set(ctx context.Context, key, value string) error {
	cmd := s.client.B().Set().Key(key).Value(value).Build()
	return s.client.Do(ctx, cmd).Error()
}

func (s *ValkeyStore) Get(ctx context.Context, key string) (string, bool, error) {
	cmd := s.client.B().Get().Key(key).Build()
	val, err := s.client.Do(ctx, cmd).ToString()
	if errors.Is(err, valkey.Nil) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return val, true, nil
}

func (s *ValkeyStore) Del(ctx context.Context, key string) error {
	cmd := s.client.B().Del().Key(key).Build()
	return s.client.Do(ctx, cmd).Error()
}

