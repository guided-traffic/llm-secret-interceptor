package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestServer_HealthHandler(t *testing.T) {
	cfg := DefaultConfig()
	srv := New(cfg)

	// Test without any health checkers
	req := httptest.NewRequest("GET", "/health", nil)
	rec := httptest.NewRecorder()

	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("health status = %d, want %d", rec.Code, http.StatusOK)
	}

	var status HealthStatus
	if err := json.Unmarshal(rec.Body.Bytes(), &status); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if status.Status != "healthy" {
		t.Errorf("status = %q, want 'healthy'", status.Status)
	}
}

func TestServer_HealthHandler_WithCheckers(t *testing.T) {
	cfg := DefaultConfig()
	srv := New(cfg)

	// Add healthy checker
	srv.RegisterHealthCheck("database", func() (bool, string) {
		return true, "connected"
	})

	// Add another healthy checker
	srv.RegisterHealthCheck("cache", func() (bool, string) {
		return true, "connected"
	})

	req := httptest.NewRequest("GET", "/health", nil)
	rec := httptest.NewRecorder()

	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("health status = %d, want %d", rec.Code, http.StatusOK)
	}

	var status HealthStatus
	if err := json.Unmarshal(rec.Body.Bytes(), &status); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if status.Checks["database"] != "ok" {
		t.Errorf("database check = %q, want 'ok'", status.Checks["database"])
	}
	if status.Checks["cache"] != "ok" {
		t.Errorf("cache check = %q, want 'ok'", status.Checks["cache"])
	}
}

func TestServer_HealthHandler_Unhealthy(t *testing.T) {
	cfg := DefaultConfig()
	srv := New(cfg)

	// Add unhealthy checker
	srv.RegisterHealthCheck("database", func() (bool, string) {
		return false, "connection refused"
	})

	req := httptest.NewRequest("GET", "/health", nil)
	rec := httptest.NewRecorder()

	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("health status = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}

	var status HealthStatus
	if err := json.Unmarshal(rec.Body.Bytes(), &status); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if status.Status != "unhealthy" {
		t.Errorf("status = %q, want 'unhealthy'", status.Status)
	}
	if status.Checks["database"] != "connection refused" {
		t.Errorf("database check = %q, want 'connection refused'", status.Checks["database"])
	}
}

func TestServer_ReadyHandler(t *testing.T) {
	cfg := DefaultConfig()
	srv := New(cfg)

	// Without any checkers, should be ready
	req := httptest.NewRequest("GET", "/ready", nil)
	rec := httptest.NewRecorder()

	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("ready status = %d, want %d", rec.Code, http.StatusOK)
	}

	if rec.Body.String() != "ready" {
		t.Errorf("body = %q, want 'ready'", rec.Body.String())
	}
}

func TestServer_ReadyHandler_NotReady(t *testing.T) {
	cfg := DefaultConfig()
	srv := New(cfg)

	// Add failing checker
	srv.RegisterHealthCheck("startup", func() (bool, string) {
		return false, "initializing"
	})

	req := httptest.NewRequest("GET", "/ready", nil)
	rec := httptest.NewRecorder()

	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("ready status = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}
}

func TestServer_LiveHandler(t *testing.T) {
	cfg := DefaultConfig()
	srv := New(cfg)

	req := httptest.NewRequest("GET", "/live", nil)
	rec := httptest.NewRecorder()

	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("live status = %d, want %d", rec.Code, http.StatusOK)
	}

	if rec.Body.String() != "alive" {
		t.Errorf("body = %q, want 'alive'", rec.Body.String())
	}
}

func TestServer_MetricsHandler(t *testing.T) {
	cfg := DefaultConfig()
	srv := New(cfg)

	req := httptest.NewRequest("GET", "/metrics", nil)
	rec := httptest.NewRecorder()

	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("metrics status = %d, want %d", rec.Code, http.StatusOK)
	}

	// Should contain some Prometheus metrics
	body := rec.Body.String()
	if len(body) == 0 {
		t.Error("metrics response should not be empty")
	}
}

func TestServer_StartStop(t *testing.T) {
	cfg := &Config{
		Addr:        ":0", // Random port
		MetricsPath: "/metrics",
		HealthPath:  "/health",
		ReadyPath:   "/ready",
		LivePath:    "/live",
	}
	srv := New(cfg)

	// Start in background
	errCh := make(chan error, 1)
	go func() {
		err := srv.Start()
		if err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
		close(errCh)
	}()

	// Give server time to start
	time.Sleep(50 * time.Millisecond)

	// Stop the server
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	if err := srv.Stop(ctx); err != nil {
		t.Errorf("Stop() error: %v", err)
	}

	// Check for errors
	if err := <-errCh; err != nil {
		t.Errorf("Start() error: %v", err)
	}
}

func TestServer_HealthStatus_HasUptime(t *testing.T) {
	cfg := DefaultConfig()
	srv := New(cfg)

	// Wait a bit so uptime is measurable
	time.Sleep(10 * time.Millisecond)

	req := httptest.NewRequest("GET", "/health", nil)
	rec := httptest.NewRecorder()

	srv.Handler().ServeHTTP(rec, req)

	var status HealthStatus
	if err := json.Unmarshal(rec.Body.Bytes(), &status); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if status.Uptime == "" {
		t.Error("uptime should not be empty")
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Addr != ":9090" {
		t.Errorf("Addr = %q, want ':9090'", cfg.Addr)
	}
	if cfg.MetricsPath != "/metrics" {
		t.Errorf("MetricsPath = %q, want '/metrics'", cfg.MetricsPath)
	}
	if cfg.HealthPath != "/health" {
		t.Errorf("HealthPath = %q, want '/health'", cfg.HealthPath)
	}
	if cfg.ReadyPath != "/ready" {
		t.Errorf("ReadyPath = %q, want '/ready'", cfg.ReadyPath)
	}
	if cfg.LivePath != "/live" {
		t.Errorf("LivePath = %q, want '/live'", cfg.LivePath)
	}
}

func TestServer_Addr(t *testing.T) {
	cfg := &Config{
		Addr:        ":8080",
		MetricsPath: "/metrics",
		HealthPath:  "/health",
		ReadyPath:   "/ready",
		LivePath:    "/live",
	}
	srv := New(cfg)

	if srv.Addr() != ":8080" {
		t.Errorf("Addr() = %q, want ':8080'", srv.Addr())
	}
}
