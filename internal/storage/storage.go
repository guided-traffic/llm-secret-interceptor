package storage

import "time"

// Mapping represents a secret-to-placeholder mapping with metadata
type Mapping struct {
	Secret      string //#nosec G117 -- Secret field is intentional - this is a secret interceptor
	Placeholder string
	LastUsed    time.Time
	CreatedAt   time.Time
}

// MappingStore defines the interface for storing secret mappings
type MappingStore interface {
	// Store saves a new secret-placeholder mapping
	Store(placeholder, secret string) error

	// Lookup retrieves a secret by its placeholder
	Lookup(placeholder string) (string, bool)

	// LookupBySecret retrieves a placeholder by the secret value
	LookupBySecret(secret string) (string, bool)

	// Touch updates the LastUsed timestamp for a mapping
	Touch(placeholder string) error

	// Cleanup removes expired mappings
	Cleanup() error

	// Size returns the number of stored mappings
	Size() int

	// Close releases any resources
	Close() error
}
