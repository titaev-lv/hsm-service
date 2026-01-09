package server

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/titaev-lv/hsm-service/internal/config"
)

func TestACLAutoReload(t *testing.T) {
	// Create temp directory
	tmpDir := t.TempDir()
	revokedFile := filepath.Join(tmpDir, "revoked.yaml")

	// Initial content
	initialYAML := `revoked:
  - cn: "test1.example.com"
    serial: "1234"
    reason: "compromised"
    date: "2024-01-01"
`
	if err := os.WriteFile(revokedFile, []byte(initialYAML), 0644); err != nil {
		t.Fatal(err)
	}

	// Create ACL checker with fast reload interval
	cfg := &config.ACLConfig{
		RevokedFile: revokedFile,
		Mappings: map[string][]string{
			"operations": {"user-keys"},
		},
	}

	checker, err := NewACLChecker(cfg)
	if err != nil {
		t.Fatal(err)
	}
	// Override reload interval for fast testing
	checker.reloadInterval = 100 * time.Millisecond
	checker.StartAutoReload()
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		checker.StopAutoReload(ctx)
	}()

	// Verify initial state
	if !checker.IsRevoked("test1.example.com") {
		t.Error("test1.example.com should be revoked initially")
	}
	if checker.IsRevoked("test2.example.com") {
		t.Error("test2.example.com should not be revoked initially")
	}

	// Wait for first check
	time.Sleep(150 * time.Millisecond)

	// Update file with new content
	updatedYAML := `revoked:
  - cn: "test1.example.com"
    serial: "1234"
    reason: "compromised"
    date: "2024-01-01"
  - cn: "test2.example.com"
    serial: "5678"
    reason: "key-compromise"
    date: "2024-01-02"
`
	if err := os.WriteFile(revokedFile, []byte(updatedYAML), 0644); err != nil {
		t.Fatal(err)
	}

	// Wait for auto-reload to detect change
	time.Sleep(250 * time.Millisecond)

	// Verify updated state
	if !checker.IsRevoked("test1.example.com") {
		t.Error("test1.example.com should still be revoked")
	}
	if !checker.IsRevoked("test2.example.com") {
		t.Error("test2.example.com should be revoked after reload")
	}
}

