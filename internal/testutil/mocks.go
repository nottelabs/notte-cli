// internal/testutil/mocks.go
package testutil

import (
	"errors"
	"sync"
)

// ErrKeyNotFound is returned when a key doesn't exist
var ErrKeyNotFound = errors.New("key not found")

// MockKeyring provides an in-memory keyring for testing
type MockKeyring struct {
	mu    sync.RWMutex
	store map[string]string
}

// NewMockKeyring creates a new mock keyring
func NewMockKeyring() *MockKeyring {
	return &MockKeyring{
		store: make(map[string]string),
	}
}

// Get retrieves a value from the mock keyring
func (m *MockKeyring) Get(key string) (string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	val, ok := m.store[key]
	if !ok {
		return "", ErrKeyNotFound
	}
	return val, nil
}

// Set stores a value in the mock keyring
func (m *MockKeyring) Set(key, value string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.store[key] = value
	return nil
}

// Delete removes a value from the mock keyring
func (m *MockKeyring) Delete(key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.store, key)
	return nil
}

// Reset clears all stored values
func (m *MockKeyring) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.store = make(map[string]string)
}

// Keys returns all stored keys
func (m *MockKeyring) Keys() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	keys := make([]string, 0, len(m.store))
	for k := range m.store {
		keys = append(keys, k)
	}
	return keys
}
