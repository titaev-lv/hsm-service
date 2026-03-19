package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCreateKEKCommand_LoadConfigError(t *testing.T) {
	t.Setenv("HSM_PIN", "1234")
	err := createKEKCommand([]string{
		"--label", "kek-a",
		"--context", "exchange-key",
		"--config", "/tmp/does-not-exist-config.yaml",
	})
	if err == nil {
		t.Fatal("createKEKCommand() expected config load error")
	}
	if !strings.Contains(err.Error(), "load config") {
		t.Fatalf("createKEKCommand() error = %v", err)
	}
}

func TestCreateKEKWithConfig_NilConfigPanicRecovered(t *testing.T) {
	err := createKEKWithConfig(nil, "1234", "kek-a", "exchange-key", 1, 256)
	if err == nil {
		t.Fatal("createKEKWithConfig() expected recovered panic error")
	}
	if !strings.Contains(err.Error(), "pkcs11 initialize panic") {
		t.Fatalf("createKEKWithConfig() error = %v", err)
	}
}

func TestFindSlotByLabel_NilContextPanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("findSlotByLabel() expected panic on nil context")
		}
	}()

	_, _ = findSlotByLabel(nil, "slot-0")
}

func TestRotateKeyCommand_InvalidMetadataPathRejected(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")

	configYAML := "server:\n" +
		"  port: \"8443\"\n" +
		"  tls:\n" +
		"    cert_path: \"/tmp/cert.crt\"\n" +
		"    key_path: \"/tmp/cert.key\"\n" +
		"    ca_path: \"/tmp/ca.crt\"\n" +
		"hsm:\n" +
		"  pkcs11_lib: \"/tmp/fake-pkcs11.so\"\n" +
		"  slot_id: \"slot-0\"\n" +
		"  metadata_file: \"../metadata.yaml\"\n" +
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

	t.Setenv("CONFIG_PATH", configPath)
	err := rotateKeyCommand([]string{"exchange-key"})
	if err == nil {
		t.Fatal("rotateKeyCommand() expected invalid metadata path error")
	}
	if !strings.Contains(err.Error(), "invalid metadata path") {
		t.Fatalf("rotateKeyCommand() error = %v", err)
	}
}

func TestRotateKeyCommand_LockFileCreateError(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")

	// Parent directory does not exist, so creating lock file must fail.
	missingMetadataPath := filepath.Join(dir, "missing", "metadata.yaml")
	configYAML := "server:\n" +
		"  port: \"8443\"\n" +
		"  tls:\n" +
		"    cert_path: \"/tmp/cert.crt\"\n" +
		"    key_path: \"/tmp/cert.key\"\n" +
		"    ca_path: \"/tmp/ca.crt\"\n" +
		"hsm:\n" +
		"  pkcs11_lib: \"/tmp/fake-pkcs11.so\"\n" +
		"  slot_id: \"slot-0\"\n" +
		"  metadata_file: \"" + missingMetadataPath + "\"\n" +
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

	t.Setenv("CONFIG_PATH", configPath)
	err := rotateKeyCommand([]string{"exchange-key"})
	if err == nil {
		t.Fatal("rotateKeyCommand() expected lock file creation error")
	}
	if !strings.Contains(err.Error(), "failed to create lock file") {
		t.Fatalf("rotateKeyCommand() error = %v", err)
	}
}
