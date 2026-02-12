package proxy

import (
	"testing"
	"time"

	"github.com/hfi/llm-secret-interceptor/internal/interceptor"
	"github.com/hfi/llm-secret-interceptor/internal/protocol"
	"github.com/hfi/llm-secret-interceptor/internal/storage"
	"github.com/hfi/llm-secret-interceptor/pkg/placeholder"
)

func setupTestService() *SecretService {
	// Create components
	manager := interceptor.NewManager()
	manager.Register(interceptor.NewEntropyInterceptor(4.0, 8, 128))
	manager.Register(interceptor.NewPatternInterceptor())

	store := storage.NewMemoryStore(time.Hour)
	generator := placeholder.NewGenerator("__SECRET_", "__")
	registry := protocol.NewRegistry()
	registry.Register(protocol.NewOpenAIHandler())

	return NewSecretService(manager, store, generator, registry)
}

func TestSecretService_ProcessRequest_WithSecrets(t *testing.T) {
	service := setupTestService()
	defer service.GetStore().Close()

	handler := protocol.NewOpenAIHandler()

	// Request with a high-entropy secret
	body := []byte(`{
		"model": "gpt-4",
		"messages": [
			{"role": "user", "content": "My API key is aB3cD4eF5gH6iJ7kL8mN9oP0qR please help"}
		]
	}`)

	result := service.ProcessRequest(body, handler)

	if result.Error != nil {
		t.Fatalf("ProcessRequest error: %v", result.Error)
	}

	if result.SecretsFound == 0 {
		t.Error("Expected to find secrets")
	}

	// The secret should not be in the modified body
	if containsBytes(result.ModifiedBody, []byte("aB3cD4eF5gH6iJ7kL8mN9oP0qR")) {
		t.Error("Original secret still in modified body")
	}

	// Should contain a placeholder
	if !containsBytes(result.ModifiedBody, []byte("__SECRET_")) {
		t.Error("Placeholder not found in modified body")
	}
}

func TestSecretService_ProcessRequest_NoSecrets(t *testing.T) {
	service := setupTestService()
	defer service.GetStore().Close()

	handler := protocol.NewOpenAIHandler()

	// Request without secrets
	body := []byte(`{
		"model": "gpt-4",
		"messages": [
			{"role": "user", "content": "Hello, how are you?"}
		]
	}`)

	result := service.ProcessRequest(body, handler)

	if result.Error != nil {
		t.Fatalf("ProcessRequest error: %v", result.Error)
	}

	if result.SecretsFound != 0 {
		t.Errorf("Expected 0 secrets, found %d", result.SecretsFound)
	}
}

func TestSecretService_ProcessResponse_RestorePlaceholder(t *testing.T) {
	service := setupTestService()
	defer service.GetStore().Close()

	handler := protocol.NewOpenAIHandler()

	// First, process a request to create mappings
	requestBody := []byte(`{
		"model": "gpt-4",
		"messages": [
			{"role": "user", "content": "My key is aB3cD4eF5gH6iJ7kL8mN9oP0qR"}
		]
	}`)

	requestResult := service.ProcessRequest(requestBody, handler)
	if requestResult.Error != nil {
		t.Fatalf("ProcessRequest error: %v", requestResult.Error)
	}

	// Get the placeholder that was used
	ph, found := service.GetStore().LookupBySecret("aB3cD4eF5gH6iJ7kL8mN9oP0qR")
	if !found {
		t.Fatal("Secret not stored")
	}

	// Simulate a response that contains the placeholder
	responseBody := []byte(`{
		"id": "chatcmpl-123",
		"object": "chat.completion",
		"choices": [
			{
				"index": 0,
				"message": {
					"role": "assistant",
					"content": "I see you shared the key ` + ph + ` - please be careful"
				}
			}
		]
	}`)

	responseResult := service.ProcessResponse(responseBody, handler)

	if responseResult.Error != nil {
		t.Fatalf("ProcessResponse error: %v", responseResult.Error)
	}

	if responseResult.PlaceholdersRestored == 0 {
		t.Error("Expected placeholders to be restored")
	}

	// The placeholder should be replaced with the original secret
	if containsBytes(responseResult.ModifiedBody, []byte(ph)) {
		t.Error("Placeholder still in response")
	}

	if !containsBytes(responseResult.ModifiedBody, []byte("aB3cD4eF5gH6iJ7kL8mN9oP0qR")) {
		t.Error("Original secret not restored in response")
	}
}

