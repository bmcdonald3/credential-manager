package secrets

import (
	"fmt"
	"sync"
)

// LocalSecretStore is an in-memory fallback implementation for local development.
type LocalSecretStore struct {
	mu   sync.RWMutex
	data map[string]string
}

func NewLocalSecretStore() *LocalSecretStore {
	return &LocalSecretStore{
		data: map[string]string{
			"node-123": "old-pass", // Add your actual current BMC password here
		},
	}
}

func (s *LocalSecretStore) Read(id string) (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	value, ok := s.data[id]
	if !ok {
		return "", fmt.Errorf("secret %q not found", id)
	}

	return value, nil
}

func (s *LocalSecretStore) Write(id string, data string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.data[id]; exists {
		return fmt.Errorf("secret %q already exists", id)
	}

	s.data[id] = data
	return nil
}

func (s *LocalSecretStore) Update(id string, data string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.data[id]; !exists {
		return fmt.Errorf("secret %q not found", id)
	}

	s.data[id] = data
	return nil
}

func (s *LocalSecretStore) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.data[id]; !exists {
		return fmt.Errorf("secret %q not found", id)
	}

	delete(s.data, id)
	return nil
}
