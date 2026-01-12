package config

import "time"

// Config represents the complete application configuration
type Config struct {
	Server    ServerConfig    `yaml:"server"`
	HSM       HSMConfig       `yaml:"hsm"`
	ACL       ACLConfig       `yaml:"acl"`
	RateLimit RateLimitConfig `yaml:"rate_limit"`
	Logging   LoggingConfig   `yaml:"logging"`
}

// ServerConfig defines HTTP server configuration
type ServerConfig struct {
	Port  string       `yaml:"port"`
	TLS   TLSConfig    `yaml:"tls"`
	HTTP2 *HTTP2Config `yaml:"http2,omitempty"` // HTTP/2 configuration (optional)
}

// TLSConfig defines TLS certificate paths
type TLSConfig struct {
	CertPath string `yaml:"cert_path"`
	KeyPath  string `yaml:"key_path"`
	CAPath   string `yaml:"ca_path"`
}

// HSMConfig defines HSM/PKCS#11 configuration
type HSMConfig struct {
	PKCS11Lib        string               `yaml:"pkcs11_lib"`
	SlotID           string               `yaml:"slot_id"`
	PIN              string               `yaml:"pin"`
	MetadataFile     string               `yaml:"metadata_file"`      // Path to metadata.yaml for rotation state
	MaxVersions      int                  `yaml:"max_versions"`       // Maximum versions to keep (default: 3)
	CleanupAfterDays int                  `yaml:"cleanup_after_days"` // Auto-cleanup versions older than N days (default: 30)
	Keys             map[string]KeyConfig `yaml:"keys"`
}

// KeyConfig defines individual key configuration (static)
type KeyConfig struct {
	Type string `yaml:"type"` // "aes" or "rsa"
}

// KeyVersion represents a single version of a key
type KeyVersion struct {
	Label     string     `yaml:"label"`
	Version   int        `yaml:"version"`
	CreatedAt *time.Time `yaml:"created_at"`
	Checksum  string     `yaml:"checksum,omitempty"` // SHA-256 of key attributes (label+id) for integrity
}

// KeyMetadata defines dynamic key rotation metadata
type KeyMetadata struct {
	Current              string       `yaml:"current"`                          // Current active version label
	RotationIntervalDays int          `yaml:"rotation_interval_days,omitempty"` // Rotation interval in days (e.g., 90 for PCI DSS)
	Versions             []KeyVersion `yaml:"versions"`                         // All versions (for overlap period)
}

// Metadata represents the metadata.yaml structure
type Metadata struct {
	Rotation map[string]KeyMetadata `yaml:"rotation"`
}

// ACLConfig defines access control configuration
type ACLConfig struct {
	RevokedFile string              `yaml:"revoked_file"` // Path to revoked.yaml
	Mappings    map[string][]string `yaml:"mappings"`     // OU -> allowed keys
}

// RateLimitConfig defines rate limiting parameters
type RateLimitConfig struct {
	RequestsPerSecond int `yaml:"requests_per_second"`
	Burst             int `yaml:"burst"`
}

// LoggingConfig defines logging configuration
type LoggingConfig struct {
	Level  string `yaml:"level"`  // debug, info, warn, error
	Format string `yaml:"format"` // json, text
}
