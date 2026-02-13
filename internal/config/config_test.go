package config

import (
	"os"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	// Create temporary config file
	configContent := `
server:
  port: "8443"
  tls:
    cert_path: "/pki/server/cert.crt"
    key_path: "/pki/server/cert.key"
    ca_path: "/pki/ca/ca.crt"

hsm:
  pkcs11_lib: "/usr/lib/softhsm/libsofthsm2.so"
  slot_id: "0"
  pin: "1234"
  metadata_file: "/app/metadata.yaml"
  keys:
    exchange-key:
      type: "aes"
      rotation_interval: "2160h"

acl:
  mappings:
    Trading:
      - exchange-key
    2FA:
      - 2fa-key

rate_limit:
  requests_per_second: 100
  burst: 200

logging:
  level: "info"
  format: "json"
`
	tmpfile, err := os.CreateTemp("", "config-*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpfile.Name())

	if _, err := tmpfile.Write([]byte(configContent)); err != nil {
		t.Fatal(err)
	}
	if err := tmpfile.Close(); err != nil {
		t.Fatal(err)
	}

	// Test loading config
	cfg, err := LoadConfig(tmpfile.Name())
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}

	// Validate loaded config
	if cfg.Server.Port != "8443" {
		t.Errorf("Server.Port = %s, want 8443", cfg.Server.Port)
	}
	if cfg.HSM.PKCS11Lib != "/usr/lib/softhsm/libsofthsm2.so" {
		t.Errorf("HSM.PKCS11Lib = %s, want /usr/lib/softhsm/libsofthsm2.so", cfg.HSM.PKCS11Lib)
	}
	if len(cfg.HSM.Keys) != 1 {
		t.Errorf("len(HSM.Keys) = %d, want 1", len(cfg.HSM.Keys))
	}
}

