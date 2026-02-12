package protocol

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// StreamChunk represents a single chunk in a streaming response
type StreamChunk struct {
	// Data contains the raw data of the chunk
	Data []byte
	// Delta contains the incremental content (for OpenAI format)
	Delta string
	// Role contains the role if present in this chunk
	Role string
	// FinishReason indicates if this is the final chunk
	FinishReason string
	// IsDone indicates if the stream has finished
	IsDone bool
	// Metadata contains any additional chunk-specific data
	Metadata map[string]interface{}
}

// StreamingHandler extends Handler with streaming capabilities
type StreamingHandler interface {
	Handler

	// IsStreaming checks if the request is for a streaming response
	IsStreaming(body []byte) bool

	// ParseStreamChunk parses a single SSE chunk
	ParseStreamChunk(data []byte) (*StreamChunk, error)

	// SerializeStreamChunk converts a chunk back to SSE format
	SerializeStreamChunk(chunk *StreamChunk) ([]byte, error)
}

// SSEParser parses Server-Sent Events format
type SSEParser struct {
	reader *bufio.Reader
}

// NewSSEParser creates a new SSE parser
func NewSSEParser(r io.Reader) *SSEParser {
	return &SSEParser{
		reader: bufio.NewReader(r),
	}
}

// ReadEvent reads the next SSE event
func (p *SSEParser) ReadEvent() (eventType string, data []byte, err error) {
	var dataLines [][]byte

	for {
		line, err := p.reader.ReadBytes('\n')
		if err != nil {
			if err == io.EOF && len(dataLines) > 0 {
				// Return accumulated data on EOF
				break
			}
			return "", nil, err
		}

		// Trim the newline
		line = bytes.TrimRight(line, "\r\n")

		// Empty line signals end of event
		if len(line) == 0 {
			if len(dataLines) > 0 {
				break
			}
			continue
		}

		// Parse the field
		switch {
		case bytes.HasPrefix(line, []byte("event:")):
			eventType = strings.TrimSpace(string(line[6:]))
		case bytes.HasPrefix(line, []byte("data:")):
			data := bytes.TrimPrefix(line, []byte("data:"))
			data = bytes.TrimSpace(data)
			dataLines = append(dataLines, data)
		case bytes.HasPrefix(line, []byte(":")):
			// Comment, ignore
			continue
		}
	}

	// Combine all data lines
	if len(dataLines) > 0 {
		data = bytes.Join(dataLines, []byte("\n"))
	}

	return eventType, data, nil
}

// SSEWriter writes Server-Sent Events format
type SSEWriter struct {
	writer io.Writer
}

// NewSSEWriter creates a new SSE writer
func NewSSEWriter(w io.Writer) *SSEWriter {
	return &SSEWriter{writer: w}
}

// WriteEvent writes an SSE event
func (w *SSEWriter) WriteEvent(eventType string, data []byte) error {
	var buf bytes.Buffer

	if eventType != "" {
		fmt.Fprintf(&buf, "event: %s\n", eventType)
	}

	// Split data into lines
	lines := bytes.Split(data, []byte("\n"))
	for _, line := range lines {
		fmt.Fprintf(&buf, "data: %s\n", line)
	}

	// End of event
	buf.WriteString("\n")

	_, err := w.writer.Write(buf.Bytes())
	return err
}

// OpenAI Streaming structures

// openAIStreamChunk represents a streaming chunk in OpenAI format
type openAIStreamChunk struct {
	ID      string               `json:"id"`
	Object  string               `json:"object"`
	Created int64                `json:"created"`
	Model   string               `json:"model"`
	Choices []openAIStreamChoice `json:"choices"`
}

type openAIStreamChoice struct {
	Index        int               `json:"index"`
	Delta        openAIStreamDelta `json:"delta"`
	FinishReason *string           `json:"finish_reason"`
}

type openAIStreamDelta struct {
	Role    string `json:"role,omitempty"`
	Content string `json:"content,omitempty"`
}

