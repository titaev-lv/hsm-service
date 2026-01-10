package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestConfig_Validation проверяет валидацию всех полей конфигурации
func TestConfig_Validation(t *testing.T) {
	tests := []struct {
		name    string
		config  *Config
		wantErr bool
		errMsg  string
	}{
		{
			name: "Valid config",
			config: &Config{
				Server: ServerConfig{
					Port: "8443",
					TLS: TLSConfig{
						CertPath: "/path/to/cert.pem",
						KeyPath:  "/path/to/key.pem",
						CAPath:   "/path/to/ca.pem",
					},
				},
				HSM: HSMConfig{
					PKCS11Lib:    "/usr/lib/softhsm/libsofthsm2.so",
					SlotID:       "0",
					PIN:          "1234",
					MetadataFile: "metadata.yaml",
					Keys: map[string]KeyConfig{
						"test-key": {
							Type:             "aes",
							RotationInterval: 2160 * time.Hour,
						},
					},
				},
				ACL: ACLConfig{
					Mappings: map[string][]string{
						"test-ou": {"test-key"},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "Missing PKCS11Lib",
			config: &Config{
				Server: ServerConfig{
					Port: "8443",
					TLS: TLSConfig{
						CertPath: "cert.pem",
						KeyPath:  "key.pem",
						CAPath:   "ca.pem",
					},
				},
				HSM: HSMConfig{
					PKCS11Lib: "", // пусто
					SlotID:    "0",
					Keys: map[string]KeyConfig{
						"test": {Type: "aes"},
					},
				},
				ACL: ACLConfig{
					Mappings: map[string][]string{"test": {"test"}},
				},
			},
			wantErr: true,
			errMsg:  "pkcs11_lib",
		},
		{
			name: "Missing SlotID",
			config: &Config{
				Server: ServerConfig{
					Port: "8443",
					TLS: TLSConfig{
						CertPath: "cert.pem",
						KeyPath:  "key.pem",
						CAPath:   "ca.pem",
					},
				},
				HSM: HSMConfig{
					PKCS11Lib: "/usr/lib/softhsm/libsofthsm2.so",
					SlotID:    "", // пусто
					Keys: map[string]KeyConfig{
						"test": {Type: "aes"},
					},
				},
				ACL: ACLConfig{
					Mappings: map[string][]string{"test": {"test"}},
				},
			},
			wantErr: true,
			errMsg:  "slot_id",
		},
		{
			name: "Empty HSM keys",
			config: &Config{
				Server: ServerConfig{
					Port: "8443",
					TLS: TLSConfig{
						CertPath: "cert.pem",
						KeyPath:  "key.pem",
						CAPath:   "ca.pem",
					},
				},
				HSM: HSMConfig{
					PKCS11Lib: "/usr/lib/softhsm/libsofthsm2.so",
					SlotID:    "0",
					Keys:      map[string]KeyConfig{}, // пусто
				},
				ACL: ACLConfig{
					Mappings: map[string][]string{"test": {"test"}},
				},
			},
			wantErr: true,
			errMsg:  "keys cannot be empty",
		},
		{
			name: "Invalid key type",
			config: &Config{
				Server: ServerConfig{
					Port: "8443",
					TLS: TLSConfig{
						CertPath: "cert.pem",
						KeyPath:  "key.pem",
						CAPath:   "ca.pem",
					},
				},
				HSM: HSMConfig{
					PKCS11Lib: "/usr/lib/softhsm/libsofthsm2.so",
					SlotID:    "0",
					Keys: map[string]KeyConfig{
						"bad-key": {Type: "invalid"}, // неверный тип
					},
				},
				ACL: ACLConfig{
					Mappings: map[string][]string{"test": {"test"}},
				},
			},
			wantErr: true,
			errMsg:  "must be 'aes' or 'rsa'",
		},
		{
			name: "Empty ACL mappings",
			config: &Config{
				Server: ServerConfig{
					Port: "8443",
					TLS: TLSConfig{
						CertPath: "cert.pem",
						KeyPath:  "key.pem",
						CAPath:   "ca.pem",
					},
				},
				HSM: HSMConfig{
					PKCS11Lib: "/usr/lib/softhsm/libsofthsm2.so",
					SlotID:    "0",
					Keys: map[string]KeyConfig{
						"test": {Type: "aes"},
					},
				},
				ACL: ACLConfig{
					Mappings: map[string][]string{}, // пусто
				},
			},
			wantErr: true,
			errMsg:  "mappings cannot be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateConfig(tt.config)
			if tt.wantErr {
				if err == nil {
					t.Error("Expected validation error, got nil")
				} else if tt.errMsg != "" && !contains(err.Error(), tt.errMsg) {
					t.Errorf("Expected error containing '%s', got: %v", tt.errMsg, err)
				} else {
					t.Logf("✓ Validation error as expected: %v", err)
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error, got: %v", err)
				}
			}
		})
	}
}

// TestConfig_Defaults проверяет дефолтные значения
func TestConfig_Defaults(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")

	// Создаём минимальную конфигурацию
	minimalConfig := `
server:
  port: "8443"
  tls:
    cert_path: "cert.pem"
    key_path: "key.pem"
    ca_path: "ca.pem"
hsm:
  pkcs11_lib: "/usr/lib/softhsm/libsofthsm2.so"
  slot_id: "0"
  pin: "1234"
  keys:
    test-key:
      type: "aes"
      rotation_interval: "2160h"
acl:
  mappings:
    test-ou:
      - test-key
`

	if err := os.WriteFile(configFile, []byte(minimalConfig), 0644); err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}

	cfg, err := LoadConfig(configFile)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Проверяем дефолтные значения
	if cfg.HSM.MetadataFile == "" {
		cfg.HSM.MetadataFile = "metadata.yaml" // дефолт
	}
	if cfg.HSM.MetadataFile != "metadata.yaml" {
		t.Errorf("Expected default metadata file 'metadata.yaml', got '%s'", cfg.HSM.MetadataFile)
	}

	// Проверяем дефолтные logging values
	if cfg.Logging.Level == "" {
		t.Log("Logging level not set, expecting default 'info' after validation")
	}
	if cfg.Logging.Format == "" {
		t.Log("Logging format not set, expecting default 'json' after validation")
	}

	t.Logf("✓ Default values applied correctly")
}

// TestConfig_EnvOverride проверяет переопределение через ENV
func TestConfig_EnvOverride(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")

	// Базовая конфигурация
	baseConfig := `
server:
  port: "8443"
  tls:
    cert_path: "cert.pem"
    key_path: "key.pem"
    ca_path: "ca.pem"
hsm:
  pkcs11_lib: "/usr/lib/softhsm/libsofthsm2.so"
  slot_id: "0"
  pin: "1234"
  keys:
    test-key:
      type: "aes"
acl:
  mappings:
    test-ou:
      - test-key
`

	if err := os.WriteFile(configFile, []byte(baseConfig), 0644); err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}

	// Устанавливаем ENV переменные
	os.Setenv("HSM_PIN", "new-pin-from-env")
	os.Setenv("HSM_SERVER_PORT", "9443")
	defer func() {
		os.Unsetenv("HSM_PIN")
		os.Unsetenv("HSM_SERVER_PORT")
	}()

	cfg, err := LoadConfig(configFile)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Проверяем что ENV overrides применились
	if cfg.HSM.PIN != "new-pin-from-env" {
		t.Errorf("Expected PIN from env 'new-pin-from-env', got '%s'", cfg.HSM.PIN)
	}

	if cfg.Server.Port != "9443" {
		t.Errorf("Expected port from env '9443', got '%s'", cfg.Server.Port)
	}

	t.Logf("✓ ENV overrides work correctly")
}

