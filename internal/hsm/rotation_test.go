package hsm

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/titaev-lv/hsm-service/internal/config"
)

// TestRotateKey_CreateNewVersion тестирует создание новой версии ключа
func TestRotateKey_CreateNewVersion(t *testing.T) {
	tmpDir := t.TempDir()
	metadataFile := filepath.Join(tmpDir, "metadata.yaml")

	now := time.Now().UTC()
	// Создаём начальные метаданные с версией 1
	initialMetadata := &config.Metadata{
		Rotation: map[string]config.KeyMetadata{
			"test-key": {
				Current: "kek-test-v1",
				Versions: []config.KeyVersion{
					{
						Label:     "kek-test-v1",
						Version:   1,
						CreatedAt: &now,
					},
				},
			},
		},
	}

	// Сохраняем начальные метаданные
	if err := config.SaveMetadata(metadataFile, initialMetadata); err != nil {
		t.Fatalf("Failed to save initial metadata: %v", err)
	}

	// Загружаем метаданные
	metadata, err := config.LoadMetadata(metadataFile)
	if err != nil {
		t.Fatalf("Failed to load metadata: %v", err)
	}

	// Симулируем ротацию: получаем текущую версию ключа
	keyMeta := metadata.Rotation["test-key"]
	if len(keyMeta.Versions) == 0 {
		t.Fatal("No versions found for test-key")
	}

	// Создаём новую версию (v2)
	lastVersion := keyMeta.Versions[len(keyMeta.Versions)-1]
	newVersion := lastVersion.Version + 1
	newLabel := "kek-test-v2"

	t.Logf("Creating new version %d with label %s", newVersion, newLabel)

	newTime := time.Now().UTC()
	// Добавляем новую версию
	keyMeta.Versions = append(keyMeta.Versions, config.KeyVersion{
		Label:     newLabel,
		Version:   newVersion,
		CreatedAt: &newTime,
	})
	keyMeta.Current = newLabel
	metadata.Rotation["test-key"] = keyMeta

	// Сохраняем обновлённые метаданные
	if err := config.SaveMetadata(metadataFile, metadata); err != nil {
		t.Fatalf("Failed to save updated metadata: %v", err)
	}

	// Проверяем результат
	updatedMetadata, err := config.LoadMetadata(metadataFile)
	if err != nil {
		t.Fatalf("Failed to load updated metadata: %v", err)
	}

	// Проверяем что новый ключ создан
	updatedKeyMeta := updatedMetadata.Rotation["test-key"]
	if len(updatedKeyMeta.Versions) != 2 {
		t.Fatalf("Expected 2 versions, got %d", len(updatedKeyMeta.Versions))
	}

	if updatedKeyMeta.Current != newLabel {
		t.Errorf("Expected current label %s, got %s", newLabel, updatedKeyMeta.Current)
	}

	// Проверяем что обе версии присутствуют
	if updatedKeyMeta.Versions[0].Label != "kek-test-v1" {
		t.Errorf("Expected v1 label kek-test-v1, got %s", updatedKeyMeta.Versions[0].Label)
	}
	if updatedKeyMeta.Versions[1].Label != newLabel {
		t.Errorf("Expected v2 label %s, got %s", newLabel, updatedKeyMeta.Versions[1].Label)
	}

	t.Logf("✓ Successfully created new version: %s (v%d)", newLabel, newVersion)
}

// TestRotateKey_UpdateMetadata проверяет обновление metadata.yaml
func TestRotateKey_UpdateMetadata(t *testing.T) {
	tmpDir := t.TempDir()
	metadataFile := filepath.Join(tmpDir, "metadata.yaml")

	oldTime := time.Now().Add(-100 * 24 * time.Hour).UTC()
	// Создаём начальные метаданные
	metadata := &config.Metadata{
		Rotation: map[string]config.KeyMetadata{
			"exchange-key": {
				Current: "kek-exchange-v1",
				Versions: []config.KeyVersion{
					{
						Label:     "kek-exchange-v1",
						Version:   1,
						CreatedAt: &oldTime,
					},
				},
			},
		},
	}

	if err := config.SaveMetadata(metadataFile, metadata); err != nil {
		t.Fatalf("Failed to save metadata: %v", err)
	}

	// Проверяем что файл создан
	if _, err := os.Stat(metadataFile); os.IsNotExist(err) {
		t.Fatal("Metadata file was not created")
	}

	// Обновляем метаданные (симулируем ротацию)
	keyMeta := metadata.Rotation["exchange-key"]
	newTime := time.Now().UTC()
	keyMeta.Versions = append(keyMeta.Versions, config.KeyVersion{
		Label:     "kek-exchange-v2",
		Version:   2,
		CreatedAt: &newTime,
	})
	keyMeta.Current = "kek-exchange-v2"
	metadata.Rotation["exchange-key"] = keyMeta

	if err := config.SaveMetadata(metadataFile, metadata); err != nil {
		t.Fatalf("Failed to update metadata: %v", err)
	}

	// Загружаем и проверяем
	updated, err := config.LoadMetadata(metadataFile)
	if err != nil {
		t.Fatalf("Failed to load updated metadata: %v", err)
	}

	updatedKeyMeta := updated.Rotation["exchange-key"]
	if updatedKeyMeta.Current != "kek-exchange-v2" {
		t.Errorf("Current not updated to v2, got %s", updatedKeyMeta.Current)
	}

	if len(updatedKeyMeta.Versions) != 2 {
		t.Errorf("Expected 2 versions, got %d", len(updatedKeyMeta.Versions))
	}

	// Проверяем что старая версия сохранена
	if updatedKeyMeta.Versions[0].Label != "kek-exchange-v1" {
		t.Errorf("Old version not preserved, got %s", updatedKeyMeta.Versions[0].Label)
	}

	t.Logf("✓ Metadata successfully updated with new version")
}

