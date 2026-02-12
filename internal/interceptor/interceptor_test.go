package interceptor

import (
	"testing"

	"github.com/hfi/llm-secret-interceptor/pkg/placeholder"
)

func TestEntropyInterceptor_Name(t *testing.T) {
	e := NewEntropyInterceptor(4.5, 8, 128)
	if e.Name() != "entropy" {
		t.Errorf("Name() = %s, want 'entropy'", e.Name())
	}
}

func TestEntropyInterceptor_IsEnabled(t *testing.T) {
	e := NewEntropyInterceptor(4.5, 8, 128)
	if !e.IsEnabled() {
		t.Error("IsEnabled() = false, want true (default)")
	}

	e.SetEnabled(false)
	if e.IsEnabled() {
		t.Error("IsEnabled() = true after SetEnabled(false)")
	}
}

func TestEntropyInterceptor_Detect(t *testing.T) {
	e := NewEntropyInterceptor(4.0, 8, 128)

	testCases := []struct {
		name     string
		input    string
		wantLen  int
		wantType string
	}{
		{
			name:     "high entropy API key",
			input:    "my api key is sk-a8Kd9fJ2mN4pQ7xR3yZ5",
			wantLen:  1,
			wantType: "high_entropy",
		},
		{
			name:    "no secrets",
			input:   "this is a normal message without secrets",
			wantLen: 0,
		},
		{
			name:     "base64 encoded secret",
			input:    "token: dXNlcjpwYXNzd29yZDEyMw==",
			wantLen:  1,
			wantType: "high_entropy",
		},
		{
			name:    "short string",
			input:   "abc",
			wantLen: 0,
		},
		{
			name:     "multiple secrets",
			input:    "key1: aB3cD4eF5gH6iJ7k key2: xY9zW8vU7tS6rQ5p",
			wantLen:  2,
			wantType: "high_entropy",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			secrets := e.Detect(tc.input)
			if len(secrets) != tc.wantLen {
				t.Errorf("Detect() found %d secrets, want %d", len(secrets), tc.wantLen)
				for _, s := range secrets {
					t.Logf("  Found: %q (entropy confidence: %.2f)", s.Value, s.Confidence)
				}
			}
			if tc.wantLen > 0 && len(secrets) > 0 {
				if secrets[0].Type != tc.wantType {
					t.Errorf("Secret type = %s, want %s", secrets[0].Type, tc.wantType)
				}
			}
		})
	}
}

func TestEntropyInterceptor_CalculateEntropy(t *testing.T) {
	e := NewEntropyInterceptor(4.5, 8, 128)

	testCases := []struct {
		input      string
		minEntropy float64
		maxEntropy float64
	}{
		{
			input:      "aaaaaaaaaa",
			minEntropy: 0.0,
			maxEntropy: 0.1,
		},
		{
			input:      "abcdefghij",
			minEntropy: 3.0,
			maxEntropy: 3.5,
		},
		{
			input:      "aB3cD4eF5gH6iJ7kL8mN",
			minEntropy: 4.0,
			maxEntropy: 5.0,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.input, func(t *testing.T) {
			entropy := e.calculateEntropy(tc.input)
			if entropy < tc.minEntropy || entropy > tc.maxEntropy {
				t.Errorf("calculateEntropy(%q) = %.2f, want between %.2f and %.2f",
					tc.input, entropy, tc.minEntropy, tc.maxEntropy)
			}
		})
	}
}

func TestEntropyInterceptor_Configure(t *testing.T) {
	e := NewEntropyInterceptor(4.5, 8, 128)

	config := map[string]interface{}{
		"threshold":  5.0,
		"min_length": 10,
		"max_length": 64,
	}

	err := e.Configure(config)
	if err != nil {
		t.Fatalf("Configure() error: %v", err)
	}

	if e.threshold != 5.0 {
		t.Errorf("threshold = %.2f, want 5.0", e.threshold)
	}
	if e.minLength != 10 {
		t.Errorf("minLength = %d, want 10", e.minLength)
	}
	if e.maxLength != 64 {
		t.Errorf("maxLength = %d, want 64", e.maxLength)
	}
}

func TestManager_DetectAll(t *testing.T) {
	manager := NewManager()
	manager.Register(NewEntropyInterceptor(4.0, 8, 128))

	text := "my password is sk-a8Kd9fJ2mN4pQ7xR3yZ5"
	secrets := manager.DetectAll(text)

	if len(secrets) == 0 {
		t.Error("DetectAll() found no secrets")
	}

	// Check that source is set
	for _, s := range secrets {
		if s.Source == "" {
			t.Error("Secret source not set")
		}
	}
}

func TestManager_Deduplication(t *testing.T) {
	manager := NewManager()

	// Register two interceptors that might detect the same thing
	manager.Register(NewEntropyInterceptor(3.5, 8, 128))
	manager.Register(NewPatternInterceptor())

	// This contains a GitHub token pattern
	text := "token: ghp_1234567890abcdefghijklmnopqrstuvwxyz"
	secrets := manager.DetectAll(text)

	// Should deduplicate overlapping detections
	// Check that we don't have duplicates with same value
	seen := make(map[string]bool)
	for _, s := range secrets {
		if seen[s.Value] {
			t.Errorf("Duplicate secret detected: %q", s.Value)
		}
		seen[s.Value] = true
	}
}

