package storage

import (
	"testing"
	"time"
)

func TestMemoryStore_StoreAndLookup(t *testing.T) {
	store := NewMemoryStore(time.Hour)
	defer store.Close()

	placeholder := "__SECRET_12345678__"
	secret := "mysecretpassword"

	// Store
	err := store.Store(placeholder, secret)
	if err != nil {
		t.Fatalf("Store() error: %v", err)
	}

	// Lookup by placeholder
	got, found := store.Lookup(placeholder)
	if !found {
		t.Error("Lookup() returned not found")
	}
	if got != secret {
		t.Errorf("Lookup() = %q, want %q", got, secret)
	}
}

func TestMemoryStore_LookupBySecret(t *testing.T) {
	store := NewMemoryStore(time.Hour)
	defer store.Close()

	placeholder := "__SECRET_12345678__"
	secret := "mysecretpassword"

	// Store
	err := store.Store(placeholder, secret)
	if err != nil {
		t.Fatalf("Store() error: %v", err)
	}

	// Lookup by secret
	got, found := store.LookupBySecret(secret)
	if !found {
		t.Error("LookupBySecret() returned not found")
	}
	if got != placeholder {
		t.Errorf("LookupBySecret() = %q, want %q", got, placeholder)
	}
}

func TestMemoryStore_LookupNotFound(t *testing.T) {
	store := NewMemoryStore(time.Hour)
	defer store.Close()

	_, found := store.Lookup("nonexistent")
	if found {
		t.Error("Lookup() should return not found for nonexistent key")
	}
}

func TestMemoryStore_Size(t *testing.T) {
	store := NewMemoryStore(time.Hour)
	defer store.Close()

	if store.Size() != 0 {
		t.Errorf("Size() = %d, want 0", store.Size())
	}

	store.Store("__SECRET_1__", "secret1")
	store.Store("__SECRET_2__", "secret2")
	store.Store("__SECRET_3__", "secret3")

	if store.Size() != 3 {
		t.Errorf("Size() = %d, want 3", store.Size())
	}
}

func TestMemoryStore_Cleanup(t *testing.T) {
	// Use very short TTL for testing
	store := NewMemoryStore(50 * time.Millisecond)
	defer store.Close()

	store.Store("__SECRET_1__", "secret1")

	// Verify it's stored
	_, found := store.Lookup("__SECRET_1__")
	if !found {
		t.Fatal("Secret should be found immediately after storing")
	}

	// Wait for TTL to expire
	time.Sleep(100 * time.Millisecond)

	// Run cleanup
	store.Cleanup()

	// Should be gone
	_, found = store.Lookup("__SECRET_1__")
	if found {
		t.Error("Secret should be cleaned up after TTL")
	}
}

func TestMemoryStore_Touch(t *testing.T) {
	store := NewMemoryStore(100 * time.Millisecond)
	defer store.Close()

	placeholder := "__SECRET_1__"
	store.Store(placeholder, "secret1")

	// Wait half the TTL
	time.Sleep(60 * time.Millisecond)

	// Touch to refresh
	store.Touch(placeholder)

	// Wait another half TTL (would have expired without touch)
	time.Sleep(60 * time.Millisecond)

	// Should still be there because we touched it
	store.Cleanup()
	_, found := store.Lookup(placeholder)
	if !found {
		t.Error("Secret should still exist after touch")
	}
}

func TestMemoryStore_Concurrency(t *testing.T) {
	store := NewMemoryStore(time.Hour)
	defer store.Close()

	// Run concurrent operations
	done := make(chan bool)

	for i := 0; i < 100; i++ {
		go func(id int) {
			placeholder := "__SECRET_" + string(rune('0'+id%10)) + "__"
			secret := "secret" + string(rune('0'+id%10))

			store.Store(placeholder, secret)
			store.Lookup(placeholder)
			store.LookupBySecret(secret)
			store.Touch(placeholder)
			store.Size()

			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 100; i++ {
		<-done
	}
}
