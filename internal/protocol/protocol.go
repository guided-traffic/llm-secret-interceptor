package protocol

import (
	"net/http"
	"sort"
)

// StandardMessage represents the internal standardized message format
// All protocol handlers convert their specific format to this
type StandardMessage struct {
	// Messages contains the conversation history
	Messages []Message `json:"messages"`
	// Metadata contains protocol-specific metadata
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// Message represents a single message in the conversation
type Message struct {
	Role    string `json:"role"`    // "user", "assistant", "system"
	Content string `json:"content"` // The message content
}

// GetAllContent returns all message contents concatenated
func (sm *StandardMessage) GetAllContent() string {
	var content string
	for _, m := range sm.Messages {
		content += m.Content + "\n"
	}
	return content
}

// GetUserMessages returns only user messages
func (sm *StandardMessage) GetUserMessages() []Message {
	var messages []Message
	for _, m := range sm.Messages {
		if m.Role == "user" {
			messages = append(messages, m)
		}
	}
	return messages
}

// Handler defines the interface for protocol handlers
type Handler interface {
	// Name returns the name of the protocol handler
	Name() string

	// CanHandle checks if this handler can process the given request
	CanHandle(req *http.Request) bool

	// Priority returns the handler priority (higher = checked first)
	Priority() int

	// ParseRequest parses a request body into StandardMessage format
	ParseRequest(body []byte) (*StandardMessage, error)

	// ParseResponse parses a response body into StandardMessage format
	ParseResponse(body []byte) (*StandardMessage, error)

	// SerializeRequest converts a StandardMessage back to protocol-specific format
	SerializeRequest(msg *StandardMessage) ([]byte, error)

	// SerializeResponse converts a StandardMessage back to protocol-specific format
	SerializeResponse(msg *StandardMessage) ([]byte, error)
}

// Registry holds all registered protocol handlers
type Registry struct {
	handlers []Handler
	sorted   bool
}

// NewRegistry creates a new protocol registry
func NewRegistry() *Registry {
	return &Registry{
		handlers: make([]Handler, 0),
		sorted:   false,
	}
}

// Register adds a new handler to the registry
func (r *Registry) Register(h Handler) {
	r.handlers = append(r.handlers, h)
	r.sorted = false
}

// sortHandlers sorts handlers by priority (descending)
func (r *Registry) sortHandlers() {
	if r.sorted {
		return
	}
	sort.Slice(r.handlers, func(i, j int) bool {
		return r.handlers[i].Priority() > r.handlers[j].Priority()
	})
	r.sorted = true
}

// Detect finds the appropriate handler for a given request
func (r *Registry) Detect(req *http.Request) Handler {
	r.sortHandlers()
	for _, h := range r.handlers {
		if h.CanHandle(req) {
			return h
		}
	}
	return nil
}

// Get returns a handler by name
func (r *Registry) Get(name string) Handler {
	for _, h := range r.handlers {
		if h.Name() == name {
			return h
		}
	}
	return nil
}

// List returns all registered handler names
func (r *Registry) List() []string {
	names := make([]string, len(r.handlers))
	for i, h := range r.handlers {
		names[i] = h.Name()
	}
	return names
}