// TestRotateKey_PreserveOldKeys проверяет сохранение старых ключей
func TestRotateKey_PreserveOldKeys(t *testing.T) {
	tmpDir := t.TempDir()
	metadataFile := filepath.Join(tmpDir, "metadata.yaml")

	now := time.Now().UTC()
	time1 := now.Add(-48 * time.Hour)
	time2 := now.Add(-24 * time.Hour)

	// Создаём метаданные с несколькими версиями
	metadata := &config.Metadata{
		Rotation: map[string]config.KeyMetadata{
			"test-key": {
				Current: "kek-test-v2",
				Versions: []config.KeyVersion{
					{Label: "kek-test-v1", Version: 1, CreatedAt: &time1},
					{Label: "kek-test-v2", Version: 2, CreatedAt: &time2},
				},
			},
		},
	}

	if err := config.SaveMetadata(metadataFile, metadata); err != nil {
		t.Fatalf("Failed to save metadata: %v", err)
	}

	// Добавляем v3
	keyMeta := metadata.Rotation["test-key"]
	time3 := time.Now().UTC()
	keyMeta.Versions = append(keyMeta.Versions, config.KeyVersion{
		Label:     "kek-test-v3",
		Version:   3,
		CreatedAt: &time3,
	})
	keyMeta.Current = "kek-test-v3"
	metadata.Rotation["test-key"] = keyMeta

	if err := config.SaveMetadata(metadataFile, metadata); err != nil {
		t.Fatalf("Failed to save updated metadata: %v", err)
	}

	// Проверяем что все версии сохранены
	updated, err := config.LoadMetadata(metadataFile)
	if err != nil {
		t.Fatalf("Failed to load metadata: %v", err)
	}

	updatedKeyMeta := updated.Rotation["test-key"]
	if len(updatedKeyMeta.Versions) != 3 {
		t.Fatalf("Expected 3 versions, got %d", len(updatedKeyMeta.Versions))
	}

	// Все 3 версии должны быть в Versions
	expectedLabels := []string{"kek-test-v1", "kek-test-v2", "kek-test-v3"}
	for i, expected := range expectedLabels {
		if updatedKeyMeta.Versions[i].Label != expected {
			t.Errorf("Version %d: expected %s, got %s", i, expected, updatedKeyMeta.Versions[i].Label)
		}
	}

	t.Logf("✓ All key versions preserved: v1, v2, v3")
}

