package protocol

import "net/http"

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

// Handler defines the interface for protocol handlers
type Handler interface {
	// Name returns the name of the protocol handler
	Name() string

	// CanHandle checks if this handler can process the given request
	CanHandle(req *http.Request) bool

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
}

// NewRegistry creates a new protocol registry
func NewRegistry() *Registry {
	return &Registry{
		handlers: make([]Handler, 0),
	}
}

// Register adds a new handler to the registry
func (r *Registry) Register(h Handler) {
	r.handlers = append(r.handlers, h)
}

// Detect finds the appropriate handler for a given request
func (r *Registry) Detect(req *http.Request) Handler {
	for _, h := range r.handlers {
		if h.CanHandle(req) {
			return h
		}
	}
	return nil
}
