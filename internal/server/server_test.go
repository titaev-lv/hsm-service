package server

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/titaev-lv/hsm-service/internal/config"
)

func buildServerConfig(certPath, keyPath, caPath string) *config.ServerConfig {
	return &config.ServerConfig{
		Port: 0,
		TLS: config.TLSConfig{
			CertPath: certPath,
			KeyPath:  keyPath,
			CAPath:   caPath,
		},
		Timeouts: config.TimeoutConfig{
			Read:       3 * time.Second,
			Write:      3 * time.Second,
			Idle:       10 * time.Second,
			ReadHeader: 2 * time.Second,
		},
		Limits: config.LimitsConfig{MaxHeaderBytes: 1 << 20},
	}
}

func writePEMFile(t *testing.T, path, blockType string, der []byte) {
	t.Helper()
	data := pem.EncodeToMemory(&pem.Block{Type: blockType, Bytes: der})
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func createTestTLSFiles(t *testing.T, dir string) (certPath, keyPath, caPath string) {
	t.Helper()

	caKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate CA key: %v", err)
	}

	now := time.Now()
	caTmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "test-ca"},
		NotBefore:             now.Add(-time.Hour),
		NotAfter:              now.Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		IsCA:                  true,
		BasicConstraintsValid: true,
	}

	caDER, err := x509.CreateCertificate(rand.Reader, caTmpl, caTmpl, &caKey.PublicKey, caKey)
	if err != nil {
		t.Fatalf("create CA cert: %v", err)
	}

	serverKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate server key: %v", err)
	}

	serverTmpl := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject:      pkix.Name{CommonName: "localhost"},
		DNSNames:     []string{"localhost"},
		NotBefore:    now.Add(-time.Hour),
		NotAfter:     now.Add(24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}

	serverDER, err := x509.CreateCertificate(rand.Reader, serverTmpl, caTmpl, &serverKey.PublicKey, caKey)
	if err != nil {
		t.Fatalf("create server cert: %v", err)
	}

	certPath = filepath.Join(dir, "server.crt")
	keyPath = filepath.Join(dir, "server.key")
	caPath = filepath.Join(dir, "ca.crt")

	writePEMFile(t, certPath, "CERTIFICATE", serverDER)
	writePEMFile(t, keyPath, "RSA PRIVATE KEY", x509.MarshalPKCS1PrivateKey(serverKey))
	writePEMFile(t, caPath, "CERTIFICATE", caDER)

	return certPath, keyPath, caPath
}

func TestNewServer_ErrorsAndSuccess(t *testing.T) {
	t.Run("cert load failure", func(t *testing.T) {
		cfg := buildServerConfig("/no/such/server.crt", "/no/such/server.key", "/no/such/ca.crt")
		srv, err := NewServer(cfg, nil, nil, NewRateLimiter(10, 20))
		if err == nil || !strings.Contains(err.Error(), "failed to load server certificate") {
			t.Fatalf("expected cert load error, got: %v", err)
		}
		if srv != nil {
			t.Fatalf("expected nil server on error")
		}
	})

	t.Run("CA read failure", func(t *testing.T) {
		dir := t.TempDir()
		certPath, keyPath, _ := createTestTLSFiles(t, dir)
		cfg := buildServerConfig(certPath, keyPath, filepath.Join(dir, "missing-ca.crt"))

		srv, err := NewServer(cfg, nil, nil, NewRateLimiter(10, 20))
		if err == nil || !strings.Contains(err.Error(), "failed to read CA certificate") {
			t.Fatalf("expected CA read error, got: %v", err)
		}
		if srv != nil {
			t.Fatalf("expected nil server on error")
		}
	})

	t.Run("CA parse failure", func(t *testing.T) {
		dir := t.TempDir()
		certPath, keyPath, _ := createTestTLSFiles(t, dir)
		badCA := filepath.Join(dir, "bad-ca.crt")
		if err := os.WriteFile(badCA, []byte("not a certificate"), 0600); err != nil {
			t.Fatalf("write bad CA: %v", err)
		}
		cfg := buildServerConfig(certPath, keyPath, badCA)

		srv, err := NewServer(cfg, nil, nil, NewRateLimiter(10, 20))
		if err == nil || !strings.Contains(err.Error(), "failed to parse CA certificate") {
			t.Fatalf("expected CA parse error, got: %v", err)
		}
		if srv != nil {
			t.Fatalf("expected nil server on error")
		}
	})

	t.Run("HTTP2 parse failure", func(t *testing.T) {
		dir := t.TempDir()
		certPath, keyPath, caPath := createTestTLSFiles(t, dir)
		cfg := buildServerConfig(certPath, keyPath, caPath)
		cfg.HTTP2 = &config.HTTP2Config{MaxConcurrentStreams: "bad"}

		srv, err := NewServer(cfg, nil, nil, NewRateLimiter(10, 20))
		if err == nil || !strings.Contains(err.Error(), "failed to parse HTTP/2 config") {
			t.Fatalf("expected HTTP2 parse error, got: %v", err)
		}
		if srv != nil {
			t.Fatalf("expected nil server on error")
		}
	})

	t.Run("success with valid TLS and no HTTP2 override", func(t *testing.T) {
		dir := t.TempDir()
		certPath, keyPath, caPath := createTestTLSFiles(t, dir)
		cfg := buildServerConfig(certPath, keyPath, caPath)

		srv, err := NewServer(cfg, nil, nil, NewRateLimiter(10, 20))
		if err != nil {
			t.Fatalf("expected success, got error: %v", err)
		}
		if srv == nil || srv.httpServer == nil {
			t.Fatalf("expected initialized server")
		}
	})
}

func TestServer_StartAndShutdown(t *testing.T) {
	t.Run("start returns error without TLS config", func(t *testing.T) {
		s := &Server{httpServer: &http.Server{Addr: "127.0.0.1:0"}}
		if err := s.Start(); err == nil {
			t.Fatalf("expected start error")
		}
	})

	t.Run("shutdown on non-started server", func(t *testing.T) {
		s := &Server{httpServer: &http.Server{Addr: "127.0.0.1:0"}}
		err := s.Shutdown(context.Background())
		if err != nil && err != http.ErrServerClosed {
			t.Fatalf("expected nil or ErrServerClosed, got: %v", err)
		}
	})
}
