package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// validateFilePath checks if the path is safe (no directory traversal)
// This prevents CWE-22 directory traversal attacks
func validateFilePath(path string) error {
	// Check for directory traversal patterns BEFORE cleaning
	// This catches attempts like "../../../etc/passwd"
	if strings.Contains(path, "..") {
		return fmt.Errorf("invalid path: contains directory traversal")
	}

	// Additional check: clean the path and verify it doesn't escape
	cleanPath := filepath.Clean(path)

	// For relative paths, ensure they don't start with ../
	if !filepath.IsAbs(cleanPath) && strings.HasPrefix(cleanPath, ".."+string(filepath.Separator)) {
		return fmt.Errorf("invalid path: attempts to escape current directory")
	}

	return nil
}

// LoadConfig loads configuration from YAML file and applies environment overrides
func LoadConfig(path string) (*Config, error) {
	// Validate path for security (prevent directory traversal)
	if err := validateFilePath(path); err != nil {
		return nil, fmt.Errorf("invalid config path: %w", err)
	}

	// Read YAML file
	// #nosec G304 - path is validated above
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config YAML: %w", err)
	}

	// Apply environment variable overrides
	applyEnvOverrides(&cfg)

	// Validate configuration
	if err := validateConfig(&cfg); err != nil {
		return nil, fmt.Errorf("validate config: %w", err)
	}

	return &cfg, nil
}

// LoadMetadata loads key rotation metadata from metadata.yaml
func LoadMetadata(path string) (*Metadata, error) {
	// Validate path for security (prevent directory traversal)
	if err := validateFilePath(path); err != nil {
		return nil, fmt.Errorf("invalid metadata path: %w", err)
	}

	// #nosec G304 - path is validated above
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read metadata file: %w", err)
	}

	var meta Metadata
	if err := yaml.Unmarshal(data, &meta); err != nil {
		return nil, fmt.Errorf("parse metadata YAML: %w", err)
	}

	return &meta, nil
}

// SaveMetadata saves key rotation metadata to metadata.yaml
func SaveMetadata(path string, meta *Metadata) error {
	// Validate path for security (prevent directory traversal)
	if err := validateFilePath(path); err != nil {
		return fmt.Errorf("invalid metadata path: %w", err)
	}

	data, err := yaml.Marshal(meta)
	if err != nil {
		return fmt.Errorf("marshal metadata to YAML: %w", err)
	}

	// Open file with O_TRUNC to preserve inode (important for bind mounts)
	// #nosec G304 - path is validated above
	// Use 0600 permissions - metadata contains sensitive key information
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("open metadata file: %w", err)
	}
	defer f.Close()

	// Write data
	if _, err := f.Write(data); err != nil {
		return fmt.Errorf("write metadata file: %w", err)
	}

	// Force sync to disk (ensures bind mount sees changes)
	if err := f.Sync(); err != nil {
		return fmt.Errorf("sync metadata file: %w", err)
	}

	return nil
}