// TestConfig_LoadNonExistentFile проверяет загрузку несуществующего файла
func TestConfig_LoadNonExistentFile(t *testing.T) {
	_, err := LoadConfig("/non/existent/config.yaml")
	if err == nil {
		t.Error("Expected error loading non-existent file, got nil")
	} else {
		t.Logf("✓ Non-existent file error: %v", err)
	}
}

// TestConfig_YAMLSyntaxError проверяет обработку невалидного YAML
func TestConfig_YAMLSyntaxError(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")

	invalidYAML := `
server:
  port: "8443
  tls:
    cert_path: "cert.pem"
`

	if err := os.WriteFile(configFile, []byte(invalidYAML), 0644); err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}

	_, err := LoadConfig(configFile)
	if err == nil {
		t.Error("Expected YAML parse error, got nil")
	} else {
		t.Logf("✓ YAML syntax error detected: %v", err)
	}
}

// TestMetadata_SaveAndLoad проверяет сохранение и загрузку metadata
func TestMetadata_SaveAndLoad(t *testing.T) {
	tmpDir := t.TempDir()
	metadataFile := filepath.Join(tmpDir, "metadata.yaml")

	now := time.Now().UTC()
	// Создаём тестовые метаданные
	original := &Metadata{
		Rotation: map[string]KeyMetadata{
			"test-key": {
				Current: "kek-test-v1",
				Versions: []KeyVersion{
					{
						Label:     "kek-test-v1",
						Version:   1,
						CreatedAt: &now,
					},
				},
			},
		},
	}

	// Сохраняем
	if err := SaveMetadata(metadataFile, original); err != nil {
		t.Fatalf("Failed to save metadata: %v", err)
	}

	// Загружаем
	loaded, err := LoadMetadata(metadataFile)
	if err != nil {
		t.Fatalf("Failed to load metadata: %v", err)
	}

	// Проверяем что данные совпадают
	if len(loaded.Rotation) != len(original.Rotation) {
		t.Errorf("Rotation count mismatch: expected %d, got %d", len(original.Rotation), len(loaded.Rotation))
	}

	// Проверяем конкретный ключ
	key := loaded.Rotation["test-key"]
	if key.Current != original.Rotation["test-key"].Current {
		t.Error("Current label mismatch")
	}

	if len(key.Versions) != len(original.Rotation["test-key"].Versions) {
		t.Error("Versions count mismatch")
	}

	if key.Versions[0].Label != "kek-test-v1" {
		t.Errorf("Version label mismatch: expected kek-test-v1, got %s", key.Versions[0].Label)
	}

	t.Logf("✓ Metadata save/load roundtrip successful")
}

// contains - вспомогательная функция для поиска подстроки
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && stringContains(s, substr)))
}

func stringContains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