func TestManager_DisabledInterceptor(t *testing.T) {
	manager := NewManager()
	entropy := NewEntropyInterceptor(4.0, 8, 128)
	entropy.SetEnabled(false)
	manager.Register(entropy)

	text := "my password is sk-a8Kd9fJ2mN4pQ7xR3yZ5"
	secrets := manager.DetectAll(text)

	if len(secrets) != 0 {
		t.Errorf("DetectAll() should return 0 secrets when interceptor is disabled, got %d", len(secrets))
	}
}

func TestManager_GetAndList(t *testing.T) {
	manager := NewManager()
	manager.Register(NewEntropyInterceptor(4.0, 8, 128))
	manager.Register(NewPatternInterceptor())

	// Test Get
	entropy := manager.Get("entropy")
	if entropy == nil {
		t.Error("Get('entropy') returned nil")
	}

	pattern := manager.Get("pattern")
	if pattern == nil {
		t.Error("Get('pattern') returned nil")
	}

	notFound := manager.Get("nonexistent")
	if notFound != nil {
		t.Error("Get('nonexistent') should return nil")
	}

	// Test List
	names := manager.List()
	if len(names) != 2 {
		t.Errorf("List() returned %d names, want 2", len(names))
	}
}

func TestPatternInterceptor_Detect(t *testing.T) {
	p := NewPatternInterceptor()

	testCases := []struct {
		name        string
		input       string
		minDetected int
		wantType    string
	}{
		{
			name:        "github token",
			input:       "ghp_1234567890abcdefghijklmnopqrstuvwxyz",
			minDetected: 1,
			wantType:    "token",
		},
		{
			name:        "aws access key",
			input:       "AKIAIOSFODNN7EXAMPLE",
			minDetected: 1,
			wantType:    "api_key",
		},
		{
			name:        "no secrets",
			input:       "just a normal text without any secrets",
			minDetected: 0,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			secrets := p.Detect(tc.input)
			if len(secrets) < tc.minDetected {
				t.Errorf("Detect() found %d secrets, want at least %d", len(secrets), tc.minDetected)
				for _, s := range secrets {
					t.Logf("  Found: %q (type: %s)", s.Value, s.Type)
				}
			}
			if tc.minDetected > 0 && len(secrets) > 0 {
				// Check that at least one has the expected type
				found := false
				for _, s := range secrets {
					if s.Type == tc.wantType {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("No secret with type %s found", tc.wantType)
				}
			}
		})
	}
}

func TestPatternInterceptor_AddRule(t *testing.T) {
	p := NewPatternInterceptor()
	initialCount := p.RuleCount()

	err := p.AddRule("custom", `CUSTOM_[A-Z0-9]{10}`, "custom_key", 0.9)
	if err != nil {
		t.Fatalf("AddRule() error: %v", err)
	}

	if p.RuleCount() != initialCount+1 {
		t.Errorf("RuleCount() = %d, want %d", p.RuleCount(), initialCount+1)
	}

	// Test detection with custom rule
	secrets := p.Detect("key: CUSTOM_ABCD123456")
	if len(secrets) == 0 {
		t.Error("Custom rule did not detect secret")
	}
}

func TestReplacer_Replace(t *testing.T) {
	manager := NewManager()
	manager.Register(NewEntropyInterceptor(4.0, 8, 128))

	gen := placeholder.NewGenerator("__SECRET_", "__")
	replacer := NewReplacer(manager, gen)

	text := "my api key is aB3cD4eF5gH6iJ7kL8mN please use it"
	result := replacer.Replace(text)

	// Should have detected and replaced the secret
	if len(result.Mappings) == 0 {
		t.Error("No mappings created")
	}

	// Original secret should not be in result text
	if containsString(result.Text, "aB3cD4eF5gH6iJ7kL8mN") {
		t.Error("Original secret still in result text")
	}

	// Should contain placeholder
	hasPlaceholder := false
	for ph := range result.Mappings {
		if containsString(result.Text, ph) {
			hasPlaceholder = true
			break
		}
	}
	if !hasPlaceholder {
		t.Error("Placeholder not found in result text")
	}
}

func TestReplacer_Restore(t *testing.T) {
	manager := NewManager()
	manager.Register(NewEntropyInterceptor(4.0, 8, 128))

	gen := placeholder.NewGenerator("__SECRET_", "__")
	replacer := NewReplacer(manager, gen)

	// First replace
	original := "password is aB3cD4eF5gH6iJ7kL8mN"
	replaceResult := replacer.Replace(original)

	// Then restore
	restoreResult := replacer.RestoreWithMappings(replaceResult.Text, replaceResult.Mappings)

	if restoreResult.Text != original {
		t.Errorf("Restored text = %q, want %q", restoreResult.Text, original)
	}

	if restoreResult.RestoredCount != len(replaceResult.Mappings) {
		t.Errorf("RestoredCount = %d, want %d", restoreResult.RestoredCount, len(replaceResult.Mappings))
	}
}

func containsString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