func TestSecretService_RoundTrip(t *testing.T) {
	service := setupTestService()
	defer service.GetStore().Close()

	handler := protocol.NewOpenAIHandler()

	secret := "xY9zW8vU7tS6rQ5pO4nM3lK2"

	// Process request
	requestBody := []byte(`{
		"model": "gpt-4",
		"messages": [
			{"role": "system", "content": "You are a helpful assistant."},
			{"role": "user", "content": "The password is ` + secret + ` remember it"}
		]
	}`)

	requestResult := service.ProcessRequest(requestBody, handler)
	if requestResult.Error != nil {
		t.Fatalf("ProcessRequest error: %v", requestResult.Error)
	}

	// Verify secret is replaced
	if containsBytes(requestResult.ModifiedBody, []byte(secret)) {
		t.Error("Secret not replaced in request")
	}

	// Get placeholder
	ph, _ := service.GetStore().LookupBySecret(secret)

	// Simulate response mentioning the placeholder
	responseBody := []byte(`{
		"id": "chatcmpl-456",
		"object": "chat.completion",
		"choices": [
			{
				"index": 0,
				"message": {
					"role": "assistant",
					"content": "I have noted the password ` + ph + ` for you."
				}
			}
		]
	}`)

	responseResult := service.ProcessResponse(responseBody, handler)
	if responseResult.Error != nil {
		t.Fatalf("ProcessResponse error: %v", responseResult.Error)
	}

	// Verify placeholder is restored
	if containsBytes(responseResult.ModifiedBody, []byte(ph)) {
		t.Error("Placeholder not restored in response")
	}

	if !containsBytes(responseResult.ModifiedBody, []byte(secret)) {
		t.Error("Secret not restored in response")
	}
}

func TestSecretService_MultipleSecrets(t *testing.T) {
	service := setupTestService()
	defer service.GetStore().Close()

	handler := protocol.NewOpenAIHandler()

	secret1 := "aB3cD4eF5gH6iJ7kL8mN"
	secret2 := "xY9zW8vU7tS6rQ5pO4nM"

	requestBody := []byte(`{
		"model": "gpt-4",
		"messages": [
			{"role": "user", "content": "Key1: ` + secret1 + ` and Key2: ` + secret2 + `"}
		]
	}`)

	result := service.ProcessRequest(requestBody, handler)
	if result.Error != nil {
		t.Fatalf("ProcessRequest error: %v", result.Error)
	}

	// Both secrets should be replaced
	if containsBytes(result.ModifiedBody, []byte(secret1)) {
		t.Error("Secret1 not replaced")
	}
	if containsBytes(result.ModifiedBody, []byte(secret2)) {
		t.Error("Secret2 not replaced")
	}

	// Store should have both mappings
	if service.GetStore().Size() < 2 {
		t.Errorf("Expected at least 2 mappings, got %d", service.GetStore().Size())
	}
}

func TestSecretService_PatternDetection(t *testing.T) {
	service := setupTestService()
	defer service.GetStore().Close()

	handler := protocol.NewOpenAIHandler()

	// GitHub token pattern
	requestBody := []byte(`{
		"model": "gpt-4",
		"messages": [
			{"role": "user", "content": "Use this token: ghp_1234567890abcdefghijklmnopqrstuvwxyz"}
		]
	}`)

	result := service.ProcessRequest(requestBody, handler)
	if result.Error != nil {
		t.Fatalf("ProcessRequest error: %v", result.Error)
	}

	if result.SecretsFound == 0 {
		t.Error("Expected to find GitHub token pattern")
	}

	if containsBytes(result.ModifiedBody, []byte("ghp_1234567890")) {
		t.Error("GitHub token not replaced")
	}
}

// Helper function
func containsBytes(data, pattern []byte) bool {
	if len(pattern) > len(data) {
		return false
	}
	for i := 0; i <= len(data)-len(pattern); i++ {
		match := true
		for j := 0; j < len(pattern); j++ {
			if data[i+j] != pattern[j] {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}
