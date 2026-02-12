package interceptor

import (
	"sort"
	"strings"

	"github.com/hfi/llm-secret-interceptor/pkg/placeholder"
)

// Replacer handles the replacement of secrets with placeholders
type Replacer struct {
	manager   *Manager
	generator *placeholder.Generator
}

// NewReplacer creates a new secret replacer
func NewReplacer(manager *Manager, generator *placeholder.Generator) *Replacer {
	return &Replacer{
		manager:   manager,
		generator: generator,
	}
}

// ReplaceResult contains the result of a replacement operation
type ReplaceResult struct {
	// Text is the modified text with secrets replaced
	Text string
	// Mappings maps placeholders to original secrets
	Mappings map[string]string
	// Detected contains all detected secrets (for logging/metrics)
	Detected []DetectedSecret
}

// Replace detects and replaces all secrets in the text
func (r *Replacer) Replace(text string) *ReplaceResult {
	result := &ReplaceResult{
		Text:     text,
		Mappings: make(map[string]string),
		Detected: nil,
	}

	// Detect all secrets
	secrets := r.manager.DetectAll(text)
	if len(secrets) == 0 {
		return result
	}

	result.Detected = secrets

	// Sort by start position (descending) to replace from end to start
	// This prevents index shifting issues
	sort.Slice(secrets, func(i, j int) bool {
		return secrets[i].StartIndex > secrets[j].StartIndex
	})

	// Replace each secret
	for _, secret := range secrets {
		placeholder := r.generator.Generate(secret.Value)
		result.Mappings[placeholder] = secret.Value

		// Replace in text (from end to start to maintain indices)
		result.Text = result.Text[:secret.StartIndex] + placeholder + result.Text[secret.EndIndex:]
	}

	return result
}

// ReplaceInMessages detects and replaces secrets in multiple message strings
func (r *Replacer) ReplaceInMessages(messages []string) *ReplaceResult {
	// Combine all messages for detection
	combined := strings.Join(messages, "\n---MESSAGE_SEPARATOR---\n")

	// Replace in combined text
	result := r.Replace(combined)

	// The text field will contain the combined result
	// Caller can split on separator if needed

	return result
}

// RestoreResult contains the result of a restoration operation
type RestoreResult struct {
	// Text is the modified text with placeholders restored to secrets
	Text string
	// RestoredCount is the number of placeholders that were restored
	RestoredCount int
	// NotFoundCount is the number of placeholders that couldn't be found
	NotFoundCount int
}

// Restore replaces placeholders back with original secrets
func (r *Replacer) Restore(text string, lookup func(placeholder string) (string, bool)) *RestoreResult {
	result := &RestoreResult{
		Text:          text,
		RestoredCount: 0,
		NotFoundCount: 0,
	}

	// Find all placeholders
	placeholders := r.generator.FindAll(text)
	if len(placeholders) == 0 {
		return result
	}

	// Find positions (from end to start)
	indices := r.generator.FindAllIndex(text)

	// Sort by position descending
	sort.Slice(indices, func(i, j int) bool {
		return indices[i][0] > indices[j][0]
	})

	// Restore each placeholder
	for _, idx := range indices {
		start, end := idx[0], idx[1]
		placeholder := text[start:end]

		if secret, found := lookup(placeholder); found {
			result.Text = result.Text[:start] + secret + result.Text[end:]
			result.RestoredCount++
		} else {
			result.NotFoundCount++
		}
	}

	return result
}

// RestoreWithMappings restores placeholders using a provided mapping
func (r *Replacer) RestoreWithMappings(text string, mappings map[string]string) *RestoreResult {
	return r.Restore(text, func(placeholder string) (string, bool) {
		secret, found := mappings[placeholder]
		return secret, found
	})
}
