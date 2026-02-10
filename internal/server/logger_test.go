package server

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/titaev-lv/hsm-service/internal/config"
)

func TestInitLogger(t *testing.T) {
	tests := []struct {
		name   string
		config *config.LoggingConfig
	}{
		{
			name: "json format",
			config: &config.LoggingConfig{
				Level:       "info",
				Format:      "json",
				MaxSizeMB:   1,
				MaxBackups:  1,
				MaxAgeDays:  1,
				Compress:    boolPtr(true),
				AuditToStdout:            boolPtr(true),
				AuditMirrorToErrorOnDebug: boolPtr(true),
			},
		},
		{
			name: "text format",
			config: &config.LoggingConfig{
				Level:       "debug",
				Format:      "text",
				MaxSizeMB:   1,
				MaxBackups:  1,
				MaxAgeDays:  1,
				Compress:    boolPtr(true),
				AuditToStdout:            boolPtr(true),
				AuditMirrorToErrorOnDebug: boolPtr(true),
			},
		},
		{
			name: "default level",
			config: &config.LoggingConfig{
				Level:       "unknown",
				Format:      "json",
				MaxSizeMB:   1,
				MaxBackups:  1,
				MaxAgeDays:  1,
				Compress:    boolPtr(true),
				AuditToStdout:            boolPtr(true),
				AuditMirrorToErrorOnDebug: boolPtr(true),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			tt.config.ErrorPath = filepath.Join(dir, "error.log")
			tt.config.AuditPath = filepath.Join(dir, "audit.log")

			err := InitLogger(tt.config)
			if err != nil {
				t.Errorf("InitLogger() error = %v", err)
			}
		})
	}
}

func TestValidateLogPaths(t *testing.T) {
	dir := t.TempDir()
	cfg := &config.LoggingConfig{
		ErrorPath: filepath.Join(dir, "error.log"),
		AuditPath: filepath.Join(dir, "audit.log"),
	}

	if err := ValidateLogPaths(cfg); err != nil {
		t.Fatalf("ValidateLogPaths() error = %v", err)
	}
}

func TestValidateLogPathsReadOnlyDir(t *testing.T) {
	dir := t.TempDir()
	if err := os.Chmod(dir, 0500); err != nil {
		t.Skipf("chmod not supported: %v", err)
	}

	cfg := &config.LoggingConfig{
		ErrorPath: filepath.Join(dir, "error.log"),
		AuditPath: filepath.Join(dir, "audit.log"),
	}

	if err := ValidateLogPaths(cfg); err == nil {
		t.Fatalf("expected error for read-only dir")
	}
}

func boolPtr(v bool) *bool {
	return &v
}

func TestSanitizeForLog(t *testing.T) {
	tests := []struct {
		name     string
		input    map[string]any
		expected map[string]any
	}{
		{
			name: "redact plaintext",
			input: map[string]any{
				"plaintext": "secret-data",
				"id":        123,
			},
			expected: map[string]any{
				"plaintext": "[REDACTED]",
				"id":        123,
			},
		},
		{
			name: "redact secret",
			input: map[string]any{
				"secret": "my-secret",
				"name":   "test",
			},
			expected: map[string]any{
				"secret": "[REDACTED]",
				"name":   "test",
			},
		},
		{
			name: "redact password",
			input: map[string]any{
				"password": "pass123",
				"user":     "admin",
			},
			expected: map[string]any{
				"password": "[REDACTED]",
				"user":     "admin",
			},
		},
		{
			name: "redact token",
			input: map[string]any{
				"token": "abc123",
				"type":  "bearer",
			},
			expected: map[string]any{
				"token": "[REDACTED]",
				"type":  "bearer",
			},
		},
		{
			name: "redact key",
			input: map[string]any{
				"key":  "encryption-key",
				"algo": "AES",
			},
			expected: map[string]any{
				"key":  "[REDACTED]",
				"algo": "AES",
			},
		},
		{
			name: "no sensitive data",
			input: map[string]any{
				"id":   456,
				"name": "test-name",
			},
			expected: map[string]any{
				"id":   456,
				"name": "test-name",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SanitizeForLog(tt.input)
			for k, v := range tt.expected {
				if result[k] != v {
					t.Errorf("SanitizeForLog()[%s] = %v, want %v", k, result[k], v)
				}
			}
		})
	}
}
