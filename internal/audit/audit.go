package audit

import (
	"encoding/json"
	"io"
	"log/slog"
	"os"
	"sync"
	"time"
)

// EventType represents the type of audit event
type EventType string

const (
	EventSecretDetected   EventType = "secret_detected"
	EventSecretReplaced   EventType = "secret_replaced"
	EventPlaceholderRestored EventType = "placeholder_restored"
	EventRequestProcessed EventType = "request_processed"
	EventResponseProcessed EventType = "response_processed"
	EventMappingCreated   EventType = "mapping_created"
	EventMappingExpired   EventType = "mapping_expired"
	EventTLSError         EventType = "tls_error"
	EventUpstreamError    EventType = "upstream_error"
)

// Event represents an audit log event
type Event struct {
	Timestamp   time.Time         `json:"timestamp"`
	Type        EventType         `json:"type"`
	RequestID   string            `json:"request_id,omitempty"`
	Interceptor string            `json:"interceptor,omitempty"`
	SecretType  string            `json:"secret_type,omitempty"`
	Host        string            `json:"host,omitempty"`
	Method      string            `json:"method,omitempty"`
	Path        string            `json:"path,omitempty"`
	Count       int               `json:"count,omitempty"`
	Duration    float64           `json:"duration_ms,omitempty"`
	Error       string            `json:"error,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// Config holds audit logger configuration
type Config struct {
	// Enabled enables/disables audit logging
	Enabled bool `yaml:"enabled"`

	// Level controls what events are logged
	// "minimal" - only secret detections
	// "standard" - secret detections + request/response events
	// "verbose" - all events including mappings
	Level string `yaml:"level"`

	// Output specifies where to write logs
	// "stdout", "stderr", or a file path
	Output string `yaml:"output"`

	// Format specifies log format: "json" or "text"
	Format string `yaml:"format"`

	// IncludeRequestDetails includes host/path in logs
	IncludeRequestDetails bool `yaml:"include_request_details"`
}

// DefaultConfig returns the default audit configuration
func DefaultConfig() *Config {
	return &Config{
		Enabled:               true,
		Level:                 "standard",
		Output:                "stdout",
		Format:                "json",
		IncludeRequestDetails: false,
	}
}

// Logger handles audit logging
type Logger struct {
	mu      sync.RWMutex
	config  *Config
	logger  *slog.Logger
	output  io.Writer
	enabled bool
}

// NewLogger creates a new audit logger
func NewLogger(cfg *Config) (*Logger, error) {
	if cfg == nil {
		cfg = DefaultConfig()
	}

	l := &Logger{
		config:  cfg,
		enabled: cfg.Enabled,
	}

	if err := l.setupOutput(); err != nil {
		return nil, err
	}

	return l, nil
}

func (l *Logger) setupOutput() error {
	var output io.Writer

	switch l.config.Output {
	case "stdout":
		output = os.Stdout
	case "stderr":
		output = os.Stderr
	default:
		// File output
		f, err := os.OpenFile(l.config.Output, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			return err
		}
		output = f
	}

	l.output = output

	var handler slog.Handler
	if l.config.Format == "json" {
		handler = slog.NewJSONHandler(output, &slog.HandlerOptions{
			Level: slog.LevelInfo,
		})
	} else {
		handler = slog.NewTextHandler(output, &slog.HandlerOptions{
			Level: slog.LevelInfo,
		})
	}

	l.logger = slog.New(handler)
	return nil
}

// Log logs an audit event
func (l *Logger) Log(event *Event) {
	l.mu.RLock()
	enabled := l.enabled
	config := l.config
	logger := l.logger
	l.mu.RUnlock()

	if !enabled || logger == nil {
		return
	}

	// Check if event should be logged based on level
	if !l.shouldLog(event.Type) {
		return
	}

	event.Timestamp = time.Now()

	// Redact request details if not enabled
	if !config.IncludeRequestDetails {
		event.Path = ""
	}

	// Build log attributes
	attrs := []any{
		slog.String("type", string(event.Type)),
	}

	if event.RequestID != "" {
		attrs = append(attrs, slog.String("request_id", event.RequestID))
	}
	if event.Interceptor != "" {
		attrs = append(attrs, slog.String("interceptor", event.Interceptor))
	}
	if event.SecretType != "" {
		attrs = append(attrs, slog.String("secret_type", event.SecretType))
	}
	if event.Host != "" {
		attrs = append(attrs, slog.String("host", event.Host))
	}
	if event.Method != "" {
		attrs = append(attrs, slog.String("method", event.Method))
	}
	if event.Path != "" {
		attrs = append(attrs, slog.String("path", event.Path))
	}
	if event.Count > 0 {
		attrs = append(attrs, slog.Int("count", event.Count))
	}
	if event.Duration > 0 {
		attrs = append(attrs, slog.Float64("duration_ms", event.Duration))
	}
	if event.Error != "" {
		attrs = append(attrs, slog.String("error", event.Error))
	}
	for k, v := range event.Metadata {
		attrs = append(attrs, slog.String(k, v))
	}

	logger.Info("audit", attrs...)
}

func (l *Logger) shouldLog(eventType EventType) bool {
	switch l.config.Level {
	case "minimal":
		return eventType == EventSecretDetected ||
			eventType == EventSecretReplaced ||
			eventType == EventPlaceholderRestored
	case "standard":
		return eventType != EventMappingCreated &&
			eventType != EventMappingExpired
	case "verbose":
		return true
	default:
		return true
	}
}

// LogSecretDetected logs a secret detection event
func (l *Logger) LogSecretDetected(requestID, interceptor, secretType string) {
	l.Log(&Event{
		Type:        EventSecretDetected,
		RequestID:   requestID,
		Interceptor: interceptor,
		SecretType:  secretType,
	})
}

// LogSecretReplaced logs a secret replacement event
func (l *Logger) LogSecretReplaced(requestID string, count int) {
	l.Log(&Event{
		Type:      EventSecretReplaced,
		RequestID: requestID,
		Count:     count,
	})
}

// LogPlaceholderRestored logs a placeholder restoration event
func (l *Logger) LogPlaceholderRestored(requestID string, count int) {
	l.Log(&Event{
		Type:      EventPlaceholderRestored,
		RequestID: requestID,
		Count:     count,
	})
}

// LogRequestProcessed logs request processing
func (l *Logger) LogRequestProcessed(requestID, method, host, path string, durationMs float64) {
	l.Log(&Event{
		Type:      EventRequestProcessed,
		RequestID: requestID,
		Method:    method,
		Host:      host,
		Path:      path,
		Duration:  durationMs,
	})
}

// LogResponseProcessed logs response processing
func (l *Logger) LogResponseProcessed(requestID, host string, durationMs float64) {
	l.Log(&Event{
		Type:      EventResponseProcessed,
		RequestID: requestID,
		Host:      host,
		Duration:  durationMs,
	})
}

// LogError logs an error event
func (l *Logger) LogError(eventType EventType, requestID, host, errorMsg string) {
	l.Log(&Event{
		Type:      eventType,
		RequestID: requestID,
		Host:      host,
		Error:     errorMsg,
	})
}

// Enable enables audit logging
func (l *Logger) Enable() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.enabled = true
}

// Disable disables audit logging
func (l *Logger) Disable() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.enabled = false
}

// SetLevel sets the logging level
func (l *Logger) SetLevel(level string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.config.Level = level
}

// Close closes the logger
func (l *Logger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if closer, ok := l.output.(io.Closer); ok {
		if l.output != os.Stdout && l.output != os.Stderr {
			return closer.Close()
		}
	}
	return nil
}

// ToJSON converts an event to JSON
func (e *Event) ToJSON() ([]byte, error) {
	return json.Marshal(e)
}

// NopLogger is a logger that does nothing
type NopLogger struct{}

// NewNopLogger creates a no-op logger
func NewNopLogger() *NopLogger {
	return &NopLogger{}
}

// Log does nothing
func (l *NopLogger) Log(_ *Event) {}

// LogSecretDetected does nothing
func (l *NopLogger) LogSecretDetected(_, _, _ string) {}

// LogSecretReplaced does nothing
func (l *NopLogger) LogSecretReplaced(_ string, _ int) {}

// LogPlaceholderRestored does nothing
func (l *NopLogger) LogPlaceholderRestored(_ string, _ int) {}

// LogRequestProcessed does nothing
func (l *NopLogger) LogRequestProcessed(_, _, _, _ string, _ float64) {}

// LogResponseProcessed does nothing
func (l *NopLogger) LogResponseProcessed(_, _ string, _ float64) {}

// LogError does nothing
func (l *NopLogger) LogError(_ EventType, _, _, _ string) {}

// Enable does nothing
func (l *NopLogger) Enable() {}

// Disable does nothing
func (l *NopLogger) Disable() {}

// SetLevel does nothing
func (l *NopLogger) SetLevel(_ string) {}

// Close does nothing
func (l *NopLogger) Close() error { return nil }
