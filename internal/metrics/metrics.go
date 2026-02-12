// Package metrics provides Prometheus metrics for monitoring the LLM Secret Interceptor proxy.
package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// RequestsTotal counts total processed requests
	RequestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "llm_proxy_requests_total",
		Help: "Total number of requests processed by the proxy",
	}, []string{"method", "host"})

	// SecretsDetectedTotal counts detected secrets by interceptor
	SecretsDetectedTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "llm_proxy_secrets_detected_total",
		Help: "Total number of secrets detected",
	}, []string{"interceptor", "type"})

	// SecretsReplacedTotal counts replaced secrets
	SecretsReplacedTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "llm_proxy_secrets_replaced_total",
		Help: "Total number of secrets replaced with placeholders",
	})

	// MappingStoreSize tracks the size of the mapping store
	MappingStoreSize = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "llm_proxy_mapping_store_size",
		Help: "Current number of secret mappings stored",
	})

	// RequestDuration tracks request processing latency
	RequestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "llm_proxy_request_duration_seconds",
		Help:    "Request processing duration in seconds",
		Buckets: prometheus.DefBuckets,
	}, []string{"direction"}) // "request" or "response"

	// StreamingChunksProcessed counts processed streaming chunks
	StreamingChunksProcessed = promauto.NewCounter(prometheus.CounterOpts{
		Name: "llm_proxy_streaming_chunks_processed_total",
		Help: "Total number of streaming chunks processed",
	})

	// PlaceholdersRestored counts restored placeholders in responses
	PlaceholdersRestored = promauto.NewCounter(prometheus.CounterOpts{
		Name: "llm_proxy_placeholders_restored_total",
		Help: "Total number of placeholders restored to secrets in responses",
	})

	// ActiveConnections tracks current active connections
	ActiveConnections = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "llm_proxy_active_connections",
		Help: "Current number of active proxy connections",
	})

	// TLSErrors counts TLS-related errors
	TLSErrors = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "llm_proxy_tls_errors_total",
		Help: "Total number of TLS errors",
	}, []string{"type"})

	// UpstreamErrors counts errors from upstream servers
	UpstreamErrors = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "llm_proxy_upstream_errors_total",
		Help: "Total number of upstream connection errors",
	}, []string{"host", "type"})

	// BytesTransferred tracks bytes transferred
	BytesTransferred = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "llm_proxy_bytes_transferred_total",
		Help: "Total bytes transferred through the proxy",
	}, []string{"direction"}) // "request" or "response"

	// InterceptorDuration tracks interceptor processing time
	InterceptorDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "llm_proxy_interceptor_duration_seconds",
		Help:    "Time spent in secret detection",
		Buckets: []float64{0.0001, 0.0005, 0.001, 0.005, 0.01, 0.05, 0.1},
	}, []string{"interceptor"})

	// MappingCleanups counts mapping store cleanup operations
	MappingCleanups = promauto.NewCounter(prometheus.CounterOpts{
		Name: "llm_proxy_mapping_cleanups_total",
		Help: "Total number of mapping store cleanup operations",
	})

	// MappingsExpired counts expired mappings
	MappingsExpired = promauto.NewCounter(prometheus.CounterOpts{
		Name: "llm_proxy_mappings_expired_total",
		Help: "Total number of mappings expired and removed",
	})
)

// RecordSecretDetected records a detected secret
func RecordSecretDetected(interceptor, secretType string) {
	SecretsDetectedTotal.WithLabelValues(interceptor, secretType).Inc()
}

// RecordRequestDuration records request processing duration
func RecordRequestDuration(direction string, seconds float64) {
	RequestDuration.WithLabelValues(direction).Observe(seconds)
}

// RecordRequest records a processed request
func RecordRequest(method, host string) {
	RequestsTotal.WithLabelValues(method, host).Inc()
}

// RecordTLSError records a TLS error
func RecordTLSError(errorType string) {
	TLSErrors.WithLabelValues(errorType).Inc()
}

// RecordUpstreamError records an upstream error
func RecordUpstreamError(host, errorType string) {
	UpstreamErrors.WithLabelValues(host, errorType).Inc()
}

// RecordBytesTransferred records bytes transferred
func RecordBytesTransferred(direction string, bytes int64) {
	BytesTransferred.WithLabelValues(direction).Add(float64(bytes))
}

// RecordInterceptorDuration records interceptor processing time
func RecordInterceptorDuration(interceptor string, seconds float64) {
	InterceptorDuration.WithLabelValues(interceptor).Observe(seconds)
}
