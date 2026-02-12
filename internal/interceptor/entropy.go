// Package interceptor provides secret detection mechanisms including entropy-based and pattern-based detection.
package interceptor

import (
	"math"
	"regexp"
	"strings"
	"unicode"
)

// EntropyInterceptor detects high-entropy strings that might be secrets
type EntropyInterceptor struct {
	BaseInterceptor
	threshold float64
	minLength int
	maxLength int
}

// NewEntropyInterceptor creates a new entropy-based interceptor
func NewEntropyInterceptor(threshold float64, minLength, maxLength int) *EntropyInterceptor {
	return &EntropyInterceptor{
		BaseInterceptor: BaseInterceptor{enabled: true},
		threshold:       threshold,
		minLength:       minLength,
		maxLength:       maxLength,
	}
}

// Name returns the interceptor name
func (e *EntropyInterceptor) Name() string {
	return "entropy"
}

// Configure applies configuration from config file
func (e *EntropyInterceptor) Configure(config map[string]interface{}) error {
	if threshold, ok := config["threshold"].(float64); ok {
		e.threshold = threshold
	}
	if minLength, ok := config["min_length"].(int); ok {
		e.minLength = minLength
	}
	if maxLength, ok := config["max_length"].(int); ok {
		e.maxLength = maxLength
	}
	return nil
}

// Detect analyzes text for high-entropy strings
func (e *EntropyInterceptor) Detect(text string) []DetectedSecret {
	var secrets []DetectedSecret

	// Find potential secret-like strings (alphanumeric with some special chars)
	// This regex finds strings that look like tokens, API keys, passwords, etc.
	pattern := regexp.MustCompile(`[A-Za-z0-9+/=_\-]{8,}`)
	matches := pattern.FindAllStringIndex(text, -1)

	for _, match := range matches {
		start, end := match[0], match[1]
		candidate := text[start:end]

		// Skip if too short or too long
		if len(candidate) < e.minLength || len(candidate) > e.maxLength {
			continue
		}

		// Skip if it looks like a common word or path
		if e.isLikelyNotSecret(candidate) {
			continue
		}

		// Calculate Shannon entropy
		entropy := e.calculateEntropy(candidate)

		if entropy >= e.threshold {
			secrets = append(secrets, DetectedSecret{
				Value:      candidate,
				StartIndex: start,
				EndIndex:   end,
				Type:       "high_entropy",
				Confidence: e.entropyToConfidence(entropy),
			})
		}
	}

	return secrets
}

// calculateEntropy calculates Shannon entropy of a string
func (e *EntropyInterceptor) calculateEntropy(s string) float64 {
	if len(s) == 0 {
		return 0
	}

	// Count character frequencies
	freq := make(map[rune]int)
	for _, c := range s {
		freq[c]++
	}

	// Calculate entropy
	length := float64(len(s))
	entropy := 0.0
	for _, count := range freq {
		p := float64(count) / length
		entropy -= p * math.Log2(p)
	}

	return entropy
}

// entropyToConfidence converts entropy to a confidence score
func (e *EntropyInterceptor) entropyToConfidence(entropy float64) float64 {
	// Higher entropy = higher confidence
	// Entropy of 4.5+ is very likely a secret
	// Entropy of 6+ is almost certainly a secret
	if entropy >= 6.0 {
		return 1.0
	}
	if entropy >= e.threshold {
		// Linear scale from threshold to 6.0
		return 0.5 + 0.5*(entropy-e.threshold)/(6.0-e.threshold)
	}
	return 0.0
}

// isLikelyNotSecret checks if a string is likely not a secret
func (e *EntropyInterceptor) isLikelyNotSecret(s string) bool {
	lower := strings.ToLower(s)

	// All lowercase and looks like a word/path
	allLower := true
	hasDigit := false
	for _, c := range s {
		if unicode.IsUpper(c) {
			allLower = false
		}
		if unicode.IsDigit(c) {
			hasDigit = true
		}
	}
	if allLower && !hasDigit {
		return true
	}

	// Common programming keywords and identifiers
	commonPatterns := []string{
		"function", "return", "import", "export",
		"const", "class", "interface", "package",
		"undefined", "null", "true", "false",
		"string", "number", "boolean", "object",
		"async", "await", "promise", "callback",
		"localhost", "githubusercontent", "example",
	}
	for _, p := range commonPatterns {
		if lower == p {
			return true
		}
	}

	// File paths and URLs
	if strings.HasPrefix(s, "/") || strings.HasPrefix(s, "./") ||
		strings.HasPrefix(lower, "http") || strings.HasPrefix(lower, "www") {
		return true
	}

	// Looks like a file extension or path component
	if strings.HasPrefix(s, ".") || strings.HasSuffix(lower, ".js") ||
		strings.HasSuffix(lower, ".ts") || strings.HasSuffix(lower, ".go") ||
		strings.HasSuffix(lower, ".py") || strings.HasSuffix(lower, ".json") {
		return true
	}

	// Base64 padding without enough entropy
	if strings.HasSuffix(s, "==") && len(s) < 20 {
		return true
	}

	// UUIDs are often not secrets (but can be)
	uuidPattern := regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
	return uuidPattern.MatchString(lower)
}
