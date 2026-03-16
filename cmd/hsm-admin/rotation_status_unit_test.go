package main

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func writeStatusTestConfig(t *testing.T, configPath, metadataPath string) {
	t.Helper()
	configYAML := "server:\n" +
		"  port: \"8443\"\n" +
		"  tls:\n" +
		"    cert_path: \"/tmp/cert.crt\"\n" +
		"    key_path: \"/tmp/cert.key\"\n" +
		"    ca_path: \"/tmp/ca.crt\"\n" +
		"hsm:\n" +
		"  pkcs11_lib: \"/tmp/fake-pkcs11.so\"\n" +
		"  slot_id: \"slot-0\"\n" +
		"  metadata_file: \"" + metadataPath + "\"\n" +
		"  keys:\n" +
		"    exchange-key:\n" +
		"      type: \"aes\"\n" +
		"acl:\n" +
		"  mappings:\n" +
		"    Trading:\n" +
		"      - exchange-key\n"

	if err := os.WriteFile(configPath, []byte(configYAML), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
}

func captureStdoutFromStatusCall(t *testing.T) (string, error) {
	t.Helper()

	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	os.Stdout = w

	callErr := checkRotationStatusCommand()

	_ = w.Close()
	os.Stdout = oldStdout

	out, readErr := io.ReadAll(r)
	_ = r.Close()
	if readErr != nil {
		t.Fatalf("io.ReadAll: %v", readErr)
	}

	return string(out), callErr
}

func TestCheckRotationStatusCommand_CurrentVersionMissing_PrintsWarning(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	metadataPath := filepath.Join(dir, "metadata.yaml")

	writeStatusTestConfig(t, configPath, metadataPath)
	metadataYAML := "rotation:\n" +
		"  exchange-key:\n" +
		"    current: \"kek-exchange-v2\"\n" +
		"    versions:\n" +
		"      - label: \"kek-exchange-v1\"\n" +
		"        version: 1\n"
	if err := os.WriteFile(metadataPath, []byte(metadataYAML), 0o600); err != nil {
		t.Fatalf("write metadata: %v", err)
	}

	t.Setenv("CONFIG_PATH", configPath)
	out, err := captureStdoutFromStatusCall(t)
	if err != nil {
		t.Fatalf("checkRotationStatusCommand() error: %v", err)
	}
	if !strings.Contains(out, "Context: exchange-key") {
		t.Fatalf("output missing context, got: %s", out)
	}
	if !strings.Contains(out, "current version not found") {
		t.Fatalf("output missing warning, got: %s", out)
	}
}

func TestCheckRotationStatusCommand_DefaultIntervalIs90Days(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	metadataPath := filepath.Join(dir, "metadata.yaml")

	writeStatusTestConfig(t, configPath, metadataPath)
	now := time.Now().UTC().Format(time.RFC3339)
	metadataYAML := "rotation:\n" +
		"  exchange-key:\n" +
		"    current: \"kek-exchange-v1\"\n" +
		"    versions:\n" +
		"      - label: \"kek-exchange-v1\"\n" +
		"        version: 1\n" +
		"        created_at: \"" + now + "\"\n"
	if err := os.WriteFile(metadataPath, []byte(metadataYAML), 0o600); err != nil {
		t.Fatalf("write metadata: %v", err)
	}

	t.Setenv("CONFIG_PATH", configPath)
	out, err := captureStdoutFromStatusCall(t)
	if err != nil {
		t.Fatalf("checkRotationStatusCommand() error: %v", err)
	}
	if !strings.Contains(out, "Rotation Interval: 2160h0m0s") {
		t.Fatalf("output missing default 90-day interval, got: %s", out)
	}
	if !strings.Contains(out, "Current:           kek-exchange-v1") {
		t.Fatalf("output missing current key line, got: %s", out)
	}
}
