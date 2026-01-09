package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
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
	cfg, err := config.LoadConfig("config.yaml")
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}
	log.Printf("Config loaded successfully")

	// 2. Load metadata
	metadataPath := cfg.HSM.MetadataFile
	if metadataPath == "" {
		metadataPath = "metadata.yaml"
	}
	log.Printf("Loading metadata from: %s", metadataPath)
	metadata, err := config.LoadMetadata(metadataPath)
	if err != nil {
		return fmt.Errorf("failed to load metadata: %w", err)
	}
	log.Printf("Loaded metadata with %d contexts", len(metadata.Rotation))

	// 3. Find the key in metadata by context
	keyMeta, found := metadata.Rotation[contextName]
	if !found {
		log.Printf("Available contexts: %v", getKeys(metadata.Rotation))
		return fmt.Errorf("key %s not found in metadata", contextName)
	}

	// 4. Get current version info
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

	// Generate new version
	newVersion := currentVersion.Version + 1

	// Parse label to extract base name and version
	// Example: kek-exchange-v1 -> kek-exchange-v2
	currentLabel := currentVersion.Label
	parts := strings.Split(currentLabel, "-v")
	if len(parts) != 2 {
		return fmt.Errorf("invalid key label format: %s (expected format: name-v1)", currentLabel)
	}

	baseName := parts[0]
	newLabel := fmt.Sprintf("%s-v%d", baseName, newVersion)

	// 5. Get HSM PIN
	hsmPIN := os.Getenv("HSM_PIN")
	if hsmPIN == "" {
		return fmt.Errorf("HSM_PIN environment variable not set")
	}

	// 6. Create new KEK using create-kek utility (ID is now auto-generated)
	cmd := fmt.Sprintf("/app/create-kek %s %s %d", newLabel, hsmPIN, newVersion)

	log.Printf("Creating new KEK: %s", newLabel)
	if err := runCommand(cmd); err != nil {
		return fmt.Errorf("failed to create new KEK: %w", err)
	}

	// 7. Add new version to metadata
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

	// 8. Backup old metadata
	backupPath := fmt.Sprintf("metadata.yaml.backup-%s", time.Now().Format("20060102-150405"))
	if err := copyFile(metadataPath, backupPath); err != nil {
		log.Printf("Warning: failed to create backup: %v", err)
	} else {
		log.Printf("Created metadata backup: %s", backupPath)
	}

	// 9. Write updated metadata
	data, err := yaml.Marshal(metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	if err := os.WriteFile(metadataPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write metadata: %w", err)
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
	log.Printf("     hsm-admin delete %s", currentLabel)
	log.Printf("     hsm-admin delete %s", currentLabel)

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
	cfg, err := config.LoadConfig("config.yaml")
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
		// Get rotation interval from config
		keyConfig := cfg.HSM.Keys[context]

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

		rotationInterval := 90 * 24 * time.Hour
		if keyConfig.RotationInterval > 0 {
			rotationInterval = keyConfig.RotationInterval
		}

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