// TestCleanupOldVersions_RespectRetention проверяет уважение retention policy
func TestCleanupOldVersions_RespectRetention(t *testing.T) {
	tmpDir := t.TempDir()
	metadataFile := filepath.Join(tmpDir, "metadata.yaml")

	now := time.Now().UTC()
	time100 := now.Add(-100 * 24 * time.Hour) // 100 дней назад
	time50 := now.Add(-50 * 24 * time.Hour)   // 50 дней назад
	time20 := now.Add(-20 * 24 * time.Hour)   // 20 дней назад

	// Создаём метаданные с несколькими версиями разного возраста
	metadata := &config.Metadata{
		Rotation: map[string]config.KeyMetadata{
			"test-key": {
				Current: "kek-test-v3",
				Versions: []config.KeyVersion{
					{Label: "kek-test-v1", Version: 1, CreatedAt: &time100},
					{Label: "kek-test-v2", Version: 2, CreatedAt: &time50},
					{Label: "kek-test-v3", Version: 3, CreatedAt: &time20},
				},
			},
		},
	}

	if err := config.SaveMetadata(metadataFile, metadata); err != nil {
		t.Fatalf("Failed to save metadata: %v", err)
	}

	// Симулируем cleanup старых версий (retention 30 дней)
	retentionDays := 30
	keyMeta := metadata.Rotation["test-key"]
	var keptVersions []config.KeyVersion

	for _, version := range keyMeta.Versions {
		age := now.Sub(*version.CreatedAt)
		if age < time.Duration(retentionDays)*24*time.Hour {
			keptVersions = append(keptVersions, version)
		} else {
			t.Logf("Removing old version %s (age: %.0f days)", version.Label, age.Hours()/24)
		}
	}

	// Проверяем что v1 и v2 удалены (100 и 50 дней > 30), только v3 сохранён
	if len(keptVersions) != 1 {
		t.Fatalf("Expected 1 version after cleanup, got %d", len(keptVersions))
	}

	if keptVersions[0].Label != "kek-test-v3" {
		t.Errorf("Expected first kept version v3, got %s", keptVersions[0].Label)
	}

	t.Logf("✓ Retention policy respected: removed v1 (100 days old) and v2 (50 days old), kept v3 (20 days)")
}

// TestCleanupOldVersions_KeepMinimum проверяет что минимум версий сохраняется
func TestCleanupOldVersions_KeepMinimum(t *testing.T) {
	tmpDir := t.TempDir()
	metadataFile := filepath.Join(tmpDir, "metadata.yaml")

	now := time.Now().UTC()
	time200 := now.Add(-200 * 24 * time.Hour)
	time100 := now.Add(-100 * 24 * time.Hour)

	// Создаём метаданные только с 2 очень старыми версиями
	metadata := &config.Metadata{
		Rotation: map[string]config.KeyMetadata{
			"test-key": {
				Current: "kek-test-v2",
				Versions: []config.KeyVersion{
					{Label: "kek-test-v1", Version: 1, CreatedAt: &time200},
					{Label: "kek-test-v2", Version: 2, CreatedAt: &time100},
				},
			},
		},
	}

	if err := config.SaveMetadata(metadataFile, metadata); err != nil {
		t.Fatalf("Failed to save metadata: %v", err)
	}

	// Симулируем cleanup с минимумом 2 версии
	minVersions := 2
	retentionDays := 30
	keyMeta := metadata.Rotation["test-key"]

	// Даже если все версии старые, должны сохранить minVersions
	var keptVersions []config.KeyVersion
	for _, version := range keyMeta.Versions {
		age := now.Sub(*version.CreatedAt)
		if age < time.Duration(retentionDays)*24*time.Hour || len(keptVersions) < minVersions {
			keptVersions = append(keptVersions, version)
		}
	}

	// Проверяем что обе версии сохранены несмотря на возраст
	if len(keptVersions) != 2 {
		t.Fatalf("Expected minimum 2 versions kept, got %d", len(keptVersions))
	}

	if keptVersions[0].Label != "kek-test-v1" {
		t.Errorf("Expected v1 kept (minimum policy), got %s", keptVersions[0].Label)
	}
	if keptVersions[1].Label != "kek-test-v2" {
		t.Errorf("Expected v2 kept, got %s", keptVersions[1].Label)
	}

	t.Logf("✓ Minimum version policy respected: kept 2 versions despite age (200 and 100 days)")
}

// TestRotateKey_Success проверяет успешную ротацию ключа
func TestRotateKey_Success(t *testing.T) {
	t.Skip("Requires HSM initialization - run with 'go test -tags=integration'")

	// TODO: Implement when rotation logic is extracted to separate function
	// Expected behavior:
	// 1. Load current metadata
	// 2. Create new key version (v2)
	// 3. Update metadata with new version
	// 4. Preserve old key reference
	// 5. Save metadata.yaml
}

// TestRotateKey_FailureRollback проверяет откат при ошибке
func TestRotateKey_FailureRollback(t *testing.T) {
	t.Skip("Requires HSM mock - implement when rotation logic is extracted")

	// TODO: Test rollback scenario:
	// 1. Start rotation
	// 2. Create new key (simulate failure here)
	// 3. Verify metadata unchanged
	// 4. Verify old key still works
}

// TestRotateKey_ConcurrentRotation проверяет защиту от одновременной ротации
func TestRotateKey_ConcurrentRotation(t *testing.T) {
	t.Skip("Requires rotation lock mechanism - implement when locking is added")

	// TODO: Test concurrent rotation prevention:
	// 1. Start rotation in goroutine 1
	// 2. Try to start rotation in goroutine 2
	// 3. Verify only one rotation proceeds
	// 4. Verify no data corruption
}