func TestEnvOverrides(t *testing.T) {
	// Set environment variables
	os.Setenv("HSM_SERVER_PORT", "9443")
	os.Setenv("HSM_LOG_LEVEL", "debug")
	os.Setenv("HSM_LOG_ERROR_PATH", "/tmp/error.log")
	os.Setenv("HSM_LOG_AUDIT_PATH", "/tmp/audit.log")
	os.Setenv("HSM_LOG_ACCESS_PATH", "/tmp/access.log")
	os.Setenv("HSM_LOG_MAX_SIZE_MB", "55")
	os.Setenv("HSM_LOG_MAX_BACKUPS", "7")
	os.Setenv("HSM_LOG_MAX_AGE_DAYS", "9")
	os.Setenv("HSM_LOG_COMPRESS", "false")
	os.Setenv("HSM_LOG_AUDIT_TO_STDOUT", "false")
	os.Setenv("HSM_LOG_ACCESS_TO_STDOUT", "false")
	os.Setenv("HSM_LOG_AUDIT_MIRROR_TO_ERROR_ON_DEBUG", "false")
	defer os.Unsetenv("HSM_SERVER_PORT")
	defer os.Unsetenv("HSM_LOG_LEVEL")
	defer os.Unsetenv("HSM_LOG_ERROR_PATH")
	defer os.Unsetenv("HSM_LOG_AUDIT_PATH")
	defer os.Unsetenv("HSM_LOG_ACCESS_PATH")
	defer os.Unsetenv("HSM_LOG_MAX_SIZE_MB")
	defer os.Unsetenv("HSM_LOG_MAX_BACKUPS")
	defer os.Unsetenv("HSM_LOG_MAX_AGE_DAYS")
	defer os.Unsetenv("HSM_LOG_COMPRESS")
	defer os.Unsetenv("HSM_LOG_AUDIT_TO_STDOUT")
	defer os.Unsetenv("HSM_LOG_ACCESS_TO_STDOUT")
	defer os.Unsetenv("HSM_LOG_AUDIT_MIRROR_TO_ERROR_ON_DEBUG")

	configContent := `
server:
  port: "8443"
  tls:
    cert_path: "/pki/server/cert.crt"
    key_path: "/pki/server/cert.key"
    ca_path: "/pki/ca/ca.crt"

hsm:
  pkcs11_lib: "/usr/lib/softhsm/libsofthsm2.so"
  slot_id: "0"
  pin: "1234"
  metadata_file: "/app/metadata.yaml"
  keys:
    test-key:
      type: "aes"
      rotation_interval: "2160h"

acl:
  mappings:
    Trading:
      - test-key

rate_limit:
  requests_per_second: 100

logging:
  level: "info"
  format: "json"
`
	tmpfile, err := os.CreateTemp("", "config-*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpfile.Name())

	if _, err := tmpfile.Write([]byte(configContent)); err != nil {
		t.Fatal(err)
	}
	if err := tmpfile.Close(); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadConfig(tmpfile.Name())
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}

	// Check environment overrides
	if cfg.Server.Port != "9443" {
		t.Errorf("Server.Port = %s, want 9443 (from env)", cfg.Server.Port)
	}
	if cfg.Logging.Level != "debug" {
		t.Errorf("Logging.Level = %s, want debug (from env)", cfg.Logging.Level)
	}
	if cfg.Logging.ErrorPath != "/tmp/error.log" || cfg.Logging.AuditPath != "/tmp/audit.log" || cfg.Logging.AccessPath != "/tmp/access.log" {
		t.Errorf("unexpected log paths: error=%s audit=%s access=%s", cfg.Logging.ErrorPath, cfg.Logging.AuditPath, cfg.Logging.AccessPath)
	}
	if cfg.Logging.MaxSizeMB != 55 || cfg.Logging.MaxBackups != 7 || cfg.Logging.MaxAgeDays != 9 {
		t.Errorf("unexpected log sizes: size=%d backups=%d age=%d", cfg.Logging.MaxSizeMB, cfg.Logging.MaxBackups, cfg.Logging.MaxAgeDays)
	}
	if cfg.Logging.Compress == nil || *cfg.Logging.Compress {
		t.Errorf("expected compress=false from env")
	}
	if cfg.Logging.AuditToStdout == nil || *cfg.Logging.AuditToStdout {
		t.Errorf("expected audit_to_stdout=false from env")
	}
	if cfg.Logging.AccessToStdout == nil || *cfg.Logging.AccessToStdout {
		t.Errorf("expected access_to_stdout=false from env")
	}
	if cfg.Logging.AuditMirrorToErrorOnDebug == nil || *cfg.Logging.AuditMirrorToErrorOnDebug {
		t.Errorf("expected audit_mirror_to_error_on_debug=false from env")
	}
}

func TestLoggingDefaults(t *testing.T) {
	configContent := `
server:
  port: "8443"
  tls:
    cert_path: "/pki/server/cert.crt"
    key_path: "/pki/server/cert.key"
    ca_path: "/pki/ca/ca.crt"

hsm:
  pkcs11_lib: "/usr/lib/softhsm/libsofthsm2.so"
  slot_id: "0"
  pin: "1234"
  metadata_file: "/app/metadata.yaml"
  keys:
    test-key:
      type: "aes"

acl:
  mappings:
    Trading:
      - test-key

rate_limit:
  requests_per_second: 100
  burst: 200
`
	tmpfile, err := os.CreateTemp("", "config-*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpfile.Name())

	if _, err := tmpfile.Write([]byte(configContent)); err != nil {
		t.Fatal(err)
	}
	if err := tmpfile.Close(); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadConfig(tmpfile.Name())
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}

	if cfg.Logging.Level != "info" {
		t.Errorf("Logging.Level = %s, want info", cfg.Logging.Level)
	}
	if cfg.Logging.Format != "json" {
		t.Errorf("Logging.Format = %s, want json", cfg.Logging.Format)
	}
	if cfg.Logging.ErrorPath == "" || cfg.Logging.AuditPath == "" || cfg.Logging.AccessPath == "" {
		t.Errorf("expected default log paths set, got error=%q audit=%q access=%q", cfg.Logging.ErrorPath, cfg.Logging.AuditPath, cfg.Logging.AccessPath)
	}
	if cfg.Logging.MaxSizeMB != 100 || cfg.Logging.MaxBackups != 10 || cfg.Logging.MaxAgeDays != 30 {
		t.Errorf("unexpected defaults: size=%d backups=%d age=%d", cfg.Logging.MaxSizeMB, cfg.Logging.MaxBackups, cfg.Logging.MaxAgeDays)
	}
	if cfg.Logging.Compress == nil || !*cfg.Logging.Compress {
		t.Errorf("expected default compress=true")
	}
	if cfg.Logging.AuditToStdout == nil || !*cfg.Logging.AuditToStdout {
		t.Errorf("expected default audit_to_stdout=true")
	}
	if cfg.Logging.AccessToStdout == nil || !*cfg.Logging.AccessToStdout {
		t.Errorf("expected default access_to_stdout=true")
	}
	if cfg.Logging.AuditMirrorToErrorOnDebug == nil || !*cfg.Logging.AuditMirrorToErrorOnDebug {
		t.Errorf("expected default audit_mirror_to_error_on_debug=true")
	}
}

