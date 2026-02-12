package proxy

import (
	"bufio"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/hfi/llm-secret-interceptor/internal/config"
	"github.com/hfi/llm-secret-interceptor/internal/interceptor"
	"github.com/hfi/llm-secret-interceptor/internal/metrics"
	"github.com/hfi/llm-secret-interceptor/internal/protocol"
	"github.com/hfi/llm-secret-interceptor/internal/storage"
	"github.com/hfi/llm-secret-interceptor/pkg/placeholder"
	"github.com/rs/zerolog"
)

// Server represents the HTTPS proxy server with TLS interception
type Server struct {
	config       *config.Config
	certManager  *CertManager
	registry     *protocol.Registry
	interceptors *interceptor.Manager
	store        storage.MappingStore
	placeholder  *placeholder.Generator
	httpServer   *http.Server
	logger       zerolog.Logger
	wg           sync.WaitGroup
}

// NewServer creates a new proxy server instance
func NewServer(cfg *config.Config, logger zerolog.Logger) (*Server, error) {
	// Initialize certificate manager
	certManager, err := NewCertManager(cfg.TLS.CACert, cfg.TLS.CAKey)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize certificate manager: %w", err)
	}

	// Initialize protocol registry
	registry := protocol.NewRegistry()
	registry.Register(protocol.NewOpenAIHandler())

	// Initialize interceptor manager
	interceptorManager := interceptor.NewManager()
	if cfg.Interceptors.Entropy.Enabled {
		entropyInterceptor := interceptor.NewEntropyInterceptor(
			cfg.Interceptors.Entropy.Threshold,
			cfg.Interceptors.Entropy.MinLength,
			cfg.Interceptors.Entropy.MaxLength,
		)
		interceptorManager.Register(entropyInterceptor)
	}

	// Initialize storage
	var store storage.MappingStore
	if cfg.Storage.Type == "redis" {
		store, err = storage.NewRedisStore(
			cfg.Storage.Redis.Address,
			cfg.Storage.Redis.Password,
			cfg.Storage.Redis.DB,
			cfg.Storage.TTL,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize Redis store: %w", err)
		}
	} else {
		store = storage.NewMemoryStore(cfg.Storage.TTL)
	}

	// Initialize placeholder generator
	placeholderGen := placeholder.NewGenerator(cfg.Placeholder.Prefix, cfg.Placeholder.Suffix)

	server := &Server{
		config:       cfg,
		certManager:  certManager,
		registry:     registry,
		interceptors: interceptorManager,
		store:        store,
		placeholder:  placeholderGen,
		logger:       logger,
	}

	return server, nil
}

// Start starts the proxy server
func (s *Server) Start() error {
	s.logger.Info().Str("listen", s.config.Proxy.Listen).Msg("Starting proxy server")

	s.httpServer = &http.Server{
		Addr:    s.config.Proxy.Listen,
		Handler: s,
		// Disable HTTP/2 for easier request manipulation
		TLSNextProto: make(map[string]func(*http.Server, *tls.Conn, http.Handler)),
	}

	ln, err := net.Listen("tcp", s.config.Proxy.Listen)
	if err != nil {
		return fmt.Errorf("failed to listen: %w", err)
	}

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		if err := s.httpServer.Serve(ln); err != nil && err != http.ErrServerClosed {
			s.logger.Error().Err(err).Msg("Server error")
		}
	}()

	return nil
}

// Stop gracefully stops the proxy server
func (s *Server) Stop() error {
	s.logger.Info().Msg("Stopping proxy server")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := s.httpServer.Shutdown(ctx); err != nil {
		return fmt.Errorf("failed to shutdown server: %w", err)
	}

	s.wg.Wait()

	// Close storage
	if err := s.store.Close(); err != nil {
		return fmt.Errorf("failed to close store: %w", err)
	}

	return nil
}

