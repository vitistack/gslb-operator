package valkey

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/valkey-io/valkey-go"
)

var sharedClient valkey.Client // shared valkey client

type ValkeyStore[T any] struct {
	client   valkey.Client
	baseKey  string
	cacheTTL time.Duration
}

func NewClient(opts valkey.ClientOption) (valkey.Client, error) {
	if sharedClient != nil {
		return sharedClient, nil
	}

	client, err := valkey.NewClient(opts)
	if err != nil {
		return nil, fmt.Errorf("failed to create valkey client: %w", err)
	}
	sharedClient = client

	return sharedClient, nil
}

func NewStore[T any](c valkey.Client, base string, ttl time.Duration) *ValkeyStore[T] {
	return &ValkeyStore[T]{
		client:   c,
		baseKey:  base,
		cacheTTL: ttl,
	}
}

// stringifies struct to valkey store
func (s *ValkeyStore[T]) Save(key string, data T) error {
	rawData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshall data: %w", err)
	}

	key = s.baseKey + ":" + key
	cmd := s.client.B().Set().Key(key).Value(string(rawData)).Build()
	return s.client.Do(context.Background(), cmd).Error()
}

// retrieves a record from valkey
func (s *ValkeyStore[T]) Load(key string) (T, error) {
	var zero T // "null" value of T
	//lookup key
	key = s.baseKey + ":" + key

	// fetch record
	cmd := s.client.B().Get().Key(key).Cache()
	result, err := s.client.DoCache(context.Background(), cmd, s.cacheTTL).ToString()

	if err == valkey.Nil { // not found
		return zero, nil
	}

	if err != nil {
		// val is still uninitialized so returning it would be the same as returning empty object!
		return zero, fmt.Errorf("failed to retrieve record from storage: %w", err)
	}

	var val T
	err = json.NewDecoder(strings.NewReader(result)).Decode(&val)
	if err != nil {
		return zero, fmt.Errorf("failed to unmarshall record: %w", err)
	}

	return val, nil
}

func (s *ValkeyStore[T]) LoadAll() ([]T, error) {
	keys, err := s.fetchKeys()
	if err != nil {
		return nil, fmt.Errorf("unable to retrieve stored records: %w", err)
	}

	if len(keys) == 0 {
		return []T{}, nil
	}

	// multi get all keys
	cmd := s.client.B().Mget().Key(keys...).Build()
	queryResults, err := s.client.Do(context.Background(), cmd).ToArray()

	if err != nil {
		return nil, fmt.Errorf("failed to load records: %w", err)
	}

	records := make([]T, 0, len(keys))
	for _, res := range queryResults {
		rawRecord, err := res.ToString()
		if err != nil {
			return nil, fmt.Errorf("could not convert record: %w", err)
		}

		var record T
		reader := strings.NewReader(rawRecord)
		if err := json.NewDecoder(reader).Decode(&record); err != nil {
			return nil, fmt.Errorf("could not unmarshall record: %w", err)
		}

		records = append(records, record)
	}

	return records, nil
}

func (s *ValkeyStore[T]) fetchKeys() ([]string, error) {
	// builds new scan command to fetch next batch of keys
	factory := func(cursor uint64, count int64) valkey.Completed {
		return s.client.B().Scan().Cursor(cursor).Match(s.baseKey + ":" + "*").Count(count).Build()
	}

	cmd := factory(uint64(0), int64(1_000))
	queryResult, err := s.client.Do(context.Background(), cmd).AsScanEntry()
	if err != nil {
		return nil, fmt.Errorf("failed to fetch storage keys: %w", err)
	}

	keys := make([]string, 0)
	keys = append(keys, queryResult.Elements...)
	cursor := queryResult.Cursor
	for cursor != 0 {
		cmd := factory(cursor, int64(1_000))
		queryResult, err := s.client.Do(context.Background(), cmd).AsScanEntry()
		if err != nil {
			return nil, fmt.Errorf("failed to fetch storage keys: %w", err)
		}

		cursor = queryResult.Cursor
		keys = append(keys, queryResult.Elements...)
	}

	return keys, nil
}

func (s *ValkeyStore[T]) Delete(key string) error {
	cmd := s.client.B().Del().Key(s.baseKey + ":" + key).Build()
	err := s.client.Do(context.Background(), cmd).Error()
	if err != nil {
		return fmt.Errorf("failed to delete record: %w", err)
	}
	return nil
}

func (s *ValkeyStore[T]) Close() error {
	s.client.Close()
	return nil
}