func TestValidateConfig(t *testing.T) {
	tests := []struct {
		name    string
		cfg     *Config
		wantErr bool
	}{
		{
			name: "valid config",
			cfg: &Config{
				Server: ServerConfig{
					Port: "8443",
					TLS: TLSConfig{
						CertPath: "/cert.crt",
						KeyPath:  "/cert.key",
						CAPath:   "/ca.crt",
					},
				},
				HSM: HSMConfig{
					PKCS11Lib:    "/lib/pkcs11.so",
					SlotID:       "0",
					PIN:          "1234",
					MetadataFile: "/app/metadata.yaml",
					Keys: map[string]KeyConfig{
						"test": {
							Type: "aes",
						},
					},
				},
				ACL: ACLConfig{
					Mappings: map[string][]string{
						"Trading": {"test"},
					},
				},
				Logging: LoggingConfig{
					Level:  "info",
					Format: "json",
				},
			},
			wantErr: false,
		},
		{
			name: "missing port",
			cfg: &Config{
				Server: ServerConfig{
					TLS: TLSConfig{
						CertPath: "/cert.crt",
						KeyPath:  "/cert.key",
						CAPath:   "/ca.crt",
					},
				},
			},
			wantErr: true,
		},
		{
			name: "invalid key type",
			cfg: &Config{
				Server: ServerConfig{
					Port: "8443",
					TLS: TLSConfig{
						CertPath: "/cert.crt",
						KeyPath:  "/cert.key",
						CAPath:   "/ca.crt",
					},
				},
				HSM: HSMConfig{
					PKCS11Lib:    "/lib/pkcs11.so",
					SlotID:       "0",
					PIN:          "1234",
					MetadataFile: "/app/metadata.yaml",
					Keys: map[string]KeyConfig{
						"test": {
							Type: "invalid",
						},
					},
				},
				ACL: ACLConfig{
					Mappings: map[string][]string{
						"Trading": {"test"},
					},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateConfig(tt.cfg)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateConfig() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateFilePath(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		wantErr bool
	}{
		{
			name:    "valid absolute path",
			path:    "/etc/hsm-service/config.yaml",
			wantErr: false,
		},
		{
			name:    "valid relative path",
			path:    "config.yaml",
			wantErr: false,
		},
		{
			name:    "valid relative path with subdirectory",
			path:    "configs/config.yaml",
			wantErr: false,
		},
		{
			name:    "directory traversal with ..",
			path:    "../../../etc/passwd",
			wantErr: true,
		},
		{
			name:    "directory traversal in middle",
			path:    "/var/lib/../../../etc/passwd",
			wantErr: true,
		},
		{
			name:    "clean path but contains .. after cleaning",
			path:    "foo/../../bar",
			wantErr: true,
		},
		{
			name:    "attempts to escape with ../",
			path:    "../../sensitive/file",
			wantErr: true,
		},
		{
			name:    "valid path with dots in filename",
			path:    "config.yaml.backup",
			wantErr: false,
		},
		{
			name:    "valid path with hidden file",
			path:    ".config.yaml",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateFilePath(tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateFilePath(%q) error = %v, wantErr %v", tt.path, err, tt.wantErr)
			}
		})
	}
}