// ServeHTTP handles incoming HTTP requests
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	metrics.RecordRequest(r.Method, r.Host)
	start := time.Now()

	if r.Method == http.MethodConnect {
		// HTTPS CONNECT tunnel
		s.handleConnect(w, r)
	} else {
		// Plain HTTP request (passthrough)
		s.handleHTTP(w, r)
	}

	metrics.RecordRequestDuration("request", time.Since(start).Seconds())
}

// handleConnect handles HTTPS CONNECT requests for TLS interception
func (s *Server) handleConnect(w http.ResponseWriter, r *http.Request) {
	s.logger.Debug().Str("host", r.Host).Msg("CONNECT request")

	// Hijack the connection
	hijacker, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "Hijacking not supported", http.StatusInternalServerError)
		return
	}

	clientConn, _, err := hijacker.Hijack()
	if err != nil {
		s.logger.Error().Err(err).Msg("Failed to hijack connection")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Send 200 Connection Established
	_, err = clientConn.Write([]byte("HTTP/1.1 200 Connection Established\r\n\r\n"))
	if err != nil {
		s.logger.Error().Err(err).Msg("Failed to send connection established")
		clientConn.Close()
		return
	}

	// Create TLS config with dynamic certificate
	tlsConfig := &tls.Config{
		GetCertificate: s.certManager.GetCertificate,
	}

	// Wrap client connection with TLS
	tlsClientConn := tls.Server(clientConn, tlsConfig)
	if err := tlsClientConn.Handshake(); err != nil {
		s.logger.Error().Err(err).Msg("TLS handshake failed")
		clientConn.Close()
		return
	}

	// Handle the TLS connection
	s.handleTLSConnection(tlsClientConn, r.Host)
}

// handleTLSConnection processes requests over an intercepted TLS connection
func (s *Server) handleTLSConnection(clientConn *tls.Conn, targetHost string) {
	defer clientConn.Close()

	reader := bufio.NewReader(clientConn)

	for {
		// Read HTTP request from client
		req, err := http.ReadRequest(reader)
		if err != nil {
			if err != io.EOF {
				s.logger.Debug().Err(err).Msg("Failed to read request")
			}
			return
		}

		// Set the correct host and scheme
		req.URL.Scheme = "https"
		req.URL.Host = targetHost
		req.RequestURI = ""

		// Process and forward the request
		resp, err := s.processRequest(req)
		if err != nil {
			s.logger.Error().Err(err).Msg("Failed to process request")
			s.sendErrorResponse(clientConn, http.StatusBadGateway, err.Error())
			return
		}

		// Process the response
		processedResp, err := s.processResponse(resp)
		if err != nil {
			s.logger.Error().Err(err).Msg("Failed to process response")
			resp.Body.Close()
			s.sendErrorResponse(clientConn, http.StatusInternalServerError, err.Error())
			return
		}

		// Write response back to client
		if err := processedResp.Write(clientConn); err != nil {
			s.logger.Debug().Err(err).Msg("Failed to write response")
			processedResp.Body.Close()
			return
		}
		processedResp.Body.Close()
	}
}

