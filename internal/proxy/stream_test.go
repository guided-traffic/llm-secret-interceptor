package proxy

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/hfi/llm-secret-interceptor/internal/interceptor"
	"github.com/hfi/llm-secret-interceptor/internal/protocol"
	"github.com/hfi/llm-secret-interceptor/internal/storage"
	"github.com/hfi/llm-secret-interceptor/pkg/placeholder"
)

// mockStreamingHandler is a minimal mock for testing stream processing
type mockStreamingHandler struct{}

func (h *mockStreamingHandler) ParseStreamChunk(data []byte) (*protocol.StreamChunk, error) {
	if bytes.Equal(bytes.TrimSpace(data), []byte("[DONE]")) {
		return &protocol.StreamChunk{IsDone: true}, nil
	}

	var chunk struct {
		Choices []struct {
			Delta struct {
				Content string `json:"content"`
				Role    string `json:"role"`
			} `json:"delta"`
			FinishReason string `json:"finish_reason"`
		} `json:"choices"`
	}

	if err := json.Unmarshal(data, &chunk); err != nil {
		return nil, err
	}

	if len(chunk.Choices) == 0 {
		return &protocol.StreamChunk{}, nil
	}

	return &protocol.StreamChunk{
		Delta:        chunk.Choices[0].Delta.Content,
		Role:         chunk.Choices[0].Delta.Role,
		FinishReason: chunk.Choices[0].FinishReason,
	}, nil
}

func (h *mockStreamingHandler) SerializeStreamChunk(chunk *protocol.StreamChunk) ([]byte, error) {
	response := map[string]interface{}{
		"choices": []map[string]interface{}{
			{
				"delta": map[string]string{
					"content": chunk.Delta,
				},
			},
		},
	}
	return json.Marshal(response)
}

// Implement remaining Handler interface methods for StreamingHandler
func (h *mockStreamingHandler) Name() string                   { return "mock" }
func (h *mockStreamingHandler) CanHandle(_ *http.Request) bool { return true }
func (h *mockStreamingHandler) Priority() int                  { return 100 }
func (h *mockStreamingHandler) ParseRequest(_ []byte) (*protocol.StandardMessage, error) {
	return nil, nil
}
func (h *mockStreamingHandler) ParseResponse(_ []byte) (*protocol.StandardMessage, error) {
	return nil, nil
}
func (h *mockStreamingHandler) SerializeRequest(_ *protocol.StandardMessage) ([]byte, error) {
	return nil, nil
}
func (h *mockStreamingHandler) SerializeResponse(_ *protocol.StandardMessage) ([]byte, error) {
	return nil, nil
}
func (h *mockStreamingHandler) IsStreaming(_ []byte) bool { return true }

func TestStreamProcessor_ProcessChunk_NoPlaceholders(t *testing.T) {
	// Setup
	manager := interceptor.NewManager()
	store := storage.NewMemoryStore(time.Hour)
	generator := placeholder.NewGenerator("__SECRET_", "__")
	registry := protocol.NewRegistry()
	replacer := interceptor.NewReplacer(manager, generator)

	service := &SecretService{
		manager:   manager,
		store:     store,
		generator: generator,
		registry:  registry,
		replacer:  replacer,
	}

	var output bytes.Buffer
	handler := &mockStreamingHandler{}

	processor := NewStreamProcessor(service, handler, &output, 30)

	// Create a chunk without placeholders
	chunk := []byte(`{"choices":[{"delta":{"content":"Hello, world!"}}]}`)

	err := processor.ProcessChunk(chunk)
	if err != nil {
		t.Fatalf("ProcessChunk failed: %v", err)
	}

	// Flush remaining
	err = processor.Flush()
	if err != nil {
		t.Fatalf("Flush failed: %v", err)
	}

	// Check output contains the content
	if !strings.Contains(output.String(), "Hello") {
		t.Errorf("Expected output to contain 'Hello', got: %s", output.String())
	}
}

func TestStreamProcessor_ProcessChunk_WithPlaceholder(t *testing.T) {
	// Setup
	manager := interceptor.NewManager()
	store := storage.NewMemoryStore(time.Hour)
	generator := placeholder.NewGenerator("__SECRET_", "__")
	registry := protocol.NewRegistry()
	replacer := interceptor.NewReplacer(manager, generator)

	service := &SecretService{
		manager:   manager,
		store:     store,
		generator: generator,
		registry:  registry,
		replacer:  replacer,
	}

	// Pre-store a mapping
	originalSecret := "sk_test_abcdef123456"
	ph := generator.Generate(originalSecret)
	store.Store(ph, originalSecret)

	var output bytes.Buffer
	handler := &mockStreamingHandler{}

	processor := NewStreamProcessor(service, handler, &output, 30)

	// Create a chunk with the placeholder
	chunk := []byte(`{"choices":[{"delta":{"content":"Your API key is ` + ph + `"}}]}`)

	err := processor.ProcessChunk(chunk)
	if err != nil {
		t.Fatalf("ProcessChunk failed: %v", err)
	}

	// Send done marker
	doneChunk := []byte(`[DONE]`)
	err = processor.ProcessChunk(doneChunk)
	if err != nil {
		t.Fatalf("ProcessChunk for done failed: %v", err)
	}

	// Check that the placeholder was restored
	if !strings.Contains(output.String(), originalSecret) {
		t.Errorf("Expected output to contain original secret '%s', got: %s", originalSecret, output.String())
	}

	// Verify placeholder is not in output
	if strings.Contains(output.String(), ph) {
		t.Errorf("Output should not contain placeholder '%s', got: %s", ph, output.String())
	}
}

