package storage

import (
	"sync"
	"time"
)

// MemoryStore is an in-memory implementation of MappingStore
type MemoryStore struct {
	mu              sync.RWMutex
	mappings        map[string]*Mapping // keyed by placeholder
	secretIndex     map[string]string   // secret -> placeholder reverse lookup
	ttl             time.Duration
	cleanupInterval time.Duration
	stopCleanup     chan struct{}
}

// NewMemoryStore creates a new in-memory mapping store
func NewMemoryStore(ttl time.Duration) *MemoryStore {
	store := &MemoryStore{
		mappings:        make(map[string]*Mapping),
		secretIndex:     make(map[string]string),
		ttl:             ttl,
		cleanupInterval: time.Minute,
		stopCleanup:     make(chan struct{}),
	}

	// Start background cleanup goroutine
	go store.cleanupLoop()

	return store
}

// Store saves a new secret-placeholder mapping
func (m *MemoryStore) Store(placeholder, secret string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	m.mappings[placeholder] = &Mapping{
		Secret:      secret,
		Placeholder: placeholder,
		LastUsed:    now,
		CreatedAt:   now,
	}
	m.secretIndex[secret] = placeholder

	return nil
}

// Lookup retrieves a secret by its placeholder
func (m *MemoryStore) Lookup(placeholder string) (string, bool) {
	m.mu.RLock()
	mapping, ok := m.mappings[placeholder]
	m.mu.RUnlock()

	if !ok {
		return "", false
	}

	// Update last used time
	m.mu.Lock()
	mapping.LastUsed = time.Now()
	m.mu.Unlock()

	return mapping.Secret, true
}

// LookupBySecret retrieves a placeholder by the secret value
func (m *MemoryStore) LookupBySecret(secret string) (string, bool) {
	m.mu.RLock()
	placeholder, ok := m.secretIndex[secret]
	m.mu.RUnlock()

	if ok {
		// Touch to update last used
		m.Touch(placeholder)
	}

	return placeholder, ok
}

// Touch updates the LastUsed timestamp
func (m *MemoryStore) Touch(placeholder string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if mapping, ok := m.mappings[placeholder]; ok {
		mapping.LastUsed = time.Now()
	}

	return nil
}

// Cleanup removes expired mappings
func (m *MemoryStore) Cleanup() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	for placeholder, mapping := range m.mappings {
		if now.Sub(mapping.LastUsed) > m.ttl {
			delete(m.secretIndex, mapping.Secret)
			delete(m.mappings, placeholder)
		}
	}

	return nil
}

// Size returns the number of stored mappings
func (m *MemoryStore) Size() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.mappings)
}

// Close stops the cleanup goroutine and releases resources
func (m *MemoryStore) Close() error {
	close(m.stopCleanup)
	return nil
}

// cleanupLoop periodically cleans up expired mappings
func (m *MemoryStore) cleanupLoop() {
	ticker := time.NewTicker(m.cleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			m.Cleanup()
		case <-m.stopCleanup:
			return
		}
	}
}
