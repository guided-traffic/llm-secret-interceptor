package audit

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLogger_Log(t *testing.T) {
	// Create temp file for output
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "audit.log")

	cfg := &Config{
		Enabled: true,
		Level:   "verbose",
		Output:  logFile,
		Format:  "json",
	}

	logger, err := NewLogger(cfg)
	if err != nil {
		t.Fatalf("NewLogger() error: %v", err)
	}
	defer logger.Close()

	// Log an event
	logger.Log(&Event{
		Type:        EventSecretDetected,
		RequestID:   "req-123",
		Interceptor: "entropy",
		SecretType:  "api_key",
	})

	// Read the log file
	content, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	// Check content
	if !strings.Contains(string(content), "secret_detected") {
		t.Error("Log should contain 'secret_detected'")
	}
	if !strings.Contains(string(content), "req-123") {
		t.Error("Log should contain request ID")
	}
	if !strings.Contains(string(content), "entropy") {
		t.Error("Log should contain interceptor name")
	}
}

func TestLogger_LogLevel_Minimal(t *testing.T) {
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "audit.log")

	cfg := &Config{
		Enabled: true,
		Level:   "minimal",
		Output:  logFile,
		Format:  "json",
	}

	logger, err := NewLogger(cfg)
	if err != nil {
		t.Fatalf("NewLogger() error: %v", err)
	}
	defer logger.Close()

	// Log secret detected - should be logged
	logger.LogSecretDetected("req-1", "entropy", "api_key")

	// Log request processed - should NOT be logged at minimal level
	logger.LogRequestProcessed("req-2", "POST", "api.openai.com", "/v1/chat", 100)

	content, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	if !strings.Contains(string(content), "req-1") {
		t.Error("Should contain secret detection event")
	}
	if strings.Contains(string(content), "req-2") {
		t.Error("Should NOT contain request processed event at minimal level")
	}
}

func TestLogger_LogLevel_Standard(t *testing.T) {
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "audit.log")

	cfg := &Config{
		Enabled: true,
		Level:   "standard",
		Output:  logFile,
		Format:  "json",
	}

	logger, err := NewLogger(cfg)
	if err != nil {
		t.Fatalf("NewLogger() error: %v", err)
	}
	defer logger.Close()

	// These should be logged
	logger.LogSecretDetected("req-1", "entropy", "api_key")
	logger.LogRequestProcessed("req-2", "POST", "api.openai.com", "/v1/chat", 100)

	// This should NOT be logged at standard level
	logger.Log(&Event{
		Type:      EventMappingCreated,
		RequestID: "req-3",
	})

	content, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	if !strings.Contains(string(content), "req-1") {
		t.Error("Should contain secret detection event")
	}
	if !strings.Contains(string(content), "req-2") {
		t.Error("Should contain request processed event")
	}
	if strings.Contains(string(content), "req-3") {
		t.Error("Should NOT contain mapping created event at standard level")
	}
}

func TestLogger_Disabled(t *testing.T) {
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "audit.log")

	cfg := &Config{
		Enabled: false,
		Level:   "verbose",
		Output:  logFile,
		Format:  "json",
	}

	logger, err := NewLogger(cfg)
	if err != nil {
		t.Fatalf("NewLogger() error: %v", err)
	}
	defer logger.Close()

	// Log events
	logger.LogSecretDetected("req-1", "entropy", "api_key")
	logger.LogRequestProcessed("req-2", "POST", "api.openai.com", "/v1/chat", 100)

	// File should not exist or be empty
	content, _ := os.ReadFile(logFile)
	if len(content) > 0 {
		t.Error("Log file should be empty when logging is disabled")
	}
}

func TestLogger_EnableDisable(t *testing.T) {
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "audit.log")

	cfg := &Config{
		Enabled: true,
		Level:   "verbose",
		Output:  logFile,
		Format:  "json",
	}

	logger, err := NewLogger(cfg)
	if err != nil {
		t.Fatalf("NewLogger() error: %v", err)
	}
	defer logger.Close()

	// Log while enabled
	logger.LogSecretDetected("req-1", "entropy", "api_key")

	// Disable and log
	logger.Disable()
	logger.LogSecretDetected("req-2", "entropy", "api_key")

	// Re-enable and log
	logger.Enable()
	logger.LogSecretDetected("req-3", "entropy", "api_key")

	content, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	if !strings.Contains(string(content), "req-1") {
		t.Error("Should contain first event (enabled)")
	}
	if strings.Contains(string(content), "req-2") {
		t.Error("Should NOT contain second event (disabled)")
	}
	if !strings.Contains(string(content), "req-3") {
		t.Error("Should contain third event (re-enabled)")
	}
}

