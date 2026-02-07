package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// validateFilePath checks if the path is safe (no directory traversal)
// This prevents CWE-22 directory traversal attacks
func validateFilePath(path string) error {
	// Clean the path to normalize it
	cleanPath := filepath.Clean(path)

	// Check for directory traversal attempts
	if strings.Contains(cleanPath, "..") {
		return fmt.Errorf("invalid path: contains directory traversal")
	}

	// Ensure it's an absolute path or relative to current directory
	// (no leading slashes for relative paths with ..)
	if strings.HasPrefix(cleanPath, ".."+string(filepath.Separator)) {
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
		cfg.Server.Port = port
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
}

// validateConfig validates the configuration
func validateConfig(cfg *Config) error {
	// Validate server config
	if cfg.Server.Port == "" {
		return fmt.Errorf("server.port is required")
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

	return nil
}