func TestACLReloadInvalidYAML(t *testing.T) {
	// Create temp directory
	tmpDir := t.TempDir()
	revokedFile := filepath.Join(tmpDir, "revoked.yaml")

	// Valid initial content
	validYAML := `revoked:
  - cn: "test1.example.com"
    serial: "1234"
    reason: "compromised"
    date: "2024-01-01"
`
	if err := os.WriteFile(revokedFile, []byte(validYAML), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := &config.ACLConfig{
		RevokedFile: revokedFile,
		Mappings: map[string][]string{
			"operations": {"user-keys"},
		},
	}

	checker, err := NewACLChecker(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		checker.StopAutoReload(ctx)
	}()

	// Verify initial state
	if !checker.IsRevoked("test1.example.com") {
		t.Error("test1.example.com should be revoked")
	}

	// Write invalid YAML
	invalidYAML := `revoked:
  - cn: "test2.example.com"
    serial: "5678"
    reason: "test
    date: invalid [unclosed quote
`
	if err := os.WriteFile(revokedFile, []byte(invalidYAML), 0644); err != nil {
		t.Fatal(err)
	}

	// Try to reload
	err = checker.TryReload()
	if err == nil {
		t.Error("Expected error for invalid YAML")
	}

	// Old data should be preserved
	if !checker.IsRevoked("test1.example.com") {
		t.Error("Old data should be preserved after failed reload")
	}
	if checker.IsRevoked("test2.example.com") {
		t.Error("Invalid data should not be loaded")
	}
}

func TestACLReloadEmptyCN(t *testing.T) {
	tmpDir := t.TempDir()
	revokedFile := filepath.Join(tmpDir, "revoked.yaml")

	validYAML := `revoked:
  - cn: "test1.example.com"
    serial: "1234"
    reason: "compromised"
    date: "2024-01-01"
`
	if err := os.WriteFile(revokedFile, []byte(validYAML), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := &config.ACLConfig{
		RevokedFile: revokedFile,
		Mappings:    map[string][]string{"operations": {"user-keys"}},
	}

	checker, err := NewACLChecker(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		checker.StopAutoReload(ctx)
	}()

	// Write YAML with empty CN
	invalidYAML := `revoked:
  - cn: ""
    serial: "5678"
    reason: "test"
    date: "2024-01-02"
`
	if err := os.WriteFile(revokedFile, []byte(invalidYAML), 0644); err != nil {
		t.Fatal(err)
	}

	// Try to reload - should fail validation
	err = checker.TryReload()
	if err == nil {
		t.Error("Expected error for empty CN")
	}

	// Old data should be preserved
	if !checker.IsRevoked("test1.example.com") {
		t.Error("Old data should be preserved after validation failure")
	}
}

func TestACLReloadDuplicateCN(t *testing.T) {
	tmpDir := t.TempDir()
	revokedFile := filepath.Join(tmpDir, "revoked.yaml")

	validYAML := `revoked:
  - cn: "test1.example.com"
    serial: "1234"
    reason: "compromised"
    date: "2024-01-01"
`
	if err := os.WriteFile(revokedFile, []byte(validYAML), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := &config.ACLConfig{
		RevokedFile: revokedFile,
		Mappings:    map[string][]string{"operations": {"user-keys"}},
	}

	checker, err := NewACLChecker(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		checker.StopAutoReload(ctx)
	}()

	// Write YAML with duplicate CNs
	duplicateYAML := `revoked:
  - cn: "test2.example.com"
    serial: "5678"
    reason: "compromised"
    date: "2024-01-02"
  - cn: "test2.example.com"
    serial: "9999"
    reason: "duplicate"
    date: "2024-01-03"
`
	if err := os.WriteFile(revokedFile, []byte(duplicateYAML), 0644); err != nil {
		t.Fatal(err)
	}

	// Try to reload - should fail validation
	err = checker.TryReload()
	if err == nil {
		t.Error("Expected error for duplicate CN")
	}

	// Old data should be preserved
	if !checker.IsRevoked("test1.example.com") {
		t.Error("Old data should be preserved")
	}
	if checker.IsRevoked("test2.example.com") {
		t.Error("Invalid duplicate data should not be loaded")
	}
}

func TestACLReloadFileDeleted(t *testing.T) {
	tmpDir := t.TempDir()
	revokedFile := filepath.Join(tmpDir, "revoked.yaml")

	validYAML := `revoked:
  - cn: "test1.example.com"
    serial: "1234"
    reason: "compromised"
    date: "2024-01-01"
`
	if err := os.WriteFile(revokedFile, []byte(validYAML), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := &config.ACLConfig{
		RevokedFile: revokedFile,
		Mappings:    map[string][]string{"operations": {"user-keys"}},
	}

	checker, err := NewACLChecker(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		checker.StopAutoReload(ctx)
	}()

	// Verify initial state
	if !checker.IsRevoked("test1.example.com") {
		t.Error("test1.example.com should be revoked initially")
	}

	// Delete file
	if err := os.Remove(revokedFile); err != nil {
		t.Fatal(err)
	}

	// Try to reload
	err = checker.TryReload()
	if err != nil {
		t.Errorf("File deletion should not cause error: %v", err)
	}

	// Revocation list should be cleared
	if checker.IsRevoked("test1.example.com") {
		t.Error("Revocation list should be cleared when file is deleted")
	}
}

func TestACLStopAutoReload(t *testing.T) {
	tmpDir := t.TempDir()
	revokedFile := filepath.Join(tmpDir, "revoked.yaml")

	if err := os.WriteFile(revokedFile, []byte("revoked: []"), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := &config.ACLConfig{
		RevokedFile: revokedFile,
		Mappings:    map[string][]string{"operations": {"user-keys"}},
	}

	checker, err := NewACLChecker(cfg)
	if err != nil {
		t.Fatal(err)
	}

	// Stop should complete without timeout
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err = checker.StopAutoReload(ctx)
	if err != nil {
		t.Errorf("StopAutoReload failed: %v", err)
	}

	// Calling stop again should be safe
	ctx2, cancel2 := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel2()
	err = checker.StopAutoReload(ctx2)
	// Should not hang or panic
}
