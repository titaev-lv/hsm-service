package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/ThalesGroup/crypto11"
	"github.com/titaev-lv/hsm-service/internal/config"
	"gopkg.in/yaml.v3"
)

func cleanupOldVersionsCommand(args []string) error {
	fs := flag.NewFlagSet("cleanup-old-versions", flag.ExitOnError)
	configPath := fs.String("config", getConfigPath(), "Path to config.yaml")
	dryRun := fs.Bool("dry-run", false, "Show what would be deleted without actually deleting")
	force := fs.Bool("force", false, "Force deletion without confirmation")

	fs.Parse(args)

	// Get HSM PIN from environment
	pin := os.Getenv("HSM_PIN")
	if pin == "" {
		return fmt.Errorf("HSM_PIN environment variable not set")
	}

	// Load configuration
	cfg, err := config.LoadConfig(*configPath)
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

	// Get cleanup parameters
	maxVersions := cfg.HSM.MaxVersions
	if maxVersions == 0 {
		maxVersions = 3 // Default
	}
	cleanupAfterDays := cfg.HSM.CleanupAfterDays
	if cleanupAfterDays == 0 {
		cleanupAfterDays = 30 // Default
	}

	fmt.Println("=== PCI DSS Key Cleanup ===")
	fmt.Printf("Max versions to keep: %d\n", maxVersions)
	fmt.Printf("Delete versions older than: %d days\n", cleanupAfterDays)
	if *dryRun {
		fmt.Println("DRY RUN MODE - No changes will be made")
	}
	fmt.Println()

	// Initialize PKCS#11 context if not dry-run
	var p11ctx *crypto11.Context
	if !*dryRun {
		p11ctx, err = crypto11.Configure(&crypto11.Config{
			Path:       cfg.HSM.PKCS11Lib,
			TokenLabel: cfg.HSM.SlotID,
			Pin:        pin,
		})
		if err != nil {
			return fmt.Errorf("failed to configure PKCS#11: %w", err)
		}
		defer p11ctx.Close()
	}

	now := time.Now()
	cutoffDate := now.AddDate(0, 0, -cleanupAfterDays)

	totalDeleted := 0
	modified := false

	for contextName, keyMeta := range metadata.Rotation {
		fmt.Printf("Context: %s (current: %s)\n", contextName, keyMeta.Current)

		if len(keyMeta.Versions) <= 1 {
			fmt.Println("  ✓ Only 1 version, skipping")
			continue
		}

		// Find versions to delete
		var toDelete []config.KeyVersion
		var toKeep []config.KeyVersion

		for _, version := range keyMeta.Versions {
			// Never delete current version
			if version.Label == keyMeta.Current {
				toKeep = append(toKeep, version)
				continue
			}

			// Check age
			shouldDelete := false
			if version.CreatedAt != nil && version.CreatedAt.Before(cutoffDate) {
				shouldDelete = true
				fmt.Printf("  ⚠ %s (v%d) - created %v - TOO OLD\n",
					version.Label, version.Version, version.CreatedAt.Format("2006-01-02"))
			}

			// Check version limit (keep only maxVersions-1 old versions + current)
			if len(toKeep)+len(toDelete) >= maxVersions {
				if !shouldDelete {
					shouldDelete = true
					fmt.Printf("  ⚠ %s (v%d) - EXCEEDS MAX VERSIONS\n",
						version.Label, version.Version)
				}
			}

			if shouldDelete {
				toDelete = append(toDelete, version)
			} else {
				toKeep = append(toKeep, version)
			}
		}

		if len(toDelete) == 0 {
			fmt.Println("  ✓ No versions to delete")
			continue
		}

		// Confirm deletion
		if !*dryRun && !*force {
			fmt.Printf("\n  Delete %d versions? (yes/no): ", len(toDelete))
			var response string
			fmt.Scanln(&response)
			if response != "yes" {
				fmt.Println("  Skipped")
				continue
			}
		}

		// Delete from HSM and metadata
		for _, version := range toDelete {
			if !*dryRun {
				// Delete from HSM
				key, err := p11ctx.FindKey(nil, []byte(version.Label))
				if err != nil {
					log.Printf("  ⚠ Failed to find key %s: %v", version.Label, err)
					continue
				}
				if key != nil {
					if err := key.Delete(); err != nil {
						log.Printf("  ✗ Failed to delete key %s: %v", version.Label, err)
						continue
					}
				}
				fmt.Printf("  ✓ Deleted %s (v%d) from HSM\n", version.Label, version.Version)
			} else {
				fmt.Printf("  [DRY-RUN] Would delete %s (v%d)\n", version.Label, version.Version)
			}
			totalDeleted++
		}

		// Update metadata
		if !*dryRun {
			keyMeta.Versions = toKeep
			metadata.Rotation[contextName] = keyMeta
			modified = true
		}

		fmt.Printf("  Summary: kept %d, deleted %d\n", len(toKeep), len(toDelete))
	}

	// Save updated metadata
	if modified && !*dryRun {
		// Backup old metadata (copy instead of rename for bind mounts)
		backupPath := metadataPath + ".backup." + time.Now().Format("20060102-150405")
		oldData, err := os.ReadFile(metadataPath)
		if err != nil {
			log.Printf("Warning: failed to read metadata for backup: %v", err)
		} else {
			if err := os.WriteFile(backupPath, oldData, 0644); err != nil {
				log.Printf("Warning: failed to write backup: %v", err)
			} else {
				fmt.Printf("\n✓ Old metadata backed up to: %s\n", backupPath)
			}
		}

		// Save new metadata
		data, err := yaml.Marshal(metadata)
		if err != nil {
			return fmt.Errorf("failed to marshal metadata: %w", err)
		}
		if err := os.WriteFile(metadataPath, data, 0644); err != nil {
			return fmt.Errorf("failed to write metadata: %w", err)
		}
		fmt.Printf("✓ Metadata updated: %s\n", metadataPath)
	}

	fmt.Println()
	if *dryRun {
		fmt.Printf("DRY RUN COMPLETE - Would delete %d versions\n", totalDeleted)
	} else {
		fmt.Printf("CLEANUP COMPLETE - Deleted %d versions\n", totalDeleted)
	}

	return nil
}
