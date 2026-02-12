package interceptor

import (
	"testing"
)

func TestEntropyInterceptor_Name(t *testing.T) {
	e := NewEntropyInterceptor(4.5, 8, 128)
	if e.Name() != "entropy" {
		t.Errorf("Name() = %s, want 'entropy'", e.Name())
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
		input       string
		minEntropy  float64
		maxEntropy  float64
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
