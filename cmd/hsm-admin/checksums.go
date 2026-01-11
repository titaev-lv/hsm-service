package main

import (
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/ThalesGroup/crypto11"
	"github.com/titaev-lv/hsm-service/internal/config"
	"gopkg.in/yaml.v3"
)

// updateChecksumsCommand computes SHA-256 checksums for all KEKs and updates metadata.yaml
func updateChecksumsCommand(args []string) error {
	fs := flag.NewFlagSet("update-checksums", flag.ExitOnError)
	configPath := fs.String("config", getConfigPath(), "Path to config.yaml")
	dryRun := fs.Bool("dry-run", false, "Show what would be updated without making changes")

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

	// Initialize PKCS#11 context
	p11ctx, err := crypto11.Configure(&crypto11.Config{
		Path:       cfg.HSM.PKCS11Lib,
		TokenLabel: cfg.HSM.SlotID,
		Pin:        pin,
	})
	if err != nil {
		return fmt.Errorf("failed to configure PKCS#11: %w", err)
	}
	defer p11ctx.Close()

	fmt.Println("Computing KEK checksums...")
	fmt.Println()

	updatedCount := 0
	for context, meta := range metadata.Rotation {
		fmt.Printf("Context: %s\n", context)

		for i, version := range meta.Versions {
			// Find key in HSM
			secretKey, err := p11ctx.FindKey(nil, []byte(version.Label))
			if err != nil {
				log.Printf("  Warning: failed to find key %s: %v", version.Label, err)
				continue
			}

			if secretKey == nil {
				log.Printf("  Warning: key %s not found in HSM", version.Label)
				continue
			}

			// Compute checksum (label-based, same as in pkcs11.go)
			h := sha256.New()
			h.Write([]byte(version.Label))
			checksum := hex.EncodeToString(h.Sum(nil))

			// Check if update needed
			if version.Checksum == checksum {
				fmt.Printf("  ✓ %s (v%d): checksum already up-to-date (%s...)\n",
					version.Label, version.Version, checksum[:8])
			} else {
				if version.Checksum == "" {
					fmt.Printf("  + %s (v%d): NEW checksum %s...\n",
						version.Label, version.Version, checksum[:8])
				} else {
					fmt.Printf("  ⚠ %s (v%d): checksum UPDATE %s... → %s...\n",
						version.Label, version.Version, version.Checksum[:8], checksum[:8])
				}

				// Update checksum in metadata
				if !*dryRun {
					metadata.Rotation[context].Versions[i].Checksum = checksum
				}
				updatedCount++
			}
		}
		fmt.Println()
	}

	if updatedCount == 0 {
		fmt.Println("✓ All checksums are up-to-date")
		return nil
	}

	if *dryRun {
		fmt.Printf("DRY RUN: Would update %d checksum(s)\n", updatedCount)
		fmt.Println("Run without --dry-run to apply changes")
		return nil
	}

	// Save updated metadata
	fmt.Printf("Saving updated metadata to %s...\n", metadataPath)

	file, err := os.Create(metadataPath)
	if err != nil {
		return fmt.Errorf("failed to create metadata file: %w", err)
	}
	defer file.Close()

	encoder := yaml.NewEncoder(file)
	defer encoder.Close()

	if err := encoder.Encode(metadata); err != nil {
		return fmt.Errorf("failed to encode metadata: %w", err)
	}

	fmt.Printf("✓ Updated %d checksum(s) successfully\n", updatedCount)
	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Println("1. Restart HSM service to verify checksums on startup")
	fmt.Println("2. Commit metadata.yaml to version control")

	return nil
}
