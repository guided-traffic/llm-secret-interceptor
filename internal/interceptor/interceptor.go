package interceptor

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

// DetectAll runs all registered interceptors and aggregates results
func (m *Manager) DetectAll(text string) []DetectedSecret {
	var allSecrets []DetectedSecret

	for _, interceptor := range m.interceptors {
		secrets := interceptor.Detect(text)
		for i := range secrets {
			secrets[i].Source = interceptor.Name()
		}
		allSecrets = append(allSecrets, secrets...)
	}

	// TODO: Deduplicate overlapping secrets
	// TODO: Sort by position

	return allSecrets
}
