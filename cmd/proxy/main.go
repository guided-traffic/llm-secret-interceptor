// Package main provides the entry point for the LLM Secret Interceptor proxy server.
package main

import (
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/hfi/llm-secret-interceptor/internal/config"
	"github.com/hfi/llm-secret-interceptor/internal/proxy"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog"
)

var (
	// Version information - set at build time
	Version   = "dev"
	GitCommit = "unknown"
	BuildTime = "unknown"
)

func main() {
	// Parse command line flags
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "version":
			fmt.Printf("LLM Secret Interceptor %s\n", Version)
			fmt.Printf("Git Commit: %s\n", GitCommit)
			fmt.Printf("Build Time: %s\n", BuildTime)
			os.Exit(0)
		case "generate-ca":
			certPath := "./certs/ca.crt"
			keyPath := "./certs/ca.key"
			if len(os.Args) > 2 {
				certPath = os.Args[2]
			}
			if len(os.Args) > 3 {
				keyPath = os.Args[3]
			}
			if err := proxy.GenerateCA(certPath, keyPath); err != nil {
				fmt.Fprintf(os.Stderr, "Failed to generate CA: %v\n", err)
				os.Exit(1)
			}
			fmt.Printf("CA certificate generated:\n  Certificate: %s\n  Key: %s\n", certPath, keyPath)
			os.Exit(0)
		}
	}

	// Setup logger
	logger := zerolog.New(os.Stdout).With().Timestamp().Logger()

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to load configuration")
	}

	// Set log level
	switch cfg.Logging.Level {
	case "debug":
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	case "warn":
		zerolog.SetGlobalLevel(zerolog.WarnLevel)
	case "error":
		zerolog.SetGlobalLevel(zerolog.ErrorLevel)
	default:
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	}

	logger.Info().
		Str("version", Version).
		Str("commit", GitCommit).
		Msg("Starting LLM Secret Interceptor")

	// Check if CA certificate exists
	if _, err := os.Stat(cfg.TLS.CACert); os.IsNotExist(err) {
		logger.Info().Msg("CA certificate not found, generating...")
		if err := proxy.GenerateCA(cfg.TLS.CACert, cfg.TLS.CAKey); err != nil {
			logger.Fatal().Err(err).Msg("Failed to generate CA certificate")
		}
		logger.Info().
			Str("cert", cfg.TLS.CACert).
			Str("key", cfg.TLS.CAKey).
			Msg("CA certificate generated")
	}

	// Create proxy server
	server, err := proxy.NewServer(cfg, logger)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to create proxy server")
	}

	// Start metrics server if enabled
	if cfg.Metrics.Enabled {
		go func() {
			metricsAddr := fmt.Sprintf(":%d", cfg.Metrics.Port)
			mux := http.NewServeMux()
			mux.Handle(cfg.Metrics.Endpoint, promhttp.Handler())
			mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
				if _, err := w.Write([]byte("OK")); err != nil {
					logger.Debug().Err(err).Msg("Failed to write health response")
				}
			})
			logger.Info().Str("addr", metricsAddr).Msg("Starting metrics server")
			metricsServer := &http.Server{
				Addr:              metricsAddr,
				Handler:           mux,
				ReadHeaderTimeout: 10 * time.Second,
				ReadTimeout:       30 * time.Second,
				WriteTimeout:      30 * time.Second,
				IdleTimeout:       60 * time.Second,
			}
			if err := metricsServer.ListenAndServe(); err != nil {
				logger.Error().Err(err).Msg("Metrics server error")
			}
		}()
	}

	// Start proxy server
	if err := server.Start(); err != nil {
		logger.Fatal().Err(err).Msg("Failed to start proxy server")
	}

	logger.Info().Str("listen", cfg.Proxy.Listen).Msg("Proxy server started")

	// Update metrics periodically
	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			server.UpdateMappingStoreSize()
		}
	}()

	// Wait for shutdown signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	logger.Info().Msg("Shutting down...")

	if err := server.Stop(); err != nil {
		logger.Error().Err(err).Msg("Error during shutdown")
	}

	logger.Info().Msg("Shutdown complete")
}
