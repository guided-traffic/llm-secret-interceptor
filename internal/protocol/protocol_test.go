package protocol

import (
	"net/http"
	"testing"
)

func TestOpenAIHandler_Name(t *testing.T) {
	h := NewOpenAIHandler()
	if h.Name() != "openai" {
		t.Errorf("Name() = %q, want 'openai'", h.Name())
	}
}

func TestOpenAIHandler_Priority(t *testing.T) {
	h := NewOpenAIHandler()
	if h.Priority() != 100 {
		t.Errorf("Priority() = %d, want 100", h.Priority())
	}
}

func TestOpenAIHandler_CanHandle(t *testing.T) {
	h := NewOpenAIHandler()

	testCases := []struct {
		name   string
		path   string
		host   string
		method string
		want   bool
	}{
		{
			name:   "chat completions",
			path:   "/v1/chat/completions",
			method: "POST",
			want:   true,
		},
		{
			name:   "messages endpoint",
			path:   "/v1/messages",
			method: "POST",
			want:   true,
		},
		{
			name:   "azure openai",
			path:   "/openai/deployments/gpt-4/chat/completions",
			method: "POST",
			want:   true,
		},
		{
			name:   "github copilot host",
			path:   "/v1/completions",
			host:   "api.githubcopilot.com",
			method: "POST",
			want:   true,
		},
		{
			name:   "other endpoint",
			path:   "/v1/embeddings",
			method: "POST",
			want:   false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			host := tc.host
			if host == "" {
				host = "api.openai.com"
			}
			req, _ := http.NewRequest(tc.method, "https://"+host+tc.path, nil)
			req.Header.Set("Content-Type", "application/json")

			got := h.CanHandle(req)
			if got != tc.want {
				t.Errorf("CanHandle() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestOpenAIHandler_ParseRequest(t *testing.T) {
	h := NewOpenAIHandler()

	body := []byte(`{
		"model": "gpt-4",
		"messages": [
			{"role": "system", "content": "You are a helpful assistant."},
			{"role": "user", "content": "Hello, world!"}
		],
		"stream": true
	}`)

	msg, err := h.ParseRequest(body)
	if err != nil {
		t.Fatalf("ParseRequest() error: %v", err)
	}

	if len(msg.Messages) != 2 {
		t.Errorf("len(Messages) = %d, want 2", len(msg.Messages))
	}

	if msg.Messages[0].Role != "system" {
		t.Errorf("Messages[0].Role = %q, want 'system'", msg.Messages[0].Role)
	}

	if msg.Messages[1].Content != "Hello, world!" {
		t.Errorf("Messages[1].Content = %q, want 'Hello, world!'", msg.Messages[1].Content)
	}

	// Check metadata
	if model, ok := msg.Metadata["model"].(string); !ok || model != "gpt-4" {
		t.Errorf("Metadata['model'] = %v, want 'gpt-4'", msg.Metadata["model"])
	}

	if stream, ok := msg.Metadata["stream"].(bool); !ok || !stream {
		t.Errorf("Metadata['stream'] = %v, want true", msg.Metadata["stream"])
	}
}

func TestOpenAIHandler_ParseResponse(t *testing.T) {
	h := NewOpenAIHandler()

	body := []byte(`{
		"id": "chatcmpl-123",
		"object": "chat.completion",
		"choices": [
			{
				"index": 0,
				"message": {
					"role": "assistant",
					"content": "Hello! How can I help you?"
				}
			}
		]
	}`)

	msg, err := h.ParseResponse(body)
	if err != nil {
		t.Fatalf("ParseResponse() error: %v", err)
	}

	if len(msg.Messages) != 1 {
		t.Errorf("len(Messages) = %d, want 1", len(msg.Messages))
	}

	if msg.Messages[0].Role != "assistant" {
		t.Errorf("Messages[0].Role = %q, want 'assistant'", msg.Messages[0].Role)
	}

	if msg.Messages[0].Content != "Hello! How can I help you?" {
		t.Errorf("Messages[0].Content = %q", msg.Messages[0].Content)
	}
}

func TestOpenAIHandler_SerializeRequest(t *testing.T) {
	h := NewOpenAIHandler()

	msg := &StandardMessage{
		Messages: []Message{
			{Role: "user", Content: "Hello!"},
		},
		Metadata: map[string]interface{}{
			"model":  "gpt-4",
			"stream": false,
		},
	}

	body, err := h.SerializeRequest(msg)
	if err != nil {
		t.Fatalf("SerializeRequest() error: %v", err)
	}

	// Parse back to verify
	parsed, err := h.ParseRequest(body)
	if err != nil {
		t.Fatalf("ParseRequest() error on serialized data: %v", err)
	}

	if len(parsed.Messages) != 1 {
		t.Errorf("Round-trip: len(Messages) = %d, want 1", len(parsed.Messages))
	}

	if parsed.Messages[0].Content != "Hello!" {
		t.Errorf("Round-trip: Content = %q, want 'Hello!'", parsed.Messages[0].Content)
	}
}

func TestRegistry_Detect(t *testing.T) {
	registry := NewRegistry()
	registry.Register(NewOpenAIHandler())

	// Should match OpenAI
	req, _ := http.NewRequest("POST", "https://api.openai.com/v1/chat/completions", nil)
	req.Header.Set("Content-Type", "application/json")

	handler := registry.Detect(req)
	if handler == nil {
		t.Fatal("Detect() returned nil for OpenAI request")
	}
	if handler.Name() != "openai" {
		t.Errorf("Detect() returned handler %q, want 'openai'", handler.Name())
	}

	// Should not match other endpoints
	req2, _ := http.NewRequest("GET", "https://example.com/api", nil)
	handler2 := registry.Detect(req2)
	if handler2 != nil {
		t.Error("Detect() should return nil for non-LLM request")
	}
}