func TestStreamProcessor_ProcessChunk_SplitPlaceholder(t *testing.T) {
	// Setup
	manager := interceptor.NewManager()
	store := storage.NewMemoryStore(time.Hour)
	generator := placeholder.NewGenerator("__SECRET_", "__")
	registry := protocol.NewRegistry()
	replacer := interceptor.NewReplacer(manager, generator)

	service := &SecretService{
		manager:   manager,
		store:     store,
		generator: generator,
		registry:  registry,
		replacer:  replacer,
	}

	// Pre-store a mapping
	originalSecret := "secret123"
	ph := generator.Generate(originalSecret) // e.g., __SECRET_abc12345__

	store.Store(ph, originalSecret)

	var output bytes.Buffer
	handler := &mockStreamingHandler{}

	// Use buffer size that accommodates the placeholder length
	processor := NewStreamProcessor(service, handler, &output, len(ph)+5)

	// Split the placeholder across chunks
	// e.g., __SECRET_abc12345__ split as __SECRET_ and abc12345__
	part1 := ph[:10]
	part2 := ph[10:]

	chunk1 := []byte(`{"choices":[{"delta":{"content":"Key: ` + part1 + `"}}]}`)
	chunk2 := []byte(`{"choices":[{"delta":{"content":"` + part2 + ` done"}}]}`)

	err := processor.ProcessChunk(chunk1)
	if err != nil {
		t.Fatalf("ProcessChunk 1 failed: %v", err)
	}

	err = processor.ProcessChunk(chunk2)
	if err != nil {
		t.Fatalf("ProcessChunk 2 failed: %v", err)
	}

	// Flush
	err = processor.Flush()
	if err != nil {
		t.Fatalf("Flush failed: %v", err)
	}

	// The accumulated content should have both parts
	accumulated := processor.GetAccumulated()
	if !strings.Contains(accumulated, ph) {
		t.Logf("Parts: '%s' + '%s' = '%s'", part1, part2, part1+part2)
		t.Errorf("Accumulated should contain full placeholder '%s', got: %s", ph, accumulated)
	}
}

func TestStreamProcessor_DoneMarker(t *testing.T) {
	// Setup
	manager := interceptor.NewManager()
	store := storage.NewMemoryStore(time.Hour)
	generator := placeholder.NewGenerator("__SECRET_", "__")
	registry := protocol.NewRegistry()
	replacer := interceptor.NewReplacer(manager, generator)

	service := &SecretService{
		manager:   manager,
		store:     store,
		generator: generator,
		registry:  registry,
		replacer:  replacer,
	}

	var output bytes.Buffer
	handler := &mockStreamingHandler{}

	processor := NewStreamProcessor(service, handler, &output, 30)

	// Send some content
	chunk := []byte(`{"choices":[{"delta":{"content":"Hello"}}]}`)
	err := processor.ProcessChunk(chunk)
	if err != nil {
		t.Fatalf("ProcessChunk failed: %v", err)
	}

	// Send done marker
	doneChunk := []byte(`[DONE]`)
	err = processor.ProcessChunk(doneChunk)
	if err != nil {
		t.Fatalf("ProcessChunk for done failed: %v", err)
	}

	// Check that done marker was forwarded
	if !strings.Contains(output.String(), "[DONE]") {
		t.Errorf("Expected output to contain [DONE], got: %s", output.String())
	}
}

func TestStreamProcessor_EmptyChunks(t *testing.T) {
	// Setup
	manager := interceptor.NewManager()
	store := storage.NewMemoryStore(time.Hour)
	generator := placeholder.NewGenerator("__SECRET_", "__")
	registry := protocol.NewRegistry()
	replacer := interceptor.NewReplacer(manager, generator)

	service := &SecretService{
		manager:   manager,
		store:     store,
		generator: generator,
		registry:  registry,
		replacer:  replacer,
	}

	var output bytes.Buffer
	handler := &mockStreamingHandler{}

	processor := NewStreamProcessor(service, handler, &output, 30)

	// Send an empty choices chunk
	chunk := []byte(`{"choices":[{"delta":{"content":""}}]}`)
	err := processor.ProcessChunk(chunk)
	if err != nil {
		t.Fatalf("ProcessChunk failed: %v", err)
	}

	// Should not panic or fail
	err = processor.Flush()
	if err != nil {
		t.Fatalf("Flush failed: %v", err)
	}
}

func BenchmarkStreamProcessor_ProcessChunk(b *testing.B) {
	manager := interceptor.NewManager()
	store := storage.NewMemoryStore(time.Hour)
	generator := placeholder.NewGenerator("__SECRET_", "__")
	registry := protocol.NewRegistry()
	replacer := interceptor.NewReplacer(manager, generator)

	service := &SecretService{
		manager:   manager,
		store:     store,
		generator: generator,
		registry:  registry,
		replacer:  replacer,
	}

	handler := &mockStreamingHandler{}

	// Pre-store some mappings
	for i := 0; i < 10; i++ {
		secret := "secret" + string(rune('0'+i))
		ph := generator.Generate(secret)
		store.Store(ph, secret)
	}

	chunk := []byte(`{"choices":[{"delta":{"content":"Processing some data..."}}]}`)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var output bytes.Buffer
		processor := NewStreamProcessor(service, handler, &output, 30)
		_ = processor.ProcessChunk(chunk)
		_ = processor.Flush()
	}
}
