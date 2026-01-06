package server

import (
"crypto/rand"
"crypto/rsa"
"crypto/x509"
"crypto/x509/pkix"
"math/big"
"os"
"path/filepath"
"testing"
"time"

"github.com/titaev-lv/hsm-service/internal/config"
)

// Helper function to create a test certificate
func createTestCert(cn string, ou string) *x509.Certificate {
privateKey, _ := rsa.GenerateKey(rand.Reader, 2048)

template := &x509.Certificate{
SerialNumber: big.NewInt(1),
Subject: pkix.Name{
CommonName:         cn,
OrganizationalUnit: []string{ou},
},
NotBefore: time.Now(),
NotAfter:  time.Now().Add(24 * time.Hour),
}

certDER, _ := x509.CreateCertificate(rand.Reader, template, template, &privateKey.PublicKey, privateKey)
cert, _ := x509.ParseCertificate(certDER)

return cert
}

func TestNewACLChecker(t *testing.T) {
// Create temporary revoked.yaml
tmpDir := t.TempDir()
revokedFile := filepath.Join(tmpDir, "revoked.yaml")

revokedYAML := `revoked:
  - cn: revoked-service
    serial: "123456"
    reason: compromised
    date: "2026-01-01"
`
if err := os.WriteFile(revokedFile, []byte(revokedYAML), 0644); err != nil {
t.Fatal(err)
}

cfg := &config.ACLConfig{
RevokedFile: revokedFile,
Mappings: map[string][]string{
"Trading": {"exchange-key"},
"2FA":     {"2fa"},
},
}

checker, err := NewACLChecker(cfg)
if err != nil {
t.Fatalf("NewACLChecker failed: %v", err)
}

if !checker.IsRevoked("revoked-service") {
t.Error("revoked-service should be revoked")
}

if checker.IsRevoked("valid-service") {
t.Error("valid-service should not be revoked")
}
}

func TestLoadRevoked_EmptyFile(t *testing.T) {
tmpDir := t.TempDir()
revokedFile := filepath.Join(tmpDir, "revoked.yaml")

// Empty revoked list
revokedYAML := `revoked: []`
if err := os.WriteFile(revokedFile, []byte(revokedYAML), 0644); err != nil {
t.Fatal(err)
}

cfg := &config.ACLConfig{
RevokedFile: revokedFile,
Mappings:    map[string][]string{},
}

checker, err := NewACLChecker(cfg)
if err != nil {
t.Fatalf("NewACLChecker failed: %v", err)
}

if checker.IsRevoked("any-service") {
t.Error("no service should be revoked")
}
}

func TestLoadRevoked_FileNotExist(t *testing.T) {
tmpDir := t.TempDir()
revokedFile := filepath.Join(tmpDir, "nonexistent.yaml")

cfg := &config.ACLConfig{
RevokedFile: revokedFile,
Mappings:    map[string][]string{},
}

// Should not fail if file doesn't exist
checker, err := NewACLChecker(cfg)
if err != nil {
t.Fatalf("NewACLChecker should not fail on missing file: %v", err)
}

if checker.IsRevoked("any-service") {
t.Error("no service should be revoked")
}
}

func TestCheckAccess_ValidAccess(t *testing.T) {
tmpDir := t.TempDir()
revokedFile := filepath.Join(tmpDir, "revoked.yaml")
os.WriteFile(revokedFile, []byte("revoked: []"), 0644)

cfg := &config.ACLConfig{
RevokedFile: revokedFile,
Mappings: map[string][]string{
"Trading": {"exchange-key", "settlement-key"},
"2FA":     {"2fa"},
},
}

checker, _ := NewACLChecker(cfg)

// Trading OU should have access to exchange-key
cert := createTestCert("trading-service-1", "Trading")
err := checker.CheckAccess(cert, "exchange-key")
if err != nil {
t.Errorf("CheckAccess should allow Trading OU to access exchange-key: %v", err)
}
}

func TestCheckAccess_ForbiddenContext(t *testing.T) {
tmpDir := t.TempDir()
revokedFile := filepath.Join(tmpDir, "revoked.yaml")
os.WriteFile(revokedFile, []byte("revoked: []"), 0644)

cfg := &config.ACLConfig{
RevokedFile: revokedFile,
Mappings: map[string][]string{
"Trading": {"exchange-key"},
"2FA":     {"2fa"},
},
}

checker, _ := NewACLChecker(cfg)

// Trading OU should NOT have access to 2fa context
cert := createTestCert("trading-service-1", "Trading")
err := checker.CheckAccess(cert, "2fa")
if err == nil {
t.Error("CheckAccess should deny Trading OU access to 2fa context")
}
}

func TestCheckAccess_RevokedCertificate(t *testing.T) {
tmpDir := t.TempDir()
revokedFile := filepath.Join(tmpDir, "revoked.yaml")

revokedYAML := `revoked:
  - cn: revoked-service
    serial: "123"
    reason: compromised
    date: "2026-01-01"
`
os.WriteFile(revokedFile, []byte(revokedYAML), 0644)

cfg := &config.ACLConfig{
RevokedFile: revokedFile,
Mappings: map[string][]string{
"Trading": {"exchange-key"},
},
}

checker, _ := NewACLChecker(cfg)

// Revoked certificate should be denied
cert := createTestCert("revoked-service", "Trading")
err := checker.CheckAccess(cert, "exchange-key")
if err == nil {
t.Error("CheckAccess should deny revoked certificate")
}
}

func TestCheckAccess_NoOU(t *testing.T) {
tmpDir := t.TempDir()
revokedFile := filepath.Join(tmpDir, "revoked.yaml")
os.WriteFile(revokedFile, []byte("revoked: []"), 0644)

cfg := &config.ACLConfig{
RevokedFile: revokedFile,
Mappings:    map[string][]string{},
}

checker, _ := NewACLChecker(cfg)

// Certificate without OU should be denied
cert := createTestCert("no-ou-service", "")
cert.Subject.OrganizationalUnit = []string{} // Remove OU

err := checker.CheckAccess(cert, "exchange-key")
if err == nil {
t.Error("CheckAccess should deny certificate without OU")
}
}

func TestCheckAccess_UnknownOU(t *testing.T) {
tmpDir := t.TempDir()
revokedFile := filepath.Join(tmpDir, "revoked.yaml")
os.WriteFile(revokedFile, []byte("revoked: []"), 0644)

cfg := &config.ACLConfig{
RevokedFile: revokedFile,
Mappings: map[string][]string{
"Trading": {"exchange-key"},
},
}

checker, _ := NewACLChecker(cfg)

// Unknown OU should be denied
cert := createTestCert("unknown-service", "UnknownOU")
err := checker.CheckAccess(cert, "exchange-key")
if err == nil {
t.Error("CheckAccess should deny unknown OU")
}
}
