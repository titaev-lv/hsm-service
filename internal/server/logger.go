package server

import (
	"log/slog"
	"os"
	"strings"

	"github.com/titaev-lv/hsm-service/internal/config"
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
		Level: level,
	}

	var handler slog.Handler
	if cfg.Format == "json" {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	} else {
		handler = slog.NewTextHandler(os.Stdout, opts)
	}

	logger := slog.New(handler)
	slog.SetDefault(logger)

	return nil
}

// AuditLogger returns a logger specifically for audit events
func AuditLogger() *slog.Logger {
	return slog.With("component", "audit")
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
