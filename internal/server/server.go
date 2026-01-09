package server

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/titaev-lv/hsm-service/internal/config"
	"github.com/titaev-lv/hsm-service/internal/hsm"
)

// Server represents the HSM HTTP server
type Server struct {
	httpServer  *http.Server
	hsmCtx      *hsm.HSMContext
	aclChecker  *ACLChecker
	rateLimiter *RateLimiter
	config      *config.ServerConfig
}

// NewServer creates a new HSM server with TLS and mTLS configuration
func NewServer(cfg *config.ServerConfig, hsmCtx *hsm.HSMContext, aclChecker *ACLChecker, rateLimiter *RateLimiter) (*Server, error) {
	// 1. Load server certificate
	serverCert, err := tls.LoadX509KeyPair(
		cfg.TLS.CertPath,
		cfg.TLS.KeyPath,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to load server certificate: %w", err)
	}

	// 2. Load CA for client verification
	caCert, err := os.ReadFile(cfg.TLS.CAPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read CA certificate: %w", err)
	}
	caCertPool := x509.NewCertPool()
	if !caCertPool.AppendCertsFromPEM(caCert) {
		return nil, fmt.Errorf("failed to parse CA certificate")
	}

	// 3. TLS Config with mTLS
	// Security: TLS 1.3 only (no TLS 1.2 fallback)
	// Rationale:
	//   - TLS 1.3 removes weak algorithms (RC4, 3DES, MD5, SHA-1)
	//   - Mandatory perfect forward secrecy (PFS)
	//   - Encrypted handshake (protects metadata)
	//   - Simplified cipher suite selection
	//   - PCI DSS 4.0 strongly recommends TLS 1.3+
	// Trade-off: Clients MUST support TLS 1.3 (all modern clients do since 2018)
	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{serverCert},
		ClientAuth:   tls.RequireAndVerifyClientCert,
		ClientCAs:    caCertPool,
		MinVersion:   tls.VersionTLS13, // No TLS 1.2 fallback - intentional security decision
		CipherSuites: []uint16{
			// TLS 1.3 cipher suites (only these two are needed)
			// See RFC 8446 - TLS 1.3 defines only 5 cipher suites, these 2 cover 99.9% of clients
			tls.TLS_AES_256_GCM_SHA384,       // AES-256-GCM with SHA-384 (primary, hardware accelerated on x86/AMD64)
			tls.TLS_CHACHA20_POLY1305_SHA256, // ChaCha20-Poly1305 (faster on ARM/mobile without AES-NI)
		},
	}

	// 4. Create HTTP router
	mux := http.NewServeMux()

	// Register endpoints
	mux.HandleFunc("/encrypt", EncryptHandler(hsmCtx, aclChecker))
	mux.HandleFunc("/decrypt", DecryptHandler(hsmCtx, aclChecker))
	mux.HandleFunc("/health", HealthHandler(hsmCtx))

	// Register Prometheus metrics endpoint (A09:2021 monitoring requirement)
	mux.Handle("/metrics", promhttp.Handler())

	// 5. Apply middleware stack (rate limit -> audit -> recovery -> request log)
	handler := RateLimitMiddleware(rateLimiter)(
		RecoveryMiddleware(
			AuditLogMiddleware(
				RequestLogMiddleware(mux),
			),
		),
	)

	// 6. Create HTTP server
	httpServer := &http.Server{
		Addr:      ":" + cfg.Port,
		Handler:   handler,
		TLSConfig: tlsConfig,
		// Timeout protection against Slowloris attacks
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
		MaxHeaderBytes:    1 << 20, // 1 MB
	}

	return &Server{
		httpServer:  httpServer,
		hsmCtx:      hsmCtx,
		aclChecker:  aclChecker,
		rateLimiter: rateLimiter,
		config:      cfg,
	}, nil
}

// Start starts the HTTPS server
func (s *Server) Start() error {
	// Server will use certificates from TLSConfig
	return s.httpServer.ListenAndServeTLS("", "")
}

// Shutdown gracefully shuts down the server
func (s *Server) Shutdown() error {
	// Close HSM context
	if s.hsmCtx != nil {
		if err := s.hsmCtx.Close(); err != nil {
			return fmt.Errorf("failed to close HSM context: %w", err)
		}
	}
	return nil
}
