package file

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
)

type Store[T any] struct {
	lock sync.RWMutex
	file *os.File
}

func NewStore[T any](fileName string) (*Store[T], error) {
	store, err := os.OpenFile(fileName, os.O_RDWR|os.O_CREATE, 0644)
	if os.IsNotExist(err) {
		store, err = os.Create(fileName)
	}

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

	saved, err := os.ReadFile(s.file.Name())
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

	err = os.WriteFile(s.file.Name(), saved, 0644)
	//_, err = s.file.Write(saved)
	if err != nil {
		return fmt.Errorf("could not write to file: %s", err.Error())
	}

	return nil
}

func (s *Store[T]) Load(key string) (T, error) {
	var zero T
	s.lock.Lock()
	defer s.lock.Unlock()

	saved, err := os.ReadFile(s.file.Name())
	if err != nil {
		return zero, fmt.Errorf("unable to read from storage: %s", err.Error())
	}

	store := make(map[string]T)
	err = json.Unmarshal(saved, &store)
	if err != nil {
		return zero, fmt.Errorf("unable to read: %s: %s", key, err.Error())
	}

	return store[key], nil
}

func (s *Store[T]) LoadAll() ([]T, error) {
	all := []T{}

	saved, err := os.ReadFile(s.file.Name())
	if err != nil {
		return nil, fmt.Errorf("unable to read from storage: %s", err.Error())
	}

	store := make(map[string]T)
	err = json.Unmarshal(saved, &store)
	if err != nil {
		return nil, fmt.Errorf("unable to parse JSON: %s", err.Error())
	}

	for _, val := range store {
		all = append(all, val)
	}

	return all, nil
}

func (s *Store[T]) Delete(key string) error {
	s.lock.Lock()
	defer s.lock.Unlock()

	saved, err := os.ReadFile(s.file.Name())
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

	err = os.WriteFile(s.file.Name(), saved, 0644)
	//_, err = s.file.Write(saved)
	if err != nil {
		return fmt.Errorf("could not write to file: %s", err.Error())
	}

	return nil
}
