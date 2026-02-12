package protocol

import (
	"bytes"
	"strings"
	"testing"
)

func TestSSEParser_ReadEvent(t *testing.T) {
	input := `event: message
data: {"id":"123","content":"Hello"}

data: {"id":"456","content":"World"}

data: [DONE]

`
	parser := NewSSEParser(strings.NewReader(input))

	// First event
	eventType, data, err := parser.ReadEvent()
	if err != nil {
		t.Fatalf("ReadEvent() error: %v", err)
	}
	if eventType != "message" {
		t.Errorf("eventType = %q, want 'message'", eventType)
	}
	if string(data) != `{"id":"123","content":"Hello"}` {
		t.Errorf("data = %q", data)
	}

	// Second event (no event type)
	eventType, data, err = parser.ReadEvent()
	if err != nil {
		t.Fatalf("ReadEvent() error: %v", err)
	}
	if eventType != "" {
		t.Errorf("eventType = %q, want ''", eventType)
	}
	if string(data) != `{"id":"456","content":"World"}` {
		t.Errorf("data = %q", data)
	}

	// Third event ([DONE])
	_, data, err = parser.ReadEvent()
	if err != nil {
		t.Fatalf("ReadEvent() error: %v", err)
	}
	if string(data) != "[DONE]" {
		t.Errorf("data = %q, want '[DONE]'", data)
	}
}

func TestSSEWriter_WriteEvent(t *testing.T) {
	var buf bytes.Buffer
	writer := NewSSEWriter(&buf)

	err := writer.WriteEvent("message", []byte(`{"content":"Hello"}`))
	if err != nil {
		t.Fatalf("WriteEvent() error: %v", err)
	}

	expected := "event: message\ndata: {\"content\":\"Hello\"}\n\n"
	if buf.String() != expected {
		t.Errorf("output = %q, want %q", buf.String(), expected)
	}
}

func TestSSEWriter_MultiLineData(t *testing.T) {
	var buf bytes.Buffer
	writer := NewSSEWriter(&buf)

	err := writer.WriteEvent("", []byte("line1\nline2\nline3"))
	if err != nil {
		t.Fatalf("WriteEvent() error: %v", err)
	}

	expected := "data: line1\ndata: line2\ndata: line3\n\n"
	if buf.String() != expected {
		t.Errorf("output = %q, want %q", buf.String(), expected)
	}
}

func TestOpenAIHandler_IsStreaming(t *testing.T) {
	h := NewOpenAIHandler()

	testCases := []struct {
		name string
		body string
		want bool
	}{
		{
			name: "streaming enabled",
			body: `{"model":"gpt-4","messages":[],"stream":true}`,
			want: true,
		},
		{
			name: "streaming disabled",
			body: `{"model":"gpt-4","messages":[],"stream":false}`,
			want: false,
		},
		{
			name: "no stream field",
			body: `{"model":"gpt-4","messages":[]}`,
			want: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := h.IsStreaming([]byte(tc.body))
			if got != tc.want {
				t.Errorf("IsStreaming() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestOpenAIHandler_ParseStreamChunk(t *testing.T) {
	h := NewOpenAIHandler()

	testCases := []struct {
		name       string
		data       string
		wantDelta  string
		wantDone   bool
		wantFinish string
	}{
		{
			name:      "content chunk",
			data:      `{"id":"chatcmpl-123","object":"chat.completion.chunk","choices":[{"index":0,"delta":{"content":"Hello"}}]}`,
			wantDelta: "Hello",
			wantDone:  false,
		},
		{
			name:     "done marker",
			data:     "[DONE]",
			wantDone: true,
		},
		{
			name:       "finish reason",
			data:       `{"id":"chatcmpl-123","object":"chat.completion.chunk","choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}`,
			wantFinish: "stop",
			wantDone:   false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			chunk, err := h.ParseStreamChunk([]byte(tc.data))
			if err != nil {
				t.Fatalf("ParseStreamChunk() error: %v", err)
			}

			if chunk.IsDone != tc.wantDone {
				t.Errorf("IsDone = %v, want %v", chunk.IsDone, tc.wantDone)
			}
			if chunk.Delta != tc.wantDelta {
				t.Errorf("Delta = %q, want %q", chunk.Delta, tc.wantDelta)
			}
			if chunk.FinishReason != tc.wantFinish {
				t.Errorf("FinishReason = %q, want %q", chunk.FinishReason, tc.wantFinish)
			}
		})
	}
}

func TestStreamBuffer(t *testing.T) {
	buf := NewStreamBuffer(10)

	// Write some data
	buf.Write([]byte("Hello"))
	if buf.Len() != 5 {
		t.Errorf("Len() = %d, want 5", buf.Len())
	}

	// Flush returns nothing (less than maxLength)
	safe := buf.Flush()
	if safe != nil {
		t.Errorf("Flush() = %q, want nil", safe)
	}

	// Write more data
	buf.Write([]byte(" World! This is a test."))

	// Now flush should return some data
	safe = buf.Flush()
	if safe == nil {
		t.Error("Flush() returned nil")
	}

	// Buffer should now have exactly maxLength bytes
	if buf.Len() != 10 {
		t.Errorf("Len() = %d, want 10", buf.Len())
	}

	// FlushAll returns everything
	all := buf.FlushAll()
	if len(all) != 10 {
		t.Errorf("FlushAll() returned %d bytes, want 10", len(all))
	}
	if buf.Len() != 0 {
		t.Errorf("Len() after FlushAll = %d, want 0", buf.Len())
	}
}

func TestStandardMessage_GetUserMessages(t *testing.T) {
	msg := &StandardMessage{
		Messages: []Message{
			{Role: "system", Content: "You are a helpful assistant."},
			{Role: "user", Content: "Hello!"},
			{Role: "assistant", Content: "Hi there!"},
			{Role: "user", Content: "How are you?"},
		},
	}

	userMsgs := msg.GetUserMessages()
	if len(userMsgs) != 2 {
		t.Errorf("GetUserMessages() returned %d messages, want 2", len(userMsgs))
	}
	if userMsgs[0].Content != "Hello!" {
		t.Errorf("First user message = %q, want 'Hello!'", userMsgs[0].Content)
	}
}

func TestStandardMessage_GetAllContent(t *testing.T) {
	msg := &StandardMessage{
		Messages: []Message{
			{Role: "user", Content: "Hello"},
			{Role: "assistant", Content: "World"},
		},
	}

	content := msg.GetAllContent()
	if !strings.Contains(content, "Hello") || !strings.Contains(content, "World") {
		t.Errorf("GetAllContent() = %q", content)
	}
}
