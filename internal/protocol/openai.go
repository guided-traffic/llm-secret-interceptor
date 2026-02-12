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
// We use json.RawMessage for unknown fields to preserve them
type openAIRequest struct {
	Model            string           `json:"model"`
	Messages         []openAIMessage  `json:"messages"`
	Stream           bool             `json:"stream,omitempty"`
	Temperature      *float64         `json:"temperature,omitempty"`
	TopP             *float64         `json:"top_p,omitempty"`
	N                *int             `json:"n,omitempty"`
	MaxTokens        *int             `json:"max_tokens,omitempty"`
	PresencePenalty  *float64         `json:"presence_penalty,omitempty"`
	FrequencyPenalty *float64         `json:"frequency_penalty,omitempty"`
	User             string           `json:"user,omitempty"`
	Stop             json.RawMessage  `json:"stop,omitempty"`
	Tools            json.RawMessage  `json:"tools,omitempty"`
	ToolChoice       json.RawMessage  `json:"tool_choice,omitempty"`
	ResponseFormat   json.RawMessage  `json:"response_format,omitempty"`
	Seed             *int             `json:"seed,omitempty"`
	Logprobs         *bool            `json:"logprobs,omitempty"`
	TopLogprobs      *int             `json:"top_logprobs,omitempty"`
}

type openAIMessage struct {
	Role         string          `json:"role"`
	Content      json.RawMessage `json:"content"` // Can be string or array of content parts
	Name         string          `json:"name,omitempty"`
	ToolCalls    json.RawMessage `json:"tool_calls,omitempty"`
	ToolCallID   string          `json:"tool_call_id,omitempty"`
	Refusal      string          `json:"refusal,omitempty"`
}

// getContentString extracts string content from the message
func (m *openAIMessage) getContentString() string {
	if m.Content == nil {
		return ""
	}
	// Try to unmarshal as string first
	var s string
	if err := json.Unmarshal(m.Content, &s); err == nil {
		return s
	}
	// If it's an array, extract text content
	var parts []map[string]interface{}
	if err := json.Unmarshal(m.Content, &parts); err == nil {
		var texts []string
		for _, part := range parts {
			if t, ok := part["type"].(string); ok && t == "text" {
				if text, ok := part["text"].(string); ok {
					texts = append(texts, text)
				}
			}
		}
		return strings.Join(texts, "\n")
	}
	return string(m.Content)
}

// setContentString sets the content as a string
func (m *openAIMessage) setContentString(s string) {
	data, _ := json.Marshal(s)
	m.Content = data
}

// OpenAI API response structure
type openAIResponse struct {
	ID                string           `json:"id"`
	Object            string           `json:"object"`
	Created           int64            `json:"created"`
	Model             string           `json:"model"`
	Choices           []openAIChoice   `json:"choices"`
	Usage             *openAIUsage     `json:"usage,omitempty"`
	SystemFingerprint string           `json:"system_fingerprint,omitempty"`
}

type openAIUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type openAIChoice struct {
	Index        int           `json:"index"`
	Message      openAIMessage `json:"message"`
	FinishReason string        `json:"finish_reason,omitempty"`
	Logprobs     interface{}   `json:"logprobs,omitempty"`
}

// NewOpenAIHandler creates a new OpenAI protocol handler
func NewOpenAIHandler() *OpenAIHandler {
	return &OpenAIHandler{}
}

// Name returns the handler name
func (h *OpenAIHandler) Name() string {
	return "openai"
}

// Priority returns handler priority (higher = checked first)
func (h *OpenAIHandler) Priority() int {
	return 100 // High priority for OpenAI format
}

// CanHandle checks if this handler can process the request
func (h *OpenAIHandler) CanHandle(req *http.Request) bool {
	// Check Content-Type first
	contentType := req.Header.Get("Content-Type")
	if !strings.Contains(contentType, "application/json") {
		return false
	}

	// Check for common OpenAI-compatible endpoints
	path := req.URL.Path

	// Direct OpenAI API
	if strings.Contains(path, "/chat/completions") {
		return true
	}

	// Azure OpenAI
	if strings.Contains(path, "/openai/deployments/") && strings.Contains(path, "/chat/completions") {
		return true
	}

	// Anthropic-style (also uses similar format)
	if strings.Contains(path, "/v1/messages") {
		return true
	}

	// GitHub Copilot / VS Code
	host := req.Host
	if strings.Contains(host, "api.githubcopilot.com") ||
		strings.Contains(host, "copilot-proxy") ||
		strings.Contains(host, "api.github.com") {
		return true
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
			"model":       req.Model,
			"stream":      req.Stream,
			"_raw_request": body, // Keep raw request for fields we don't parse
		},
	}

	// Store optional fields in metadata
	if req.Temperature != nil {
		msg.Metadata["temperature"] = *req.Temperature
	}
	if req.TopP != nil {
		msg.Metadata["top_p"] = *req.TopP
	}
	if req.MaxTokens != nil {
		msg.Metadata["max_tokens"] = *req.MaxTokens
	}
	if req.N != nil {
		msg.Metadata["n"] = *req.N
	}
	if req.User != "" {
		msg.Metadata["user"] = req.User
	}

	for i, m := range req.Messages {
		msg.Messages[i] = Message{
			Role:    m.Role,
			Content: m.getContentString(),
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
			"id":                 resp.ID,
			"object":             resp.Object,
			"created":            resp.Created,
			"model":              resp.Model,
			"system_fingerprint": resp.SystemFingerprint,
			"_raw_response":      body, // Keep raw response for fields we don't parse
		},
	}

	if resp.Usage != nil {
		msg.Metadata["usage"] = map[string]int{
			"prompt_tokens":     resp.Usage.PromptTokens,
			"completion_tokens": resp.Usage.CompletionTokens,
			"total_tokens":      resp.Usage.TotalTokens,
		}
	}

	for _, choice := range resp.Choices {
		msg.Messages = append(msg.Messages, Message{
			Role:    choice.Message.Role,
			Content: choice.Message.getContentString(),
		})
	}

	return msg, nil
}