// handleHTTP handles plain HTTP requests (passthrough)
func (s *Server) handleHTTP(w http.ResponseWriter, r *http.Request) {
	s.logger.Debug().Str("url", r.URL.String()).Msg("HTTP request")

	// For plain HTTP, just proxy through
	resp, err := http.DefaultTransport.RoundTrip(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	// Copy headers
	for key, values := range resp.Header {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}
	w.WriteHeader(resp.StatusCode)

	// Copy body
	io.Copy(w, resp.Body)
}

// processRequest intercepts and modifies outgoing requests
func (s *Server) processRequest(req *http.Request) (*http.Response, error) {
	// Check if we can handle this protocol
	handler := s.registry.Detect(req)
	if handler == nil {
		// Passthrough - no protocol handler
		s.logger.Debug().Str("url", req.URL.String()).Msg("Passthrough request (no handler)")
		return http.DefaultTransport.RoundTrip(req)
	}

	s.logger.Debug().
		Str("url", req.URL.String()).
		Str("handler", handler.Name()).
		Msg("Processing request")

	// Read request body
	body, err := io.ReadAll(req.Body)
	req.Body.Close()
	if err != nil {
		return nil, fmt.Errorf("failed to read request body: %w", err)
	}

	// Parse request
	msg, err := handler.ParseRequest(body)
	if err != nil {
		s.logger.Warn().Err(err).Msg("Failed to parse request, passing through")
		// Restore body and passthrough
		req.Body = io.NopCloser(io.NopCloser(io.LimitReader(io.MultiReader(io.NopCloser(io.LimitReader(nil, 0))), 0)))
		return http.DefaultTransport.RoundTrip(req)
	}

	// Process each message for secrets
	modified := false
	for i, m := range msg.Messages {
		// Detect secrets
		secrets := s.interceptors.DetectAll(m.Content)
		if len(secrets) == 0 {
			continue
		}

		modified = true
		s.logger.Info().
			Int("secrets_found", len(secrets)).
			Str("role", m.Role).
			Msg("Detected secrets in message")

		// Replace secrets with placeholders
		content := m.Content
		for _, secret := range secrets {
			ph := s.placeholder.Generate(secret.Value)

			// Store mapping
			if err := s.store.Store(ph, secret.Value); err != nil {
				s.logger.Error().Err(err).Msg("Failed to store mapping")
			}

			// Replace in content
			content = replaceSecret(content, secret, ph)

			// Update metrics
			metrics.RecordSecretDetected(secret.Source, secret.Type)
			metrics.SecretsReplacedTotal.Inc()
		}

		msg.Messages[i].Content = content
	}

	// Serialize back if modified
	if modified {
		body, err = handler.SerializeRequest(msg)
		if err != nil {
			return nil, fmt.Errorf("failed to serialize request: %w", err)
		}
	}

	// Create new request with modified body
	newReq, err := http.NewRequest(req.Method, req.URL.String(), io.NopCloser(newBytesReader(body)))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Copy headers
	newReq.Header = req.Header.Clone()
	newReq.Header.Set("Content-Length", fmt.Sprintf("%d", len(body)))

	// Forward request
	return http.DefaultTransport.RoundTrip(newReq)
}

// processResponse intercepts and modifies incoming responses
func (s *Server) processResponse(resp *http.Response) (*http.Response, error) {
	start := time.Now()
	defer func() {
		metrics.RecordRequestDuration("response", time.Since(start).Seconds())
	}()

	// Check content type
	contentType := resp.Header.Get("Content-Type")

	// Handle streaming responses (SSE)
	if isStreamingResponse(contentType) {
		return s.processStreamingResponse(resp)
	}

	// Handle regular JSON responses
	return s.processJSONResponse(resp)
}

// processJSONResponse handles non-streaming JSON responses
func (s *Server) processJSONResponse(resp *http.Response) (*http.Response, error) {
	// Read response body
	body, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Restore placeholders
	newBody := s.placeholder.RestorePlaceholders(string(body), func(ph string) (string, bool) {
		secret, found := s.store.Lookup(ph)
		if found {
			metrics.PlaceholdersRestored.Inc()
		}
		return secret, found
	})

	// Create new response with restored body
	resp.Body = io.NopCloser(newBytesReader([]byte(newBody)))
	resp.ContentLength = int64(len(newBody))
	resp.Header.Set("Content-Length", fmt.Sprintf("%d", len(newBody)))

	return resp, nil
}

// processStreamingResponse handles SSE streaming responses
func (s *Server) processStreamingResponse(resp *http.Response) (*http.Response, error) {
	// Create a pipe for streaming
	pr, pw := io.Pipe()

	// Start goroutine to process stream
	go func() {
		defer pw.Close()
		defer resp.Body.Close()

		// Buffer for read-ahead
		bufferSize := s.placeholder.MaxLength()
		buffer := make([]byte, 0, bufferSize*2)

		reader := bufio.NewReader(resp.Body)

		for {
			// Read chunk
			chunk, err := reader.ReadBytes('\n')
			if err != nil && err != io.EOF {
				s.logger.Error().Err(err).Msg("Error reading stream")
				return
			}

			if len(chunk) > 0 {
				metrics.StreamingChunksProcessed.Inc()

				// Append to buffer
				buffer = append(buffer, chunk...)

				// Process buffer - keep last bufferSize bytes for potential partial placeholders
				if len(buffer) > bufferSize {
					// Process safe part
					safeLen := len(buffer) - bufferSize
					safePart := string(buffer[:safeLen])

					// Restore placeholders in safe part
					restored := s.placeholder.RestorePlaceholders(safePart, func(ph string) (string, bool) {
						secret, found := s.store.Lookup(ph)
						if found {
							metrics.PlaceholdersRestored.Inc()
						}
						return secret, found
					})

					// Write restored content
					if _, err := pw.Write([]byte(restored)); err != nil {
						s.logger.Error().Err(err).Msg("Error writing to pipe")
						return
					}

					// Keep remaining buffer
					buffer = buffer[safeLen:]
				}
			}

			if err == io.EOF {
				// Flush remaining buffer
				if len(buffer) > 0 {
					restored := s.placeholder.RestorePlaceholders(string(buffer), func(ph string) (string, bool) {
						secret, found := s.store.Lookup(ph)
						if found {
							metrics.PlaceholdersRestored.Inc()
						}
						return secret, found
					})
					pw.Write([]byte(restored))
				}
				return
			}
		}
	}()

	// Create new response with piped body
	newResp := &http.Response{
		Status:        resp.Status,
		StatusCode:    resp.StatusCode,
		Proto:         resp.Proto,
		ProtoMajor:    resp.ProtoMajor,
		ProtoMinor:    resp.ProtoMinor,
		Header:        resp.Header.Clone(),
		Body:          pr,
		ContentLength: -1, // Unknown for streaming
	}

	// Remove Content-Length for streaming
	newResp.Header.Del("Content-Length")

	return newResp, nil
}

// sendErrorResponse sends an HTTP error response
func (s *Server) sendErrorResponse(conn net.Conn, statusCode int, message string) {
	resp := &http.Response{
		StatusCode: statusCode,
		Status:     http.StatusText(statusCode),
		Proto:      "HTTP/1.1",
		ProtoMajor: 1,
		ProtoMinor: 1,
		Header:     make(http.Header),
		Body:       io.NopCloser(newBytesReader([]byte(message))),
	}
	resp.Header.Set("Content-Type", "text/plain")
	resp.Header.Set("Content-Length", fmt.Sprintf("%d", len(message)))
	resp.Write(conn)
}

// UpdateMappingStoreSize updates the mapping store size metric
func (s *Server) UpdateMappingStoreSize() {
	metrics.MappingStoreSize.Set(float64(s.store.Size()))
}

// Helper functions

func isStreamingResponse(contentType string) bool {
	return contentType == "text/event-stream" ||
		contentType == "application/x-ndjson" ||
		contentType == "application/stream+json"
}

func replaceSecret(content string, secret interceptor.DetectedSecret, placeholder string) string {
	// Simple replacement - could be optimized for multiple occurrences
	return content[:secret.StartIndex] + placeholder + content[secret.EndIndex:]
}

type bytesReader struct {
	data []byte
	pos  int
}

func newBytesReader(data []byte) *bytesReader {
	return &bytesReader{data: data}
}

func (r *bytesReader) Read(p []byte) (n int, err error) {
	if r.pos >= len(r.data) {
		return 0, io.EOF
	}
	n = copy(p, r.data[r.pos:])
	r.pos += n
	return n, nil
}
