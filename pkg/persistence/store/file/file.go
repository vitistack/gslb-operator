package file

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
)

type Store[T any] struct {
	lock     sync.RWMutex
	cache    map[string]T
	fileName string
}

func NewStore[T any](fileName string) (*Store[T], error) {
	store, err := os.OpenFile(fileName, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to create storage file: %s", err.Error())
	}
	store.Close()

	return &Store[T]{
		lock:     sync.RWMutex{},
		fileName: fileName,
		cache:    make(map[string]T),
	}, nil
}

func (s *Store[T]) Save(key string, data T) error {
	s.lock.Lock()
	defer s.lock.Unlock()

	s.cache[key] = data

	saved, err := os.ReadFile(s.fileName)
	if err != nil {
		return fmt.Errorf("could not read file: %s", err.Error())
	}

	store := make(map[string]any)
	if len(saved) != 0 {
		err = json.Unmarshal(saved, &store)
		if err != nil {
			return fmt.Errorf("could not read existing data: %s", err.Error())
		}
	}
	store[key] = data

	saved, err = json.Marshal(store)
	if err != nil {
		return fmt.Errorf("unable to marshall data: %s", err.Error())
	}

	err = os.WriteFile(s.fileName, saved, 0644)
	//_, err = s.file.Write(saved)
	if err != nil {
		return fmt.Errorf("could not write to file: %s", err.Error())
	}

	return nil
}

func (s *Store[T]) Load(key string) (T, error) {
	s.lock.Lock()
	defer s.lock.Unlock()
	var zero T

	data, ok := s.cache[key]
	if ok {
		return data, nil
	}

	file, err := os.ReadFile(s.fileName)
	if err != nil {
		return zero, fmt.Errorf("unable to read storage: %w", err)
	}

	if len(file) == 0 {
		return zero, nil
	}

	err = json.Unmarshal(file, &s.cache)
	if err != nil {
		return zero, fmt.Errorf("unable to parse: %s: %s", key, err.Error())
	}

	return s.cache[key], nil
}

func (s *Store[T]) LoadAll() ([]T, error) {
	s.lock.Lock()
	defer s.lock.Unlock()
	all := []T{}

	saved, err := os.ReadFile(s.fileName)
	if err != nil {
		return nil, fmt.Errorf("unable to read from storage: %s", err.Error())
	}

	if len(saved) == 0 {
		return all, nil
	}

	store := make(map[string]T)
	err = json.Unmarshal(saved, &store)
	if err != nil {
		return nil, fmt.Errorf("unable to parse JSON: %s", err.Error())
	}

	for key, val := range store {
		s.cache[key] = val
		all = append(all, val)
	}

	return all, nil
}

func (s *Store[T]) Delete(key string) error {
	s.lock.Lock()
	defer s.lock.Unlock()

	delete(s.cache, key)

	saved, err := os.ReadFile(s.fileName)
	if err != nil {
		return fmt.Errorf("could not read file: %s", err.Error())
	}

	store := make(map[string]any)
	if len(saved) != 0 {
		err = json.Unmarshal(saved, &store)
		if err != nil {
			return fmt.Errorf("could not read existing data: %s", err.Error())
		}
	}
	delete(store, key)

	saved, err = json.Marshal(store)
	if err != nil {
		return fmt.Errorf("unable to marshall data: %s", err.Error())
	}

	err = os.WriteFile(s.fileName, saved, 0644)
	//_, err = s.file.Write(saved)
	if err != nil {
		return fmt.Errorf("could not write to file: %s", err.Error())
	}

	return nil
}

func (s *Store[T]) Close() error {
	return nil
}
