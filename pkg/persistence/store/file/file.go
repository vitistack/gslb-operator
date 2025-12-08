package file

import (
	"fmt"
	"os"
	"sync"
)

type Store[T any] struct {
	lock sync.RWMutex
	file *os.File
}

func NewStore[T any](fileName string) (*Store[T], error) {
	store, err := os.Create(fileName)
	if err != nil {
		return nil, fmt.Errorf("failed to create storage file: %s", err.Error())
	}

	return &Store[T]{
		lock: sync.RWMutex{},
		file: store,
	}, nil
}

func (s *Store[T]) Save(key string, data T) error {
	s.lock.Lock()
	defer s.lock.Unlock()

	return nil
}

func (s *Store[T]) Load(key string) (T, error) {
	s.lock.Lock()
	defer s.lock.Unlock()
	var zero T
	return zero, nil
}


func (s *Store[T]) Delete(key string) error {
	s.lock.Lock()
	defer s.lock.Unlock()

	return nil
}