func TestLogger_IncludeRequestDetails(t *testing.T) {
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "audit.log")

	// Without request details
	cfg := &Config{
		Enabled:               true,
		Level:                 "verbose",
		Output:                logFile,
		Format:                "json",
		IncludeRequestDetails: false,
	}

	logger, err := NewLogger(cfg)
	if err != nil {
		t.Fatalf("NewLogger() error: %v", err)
	}

	logger.LogRequestProcessed("req-1", "POST", "api.openai.com", "/v1/chat/completions", 100)
	logger.Close()

	content, _ := os.ReadFile(logFile)
	// Path should be redacted
	if strings.Contains(string(content), "/v1/chat/completions") {
		t.Error("Path should be redacted when IncludeRequestDetails is false")
	}

	// With request details
	logFile2 := filepath.Join(tmpDir, "audit2.log")
	cfg2 := &Config{
		Enabled:               true,
		Level:                 "verbose",
		Output:                logFile2,
		Format:                "json",
		IncludeRequestDetails: true,
	}

	logger2, err := NewLogger(cfg2)
	if err != nil {
		t.Fatalf("NewLogger() error: %v", err)
	}

	logger2.LogRequestProcessed("req-2", "POST", "api.openai.com", "/v1/chat/completions", 100)
	logger2.Close()

	content2, _ := os.ReadFile(logFile2)
	if !strings.Contains(string(content2), "/v1/chat/completions") {
		t.Error("Path should be included when IncludeRequestDetails is true")
	}
}

func TestLogger_StdoutOutput(t *testing.T) {
	cfg := &Config{
		Enabled: true,
		Level:   "verbose",
		Output:  "stdout",
		Format:  "json",
	}

	logger, err := NewLogger(cfg)
	if err != nil {
		t.Fatalf("NewLogger() error: %v", err)
	}
	defer logger.Close()

	// Should not panic
	logger.LogSecretDetected("req-1", "entropy", "api_key")
}

func TestNopLogger(t *testing.T) {
	logger := NewNopLogger()

	// All these should do nothing without panicking
	logger.Log(&Event{Type: EventSecretDetected})
	logger.LogSecretDetected("req-1", "entropy", "api_key")
	logger.LogSecretReplaced("req-1", 1)
	logger.LogPlaceholderRestored("req-1", 1)
	logger.LogRequestProcessed("req-1", "POST", "host", "/path", 100)
	logger.LogResponseProcessed("req-1", "host", 100)
	logger.LogError(EventTLSError, "req-1", "host", "error")
	logger.Enable()
	logger.Disable()
	logger.SetLevel("verbose")
	err := logger.Close()
	if err != nil {
		t.Errorf("Close() error: %v", err)
	}
}

func TestEvent_ToJSON(t *testing.T) {
	event := &Event{
		Type:        EventSecretDetected,
		RequestID:   "req-123",
		Interceptor: "pattern",
		SecretType:  "github_token",
		Count:       2,
	}

	data, err := event.ToJSON()
	if err != nil {
		t.Fatalf("ToJSON() error: %v", err)
	}

	if !bytes.Contains(data, []byte("secret_detected")) {
		t.Error("JSON should contain event type")
	}
	if !bytes.Contains(data, []byte("pattern")) {
		t.Error("JSON should contain interceptor")
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if !cfg.Enabled {
		t.Error("Default config should be enabled")
	}
	if cfg.Level != "standard" {
		t.Errorf("Default level = %q, want 'standard'", cfg.Level)
	}
	if cfg.Output != "stdout" {
		t.Errorf("Default output = %q, want 'stdout'", cfg.Output)
	}
	if cfg.Format != "json" {
		t.Errorf("Default format = %q, want 'json'", cfg.Format)
	}
}
