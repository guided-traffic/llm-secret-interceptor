package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// RequestsTotal counts total processed requests
	RequestsTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "llm_proxy_requests_total",
		Help: "Total number of requests processed by the proxy",
	})

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
)

// RecordSecretDetected records a detected secret
func RecordSecretDetected(interceptor, secretType string) {
	SecretsDetectedTotal.WithLabelValues(interceptor, secretType).Inc()
}

// RecordRequestDuration records request processing duration
func RecordRequestDuration(direction string, seconds float64) {
	RequestDuration.WithLabelValues(direction).Observe(seconds)
}
