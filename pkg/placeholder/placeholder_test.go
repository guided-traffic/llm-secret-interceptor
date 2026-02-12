package placeholder

import (
	"testing"
)

func TestGenerator_Generate(t *testing.T) {
	g := NewGenerator("__SECRET_", "__")

	// __SECRET_ (9) + 8 hex chars + __ (2) = 19
	expectedLen := 19

	testCases := []struct {
		secret string
	}{
		{"mysecretpassword"},
		{"another-secret"},
		{"sk-a8Kd9fJ2mN4pQ7xR3yZ5"},
	}

	for _, tc := range testCases {
		t.Run(tc.secret, func(t *testing.T) {
			placeholder := g.Generate(tc.secret)
			if len(placeholder) != expectedLen {
				t.Errorf("Generate(%q) length = %d, want %d", tc.secret, len(placeholder), expectedLen)
			}
			if !g.IsPlaceholder(placeholder) {
				t.Errorf("IsPlaceholder(%q) = false, want true", placeholder)
			}
		})
	}
}

func TestGenerator_Deterministic(t *testing.T) {
	g := NewGenerator("__SECRET_", "__")

	secret := "mysecretpassword"
	placeholder1 := g.Generate(secret)
	placeholder2 := g.Generate(secret)

	if placeholder1 != placeholder2 {
		t.Errorf("Generate() not deterministic: %q != %q", placeholder1, placeholder2)
	}
}

func TestGenerator_UniqueForDifferentSecrets(t *testing.T) {
	g := NewGenerator("__SECRET_", "__")

	placeholder1 := g.Generate("secret1")
	placeholder2 := g.Generate("secret2")

	if placeholder1 == placeholder2 {
		t.Errorf("Different secrets produced same placeholder: %q", placeholder1)
	}
}

func TestGenerator_IsPlaceholder(t *testing.T) {
	g := NewGenerator("__SECRET_", "__")

	testCases := []struct {
		input string
		want  bool
	}{
		{"__SECRET_a1b2c3d4__", true},  // Valid hex hash
		{"__SECRET_12345678__", true},  // Valid hex hash
		{"__SECRET_abcdef12__", true},  // Valid hex hash
		{"not a placeholder", false},
		{"__SECRET_short__", false},    // Too short hash (5 chars)
		{"SECRET_a1b2c3d4", false},     // Missing prefix (__) and suffix
		{"__OTHER_a1b2c3d4__", false},  // Wrong prefix
		{"__SECRET_ghijklmn__", false}, // Invalid hex chars (g-n are not hex)
	}

	for _, tc := range testCases {
		t.Run(tc.input, func(t *testing.T) {
			got := g.IsPlaceholder(tc.input)
			if got != tc.want {
				t.Errorf("IsPlaceholder(%q) = %v, want %v", tc.input, got, tc.want)
			}
		})
	}
}

func TestGenerator_FindAll(t *testing.T) {
	g := NewGenerator("__SECRET_", "__")

	text := "The password is __SECRET_a1b2c3d4__ and the token is __SECRET_12345678__"
	placeholders := g.FindAll(text)

	if len(placeholders) != 2 {
		t.Errorf("FindAll() found %d placeholders, want 2", len(placeholders))
	}
}

func TestGenerator_FindAllIndex(t *testing.T) {
	g := NewGenerator("__SECRET_", "__")

	text := "prefix __SECRET_a1b2c3d4__ suffix"
	indices := g.FindAllIndex(text)

	if len(indices) != 1 {
		t.Fatalf("FindAllIndex() found %d matches, want 1", len(indices))
	}

	start, end := indices[0][0], indices[0][1]
	found := text[start:end]
	if found != "__SECRET_a1b2c3d4__" {
		t.Errorf("Found placeholder = %q, want __SECRET_a1b2c3d4__", found)
	}
}

func TestGenerator_ReplaceSecrets(t *testing.T) {
	g := NewGenerator("__SECRET_", "__")

	text := "password is secret123 and token is apikey456"
	secrets := []string{"secret123", "apikey456"}

	result, mappings := g.ReplaceSecrets(text, secrets)

	// Should have 2 mappings
	if len(mappings) != 2 {
		t.Errorf("ReplaceSecrets() created %d mappings, want 2", len(mappings))
	}

	// Original secrets should not be in result
	if containsAny(result, secrets) {
		t.Errorf("Result still contains original secrets: %q", result)
	}

	// Placeholders should be in result
	for placeholder := range mappings {
		if !contains(result, placeholder) {
			t.Errorf("Result doesn't contain placeholder %q", placeholder)
		}
	}
}

func TestGenerator_RestorePlaceholders(t *testing.T) {
	g := NewGenerator("__SECRET_", "__")

	// Setup
	secret := "mysecretpassword"
	placeholder := g.Generate(secret)
	text := "The password is " + placeholder + " please use it"

	// Restore
	lookup := func(ph string) (string, bool) {
		if ph == placeholder {
			return secret, true
		}
		return "", false
	}

	result := g.RestorePlaceholders(text, lookup)

	expected := "The password is " + secret + " please use it"
	if result != expected {
		t.Errorf("RestorePlaceholders() = %q, want %q", result, expected)
	}
}

func TestGenerator_RestorePlaceholders_NotFound(t *testing.T) {
	g := NewGenerator("__SECRET_", "__")

	text := "The password is __SECRET_unknown1__ keep it"

	// Lookup always returns not found
	lookup := func(ph string) (string, bool) {
		return "", false
	}

	result := g.RestorePlaceholders(text, lookup)

	// Placeholder should remain unchanged
	if result != text {
		t.Errorf("RestorePlaceholders() = %q, want %q", result, text)
	}
}

func TestGenerator_MaxLength(t *testing.T) {
	g := NewGenerator("__SECRET_", "__")

	maxLen := g.MaxLength()
	// __SECRET_ (9) + 8 hex chars + __ (2) = 19
	expected := 19

	if maxLen != expected {
		t.Errorf("MaxLength() = %d, want %d", maxLen, expected)
	}
}

// Helper functions

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func containsAny(s string, substrs []string) bool {
	for _, substr := range substrs {
		if contains(s, substr) {
			return true
		}
	}
	return false
}