// Ensure OpenAIHandler implements StreamingHandler
var _ StreamingHandler = (*OpenAIHandler)(nil)

// IsStreaming checks if the request is for streaming
func (h *OpenAIHandler) IsStreaming(body []byte) bool {
	var req struct {
		Stream bool `json:"stream"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		return false
	}
	return req.Stream
}

// ParseStreamChunk parses an OpenAI streaming chunk
func (h *OpenAIHandler) ParseStreamChunk(data []byte) (*StreamChunk, error) {
	// Check for [DONE] marker
	trimmed := bytes.TrimSpace(data)
	if bytes.Equal(trimmed, []byte("[DONE]")) {
		return &StreamChunk{
			Data:   data,
			IsDone: true,
		}, nil
	}

	var chunk openAIStreamChunk
	if err := json.Unmarshal(data, &chunk); err != nil {
		return nil, fmt.Errorf("failed to parse stream chunk: %w", err)
	}

	result := &StreamChunk{
		Data: data,
		Metadata: map[string]interface{}{
			"id":      chunk.ID,
			"object":  chunk.Object,
			"created": chunk.Created,
			"model":   chunk.Model,
		},
	}

	if len(chunk.Choices) > 0 {
		choice := chunk.Choices[0]
		result.Delta = choice.Delta.Content
		result.Role = choice.Delta.Role
		if choice.FinishReason != nil {
			result.FinishReason = *choice.FinishReason
		}
	}

	return result, nil
}

// SerializeStreamChunk converts a chunk back to SSE format
func (h *OpenAIHandler) SerializeStreamChunk(chunk *StreamChunk) ([]byte, error) {
	if chunk.IsDone {
		return []byte("[DONE]"), nil
	}

	// Reconstruct the OpenAI chunk
	streamChunk := openAIStreamChunk{
		Choices: []openAIStreamChoice{
			{
				Index: 0,
				Delta: openAIStreamDelta{
					Role:    chunk.Role,
					Content: chunk.Delta,
				},
			},
		},
	}

	// Restore metadata
	if id, ok := chunk.Metadata["id"].(string); ok {
		streamChunk.ID = id
	}
	if object, ok := chunk.Metadata["object"].(string); ok {
		streamChunk.Object = object
	}
	if created, ok := chunk.Metadata["created"].(int64); ok {
		streamChunk.Created = created
	}
	if model, ok := chunk.Metadata["model"].(string); ok {
		streamChunk.Model = model
	}

	if chunk.FinishReason != "" {
		streamChunk.Choices[0].FinishReason = &chunk.FinishReason
	}

	return json.Marshal(streamChunk)
}

// StreamBuffer handles buffering for placeholder detection in streams
type StreamBuffer struct {
	buffer    []byte
	maxLength int
}

// NewStreamBuffer creates a new stream buffer
func NewStreamBuffer(maxLength int) *StreamBuffer {
	return &StreamBuffer{
		buffer:    make([]byte, 0, maxLength*2),
		maxLength: maxLength,
	}
}

// Write adds data to the buffer
func (b *StreamBuffer) Write(data []byte) {
	b.buffer = append(b.buffer, data...)
}

// Flush returns all safe data and keeps only the last maxLength bytes
func (b *StreamBuffer) Flush() []byte {
	if len(b.buffer) <= b.maxLength {
		return nil
	}

	// Return everything except the last maxLength bytes
	safeLen := len(b.buffer) - b.maxLength
	safe := make([]byte, safeLen)
	copy(safe, b.buffer[:safeLen])

	// Keep only the last maxLength bytes
	copy(b.buffer, b.buffer[safeLen:])
	b.buffer = b.buffer[:b.maxLength]

	return safe
}

// FlushAll returns all buffered data
func (b *StreamBuffer) FlushAll() []byte {
	data := make([]byte, len(b.buffer))
	copy(data, b.buffer)
	b.buffer = b.buffer[:0]
	return data
}

// Len returns the current buffer length
func (b *StreamBuffer) Len() int {
	return len(b.buffer)
}
