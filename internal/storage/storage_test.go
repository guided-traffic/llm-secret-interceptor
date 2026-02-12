package storage

import (
	"testing"
	"time"
)

// MockStore is a mock implementation of MappingStore for testing
type MockStore struct {
	mappings    map[string]string
	secrets     map[string]string
	storeErr    error
	lookupErr   error
	cleanupErr  error
	storeCalls  int
	lookupCalls int
}

// NewMockStore creates a new mock store
func NewMockStore() *MockStore {
	return &MockStore{
		mappings: make(map[string]string),
		secrets:  make(map[string]string),
	}
}

func (m *MockStore) Store(placeholder, secret string) error {
	m.storeCalls++
	if m.storeErr != nil {
		return m.storeErr
	}
	m.mappings[placeholder] = secret
	m.secrets[secret] = placeholder
	return nil
}

func (m *MockStore) Lookup(placeholder string) (string, bool) {
	m.lookupCalls++
	if m.lookupErr != nil {
		return "", false
	}
	secret, ok := m.mappings[placeholder]
	return secret, ok
}

func (m *MockStore) LookupBySecret(secret string) (string, bool) {
	m.lookupCalls++
	placeholder, ok := m.secrets[secret]
	return placeholder, ok
}

func (m *MockStore) Touch(placeholder string) error {
	return nil
}

func (m *MockStore) Cleanup() error {
	return m.cleanupErr
}

func (m *MockStore) Size() int {
	return len(m.mappings)
}

func (m *MockStore) Close() error {
	return nil
}

// SetStoreError sets an error to be returned by Store
func (m *MockStore) SetStoreError(err error) {
	m.storeErr = err
}

// SetLookupError sets an error to be returned by Lookup
func (m *MockStore) SetLookupError(err error) {
	m.lookupErr = err
}

// TestMockStore_Interface ensures MockStore implements MappingStore
func TestMockStore_Interface(t *testing.T) {
	var _ MappingStore = (*MockStore)(nil)
}

// TestMockStore_StoreAndLookup tests basic functionality
func TestMockStore_StoreAndLookup(t *testing.T) {
	store := NewMockStore()

	err := store.Store("__SECRET_12345678__", "mysecret")
	if err != nil {
		t.Fatalf("Store() error: %v", err)
	}

	secret, found := store.Lookup("__SECRET_12345678__")
	if !found {
		t.Error("Lookup() not found")
	}
	if secret != "mysecret" {
		t.Errorf("Lookup() = %q, want 'mysecret'", secret)
	}

	placeholder, found := store.LookupBySecret("mysecret")
	if !found {
		t.Error("LookupBySecret() not found")
	}
	if placeholder != "__SECRET_12345678__" {
		t.Errorf("LookupBySecret() = %q", placeholder)
	}
}

// TestRedisStore_Interface ensures RedisStore implements MappingStore
func TestRedisStore_Interface(t *testing.T) {
	var _ MappingStore = (*RedisStore)(nil)
}

// TestMemoryStore_Interface ensures MemoryStore implements MappingStore
func TestMemoryStore_Interface(t *testing.T) {
	var _ MappingStore = (*MemoryStore)(nil)
}

// TestMemoryStore_AutoCleanup tests automatic cleanup
func TestMemoryStore_AutoCleanup(t *testing.T) {
	// Create store with very short TTL
	store := &MemoryStore{
		mappings:        make(map[string]*Mapping),
		secretIndex:     make(map[string]string),
		ttl:             50 * time.Millisecond,
		cleanupInterval: 20 * time.Millisecond,
		stopCleanup:     make(chan struct{}),
	}

	// Start cleanup loop
	go store.cleanupLoop()
	defer store.Close()

	// Store a value
	store.Store("__SECRET_1__", "secret1")

	// Verify it's stored
	if store.Size() != 1 {
		t.Fatalf("Size() = %d, want 1", store.Size())
	}

	// Wait for TTL + cleanup interval
	time.Sleep(100 * time.Millisecond)

	// Should be cleaned up
	if store.Size() != 0 {
		t.Errorf("Size() = %d after cleanup, want 0", store.Size())
	}
}

// TestMemoryStore_TouchPreventCleanup tests that Touch prevents cleanup
func TestMemoryStore_TouchPreventCleanup(t *testing.T) {
	store := &MemoryStore{
		mappings:        make(map[string]*Mapping),
		secretIndex:     make(map[string]string),
		ttl:             80 * time.Millisecond,
		cleanupInterval: 30 * time.Millisecond,
		stopCleanup:     make(chan struct{}),
	}

	go store.cleanupLoop()
	defer store.Close()

	store.Store("__SECRET_1__", "secret1")

	// Touch every 40ms to keep it alive
	for i := 0; i < 3; i++ {
		time.Sleep(40 * time.Millisecond)
		store.Touch("__SECRET_1__")
	}

	// Should still exist
	_, found := store.Lookup("__SECRET_1__")
	if !found {
		t.Error("Secret should still exist after being touched")
	}
}

// Benchmark for store operations
func BenchmarkMemoryStore_Store(b *testing.B) {
	store := NewMemoryStore(time.Hour)
	defer store.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		store.Store("__SECRET_test__", "testsecret")
	}
}

func BenchmarkMemoryStore_Lookup(b *testing.B) {
	store := NewMemoryStore(time.Hour)
	defer store.Close()
	store.Store("__SECRET_test__", "testsecret")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		store.Lookup("__SECRET_test__")
	}
}

func BenchmarkMemoryStore_Concurrent(b *testing.B) {
	store := NewMemoryStore(time.Hour)
	defer store.Close()

	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			placeholder := "__SECRET_" + string(rune('a'+i%26)) + "__"
			store.Store(placeholder, "secret"+string(rune('a'+i%26)))
			store.Lookup(placeholder)
			i++
		}
	})
}
