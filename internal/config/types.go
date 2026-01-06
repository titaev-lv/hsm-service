package config

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
	Port string    `yaml:"port"`
	TLS  TLSConfig `yaml:"tls"`
}

// TLSConfig defines TLS certificate paths
type TLSConfig struct {
	CertPath string `yaml:"cert_path"`
	KeyPath  string `yaml:"key_path"`
	CAPath   string `yaml:"ca_path"`
}

// HSMConfig defines HSM/PKCS#11 configuration
type HSMConfig struct {
	PKCS11Lib string               `yaml:"pkcs11_lib"`
	SlotID    string               `yaml:"slot_id"`
	PIN       string               `yaml:"pin"`
	Keys      map[string]KeyConfig `yaml:"keys"`
}

// KeyConfig defines individual key configuration
type KeyConfig struct {
	Label string `yaml:"label"`
	Type  string `yaml:"type"` // "aes" or "rsa"
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
