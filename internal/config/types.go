package config

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

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
	Port     int           `yaml:"port"`
	TLS      TLSConfig     `yaml:"tls"`
	Timeouts TimeoutConfig `yaml:"timeouts"`
	Limits   LimitsConfig  `yaml:"limits"`
	HTTP2    *HTTP2Config  `yaml:"http2,omitempty"` // HTTP/2 configuration (optional)
}

func (c *ServerConfig) UnmarshalYAML(value *yaml.Node) error {
	type rawServerConfig struct {
		Port     any           `yaml:"port"`
		TLS      TLSConfig     `yaml:"tls"`
		Timeouts TimeoutConfig `yaml:"timeouts"`
		Limits   LimitsConfig  `yaml:"limits"`
		HTTP2    *HTTP2Config  `yaml:"http2,omitempty"`
	}

	var raw rawServerConfig
	if err := value.Decode(&raw); err != nil {
		return err
	}

	port, err := parsePort(raw.Port)
	if err != nil {
		return fmt.Errorf("invalid server.port: %w", err)
	}

	c.Port = port
	c.TLS = raw.TLS
	c.Timeouts = raw.Timeouts
	c.Limits = raw.Limits
	c.HTTP2 = raw.HTTP2

	return nil
}

// TimeoutConfig contains HTTP server timeout settings.
type TimeoutConfig struct {
	Read          time.Duration `yaml:"read"`
	Write         time.Duration `yaml:"write"`
	Idle          time.Duration `yaml:"idle"`
	ReadHeader    time.Duration `yaml:"read_header"`
	ShutdownGrace time.Duration `yaml:"shutdown_grace"`
}

// LimitsConfig contains HTTP server limits settings.
type LimitsConfig struct {
	MaxHeaderBytes int `yaml:"max_header_bytes"`
}

func parsePort(v any) (int, error) {
	switch port := v.(type) {
	case nil:
		return 0, nil
	case int:
		return port, nil
	case int64:
		return int(port), nil
	case uint64:
		return int(port), nil
	case float64:
		return int(port), nil
	case string:
		trimmed := strings.TrimSpace(port)
		if trimmed == "" {
			return 0, nil
		}
		parsed, err := strconv.Atoi(trimmed)
		if err != nil {
			return 0, err
		}
		return parsed, nil
	default:
		return 0, fmt.Errorf("unsupported type %T", v)
	}
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
	Mode string `yaml:"mode"` // "shared" (AAD=context+OU) or "private" (AAD=context+clientCN), default: "private"
}

// RFC3339Micro is a custom time type that always marshals with microsecond precision
type RFC3339Micro time.Time

// MarshalYAML implements yaml.Marshaler to ensure consistent timestamp format
func (t RFC3339Micro) MarshalYAML() (interface{}, error) {
	// Format with microseconds: 2026-02-07T12:34:56.123456Z
	return time.Time(t).UTC().Format("2006-01-02T15:04:05.000000Z07:00"), nil
}

// UnmarshalYAML implements yaml.Unmarshaler
func (t *RFC3339Micro) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var s string
	if err := unmarshal(&s); err != nil {
		return err
	}
	parsed, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		return err
	}
	*t = RFC3339Micro(parsed)
	return nil
}

// KeyVersion represents a single version of a key
type KeyVersion struct {
	Label     string        `yaml:"label"`
	Version   int           `yaml:"version"`
	CreatedAt *RFC3339Micro `yaml:"created_at"`
	Checksum  string        `yaml:"checksum,omitempty"` // SHA-256 of key attributes (label+id) for integrity
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
	Level                     string `yaml:"level"`                          // debug, info, warn, error
	Format                    string `yaml:"format"`                         // json, text
	ErrorPath                 string `yaml:"error_path"`                     // /var/log/hsm-service/error.log
	AuditPath                 string `yaml:"audit_path"`                     // /var/log/hsm-service/audit.log
	AccessPath                string `yaml:"access_path"`                    // /var/log/hsm-service/access.log
	MaxSizeMB                 int    `yaml:"max_size_mb"`                    // MB
	MaxBackups                int    `yaml:"max_backups"`                    // count
	MaxAgeDays                int    `yaml:"max_age_days"`                   // days
	Compress                  *bool  `yaml:"compress"`                       // default true
	AuditToStdout             *bool  `yaml:"audit_to_stdout"`                // default true
	AccessToStdout            *bool  `yaml:"access_to_stdout"`               // default true
	AuditMirrorToErrorOnDebug *bool  `yaml:"audit_mirror_to_error_on_debug"` // default true
}
