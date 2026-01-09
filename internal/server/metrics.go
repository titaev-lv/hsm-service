package server

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Prometheus metrics for monitoring and alerting (A09:2021 compliance)
var (
	// Request counters by endpoint, client, and status
	RequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "hsm_requests_total",
			Help: "Total number of HSM requests by endpoint, client CN, and status",
		},
		[]string{"endpoint", "client_cn", "status"},
	)

	// ACL failures counter (security monitoring)
	ACLFailuresTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "hsm_acl_failures_total",
			Help: "Total number of ACL check failures (potential security incidents)",
		},
	)

	// Certificate revocation check failures
	RevocationFailuresTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "hsm_revocation_failures_total",
			Help: "Total number of certificate revocation check failures",
		},
	)

	// Encryption/Decryption operation counters
	EncryptOpsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "hsm_encrypt_operations_total",
			Help: "Total number of encryption operations by context and status",
		},
		[]string{"context", "status"},
	)

	DecryptOpsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "hsm_decrypt_operations_total",
			Help: "Total number of decryption operations by context and status",
		},
		[]string{"context", "status"},
	)

	// Request duration histogram
	RequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "hsm_request_duration_seconds",
			Help:    "Request duration in seconds by endpoint",
			Buckets: prometheus.DefBuckets, // 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10
		},
		[]string{"endpoint"},
	)

	// Rate limiter hits
	RateLimitHitsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "hsm_rate_limit_hits_total",
			Help: "Total number of rate limit hits by client CN",
		},
		[]string{"client_cn"},
	)

	// HSM errors (crypto failures)
	HSMErrorsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "hsm_errors_total",
			Help: "Total number of HSM errors by operation type",
		},
		[]string{"operation"},
	)

	// Active connections gauge
	ActiveConnections = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "hsm_active_connections",
			Help: "Current number of active connections",
		},
	)
)

// Helper functions for common metric operations

// RecordRequest records a request with endpoint, client, and status
func RecordRequest(endpoint, clientCN, status string) {
	RequestsTotal.WithLabelValues(endpoint, clientCN, status).Inc()
}

// RecordACLFailure records an ACL check failure
func RecordACLFailure() {
	ACLFailuresTotal.Inc()
}

// RecordRevocationFailure records a certificate revocation failure
func RecordRevocationFailure() {
	RevocationFailuresTotal.Inc()
}

// RecordEncryptOp records an encryption operation
func RecordEncryptOp(context, status string) {
	EncryptOpsTotal.WithLabelValues(context, status).Inc()
}

// RecordDecryptOp records a decryption operation
func RecordDecryptOp(context, status string) {
	DecryptOpsTotal.WithLabelValues(context, status).Inc()
}

// RecordRateLimitHit records a rate limit hit
func RecordRateLimitHit(clientCN string) {
	RateLimitHitsTotal.WithLabelValues(clientCN).Inc()
}

// RecordHSMError records an HSM error
func RecordHSMError(operation string) {
	HSMErrorsTotal.WithLabelValues(operation).Inc()
}
