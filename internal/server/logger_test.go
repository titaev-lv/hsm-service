package server

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
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
				Level:          "info",
				Format:         "json",
				MaxSizeMB:      1,
				MaxBackups:     1,
				MaxAgeDays:     1,
				Compress:       boolPtr(true),
				AuditToStdout:  boolPtr(true),
				AccessToStdout: boolPtr(true),
			},
		},
		{
			name: "text format",
			config: &config.LoggingConfig{
				Level:          "debug",
				Format:         "text",
				MaxSizeMB:      1,
				MaxBackups:     1,
				MaxAgeDays:     1,
				Compress:       boolPtr(true),
				AuditToStdout:  boolPtr(true),
				AccessToStdout: boolPtr(true),
			},
		},
		{
			name: "default level",
			config: &config.LoggingConfig{
				Level:          "unknown",
				Format:         "json",
				MaxSizeMB:      1,
				MaxBackups:     1,
				MaxAgeDays:     1,
				Compress:       boolPtr(true),
				AuditToStdout:  boolPtr(true),
				AccessToStdout: boolPtr(true),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			tt.config.ErrorPath = filepath.Join(dir, "error.log")
			tt.config.AuditPath = filepath.Join(dir, "audit.log")
			tt.config.AccessPath = filepath.Join(dir, "access.log")

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
		ErrorPath:  filepath.Join(dir, "error.log"),
		AuditPath:  filepath.Join(dir, "audit.log"),
		AccessPath: filepath.Join(dir, "access.log"),
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
		ErrorPath:  filepath.Join(dir, "error.log"),
		AuditPath:  filepath.Join(dir, "audit.log"),
		AccessPath: filepath.Join(dir, "access.log"),
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

func TestLoggerFallbacksAndClose(t *testing.T) {
	oldErrorLogger := errorLogger
	oldAuditLogger := auditLogger
	oldAccessLogger := accessLogger
	oldErrorLogFile := errorLogFile
	oldAuditLogFile := auditLogFile
	oldAccessLogFile := accessLogFile

	defer func() {
		errorLogger = oldErrorLogger
		auditLogger = oldAuditLogger
		accessLogger = oldAccessLogger
		errorLogFile = oldErrorLogFile
		auditLogFile = oldAuditLogFile
		accessLogFile = oldAccessLogFile
	}()

	// Force fallback branches.
	errorLogger = nil
	auditLogger = nil
	accessLogger = nil
	errorLogFile = nil
	auditLogFile = nil
	accessLogFile = nil

	if got := ErrorLogger(); got == nil {
		t.Fatalf("ErrorLogger() returned nil")
	}
	if got := AuditLogger(); got == nil {
		t.Fatalf("AuditLogger() returned nil")
	}
	if got := AccessLogger(); got == nil {
		t.Fatalf("AccessLogger() returned nil")
	}

	if err := CloseLogger(); err != nil {
		t.Fatalf("CloseLogger() with nil files error = %v", err)
	}

	// Initialize actual lumberjack writers and close them.
	dir := t.TempDir()
	cfg := &config.LoggingConfig{
		Level:          "info",
		Format:         "json",
		ErrorPath:      filepath.Join(dir, "error.log"),
		AuditPath:      filepath.Join(dir, "audit.log"),
		AccessPath:     filepath.Join(dir, "access.log"),
		MaxSizeMB:      1,
		MaxBackups:     1,
		MaxAgeDays:     1,
		Compress:       boolPtr(false),
		ErrorToStdout:  boolPtr(false),
		AuditToStdout:  boolPtr(false),
		AccessToStdout: boolPtr(false),
	}
	if err := InitLogger(cfg); err != nil {
		t.Fatalf("InitLogger() error = %v", err)
	}
	if err := CloseLogger(); err != nil {
		t.Fatalf("CloseLogger() error = %v", err)
	}
}

func TestValidateLogPath_EmptyPath(t *testing.T) {
	if err := validateLogPath(""); err == nil {
		t.Fatalf("expected error for empty log path")
	}
}

func TestValidateLogPath_CreateDirError(t *testing.T) {
	base := t.TempDir()
	blockedParent := filepath.Join(base, "blocked")
	if err := os.WriteFile(blockedParent, []byte("x"), 0600); err != nil {
		t.Fatalf("write blocked parent: %v", err)
	}

	badLogPath := filepath.Join(blockedParent, "error.log")
	err := validateLogPath(badLogPath)
	if err == nil {
		t.Fatalf("expected create dir error for path under file parent")
	}
	if !strings.Contains(err.Error(), "create log dir") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateLogPaths_ErrorPrefixes(t *testing.T) {
	goodDir := t.TempDir()
	goodError := filepath.Join(goodDir, "error.log")
	goodAudit := filepath.Join(goodDir, "audit.log")
	goodAccess := filepath.Join(goodDir, "access.log")

	makeBadPath := func(t *testing.T) string {
		t.Helper()
		base := t.TempDir()
		blockedParent := filepath.Join(base, "blocked")
		if err := os.WriteFile(blockedParent, []byte("x"), 0600); err != nil {
			t.Fatalf("write blocked parent: %v", err)
		}
		return filepath.Join(blockedParent, "bad.log")
	}

	t.Run("error path prefix", func(t *testing.T) {
		cfg := &config.LoggingConfig{
			ErrorPath:  makeBadPath(t),
			AuditPath:  goodAudit,
			AccessPath: goodAccess,
		}
		err := ValidateLogPaths(cfg)
		if err == nil || !strings.Contains(err.Error(), "error log path validation failed") {
			t.Fatalf("expected error prefix for error path, got: %v", err)
		}
	})

	t.Run("audit path prefix", func(t *testing.T) {
		cfg := &config.LoggingConfig{
			ErrorPath:  goodError,
			AuditPath:  makeBadPath(t),
			AccessPath: goodAccess,
		}
		err := ValidateLogPaths(cfg)
		if err == nil || !strings.Contains(err.Error(), "audit log path validation failed") {
			t.Fatalf("expected error prefix for audit path, got: %v", err)
		}
	})

	t.Run("access path prefix", func(t *testing.T) {
		cfg := &config.LoggingConfig{
			ErrorPath:  goodError,
			AuditPath:  goodAudit,
			AccessPath: makeBadPath(t),
		}
		err := ValidateLogPaths(cfg)
		if err == nil || !strings.Contains(err.Error(), "access log path validation failed") {
			t.Fatalf("expected error prefix for access path, got: %v", err)
		}
	})
}

type mockTempLogFile struct {
	name     string
	writeErr error
	closeErr error
}

func (m *mockTempLogFile) Name() string { return m.name }

func (m *mockTempLogFile) WriteString(_ string) (int, error) {
	if m.writeErr != nil {
		return 0, m.writeErr
	}
	return 4, nil
}

func (m *mockTempLogFile) Close() error { return m.closeErr }

func TestValidateLogPathWithOps_ErrorBranches(t *testing.T) {
	base := t.TempDir()
	path := filepath.Join(base, "error.log")

	t.Run("create temp failure", func(t *testing.T) {
		err := validateLogPathWithOps(path, validateLogPathOps{
			mkdirAll: func(_ string, _ os.FileMode) error { return nil },
			createTemp: func(_, _ string) (tempLogFile, error) {
				return nil, errors.New("create-temp-fail")
			},
			rename: os.Rename,
			remove: os.Remove,
		})
		if err == nil || !strings.Contains(err.Error(), "create write test") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("write failure", func(t *testing.T) {
		err := validateLogPathWithOps(path, validateLogPathOps{
			mkdirAll: func(_ string, _ os.FileMode) error { return nil },
			createTemp: func(_, _ string) (tempLogFile, error) {
				return &mockTempLogFile{name: filepath.Join(base, "tmp-write"), writeErr: errors.New("write-fail")}, nil
			},
			rename: os.Rename,
			remove: func(_ string) error { return nil },
		})
		if err == nil || !strings.Contains(err.Error(), "write test") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("close failure", func(t *testing.T) {
		err := validateLogPathWithOps(path, validateLogPathOps{
			mkdirAll: func(_ string, _ os.FileMode) error { return nil },
			createTemp: func(_, _ string) (tempLogFile, error) {
				return &mockTempLogFile{name: filepath.Join(base, "tmp-close"), closeErr: errors.New("close-fail")}, nil
			},
			rename: os.Rename,
			remove: func(_ string) error { return nil },
		})
		if err == nil || !strings.Contains(err.Error(), "close write test") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("rename failure", func(t *testing.T) {
		err := validateLogPathWithOps(path, validateLogPathOps{
			mkdirAll: func(_ string, _ os.FileMode) error { return nil },
			createTemp: func(_, _ string) (tempLogFile, error) {
				return &mockTempLogFile{name: filepath.Join(base, "tmp-rename")}, nil
			},
			rename: func(_, _ string) error { return errors.New("rename-fail") },
			remove: func(_ string) error { return nil },
		})
		if err == nil || !strings.Contains(err.Error(), "rename write test") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("cleanup remove failure", func(t *testing.T) {
		err := validateLogPathWithOps(path, validateLogPathOps{
			mkdirAll: func(_ string, _ os.FileMode) error { return nil },
			createTemp: func(_, _ string) (tempLogFile, error) {
				return &mockTempLogFile{name: filepath.Join(base, "tmp-cleanup")}, nil
			},
			rename: func(_, _ string) error { return nil },
			remove: func(name string) error {
				if strings.HasSuffix(name, ".rotate") {
					return errors.New("remove-fail")
				}
				return nil
			},
		})
		if err == nil || !strings.Contains(err.Error(), "cleanup write test") {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}
