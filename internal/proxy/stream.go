package proxy

import (
	"bytes"
	"io"

	"github.com/hfi/llm-secret-interceptor/internal/protocol"
)

// StreamProcessor handles streaming response processing with buffering
type StreamProcessor struct {
	service       *SecretService
	handler       protocol.StreamingHandler
	buffer        *protocol.StreamBuffer
	writer        io.Writer
	accumulated   string
}

// NewStreamProcessor creates a new stream processor
func NewStreamProcessor(
	service *SecretService,
	handler protocol.StreamingHandler,
	writer io.Writer,
	maxPlaceholderLen int,
) *StreamProcessor {
	return &StreamProcessor{
		service: service,
		handler: handler,
		buffer:  protocol.NewStreamBuffer(maxPlaceholderLen),
		writer:  writer,
	}
}

// ProcessChunk processes a single SSE data chunk
func (sp *StreamProcessor) ProcessChunk(data []byte) error {
	// Parse the chunk
	chunk, err := sp.handler.ParseStreamChunk(data)
	if err != nil {
		// If we can't parse, pass through
		return sp.writeSSEEvent(data)
	}

	// If done marker, flush buffer and forward
	if chunk.IsDone {
		if err := sp.flushAll(); err != nil {
			return err
		}
		return sp.writeSSEEvent(data)
	}

	// Add delta to accumulated content
	sp.accumulated += chunk.Delta

	// Add delta to buffer
	sp.buffer.Write([]byte(chunk.Delta))

	// Flush safe content
	safe := sp.buffer.Flush()
	if safe != nil {
		// Process the safe content for placeholder restoration
		processed := sp.processContent(string(safe))

		// Create a new chunk with processed content
		outputChunk := &protocol.StreamChunk{
			Delta:        processed,
			Role:         chunk.Role,
			FinishReason: "",
			Metadata:     chunk.Metadata,
		}

		serialized, err := sp.handler.SerializeStreamChunk(outputChunk)
		if err != nil {
			return err
		}

		if err := sp.writeSSEEvent(serialized); err != nil {
			return err
		}
	}

	return nil
}

// Flush processes and sends any remaining buffered content
func (sp *StreamProcessor) Flush() error {
	return sp.flushAll()
}

func (sp *StreamProcessor) flushAll() error {
	remaining := sp.buffer.FlushAll()
	if len(remaining) == 0 {
		return nil
	}

	// Process remaining content
	processed := sp.processContent(string(remaining))

	// Create final chunk
	chunk := &protocol.StreamChunk{
		Delta: processed,
	}

	serialized, err := sp.handler.SerializeStreamChunk(chunk)
	if err != nil {
		return err
	}

	return sp.writeSSEEvent(serialized)
}

func (sp *StreamProcessor) processContent(content string) string {
	result := sp.service.replacer.Restore(content, func(ph string) (string, bool) {
		return sp.service.store.Lookup(ph)
	})
	return result.Text
}

func (sp *StreamProcessor) writeSSEEvent(data []byte) error {
	// Write in SSE format
	var buf bytes.Buffer
	buf.WriteString("data: ")
	buf.Write(data)
	buf.WriteString("\n\n")

	_, err := sp.writer.Write(buf.Bytes())
	return err
}

// GetAccumulated returns the accumulated content from all chunks
func (sp *StreamProcessor) GetAccumulated() string {
	return sp.accumulated
}

// StreamReader wraps an io.Reader to process SSE events
type StreamReader struct {
	reader    *protocol.SSEParser
	processor *StreamProcessor
	buffer    bytes.Buffer
	done      bool
}

// NewStreamReader creates a new stream reader
func NewStreamReader(
	r io.Reader,
	service *SecretService,
	handler protocol.StreamingHandler,
	maxPlaceholderLen int,
) *StreamReader {
	return &StreamReader{
		reader:    protocol.NewSSEParser(r),
		processor: NewStreamProcessor(service, handler, nil, maxPlaceholderLen),
		done:      false,
	}
}

// ReadProcessedEvent reads and processes the next SSE event
func (sr *StreamReader) ReadProcessedEvent() ([]byte, error) {
	if sr.done {
		return nil, io.EOF
	}

	_, data, err := sr.reader.ReadEvent()
	if err != nil {
		if err == io.EOF {
			sr.done = true
			// Flush any remaining
			sr.processor.Flush()
		}
		return nil, err
	}

	// Check for done marker
	if bytes.Equal(bytes.TrimSpace(data), []byte("[DONE]")) {
		sr.done = true
		sr.processor.Flush()
		return data, nil
	}

	// Parse and process the chunk
	chunk, err := sr.processor.handler.ParseStreamChunk(data)
	if err != nil {
		// Can't parse, return as-is
		return data, nil
	}

	// Process delta content
	if chunk.Delta != "" {
		result := sr.processor.processContent(chunk.Delta)
		if result != chunk.Delta {
			chunk.Delta = result
			return sr.processor.handler.SerializeStreamChunk(chunk)
		}
	}

	return data, nil
}
