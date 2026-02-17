package server

import (
	"fmt"
	"io"
	"log"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/titaev-lv/hsm-service/internal/config"
	"gopkg.in/natefinch/lumberjack.v2"
)

var (
	errorLogger   *slog.Logger
	auditLogger   *slog.Logger
	accessLogger  *slog.Logger
	errorLogFile  *lumberjack.Logger
	auditLogFile  *lumberjack.Logger
	accessLogFile *lumberjack.Logger
)

// InitLogger initializes the global slog logger based on configuration
func InitLogger(cfg *config.LoggingConfig) error {
	var level slog.Level
	switch cfg.Level {
	case "debug":
		level = slog.LevelDebug
	case "info":
		level = slog.LevelInfo
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{
		Level:       level,
		ReplaceAttr: replaceTimeAttr,
	}

	errorFile := &lumberjack.Logger{
		Filename:   cfg.ErrorPath,
		MaxSize:    cfg.MaxSizeMB,
		MaxBackups: cfg.MaxBackups,
		MaxAge:     cfg.MaxAgeDays,
		Compress:   cfg.Compress != nil && *cfg.Compress,
	}
	errorLogFile = errorFile

	errorWriter := io.MultiWriter(os.Stdout, errorFile)

	var errorHandler slog.Handler
	if cfg.Format == "json" {
		errorHandler = slog.NewJSONHandler(errorWriter, opts)
	} else {
		errorHandler = slog.NewTextHandler(errorWriter, opts)
	}

	errorLogger = slog.New(errorHandler)
	slog.SetDefault(errorLogger)
	log.SetOutput(errorWriter)

	auditFile := &lumberjack.Logger{
		Filename:   cfg.AuditPath,
		MaxSize:    cfg.MaxSizeMB,
		MaxBackups: cfg.MaxBackups,
		MaxAge:     cfg.MaxAgeDays,
		Compress:   cfg.Compress != nil && *cfg.Compress,
	}
	auditLogFile = auditFile

	auditWriter := io.Writer(auditFile)
	if cfg.AuditToStdout != nil && *cfg.AuditToStdout {
		auditWriter = io.MultiWriter(os.Stdout, auditWriter)
	}
	if level == slog.LevelDebug && cfg.AuditMirrorToErrorOnDebug != nil && *cfg.AuditMirrorToErrorOnDebug {
		auditWriter = io.MultiWriter(auditWriter, errorWriter)
	}

	var auditHandler slog.Handler
	if cfg.Format == "json" {
		auditHandler = slog.NewJSONHandler(auditWriter, opts)
	} else {
		auditHandler = slog.NewTextHandler(auditWriter, opts)
	}

	auditLogger = slog.New(auditHandler).With("component", "audit", "module", "audit")

	accessFile := &lumberjack.Logger{
		Filename:   cfg.AccessPath,
		MaxSize:    cfg.MaxSizeMB,
		MaxBackups: cfg.MaxBackups,
		MaxAge:     cfg.MaxAgeDays,
		Compress:   cfg.Compress != nil && *cfg.Compress,
	}
	accessLogFile = accessFile

	accessWriter := io.Writer(accessFile)
	if cfg.AccessToStdout != nil && *cfg.AccessToStdout {
		accessWriter = io.MultiWriter(os.Stdout, accessWriter)
	}

	var accessHandler slog.Handler
	if cfg.Format == "json" {
		accessHandler = slog.NewJSONHandler(accessWriter, opts)
	} else {
		accessHandler = slog.NewTextHandler(accessWriter, opts)
	}

	accessLogger = slog.New(accessHandler).With("component", "access", "module", "access")

	return nil
}

func replaceTimeAttr(_ []string, attr slog.Attr) slog.Attr {
	if attr.Key != slog.TimeKey {
		return attr
	}
	if t, ok := attr.Value.Any().(time.Time); ok {
		attr.Value = slog.StringValue(t.UTC().Format("2006-01-02T15:04:05.000000Z"))
	}
	return attr
}

// CloseLogger closes the underlying log writers.
func CloseLogger() error {
	var firstErr error
	closeFile := func(logFile *lumberjack.Logger) {
		if logFile == nil {
			return
		}
		if err := logFile.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	closeFile(accessLogFile)
	closeFile(auditLogFile)
	closeFile(errorLogFile)
	return firstErr
}

// ErrorLogger returns the main application logger (error.log).
func ErrorLogger() *slog.Logger {
	if errorLogger == nil {
		return slog.Default()
	}
	return errorLogger
}

// AuditLogger returns a logger specifically for audit events
func AuditLogger() *slog.Logger {
	if auditLogger == nil {
		return slog.With("component", "audit", "module", "audit")
	}
	return auditLogger
}

// AccessLogger returns a logger specifically for access events
func AccessLogger() *slog.Logger {
	if accessLogger == nil {
		return slog.With("component", "access", "module", "access")
	}
	return accessLogger
}

// ValidateLogPaths ensures log directories are writable and support rename.
func ValidateLogPaths(cfg *config.LoggingConfig) error {
	if err := validateLogPath(cfg.ErrorPath); err != nil {
		return fmt.Errorf("error log path validation failed: %w", err)
	}
	if err := validateLogPath(cfg.AuditPath); err != nil {
		return fmt.Errorf("audit log path validation failed: %w", err)
	}
	if err := validateLogPath(cfg.AccessPath); err != nil {
		return fmt.Errorf("access log path validation failed: %w", err)
	}
	return nil
}

func validateLogPath(path string) error {
	if path == "" {
		return fmt.Errorf("log path is empty")
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0750); err != nil {
		return fmt.Errorf("create log dir %s: %w", dir, err)
	}

	// Create, write, and rename a temp file to validate write and rotation rights.
	f, err := os.CreateTemp(dir, ".write-test-*")
	if err != nil {
		return fmt.Errorf("create write test in %s: %w", dir, err)
	}
	name := f.Name()
	if _, err := f.WriteString("test"); err != nil {
		_ = f.Close()
		_ = os.Remove(name)
		return fmt.Errorf("write test in %s: %w", dir, err)
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(name)
		return fmt.Errorf("close write test in %s: %w", dir, err)
	}

	rotated := name + ".rotate"
	if err := os.Rename(name, rotated); err != nil {
		_ = os.Remove(name)
		return fmt.Errorf("rename write test in %s: %w", dir, err)
	}
	if err := os.Remove(rotated); err != nil {
		return fmt.Errorf("cleanup write test in %s: %w", dir, err)
	}

	return nil
}

// SanitizeForLog removes or redacts sensitive fields from log data
func SanitizeForLog(data map[string]any) map[string]any {
	sanitized := make(map[string]any)
	for k, v := range data {
		key := strings.ToLower(k)
		// Redact sensitive fields
		if strings.Contains(key, "plaintext") ||
			strings.Contains(key, "secret") ||
			strings.Contains(key, "password") ||
			strings.Contains(key, "token") ||
			key == "key" ||
			key == "kek" {
			sanitized[k] = "[REDACTED]"
		} else {
			sanitized[k] = v
		}
	}
	return sanitized
}
