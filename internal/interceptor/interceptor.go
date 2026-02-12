package interceptor

import (
	"sort"
)

// DetectedSecret represents a secret found by an interceptor
type DetectedSecret struct {
	// Value is the actual secret value
	Value string
	// StartIndex is the position where the secret starts in the text
	StartIndex int
	// EndIndex is the position where the secret ends in the text
	EndIndex int
	// Type describes the kind of secret (e.g., "password", "api_key", "token")
	Type string
	// Confidence indicates how confident the interceptor is (0.0 - 1.0)
	Confidence float64
	// Source is the name of the interceptor that found this secret
	Source string
}

// SecretInterceptor defines the interface for secret detection plugins
type SecretInterceptor interface {
	// Name returns the interceptor name for logging/metrics
	Name() string

	// Detect analyzes text and returns found secrets
	Detect(text string) []DetectedSecret

	// Configure applies configuration from the config file
	Configure(config map[string]interface{}) error

	// IsEnabled returns whether the interceptor is enabled
	IsEnabled() bool

	// SetEnabled enables or disables the interceptor
	SetEnabled(enabled bool)
}

// BaseInterceptor provides common functionality for interceptors
type BaseInterceptor struct {
	enabled bool
}

// IsEnabled returns whether the interceptor is enabled
func (b *BaseInterceptor) IsEnabled() bool {
	return b.enabled
}

// SetEnabled enables or disables the interceptor
func (b *BaseInterceptor) SetEnabled(enabled bool) {
	b.enabled = enabled
}

// Manager manages multiple secret interceptors
type Manager struct {
	interceptors []SecretInterceptor
}

// NewManager creates a new interceptor manager
func NewManager() *Manager {
	return &Manager{
		interceptors: make([]SecretInterceptor, 0),
	}
}

// Register adds an interceptor to the manager
func (m *Manager) Register(i SecretInterceptor) {
	m.interceptors = append(m.interceptors, i)
}

// Get returns an interceptor by name
func (m *Manager) Get(name string) SecretInterceptor {
	for _, i := range m.interceptors {
		if i.Name() == name {
			return i
		}
	}
	return nil
}

// List returns all registered interceptor names
func (m *Manager) List() []string {
	names := make([]string, len(m.interceptors))
	for i, interceptor := range m.interceptors {
		names[i] = interceptor.Name()
	}
	return names
}

// DetectAll runs all registered interceptors and aggregates results
func (m *Manager) DetectAll(text string) []DetectedSecret {
	var allSecrets []DetectedSecret

	for _, interceptor := range m.interceptors {
		// Skip disabled interceptors
		if !interceptor.IsEnabled() {
			continue
		}

		secrets := interceptor.Detect(text)
		for i := range secrets {
			secrets[i].Source = interceptor.Name()
		}
		allSecrets = append(allSecrets, secrets...)
	}

	// Deduplicate overlapping secrets
	allSecrets = m.deduplicateSecrets(allSecrets)

	// Sort by position
	sort.Slice(allSecrets, func(i, j int) bool {
		return allSecrets[i].StartIndex < allSecrets[j].StartIndex
	})

	return allSecrets
}

// deduplicateSecrets removes duplicate and overlapping secrets
// When secrets overlap, keep the one with higher confidence
func (m *Manager) deduplicateSecrets(secrets []DetectedSecret) []DetectedSecret {
	if len(secrets) <= 1 {
		return secrets
	}

	// Group by value first (exact duplicates)
	seen := make(map[string]DetectedSecret)
	for _, s := range secrets {
		if existing, ok := seen[s.Value]; ok {
			// Keep the one with higher confidence
			if s.Confidence > existing.Confidence {
				seen[s.Value] = s
			}
		} else {
			seen[s.Value] = s
		}
	}

	// Convert back to slice
	result := make([]DetectedSecret, 0, len(seen))
	for _, s := range seen {
		result = append(result, s)
	}

	// Sort by start position
	sort.Slice(result, func(i, j int) bool {
		return result[i].StartIndex < result[j].StartIndex
	})

	// Remove overlapping secrets (keep higher confidence)
	if len(result) <= 1 {
		return result
	}

	final := []DetectedSecret{result[0]}
	for i := 1; i < len(result); i++ {
		current := result[i]
		last := &final[len(final)-1]

		// Check for overlap
		if current.StartIndex < last.EndIndex {
			// Overlapping - keep the one with higher confidence
			if current.Confidence > last.Confidence {
				final[len(final)-1] = current
			}
		} else {
			final = append(final, current)
		}
	}

	return final
}

// ConfigureAll configures all interceptors from a config map
func (m *Manager) ConfigureAll(configs map[string]map[string]interface{}) error {
	for name, config := range configs {
		interceptor := m.Get(name)
		if interceptor != nil {
			if err := interceptor.Configure(config); err != nil {
				return err
			}
			// Check for enabled flag
			if enabled, ok := config["enabled"].(bool); ok {
				interceptor.SetEnabled(enabled)
			}
		}
	}
	return nil
}
