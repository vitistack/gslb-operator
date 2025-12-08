package persistence

// operate on repository data
type Repository[T any] interface {
	Create(new *T) error            // create a new entity of type T
	Update(id string, new *T) error //update an existing entity, with a new entity of type T
	Delete(id string) error         // delete an entity
	Read(id string) (T, error)     // retrieve an entity
	ReadAll() ([]T, error)			// retrieves all entities
}

type Store[T any] interface {
	Save(key string, data T) error
	Load(key string) (T, error)
	LoadAll() ([]T, error)
	Delete(key string) error
}
