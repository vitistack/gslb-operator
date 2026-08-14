package persistence

import "iter"

// operate on repository data
type Repository[T any] interface {
	Create(id string, new *T) error // create a new entity of type T
	Update(id string, new *T) error //update an existing entity, with a new entity of type T
	Delete(id string) error         // delete an entity
	Read(id string) (T, error)      // retrieve an entity
	ReadAll() ([]T, error)          // retrieves all entities
}

type Store[T any] interface {
	Save(key string, data T) error
	Load(key string) (T, error)
	LoadAll() (iter.Seq[T], func() error)
	Delete(key string) error
	Close() error
}

type MigrateFunc func(map[string]any) (map[string]any, error)

func Chain(fns ...MigrateFunc) MigrateFunc {
	return func(raw map[string]any) (map[string]any, error) {
		var err error
		for _, fn := range fns {
			raw, err = fn(raw)
			if err != nil {
				return nil, err
			}
		}
		return raw, nil
	}
}
