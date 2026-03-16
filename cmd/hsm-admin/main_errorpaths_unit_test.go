package main

import (
	"strings"
	"testing"

	"github.com/titaev-lv/hsm-service/internal/config"
)

func TestCreateKEKCommand_ConfigPathTraversalRejected(t *testing.T) {
	t.Setenv("HSM_PIN", "1234")
	err := createKEKCommand([]string{
		"--label", "kek-a",
		"--context", "exchange-key",
		"--config", "../bad-config.yaml",
	})
	if err == nil {
		t.Fatal("createKEKCommand() expected config path validation error")
	}
	if !strings.Contains(err.Error(), "load config") || !strings.Contains(err.Error(), "invalid config path") {
		t.Fatalf("createKEKCommand() error = %v", err)
	}
}

func TestCreateKEKCommand_ParseFlagError(t *testing.T) {
	err := createKEKCommand([]string{"--size", "not-int"})
	if err == nil {
		t.Fatal("createKEKCommand() expected parse flags error")
	}
	if !strings.Contains(err.Error(), "parse flags") {
		t.Fatalf("createKEKCommand() error = %v", err)
	}
}

func TestRotateKeyCommand_ConfigPathTraversalRejected(t *testing.T) {
	t.Setenv("CONFIG_PATH", "../bad-config.yaml")
	err := rotateKeyCommand([]string{"exchange-key"})
	if err == nil {
		t.Fatal("rotateKeyCommand() expected config path validation error")
	}
	if !strings.Contains(err.Error(), "failed to load config") || !strings.Contains(err.Error(), "invalid config path") {
		t.Fatalf("rotateKeyCommand() error = %v", err)
	}
}

func TestCheckRotationStatusCommand_ConfigPathTraversalRejected(t *testing.T) {
	t.Setenv("CONFIG_PATH", "../bad-config.yaml")
	err := checkRotationStatusCommand()
	if err == nil {
		t.Fatal("checkRotationStatusCommand() expected config path validation error")
	}
	if !strings.Contains(err.Error(), "failed to load config") || !strings.Contains(err.Error(), "invalid config path") {
		t.Fatalf("checkRotationStatusCommand() error = %v", err)
	}
}

func TestCleanupOldVersionsCommand_ConfigPathTraversalRejected(t *testing.T) {
	t.Setenv("HSM_PIN", "1234")
	err := cleanupOldVersionsCommand([]string{"--config", "../bad-config.yaml"})
	if err == nil {
		t.Fatal("cleanupOldVersionsCommand() expected config path validation error")
	}
	if !strings.Contains(err.Error(), "failed to load config") || !strings.Contains(err.Error(), "invalid config path") {
		t.Fatalf("cleanupOldVersionsCommand() error = %v", err)
	}
}

func TestUpdateChecksumsCommand_ConfigPathTraversalRejected(t *testing.T) {
	t.Setenv("HSM_PIN", "1234")
	err := updateChecksumsCommand([]string{"--config", "../bad-config.yaml"})
	if err == nil {
		t.Fatal("updateChecksumsCommand() expected config path validation error")
	}
	if !strings.Contains(err.Error(), "failed to load config") || !strings.Contains(err.Error(), "invalid config path") {
		t.Fatalf("updateChecksumsCommand() error = %v", err)
	}
}

func TestCreateKEKWithConfig_InvalidModuleReturnsError(t *testing.T) {
	cfg := &config.Config{
		HSM: config.HSMConfig{
			PKCS11Lib: "/definitely/nonexistent/pkcs11.so",
			SlotID:    "0",
		},
	}

	err := createKEKWithConfig(cfg, "1234", "kek-a", "exchange-key", 1, 256)
	if err == nil {
		t.Fatal("createKEKWithConfig() expected error on invalid module path")
	}
	if !strings.Contains(err.Error(), "pkcs11 initialize") {
		t.Fatalf("createKEKWithConfig() error = %v", err)
	}
}
