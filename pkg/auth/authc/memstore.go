package authc

import (
	"context"
	"sync"
	"time"
)

type memEntry struct {
	value  string
	expiry time.Time // zero = no expiry
}

// MemStore is an in-memory KVStore with TTL support, for tests.
type MemStore struct {
	mu   sync.Mutex
	data map[string]memEntry
}

func NewMemStore() *MemStore {
	return &MemStore{data: make(map[string]memEntry)}
}

func (m *MemStore) live(key string) (memEntry, bool) {
	e, ok := m.data[key]
	if !ok {
		return memEntry{}, false
	}
	if !e.expiry.IsZero() && time.Now().After(e.expiry) {
		delete(m.data, key)
		return memEntry{}, false
	}
	return e, true
}

func (m *MemStore) SetNX(_ context.Context, key, value string, ttl time.Duration) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.live(key); ok {
		return false, nil
	}
	m.data[key] = m.entry(value, ttl)
	return true, nil
}

func (m *MemStore) Set(_ context.Context, key, value string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.data[key] = memEntry{value: value}
	return nil
}

func (m *MemStore) Get(_ context.Context, key string) (string, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	e, ok := m.live(key)
	if !ok {
		return "", false, nil
	}
	return e.value, true, nil
}

func (m *MemStore) Del(_ context.Context, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.data, key)
	return nil
}

func (m *MemStore) entry(value string, ttl time.Duration) memEntry {
	e := memEntry{value: value}
	if ttl > 0 {
		e.expiry = time.Now().Add(ttl)
	}
	return e
}
