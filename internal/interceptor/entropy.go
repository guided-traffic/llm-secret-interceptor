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

// commonKeywords contains programming keywords and identifiers that are not secrets
var commonKeywords = map[string]bool{
	"function": true, "return": true, "import": true, "export": true,
	"const": true, "class": true, "interface": true, "package": true,
	"undefined": true, "null": true, "true": true, "false": true,
	"string": true, "number": true, "boolean": true, "object": true,
	"async": true, "await": true, "promise": true, "callback": true,
	"localhost": true, "githubusercontent": true, "example": true,
}

// uuidPattern matches UUID strings
var uuidPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

// isLikelyNotSecret checks if a string is likely not a secret
func (e *EntropyInterceptor) isLikelyNotSecret(s string) bool {
	lower := strings.ToLower(s)

	return e.isLowercaseWord(s) ||
		e.isCommonKeyword(lower) ||
		e.isPathOrURL(s, lower) ||
		e.isFileExtension(s, lower) ||
		e.isShortBase64(s) ||
		e.isUUID(lower)
}

// isLowercaseWord checks if string is all lowercase without digits (likely a word)
func (e *EntropyInterceptor) isLowercaseWord(s string) bool {
	for _, c := range s {
		if unicode.IsUpper(c) || unicode.IsDigit(c) {
			return false
		}
	}
	return true
}

// isCommonKeyword checks if string matches a common programming keyword
func (e *EntropyInterceptor) isCommonKeyword(lower string) bool {
	return commonKeywords[lower]
}

// isPathOrURL checks if string looks like a file path or URL
func (e *EntropyInterceptor) isPathOrURL(s, lower string) bool {
	return strings.HasPrefix(s, "/") ||
		strings.HasPrefix(s, "./") ||
		strings.HasPrefix(lower, "http") ||
		strings.HasPrefix(lower, "www")
}

// isFileExtension checks if string looks like a file with extension
func (e *EntropyInterceptor) isFileExtension(s, lower string) bool {
	if strings.HasPrefix(s, ".") {
		return true
	}
	fileExtensions := []string{".js", ".ts", ".go", ".py", ".json"}
	for _, ext := range fileExtensions {
		if strings.HasSuffix(lower, ext) {
			return true
		}
	}
	return false
}

// isShortBase64 checks if string is short base64 padding (unlikely secret)
func (e *EntropyInterceptor) isShortBase64(s string) bool {
	return strings.HasSuffix(s, "==") && len(s) < 20
}

// isUUID checks if string matches UUID pattern
func (e *EntropyInterceptor) isUUID(lower string) bool {
	return uuidPattern.MatchString(lower)
}