// applyEnvOverrides applies environment variable overrides to configuration
func applyEnvOverrides(cfg *Config) {
	// Server overrides
	if port := os.Getenv("HSM_SERVER_PORT"); port != "" {
		if v, err := strconv.Atoi(port); err == nil {
			cfg.Server.Port = v
		}
	}
	if certPath := os.Getenv("HSM_SERVER_CERT"); certPath != "" {
		cfg.Server.TLS.CertPath = certPath
	}
	if keyPath := os.Getenv("HSM_SERVER_KEY"); keyPath != "" {
		cfg.Server.TLS.KeyPath = keyPath
	}
	if caPath := os.Getenv("HSM_SERVER_CA"); caPath != "" {
		cfg.Server.TLS.CAPath = caPath
	}

	// HSM overrides
	if lib := os.Getenv("HSM_PKCS11_LIB"); lib != "" {
		cfg.HSM.PKCS11Lib = lib
	}
	if slot := os.Getenv("HSM_SLOT_ID"); slot != "" {
		cfg.HSM.SlotID = slot
	}
	if pin := os.Getenv("HSM_PIN"); pin != "" {
		cfg.HSM.PIN = pin
	}

	// Logging overrides
	if level := os.Getenv("HSM_LOG_LEVEL"); level != "" {
		cfg.Logging.Level = level
	}
	if format := os.Getenv("HSM_LOG_FORMAT"); format != "" {
		cfg.Logging.Format = format
	}
	if errorPath := os.Getenv("HSM_LOG_ERROR_PATH"); errorPath != "" {
		cfg.Logging.ErrorPath = errorPath
	}
	if auditPath := os.Getenv("HSM_LOG_AUDIT_PATH"); auditPath != "" {
		cfg.Logging.AuditPath = auditPath
	}
	if accessPath := os.Getenv("HSM_LOG_ACCESS_PATH"); accessPath != "" {
		cfg.Logging.AccessPath = accessPath
	}
	if maxSize := os.Getenv("HSM_LOG_MAX_SIZE_MB"); maxSize != "" {
		if v, err := strconv.Atoi(maxSize); err == nil {
			cfg.Logging.MaxSizeMB = v
		}
	}
	if maxBackups := os.Getenv("HSM_LOG_MAX_BACKUPS"); maxBackups != "" {
		if v, err := strconv.Atoi(maxBackups); err == nil {
			cfg.Logging.MaxBackups = v
		}
	}
	if maxAge := os.Getenv("HSM_LOG_MAX_AGE_DAYS"); maxAge != "" {
		if v, err := strconv.Atoi(maxAge); err == nil {
			cfg.Logging.MaxAgeDays = v
		}
	}
	if compress := os.Getenv("HSM_LOG_COMPRESS"); compress != "" {
		if v, err := strconv.ParseBool(compress); err == nil {
			cfg.Logging.Compress = &v
		}
	}
	if errorStdout := os.Getenv("HSM_LOG_ERROR_TO_STDOUT"); errorStdout != "" {
		if v, err := strconv.ParseBool(errorStdout); err == nil {
			cfg.Logging.ErrorToStdout = &v
		}
	}
	if auditStdout := os.Getenv("HSM_LOG_AUDIT_TO_STDOUT"); auditStdout != "" {
		if v, err := strconv.ParseBool(auditStdout); err == nil {
			cfg.Logging.AuditToStdout = &v
		}
	}
	if accessStdout := os.Getenv("HSM_LOG_ACCESS_TO_STDOUT"); accessStdout != "" {
		if v, err := strconv.ParseBool(accessStdout); err == nil {
			cfg.Logging.AccessToStdout = &v
		}
	}
}

