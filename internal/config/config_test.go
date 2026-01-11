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
	defer os.Unsetenv("HSM_SERVER_PORT")
	defer os.Unsetenv("HSM_LOG_LEVEL")

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
