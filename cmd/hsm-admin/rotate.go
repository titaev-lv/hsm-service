package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"

	"github.com/titaev-lv/hsm-service/internal/config"
	"gopkg.in/yaml.v3"
)

// rotateKeyCommand rotates a KEK by creating a new version
func rotateKeyCommand(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: hsm-admin rotate <context>")
	}

	contextName := args[0]
	log.Printf("Starting rotation for context: %s", contextName)

	// 1. Load config to get metadata path
	cfg, err := config.LoadConfig(getConfigPath())
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}
	log.Printf("Config loaded successfully")

	// 2. Get metadata path
	metadataPath := cfg.HSM.MetadataFile
	if metadataPath == "" {
		metadataPath = "metadata.yaml"
	}

	// 3. Acquire exclusive lock on metadata file to prevent concurrent rotations
	lockFile, err := os.OpenFile(metadataPath+".lock", os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return fmt.Errorf("failed to create lock file: %w", err)
	}
	defer lockFile.Close()
	defer os.Remove(metadataPath + ".lock")

	// Acquire exclusive lock (blocks if another rotation is in progress)
	if err := syscall.Flock(int(lockFile.Fd()), syscall.LOCK_EX); err != nil {
		return fmt.Errorf("failed to acquire lock: %w", err)
	}
	defer syscall.Flock(int(lockFile.Fd()), syscall.LOCK_UN)

	// 4. Load metadata (after acquiring lock)
	log.Printf("Loading metadata from: %s", metadataPath)
	metadata, err := config.LoadMetadata(metadataPath)
	if err != nil {
		return fmt.Errorf("failed to load metadata: %w", err)
	}
	log.Printf("Loaded metadata with %d contexts", len(metadata.Rotation))

	// 5. Find the key in metadata by context
	keyMeta, found := metadata.Rotation[contextName]
	if !found {
		log.Printf("Available contexts: %v", getKeys(metadata.Rotation))
		return fmt.Errorf("key %s not found in metadata", contextName)
	}

	// 6. Get current version info
	if len(keyMeta.Versions) == 0 {
		return fmt.Errorf("no versions found for context: %s", contextName)
	}

	// Find current version by label
	var currentVersion *config.KeyVersion
	for i := range keyMeta.Versions {
		if keyMeta.Versions[i].Label == keyMeta.Current {
			currentVersion = &keyMeta.Versions[i]
			break
		}
	}
	if currentVersion == nil {
		return fmt.Errorf("current version %s not found in versions list", keyMeta.Current)
	}

	// Find the highest version number among all versions
	highestVersion := 0
	for _, v := range keyMeta.Versions {
		if v.Version > highestVersion {
			highestVersion = v.Version
		}
	}

	// Generate new version (increment from highest, not current)
	newVersion := highestVersion + 1

	// Parse label to extract base name and version
	// Example: kek-exchange-v1 -> kek-exchange-v2
	currentLabel := currentVersion.Label
	parts := strings.Split(currentLabel, "-v")
	if len(parts) != 2 {
		return fmt.Errorf("invalid key label format: %s (expected format: name-v1)", currentLabel)
	}

	baseName := parts[0]
	newLabel := fmt.Sprintf("%s-v%d", baseName, newVersion)

	// Check if this version already exists
	for _, v := range keyMeta.Versions {
		if v.Label == newLabel {
			return fmt.Errorf("version %s already exists, cannot create duplicate", newLabel)
		}
	}

	// 7. Get HSM PIN
	hsmPIN := os.Getenv("HSM_PIN")
	if hsmPIN == "" {
		return fmt.Errorf("HSM_PIN environment variable not set")
	}

	// 8. Create new KEK using create-kek utility (ID is now auto-generated)
	cmd := fmt.Sprintf("/app/create-kek %s %s %d", newLabel, hsmPIN, newVersion)

	log.Printf("Creating new KEK: %s", newLabel)
	if err := runCommand(cmd); err != nil {
		return fmt.Errorf("failed to create new KEK: %w", err)
	}

	// 9. Add new version to metadata
	now := time.Now()
	newKeyVersion := config.KeyVersion{
		Label:     newLabel,
		Version:   newVersion,
		CreatedAt: &now,
	}

	// Append new version and update current
	keyMeta.Versions = append(keyMeta.Versions, newKeyVersion)
	keyMeta.Current = newLabel

	metadata.Rotation[contextName] = keyMeta

	// 10. Backup old metadata
	backupPath := fmt.Sprintf("metadata.yaml.backup-%s", time.Now().Format("20060102-150405"))
	if err := copyFile(metadataPath, backupPath); err != nil {
		log.Printf("Warning: failed to create backup: %v", err)
	} else {
		log.Printf("Created metadata backup: %s", backupPath)
	}

	// 11. Write updated metadata with explicit sync
	data, err := yaml.Marshal(metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	// Open file for writing
	f, err := os.OpenFile(metadataPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("failed to open metadata for writing: %w", err)
	}
	defer f.Close()

	// Write data
	if _, err := f.Write(data); err != nil {
		return fmt.Errorf("failed to write metadata: %w", err)
	}

	// Force sync to disk before closing
	if err := f.Sync(); err != nil {
		log.Printf("Warning: failed to sync metadata to disk: %v", err)
	}

	log.Printf("✓ Key rotation completed:")
	log.Printf("  Context: %s", contextName)
	log.Printf("  Old key: %s (version %d)", currentLabel, currentVersion.Version)
	log.Printf("  New key: %s (version %d)", newLabel, newVersion)
	log.Printf("")
	log.Printf("⚠️  IMPORTANT:")
	log.Printf("  1. Restart the HSM service to load the new key")
	log.Printf("  2. Re-encrypt all data encrypted with the old key")
	log.Printf("  3. After 7 days overlap period, delete the old key:")
	log.Printf("     hsm-admin delete-kek --label %s --confirm", currentLabel)

	return nil
}

