package proxy

import (
	"github.com/hfi/llm-secret-interceptor/internal/interceptor"
	"github.com/hfi/llm-secret-interceptor/internal/protocol"
	"github.com/hfi/llm-secret-interceptor/internal/storage"
	"github.com/hfi/llm-secret-interceptor/pkg/placeholder"
)

// SecretService coordinates secret detection, replacement, and storage
type SecretService struct {
	manager   *interceptor.Manager
	store     storage.MappingStore
	generator *placeholder.Generator
	replacer  *interceptor.Replacer
	registry  *protocol.Registry
}

// NewSecretService creates a new secret service
func NewSecretService(
	manager *interceptor.Manager,
	store storage.MappingStore,
	generator *placeholder.Generator,
	registry *protocol.Registry,
) *SecretService {
	return &SecretService{
		manager:   manager,
		store:     store,
		generator: generator,
		replacer:  interceptor.NewReplacer(manager, generator),
		registry:  registry,
	}
}

// ProcessRequestResult contains the result of processing a request
type ProcessRequestResult struct {
	// ModifiedBody contains the request body with secrets replaced
	ModifiedBody []byte
	// SecretsFound is the number of secrets detected
	SecretsFound int
	// SecretsReplaced is the number of secrets replaced
	SecretsReplaced int
	// Error contains any error that occurred
	Error error
}

// ProcessRequest detects and replaces secrets in an LLM request
func (s *SecretService) ProcessRequest(body []byte, handler protocol.Handler) *ProcessRequestResult {
	result := &ProcessRequestResult{
		ModifiedBody: body,
	}

	// Parse the request
	msg, err := handler.ParseRequest(body)
	if err != nil {
		result.Error = err
		return result
	}

	// Process each message
	modified := false
	for i, message := range msg.Messages {
		// Detect and replace secrets
		replaceResult := s.replacer.Replace(message.Content)

		if len(replaceResult.Mappings) > 0 {
			modified = true
			result.SecretsFound += len(replaceResult.Detected)
			result.SecretsReplaced += len(replaceResult.Mappings)

			// Store mappings
			for ph, secret := range replaceResult.Mappings {
				// Check if we already have this secret stored
				if existingPh, found := s.store.LookupBySecret(secret); found {
					// Reuse existing placeholder
					replaceResult.Text = replaceWithPlaceholder(replaceResult.Text, ph, existingPh)
				} else {
					// Store new mapping
					if err := s.store.Store(ph, secret); err != nil {
						// Storage error - continue but log
						result.Error = err
					}
				}
			}

			// Update message content
			msg.Messages[i].Content = replaceResult.Text
		}
	}

	// Serialize back if modified
	if modified {
		newBody, err := handler.SerializeRequest(msg)
		if err != nil {
			result.Error = err
			return result
		}
		result.ModifiedBody = newBody
	}

	return result
}

// ProcessResponseResult contains the result of processing a response
type ProcessResponseResult struct {
	// ModifiedBody contains the response body with placeholders restored
	ModifiedBody []byte
	// PlaceholdersRestored is the number of placeholders restored
	PlaceholdersRestored int
	// PlaceholdersNotFound is the number of placeholders that couldn't be restored
	PlaceholdersNotFound int
	// Error contains any error that occurred
	Error error
}

// ProcessResponse restores placeholders to secrets in an LLM response
func (s *SecretService) ProcessResponse(body []byte, handler protocol.Handler) *ProcessResponseResult {
	result := &ProcessResponseResult{
		ModifiedBody: body,
	}

	// Parse the response
	msg, err := handler.ParseResponse(body)
	if err != nil {
		result.Error = err
		return result
	}

	// Process each message
	modified := false
	for i, message := range msg.Messages {
		// Restore placeholders
		restoreResult := s.replacer.Restore(message.Content, func(ph string) (string, bool) {
			return s.store.Lookup(ph)
		})

		if restoreResult.RestoredCount > 0 || restoreResult.NotFoundCount > 0 {
			modified = true
			result.PlaceholdersRestored += restoreResult.RestoredCount
			result.PlaceholdersNotFound += restoreResult.NotFoundCount

			// Update message content
			msg.Messages[i].Content = restoreResult.Text
		}
	}

	// Serialize back if modified
	if modified {
		newBody, err := handler.SerializeResponse(msg)
		if err != nil {
			result.Error = err
			return result
		}
		result.ModifiedBody = newBody
	}

	return result
}

// ProcessStreamChunk processes a single streaming chunk
func (s *SecretService) ProcessStreamChunk(data []byte, handler protocol.StreamingHandler) ([]byte, error) {
	chunk, err := handler.ParseStreamChunk(data)
	if err != nil {
		return data, err
	}

	// If done marker, pass through
	if chunk.IsDone {
		return data, nil
	}

	// Restore placeholders in delta content
	if chunk.Delta != "" {
		restoreResult := s.replacer.Restore(chunk.Delta, func(ph string) (string, bool) {
			return s.store.Lookup(ph)
		})

		if restoreResult.RestoredCount > 0 {
			chunk.Delta = restoreResult.Text
			return handler.SerializeStreamChunk(chunk)
		}
	}

	return data, nil
}

// replaceWithPlaceholder replaces one placeholder with another in text
func replaceWithPlaceholder(text, oldPh, newPh string) string {
	result := ""
	for i := 0; i < len(text); {
		if i+len(oldPh) <= len(text) && text[i:i+len(oldPh)] == oldPh {
			result += newPh
			i += len(oldPh)
		} else {
			result += string(text[i])
			i++
		}
	}
	return result
}

// GetStore returns the underlying store
func (s *SecretService) GetStore() storage.MappingStore {
	return s.store
}

// GetManager returns the underlying interceptor manager
func (s *SecretService) GetManager() *interceptor.Manager {
	return s.manager
}