// validateConfig validates the configuration
func validateConfig(cfg *Config) error {
	// Validate server config
	if cfg.Server.Port <= 0 || cfg.Server.Port > 65535 {
		return fmt.Errorf("server.port must be in range 1..65535")
	}
	if cfg.Server.TLS.CertPath == "" {
		return fmt.Errorf("server.tls.cert_path is required")
	}
	if cfg.Server.TLS.KeyPath == "" {
		return fmt.Errorf("server.tls.key_path is required")
	}
	if cfg.Server.TLS.CAPath == "" {
		return fmt.Errorf("server.tls.ca_path is required")
	}

	if cfg.Server.Timeouts.Read == 0 {
		cfg.Server.Timeouts.Read = 10 * time.Second
	}
	if cfg.Server.Timeouts.Write == 0 {
		cfg.Server.Timeouts.Write = 10 * time.Second
	}
	if cfg.Server.Timeouts.Idle == 0 {
		cfg.Server.Timeouts.Idle = 60 * time.Second
	}
	if cfg.Server.Timeouts.ReadHeader == 0 {
		cfg.Server.Timeouts.ReadHeader = 5 * time.Second
	}
	if cfg.Server.Timeouts.ShutdownGrace == 0 {
		cfg.Server.Timeouts.ShutdownGrace = 15 * time.Second
	}
	if cfg.Server.Limits.MaxHeaderBytes == 0 {
		cfg.Server.Limits.MaxHeaderBytes = 1 << 20 // 1 MiB
	}

	if cfg.Server.Timeouts.Read < 0 || cfg.Server.Timeouts.Read > 24*time.Hour {
		return fmt.Errorf("server.timeouts.read is invalid: %s", cfg.Server.Timeouts.Read)
	}
	if cfg.Server.Timeouts.Write < 0 || cfg.Server.Timeouts.Write > 24*time.Hour {
		return fmt.Errorf("server.timeouts.write is invalid: %s", cfg.Server.Timeouts.Write)
	}
	if cfg.Server.Timeouts.Idle < 0 || cfg.Server.Timeouts.Idle > 24*time.Hour {
		return fmt.Errorf("server.timeouts.idle is invalid: %s", cfg.Server.Timeouts.Idle)
	}
	if cfg.Server.Timeouts.ReadHeader < 0 || cfg.Server.Timeouts.ReadHeader > 24*time.Hour {
		return fmt.Errorf("server.timeouts.read_header is invalid: %s", cfg.Server.Timeouts.ReadHeader)
	}
	if cfg.Server.Timeouts.ShutdownGrace < 0 || cfg.Server.Timeouts.ShutdownGrace > 24*time.Hour {
		return fmt.Errorf("server.timeouts.shutdown_grace is invalid: %s", cfg.Server.Timeouts.ShutdownGrace)
	}
	if cfg.Server.Limits.MaxHeaderBytes < 4096 || cfg.Server.Limits.MaxHeaderBytes > (16<<20) {
		return fmt.Errorf("server.limits.max_header_bytes is invalid: %d", cfg.Server.Limits.MaxHeaderBytes)
	}

	// Validate HSM config
	if cfg.HSM.PKCS11Lib == "" {
		return fmt.Errorf("hsm.pkcs11_lib is required")
	}
	if cfg.HSM.SlotID == "" {
		return fmt.Errorf("hsm.slot_id is required")
	}
	// PIN is provided via ENV variable HSM_PIN, not in config
	if len(cfg.HSM.Keys) == 0 {
		return fmt.Errorf("hsm.keys cannot be empty")
	}

	// Validate key configurations
	for name, key := range cfg.HSM.Keys {
		if key.Type == "" {
			return fmt.Errorf("hsm.keys.%s.type is required", name)
		}
		if key.Type != "aes" && key.Type != "rsa" {
			return fmt.Errorf("hsm.keys.%s.type must be 'aes' or 'rsa', got '%s'", name, key.Type)
		}
		// Validate mode (default: private if not set)
		if key.Mode == "" {
			cfg.HSM.Keys[name] = KeyConfig{
				Type: key.Type,
				Mode: "private", // default
			}
		} else if key.Mode != "shared" && key.Mode != "private" {
			return fmt.Errorf("hsm.keys.%s.mode must be 'shared' or 'private', got '%s'", name, key.Mode)
		}
	}

	// Validate ACL config
	if len(cfg.ACL.Mappings) == 0 {
		return fmt.Errorf("acl.mappings cannot be empty")
	}

	// Validate logging config
	if cfg.Logging.Level == "" {
		cfg.Logging.Level = "info" // default
	}
	if cfg.Logging.Format == "" {
		cfg.Logging.Format = "json" // default
	}
	if cfg.Logging.ErrorPath == "" {
		cfg.Logging.ErrorPath = "/var/log/hsm-service/error.log"
	}
	if cfg.Logging.AuditPath == "" {
		cfg.Logging.AuditPath = "/var/log/hsm-service/audit.log"
	}
	if cfg.Logging.AccessPath == "" {
		cfg.Logging.AccessPath = "/var/log/hsm-service/access.log"
	}
	if cfg.Logging.MaxSizeMB == 0 {
		cfg.Logging.MaxSizeMB = 100
	}
	if cfg.Logging.MaxBackups == 0 {
		cfg.Logging.MaxBackups = 10
	}
	if cfg.Logging.MaxAgeDays == 0 {
		cfg.Logging.MaxAgeDays = 30
	}
	if cfg.Logging.Compress == nil {
		v := true
		cfg.Logging.Compress = &v
	}
	if cfg.Logging.ErrorToStdout == nil {
		v := true
		cfg.Logging.ErrorToStdout = &v
	}
	if cfg.Logging.AuditToStdout == nil {
		v := true
		cfg.Logging.AuditToStdout = &v
	}
	if cfg.Logging.AccessToStdout == nil {
		v := true
		cfg.Logging.AccessToStdout = &v
	}

	return nil
}