// runCommand executes a shell command
func runCommand(cmd string) error {
	parts := strings.Fields(cmd)
	if len(parts) == 0 {
		return fmt.Errorf("empty command")
	}

	log.Printf("Executing: %s", cmd)

	// Execute the command using exec.Command
	command := exec.Command(parts[0], parts[1:]...)
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr

	if err := command.Run(); err != nil {
		return fmt.Errorf("command failed: %w", err)
	}

	return nil
}

// copyFile copies a file from src to dst
func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0644)
}

// checkRotationStatus checks rotation status for all keys
func checkRotationStatusCommand() error {
	cfg, err := config.LoadConfig(getConfigPath())
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Load metadata
	metadataPath := cfg.HSM.MetadataFile
	if metadataPath == "" {
		metadataPath = "metadata.yaml"
	}
	metadata, err := config.LoadMetadata(metadataPath)
	if err != nil {
		return fmt.Errorf("failed to load metadata: %w", err)
	}

	fmt.Println("Key Rotation Status:")
	fmt.Println("====================")

	for context, keyMeta := range metadata.Rotation {
		// Get rotation interval from metadata (with fallback to 90 days)
		rotationIntervalDays := keyMeta.RotationIntervalDays
		if rotationIntervalDays == 0 {
			rotationIntervalDays = 90 // Default: 90 days (PCI DSS compliant)
		}

		// Get current version info
		var currentVersion *config.KeyVersion
		for i := range keyMeta.Versions {
			if keyMeta.Versions[i].Label == keyMeta.Current {
				currentVersion = &keyMeta.Versions[i]
				break
			}
		}

		if currentVersion == nil {
			fmt.Printf("\n⚠️  Context: %s\n", context)
			fmt.Printf("  Error: current version not found\n")
			continue
		}

		createdAt := time.Now()
		if currentVersion.CreatedAt != nil {
			createdAt = *currentVersion.CreatedAt
		}

		rotationInterval := time.Duration(rotationIntervalDays) * 24 * time.Hour
		nextRotation := createdAt.Add(rotationInterval)
		daysUntilRotation := int(time.Until(nextRotation).Hours() / 24)

		status := "OK"
		symbol := "✓"
		if time.Now().After(nextRotation) {
			status = "NEEDS ROTATION"
			symbol = "⚠️ "
			daysUntilRotation = -daysUntilRotation
		}

		fmt.Printf("\n%s Context: %s\n", symbol, context)
		fmt.Printf("  Current:           %s\n", keyMeta.Current)
		fmt.Printf("  Version:           %d\n", currentVersion.Version)
		fmt.Printf("  Total Versions:    %d\n", len(keyMeta.Versions))
		fmt.Printf("  Created:           %s\n", createdAt.Format("2006-01-02 15:04:05"))
		fmt.Printf("  Rotation Interval: %v\n", rotationInterval)
		fmt.Printf("  Next Rotation:     %s\n", nextRotation.Format("2006-01-02"))

		if time.Now().After(nextRotation) {
			fmt.Printf("  Status:            %s (%d days overdue)\n", status, daysUntilRotation)
		} else {
			fmt.Printf("  Status:            %s (%d days remaining)\n", status, daysUntilRotation)
		}
	}

	return nil
}

// getKeys returns the keys of a map as a slice
func getKeys(m map[string]config.KeyMetadata) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
