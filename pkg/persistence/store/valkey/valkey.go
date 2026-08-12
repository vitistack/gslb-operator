package valkey

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"iter"
	"strings"
	"time"

	"github.com/valkey-io/valkey-go"
	"github.com/vitistack/gslb-operator/pkg/persistence"
)

var sharedClient valkey.Client // shared valkey client

type ValkeyStore[T any] struct {
	client   valkey.Client
	baseKey  string
	cacheTTL time.Duration
	migrate  persistence.MigrateFunc
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

func NewStore[T any](c valkey.Client, base string, ttl time.Duration, opts ...Option[T]) (*ValkeyStore[T], error) {
	store := &ValkeyStore[T]{
		client:   c,
		baseKey:  base,
		cacheTTL: ttl,
	}

	for _, opt := range opts {
		opt(store)
	}

	if store.migrate != nil {
		if err := store.runMigration(); err != nil {
			return nil, fmt.Errorf("valkey store migration failed: %w", err)
		}
	}

	return store, nil
}

func (s *ValkeyStore[T]) runMigration() error {
	keys, err := s.fetchKeys()
	if err != nil {
		return fmt.Errorf("failed to fetch keys: %w", err)
	}

	const batchSize = 100
	migrationErrors := make([]error, len(keys))

	for i := 0; i < len(keys); i += batchSize {
		batch := keys[i:min(i+batchSize, len(keys))]

		cmd := s.client.B().Mget().Key(batch...).Build()
		results, err := s.client.Do(context.Background(), cmd).ToArray()
		if err != nil {
			migrationErrors = append(migrationErrors, fmt.Errorf("failed to load records: %w", err))
			continue
		}

		for _, result := range results {
			rawRecord, err := result.ToString()
			if err != nil {
				migrationErrors = append(migrationErrors, fmt.Errorf("could not convert record: %w", err))
				continue
			}

			var record map[string]any
			if err := json.NewDecoder(strings.NewReader(rawRecord)).Decode(&record); err != nil {
				migrationErrors = append(migrationErrors, fmt.Errorf("could not unmarshall record: %w", err))
				continue
			}

			newRecord, err := s.migrate(record)
			if err != nil {
				migrationErrors = append(migrationErrors, fmt.Errorf("failed to migrate: %v: %w", record, err))
				continue
			}

			newRecordRaw, err := json.Marshal(newRecord)
			if err != nil {
				migrationErrors = append(migrationErrors, fmt.Errorf("failed to marshall new record: %w", err))
				continue
			}

			key := batch[i%min(batchSize, len(keys))]
			cmd := s.client.B().Set().Key(key).Value(string(newRecordRaw)).Build()

			err = s.client.Do(context.Background(), cmd).Error()
			if err != nil {
				migrationErrors = append(migrationErrors, fmt.Errorf("failed to set key: %s: %w", key, err))
				continue
			}
		}
	}

	return errors.Join(migrationErrors...)
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
	cmd := s.client.B().Get().Key(key).Build()
	result, err := s.client.Do(context.Background(), cmd).ToString()

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

// pre-loads all keys, and iterates over a batch of records to preserve memory
func (s *ValkeyStore[T]) LoadAll() (iter.Seq[T], func() error) {
	var iterErr error
	seq := func(yield func(T) bool) {
		keys, err := s.fetchKeys()
		if err != nil {
			iterErr = fmt.Errorf("unable to retrieve stored records: %w", err)
			return
		}

		const batchSize = 100
		for i := 0; i < len(keys); i += batchSize {
			batch := keys[i:min(i+batchSize, len(keys))]

			cmd := s.client.B().Mget().Key(batch...).Build()
			results, err := s.client.Do(context.Background(), cmd).ToArray()
			if err != nil {
				iterErr = fmt.Errorf("failed to load records: %w", err)
				return
			}

			for _, result := range results {
				rawRecord, err := result.ToString()
				if err != nil {
					iterErr = fmt.Errorf("could not convert record: %w", err)
					return
				}

				var record T
				if err := json.NewDecoder(strings.NewReader(rawRecord)).Decode(&record); err != nil {
					iterErr = fmt.Errorf("could not unmarshall record: %w", err)
					return
				}

				if !yield(record) {
					return
				}
			}
		}
	}

	finish := func() error { // iteration callback function for errors
		return iterErr
	}

	return seq, finish
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
