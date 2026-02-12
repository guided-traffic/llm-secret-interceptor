package protocol

import (
	"encoding/json"
	"net/http"
	"strings"
)

// OpenAIHandler handles OpenAI Chat Completions API format
// This format is also used by GitHub Copilot, Azure OpenAI, and many compatible services
type OpenAIHandler struct{}

// OpenAI API request structure
type openAIRequest struct {
	Model    string          `json:"model"`
	Messages []openAIMessage `json:"messages"`
	Stream   bool            `json:"stream,omitempty"`
	// Other fields omitted - we pass them through unchanged
}

type openAIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// OpenAI API response structure
type openAIResponse struct {
	ID      string         `json:"id"`
	Object  string         `json:"object"`
	Choices []openAIChoice `json:"choices"`
	// Other fields omitted - we pass them through unchanged
}

type openAIChoice struct {
	Index   int           `json:"index"`
	Message openAIMessage `json:"message"`
}

// NewOpenAIHandler creates a new OpenAI protocol handler
func NewOpenAIHandler() *OpenAIHandler {
	return &OpenAIHandler{}
}

// Name returns the handler name
func (h *OpenAIHandler) Name() string {
	return "openai"
}

// CanHandle checks if this handler can process the request
func (h *OpenAIHandler) CanHandle(req *http.Request) bool {
	// Check for common OpenAI-compatible endpoints
	path := req.URL.Path
	if strings.Contains(path, "/chat/completions") {
		return true
	}
	if strings.Contains(path, "/v1/messages") {
		return true
	}

	// Check Content-Type
	contentType := req.Header.Get("Content-Type")
	if !strings.Contains(contentType, "application/json") {
		return false
	}

	return false
}

// ParseRequest parses an OpenAI request into StandardMessage format
func (h *OpenAIHandler) ParseRequest(body []byte) (*StandardMessage, error) {
	var req openAIRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return nil, err
	}

	msg := &StandardMessage{
		Messages: make([]Message, len(req.Messages)),
		Metadata: map[string]interface{}{
			"model":  req.Model,
			"stream": req.Stream,
		},
	}

	for i, m := range req.Messages {
		msg.Messages[i] = Message{
			Role:    m.Role,
			Content: m.Content,
		}
	}

	return msg, nil
}

// ParseResponse parses an OpenAI response into StandardMessage format
func (h *OpenAIHandler) ParseResponse(body []byte) (*StandardMessage, error) {
	var resp openAIResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, err
	}

	msg := &StandardMessage{
		Messages: make([]Message, 0),
		Metadata: map[string]interface{}{
			"id":     resp.ID,
			"object": resp.Object,
		},
	}

	for _, choice := range resp.Choices {
		msg.Messages = append(msg.Messages, Message{
			Role:    choice.Message.Role,
			Content: choice.Message.Content,
		})
	}

	return msg, nil
}

// SerializeRequest converts StandardMessage back to OpenAI request format
func (h *OpenAIHandler) SerializeRequest(msg *StandardMessage) ([]byte, error) {
	req := openAIRequest{
		Messages: make([]openAIMessage, len(msg.Messages)),
	}

	// Restore metadata
	if model, ok := msg.Metadata["model"].(string); ok {
		req.Model = model
	}
	if stream, ok := msg.Metadata["stream"].(bool); ok {
		req.Stream = stream
	}

	for i, m := range msg.Messages {
		req.Messages[i] = openAIMessage{
			Role:    m.Role,
			Content: m.Content,
		}
	}

	return json.Marshal(req)
}

// SerializeResponse converts StandardMessage back to OpenAI response format
func (h *OpenAIHandler) SerializeResponse(msg *StandardMessage) ([]byte, error) {
	resp := openAIResponse{
		Choices: make([]openAIChoice, len(msg.Messages)),
	}

	// Restore metadata
	if id, ok := msg.Metadata["id"].(string); ok {
		resp.ID = id
	}
	if object, ok := msg.Metadata["object"].(string); ok {
		resp.Object = object
	}

	for i, m := range msg.Messages {
		resp.Choices[i] = openAIChoice{
			Index: i,
			Message: openAIMessage{
				Role:    m.Role,
				Content: m.Content,
			},
		}
	}

	return json.Marshal(resp)
}
