package memory

import (
	"fmt"
	"sync"
)

type Store[T any] struct {
	lock sync.Mutex
	data map[string]T
}

func NewStore[T any]() *Store[T] {
	return &Store[T]{
		lock: sync.Mutex{},
		data: make(map[string]T),
	}
}

func (s *Store[T]) Save(key string, data T) error {
	s.lock.Lock()
	defer s.lock.Unlock()
	s.data[key] = data
	return nil
}

func (s *Store[T]) Load(key string) (T, error) {
	s.lock.Lock()
	defer s.lock.Unlock()
	val, exist := s.data[key]
	if !exist {
		var zero T
		return zero, fmt.Errorf("resource: %s, does not exist", key)
	}
	return val, nil
}

func (s *Store[T]) LoadAll() ([]T, error) {
	s.lock.Lock()
	defer s.lock.Unlock()

	result := make([]T, 0, len(s.data))
	for _, val := range s.data {
		result = append(result, val)
	}
	
	return result, nil
}

func (s *Store[T]) Delete(key string) error {
	s.lock.Lock()
	defer s.lock.Unlock()
	delete(s.data, key)
	return nil
}

func (s *Store[T]) Close() error {
	return nil
}
