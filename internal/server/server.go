// Package server provides HTTP server utilities for health checks and metrics endpoints.
package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// HealthStatus represents the health status of the server
type HealthStatus struct {
	Status    string            `json:"status"`
	Timestamp time.Time         `json:"timestamp"`
	Version   string            `json:"version,omitempty"`
	Uptime    string            `json:"uptime,omitempty"`
	Checks    map[string]string `json:"checks,omitempty"`
}

// HealthChecker is a function that checks component health
type HealthChecker func() (ok bool, message string)

// Server provides HTTP endpoints for metrics and health
type Server struct {
	mu        sync.RWMutex
	server    *http.Server
	mux       *http.ServeMux
	checkers  map[string]HealthChecker
	startTime time.Time
	version   string
}

// Config holds management server configuration
type Config struct {
	// Addr is the address to listen on (e.g., ":9090")
	Addr string `yaml:"addr"`

	// MetricsPath is the path for Prometheus metrics
	MetricsPath string `yaml:"metrics_path"`

	// HealthPath is the path for health checks
	HealthPath string `yaml:"health_path"`

	// ReadyPath is the path for readiness checks
	ReadyPath string `yaml:"ready_path"`

	// LivePath is the path for liveness checks
	LivePath string `yaml:"live_path"`

	// Version is the application version
	Version string `yaml:"-"`
}

// DefaultConfig returns the default server configuration
func DefaultConfig() *Config {
	return &Config{
		Addr:        ":9090",
		MetricsPath: "/metrics",
		HealthPath:  "/health",
		ReadyPath:   "/ready",
		LivePath:    "/live",
		Version:     "dev",
	}
}

// New creates a new management server
func New(cfg *Config) *Server {
	if cfg == nil {
		cfg = DefaultConfig()
	}

	s := &Server{
		mux:       http.NewServeMux(),
		checkers:  make(map[string]HealthChecker),
		startTime: time.Now(),
		version:   cfg.Version,
	}

	// Register routes
	s.mux.Handle(cfg.MetricsPath, promhttp.Handler())
	s.mux.HandleFunc(cfg.HealthPath, s.healthHandler)
	s.mux.HandleFunc(cfg.ReadyPath, s.readyHandler)
	s.mux.HandleFunc(cfg.LivePath, s.liveHandler)

	s.server = &http.Server{
		Addr:         cfg.Addr,
		Handler:      s.mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	return s
}

// RegisterHealthCheck registers a health checker
func (s *Server) RegisterHealthCheck(name string, checker HealthChecker) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.checkers[name] = checker
}

// Start starts the management server
func (s *Server) Start() error {
	return s.server.ListenAndServe()
}

// Stop gracefully shuts down the server
func (s *Server) Stop(ctx context.Context) error {
	return s.server.Shutdown(ctx)
}

// healthHandler returns detailed health status
func (s *Server) healthHandler(w http.ResponseWriter, _ *http.Request) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	status := &HealthStatus{
		Status:    "healthy",
		Timestamp: time.Now(),
		Version:   s.version,
		Uptime:    time.Since(s.startTime).Round(time.Second).String(),
		Checks:    make(map[string]string),
	}

	// Run all health checks
	allHealthy := true
	for name, checker := range s.checkers {
		ok, msg := checker()
		if ok {
			status.Checks[name] = "ok"
		} else {
			status.Checks[name] = msg
			allHealthy = false
		}
	}

	if !allHealthy {
		status.Status = "unhealthy"
		w.WriteHeader(http.StatusServiceUnavailable)
	} else {
		w.WriteHeader(http.StatusOK)
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(status); err != nil {
		http.Error(w, "Failed to encode status", http.StatusInternalServerError)
	}
}

// readyHandler indicates if the service is ready to receive traffic
func (s *Server) readyHandler(w http.ResponseWriter, _ *http.Request) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Check all health checkers
	for name, checker := range s.checkers {
		ok, _ := checker()
		if !ok {
			w.WriteHeader(http.StatusServiceUnavailable)
			if _, err := fmt.Fprintf(w, "not ready: %s check failed", name); err != nil {
				return
			}
			return
		}
	}

	w.WriteHeader(http.StatusOK)
	if _, err := w.Write([]byte("ready")); err != nil {
		// Connection closed, nothing we can do
		return
	}
}

// liveHandler indicates if the service is alive
func (s *Server) liveHandler(w http.ResponseWriter, _ *http.Request) {
	// Simple liveness check - if we can respond, we're alive
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write([]byte("alive")); err != nil {
		// Connection closed, nothing we can do
		return
	}
}

// Handler returns the HTTP handler for testing
func (s *Server) Handler() http.Handler {
	return s.mux
}

// Addr returns the server address
func (s *Server) Addr() string {
	return s.server.Addr
}
