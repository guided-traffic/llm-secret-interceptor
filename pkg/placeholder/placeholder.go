package placeholder

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"
)

// Generator handles placeholder generation and recognition
type Generator struct {
	prefix    string
	suffix    string
	hashLen   int
	maxLength int
	pattern   *regexp.Regexp
}

// NewGenerator creates a new placeholder generator
func NewGenerator(prefix, suffix string) *Generator {
	hashLen := 8 // Use first 8 characters of hash
	maxLength := len(prefix) + hashLen + len(suffix)

	// Build regex pattern for matching placeholders
	escapedPrefix := regexp.QuoteMeta(prefix)
	escapedSuffix := regexp.QuoteMeta(suffix)
	pattern := regexp.MustCompile(escapedPrefix + `[a-f0-9]{` + fmt.Sprintf("%d", hashLen) + `}` + escapedSuffix)

	return &Generator{
		prefix:    prefix,
		suffix:    suffix,
		hashLen:   hashLen,
		maxLength: maxLength,
		pattern:   pattern,
	}
}

// Generate creates a placeholder for a given secret
func (g *Generator) Generate(secret string) string {
	hash := sha256.Sum256([]byte(secret))
	hashStr := hex.EncodeToString(hash[:])[:g.hashLen]
	return g.prefix + hashStr + g.suffix
}

// MaxLength returns the maximum length of a placeholder
func (g *Generator) MaxLength() int {
	return g.maxLength
}

// IsPlaceholder checks if a string is a valid placeholder
func (g *Generator) IsPlaceholder(s string) bool {
	return g.pattern.MatchString(s)
}

// FindAll finds all placeholders in a text
func (g *Generator) FindAll(text string) []string {
	return g.pattern.FindAllString(text, -1)
}

// FindAllIndex finds all placeholders and their positions
func (g *Generator) FindAllIndex(text string) [][]int {
	return g.pattern.FindAllStringIndex(text, -1)
}

// ReplaceSecrets replaces all secrets in text with their placeholders
// Returns the modified text and a map of placeholder -> secret
func (g *Generator) ReplaceSecrets(text string, secrets []string) (string, map[string]string) {
	mappings := make(map[string]string)
	result := text

	for _, secret := range secrets {
		placeholder := g.Generate(secret)
		mappings[placeholder] = secret
		result = strings.ReplaceAll(result, secret, placeholder)
	}

	return result, mappings
}

// RestorePlaceholders replaces all placeholders with their original secrets
func (g *Generator) RestorePlaceholders(text string, lookup func(placeholder string) (string, bool)) string {
	return g.pattern.ReplaceAllStringFunc(text, func(placeholder string) string {
		if secret, ok := lookup(placeholder); ok {
			return secret
		}
		return placeholder // Keep placeholder if not found
	})
}