// SerializeRequest converts StandardMessage back to OpenAI request format
// This reconstructs the request from the raw original, only replacing message contents
func (h *OpenAIHandler) SerializeRequest(msg *StandardMessage) ([]byte, error) {
	// If we have the raw request, modify it in place to preserve all fields
	if rawBytes, ok := msg.Metadata["_raw_request"].([]byte); ok {
		var raw map[string]json.RawMessage
		if err := json.Unmarshal(rawBytes, &raw); err == nil {
			// Update the messages array with potentially modified content
			var messages []openAIMessage
			if err := json.Unmarshal(raw["messages"], &messages); err == nil {
				// Update content from StandardMessage
				for i, m := range msg.Messages {
					if i < len(messages) {
						messages[i].setContentString(m.Content)
					}
				}
				// Re-serialize messages
				messagesBytes, err := json.Marshal(messages)
				if err == nil {
					raw["messages"] = messagesBytes
				}
			}
			return json.Marshal(raw)
		}
	}

	// Fallback: construct from scratch
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
	if temp, ok := msg.Metadata["temperature"].(float64); ok {
		req.Temperature = &temp
	}
	if topP, ok := msg.Metadata["top_p"].(float64); ok {
		req.TopP = &topP
	}
	if maxTokens, ok := msg.Metadata["max_tokens"].(int); ok {
		req.MaxTokens = &maxTokens
	}
	if user, ok := msg.Metadata["user"].(string); ok {
		req.User = user
	}

	for i, m := range msg.Messages {
		req.Messages[i] = openAIMessage{
			Role: m.Role,
		}
		req.Messages[i].setContentString(m.Content)
	}

	return json.Marshal(req)
}

// SerializeResponse converts StandardMessage back to OpenAI response format
// This reconstructs the response from the raw original, only replacing message contents
func (h *OpenAIHandler) SerializeResponse(msg *StandardMessage) ([]byte, error) {
	// If we have the raw response, modify it in place to preserve all fields
	if rawBytes, ok := msg.Metadata["_raw_response"].([]byte); ok {
		var raw map[string]json.RawMessage
		if err := json.Unmarshal(rawBytes, &raw); err == nil {
			// Update the choices array with potentially modified content
			var choices []struct {
				Index        int             `json:"index"`
				Message      json.RawMessage `json:"message"`
				FinishReason string          `json:"finish_reason,omitempty"`
				Logprobs     interface{}     `json:"logprobs,omitempty"`
			}
			if err := json.Unmarshal(raw["choices"], &choices); err == nil {
				for i, m := range msg.Messages {
					if i < len(choices) {
						var message openAIMessage
						if err := json.Unmarshal(choices[i].Message, &message); err == nil {
							message.setContentString(m.Content)
							if messageBytes, err := json.Marshal(message); err == nil {
								choices[i].Message = messageBytes
							}
						}
					}
				}
				if choicesBytes, err := json.Marshal(choices); err == nil {
					raw["choices"] = choicesBytes
				}
			}
			return json.Marshal(raw)
		}
	}

	// Fallback: construct from scratch
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
	if created, ok := msg.Metadata["created"].(int64); ok {
		resp.Created = created
	}
	if model, ok := msg.Metadata["model"].(string); ok {
		resp.Model = model
	}
	if fingerprint, ok := msg.Metadata["system_fingerprint"].(string); ok {
		resp.SystemFingerprint = fingerprint
	}

	for i, m := range msg.Messages {
		resp.Choices[i] = openAIChoice{
			Index: i,
			Message: openAIMessage{
				Role: m.Role,
			},
		}
		resp.Choices[i].Message.setContentString(m.Content)
	}

	return json.Marshal(resp)
}
