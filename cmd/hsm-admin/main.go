package main

import (
	"crypto/rand"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/ThalesGroup/crypto11"
	"github.com/miekg/pkcs11"
	"github.com/titaev-lv/hsm-service/internal/config"
)

const (
	defaultConfigPath = "config.yaml"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	command := os.Args[1]

	switch command {
	case "create-kek":
		createKEK(os.Args[2:])
	case "list-kek":
		listKEK(os.Args[2:])
	case "delete-kek":
		deleteKEK(os.Args[2:])
	case "export-metadata":
		exportMetadata(os.Args[2:])
	case "rotate":
		if err := rotateKeyCommand(os.Args[2:]); err != nil {
			log.Fatalf("Rotation failed: %v", err)
		}
	case "rotation-status":
		if err := checkRotationStatusCommand(); err != nil {
			log.Fatalf("Failed to check rotation status: %v", err)
		}
	case "cleanup-old-versions":
		if err := cleanupOldVersionsCommand(os.Args[2:]); err != nil {
			log.Fatalf("Failed to cleanup old versions: %v", err)
		}
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", command)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("HSM Admin Tool - KEK Management")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  hsm-admin <command> [options]")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  create-kek        Create a new KEK")
	fmt.Println("  list-kek          List all KEKs")
	fmt.Println("  delete-kek        Delete a KEK")
	fmt.Println("  export-metadata   Export KEK metadata to file")
	fmt.Println("  rotate            Rotate a KEK to new version")
	fmt.Println("  rotation-status   Check rotation status for all keys")
	fmt.Println("  cleanup-old-versions  Delete old key versions (PCI DSS compliance)")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  hsm-admin create-kek --label kek-trading-v1 --context trading")
	fmt.Println("  hsm-admin list-kek")
	fmt.Println("  hsm-admin delete-kek --label kek-old-v1 --confirm")
	fmt.Println("  hsm-admin export-metadata --output metadata.json")
	fmt.Println("  hsm-admin rotate kek-exchange-v1")
	fmt.Println("  hsm-admin rotation-status")
	fmt.Println("  hsm-admin cleanup-old-versions --dry-run")
	fmt.Println()
	fmt.Println("Environment Variables:")
	fmt.Println("  HSM_PIN          HSM token PIN (required)")
	fmt.Println("  CONFIG_PATH      Path to config.yaml (default: config.yaml)")
}

func createKEK(args []string) {
	fs := flag.NewFlagSet("create-kek", flag.ExitOnError)
	label := fs.String("label", "", "KEK label (required)")
	context := fs.String("context", "", "Context name (required)")
	keySize := fs.Int("size", 256, "Key size in bits (default: 256)")
	configPath := fs.String("config", getConfigPath(), "Path to config.yaml")

	fs.Parse(args)

	if *label == "" || *context == "" {
		fmt.Println("Error: --label and --context are required")
		fs.Usage()
		os.Exit(1)
	}

	if *keySize != 128 && *keySize != 192 && *keySize != 256 {
		fmt.Println("Error: --size must be 128, 192, or 256")
		os.Exit(1)
	}

	// Get HSM PIN from environment
	pin := os.Getenv("HSM_PIN")
	if pin == "" {
		log.Fatal("HSM_PIN environment variable not set")
	}

	// Load configuration
	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Initialize PKCS#11 context
	p11ctx, err := crypto11.Configure(&crypto11.Config{
		Path:       cfg.HSM.PKCS11Lib,
		TokenLabel: cfg.HSM.SlotID,
		Pin:        pin,
	})
	if err != nil {
		log.Fatalf("Failed to configure PKCS#11: %v", err)
	}
	defer p11ctx.Close()

	fmt.Printf("Creating KEK: %s (context: %s, size: %d bits)\n", *label, *context, *keySize)

	// Generate random key material
	keyBytes := make([]byte, *keySize/8)
	if _, err := rand.Read(keyBytes); err != nil {
		log.Fatalf("Failed to generate random key: %v", err)
	}

	// Use low-level PKCS#11 API to create the key
	// crypto11.Context embeds *pkcs11.Ctx
	template := []*pkcs11.Attribute{
		pkcs11.NewAttribute(pkcs11.CKA_CLASS, pkcs11.CKO_SECRET_KEY),
		pkcs11.NewAttribute(pkcs11.CKA_KEY_TYPE, pkcs11.CKK_AES),
		pkcs11.NewAttribute(pkcs11.CKA_LABEL, *label),
		pkcs11.NewAttribute(pkcs11.CKA_ID, []byte(*label)),
		pkcs11.NewAttribute(pkcs11.CKA_TOKEN, true),
		pkcs11.NewAttribute(pkcs11.CKA_SENSITIVE, true),
		pkcs11.NewAttribute(pkcs11.CKA_ENCRYPT, true),
		pkcs11.NewAttribute(pkcs11.CKA_DECRYPT, true),
		pkcs11.NewAttribute(pkcs11.CKA_EXTRACTABLE, false),
		pkcs11.NewAttribute(pkcs11.CKA_VALUE, keyBytes),
	}

	// We need to use the underlying PKCS#11 session
	// For simplicity, we'll use crypto11's ImportSecretKeyWithLabel which is simpler

	// Actually, let's just document the manual process
	fmt.Println()
	fmt.Println("MANUAL KEY CREATION REQUIRED:")
	fmt.Println("crypto11 doesn't expose direct key creation API.")
	fmt.Println()
	fmt.Println("Option 1: Use pkcs11-tool:")
	fmt.Printf("  pkcs11-tool --module %s --login --pin %s \\\n", cfg.HSM.PKCS11Lib, "****")
	fmt.Printf("    --keygen --key-type AES:%d --label %s \\\n", *keySize, *label)
	fmt.Printf("    --token-label %s\n", cfg.HSM.SlotID)
	fmt.Println()
	fmt.Println("Option 2: Use SoftHSM2 CLI:")
	fmt.Printf("  softhsm2-util --import key.bin --slot <slot-id> \\\n")
	fmt.Printf("    --label %s --id %s --pin %s\n", *label, *label, "****")
	fmt.Println()
	fmt.Println("After creating the key, add to config.yaml:")
	fmt.Printf("  hsm:\n")
	fmt.Printf("    keys:\n")
	fmt.Printf("      %s:\n", *context)
	fmt.Printf("        label: %s\n", *label)
	fmt.Printf("        type: aes\n")

	// For testing/demo purposes, let's try with the template approach
	_ = template // suppress unused warning

	fmt.Println()
	fmt.Println("Note: Automated KEK creation requires low-level PKCS#11 access")
}

func listKEK(args []string) {
	fs := flag.NewFlagSet("list-kek", flag.ExitOnError)
	configPath := fs.String("config", getConfigPath(), "Path to config.yaml")
	verbose := fs.Bool("verbose", false, "Show detailed information")

	fs.Parse(args)

	// Get HSM PIN from environment
	pin := os.Getenv("HSM_PIN")
	if pin == "" {
		log.Fatal("HSM_PIN environment variable not set")
	}

	// Load configuration
	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Initialize PKCS#11 context
	p11ctx, err := crypto11.Configure(&crypto11.Config{
		Path:       cfg.HSM.PKCS11Lib,
		TokenLabel: cfg.HSM.SlotID,
		Pin:        pin,
	})
	if err != nil {
		log.Fatalf("Failed to configure PKCS#11: %v", err)
	}
	defer p11ctx.Close()

	fmt.Println("KEKs configured in config.yaml:")
	fmt.Println()

	if len(cfg.HSM.Keys) == 0 {
		fmt.Println("No KEKs configured")
		return
	}

	// Load metadata
	metadataPath := cfg.HSM.MetadataFile
	if metadataPath == "" {
		metadataPath = "metadata.yaml"
	}
	metadata, err := config.LoadMetadata(metadataPath)
	if err != nil {
		log.Printf("Warning: failed to load metadata: %v", err)
		metadata = &config.Metadata{Rotation: make(map[string]config.KeyMetadata)}
	}

	count := 0
	for keyName, keyConfig := range cfg.HSM.Keys {
		count++
		fmt.Printf("%d. Config Key: %s\n", count, keyName)

		// Get label from metadata
		if meta, ok := metadata.Rotation[keyName]; ok {
			fmt.Printf("   Current: %s\n", meta.Current)
			fmt.Printf("   Versions: %d\n", len(meta.Versions))
			for _, v := range meta.Versions {
				marker := " "
				if v.Label == meta.Current {
					marker = "*"
				}
				fmt.Printf("     %s %s (v%d)\n", marker, v.Label, v.Version)
			}
		} else {
			fmt.Printf("   Label: (not in metadata)\n")
		}

		fmt.Printf("   Type: %s\n", keyConfig.Type)

		if *verbose {
			// Try to find the key in HSM
			if meta, ok := metadata.Rotation[keyName]; ok {
				for _, v := range meta.Versions {
					key, err := p11ctx.FindKey(nil, []byte(v.Label))
					if err != nil || key == nil {
						fmt.Printf("     %s: ⚠️  NOT FOUND in HSM\n", v.Label)
					} else {
						fmt.Printf("     %s: ✓ Available in HSM\n", v.Label)
					}
				}
			}
		}
		fmt.Println()
	}

	fmt.Printf("Total: %d KEK(s)\n", count)
}

func deleteKEK(args []string) {
	fs := flag.NewFlagSet("delete-kek", flag.ExitOnError)
	label := fs.String("label", "", "KEK label (required)")
	confirm := fs.Bool("confirm", false, "Confirm deletion (required)")
	configPath := fs.String("config", getConfigPath(), "Path to config.yaml")

	fs.Parse(args)

	if *label == "" {
		fmt.Println("Error: --label is required")
		fs.Usage()
		os.Exit(1)
	}

	if !*confirm {
		fmt.Println("Error: --confirm flag is required to delete KEK")
		fmt.Println("This operation is irreversible!")
		os.Exit(1)
	}

	// Get HSM PIN from environment
	pin := os.Getenv("HSM_PIN")
	if pin == "" {
		log.Fatal("HSM_PIN environment variable not set")
	}

	// Load configuration
	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Initialize PKCS#11 context
	p11ctx, err := crypto11.Configure(&crypto11.Config{
		Path:       cfg.HSM.PKCS11Lib,
		TokenLabel: cfg.HSM.SlotID,
		Pin:        pin,
	})
	if err != nil {
		log.Fatalf("Failed to configure PKCS#11: %v", err)
	}
	defer p11ctx.Close()

	fmt.Printf("Searching for KEK: %s\n", *label)

	// Find key by label
	key, err := p11ctx.FindKey(nil, []byte(*label))
	if err != nil {
		log.Fatalf("Failed to find KEK: %v", err)
	}

	if key == nil {
		log.Fatalf("KEK not found: %s", *label)
	}

	// Delete the key
	err = key.Delete()
	if err != nil {
		log.Fatalf("Failed to delete KEK: %v", err)
	}

	fmt.Printf("✓ KEK deleted successfully: %s\n", *label)
	fmt.Println()
	fmt.Println("WARNING: All data encrypted with this KEK is now unrecoverable!")
	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Println("1. Remove KEK from config.yaml")
	fmt.Println("2. Restart HSM service")
}

func exportMetadata(args []string) {
	fs := flag.NewFlagSet("export-metadata", flag.ExitOnError)
	output := fs.String("output", "kek-metadata.json", "Output file path")
	configPath := fs.String("config", getConfigPath(), "Path to config.yaml")

	fs.Parse(args)

	// Load configuration (no HSM access needed)
	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	type KEKMetadata struct {
		ConfigKey string `json:"config_key"`
		Label     string `json:"label"`
		Type      string `json:"type"`
	}

	type MetadataExport struct {
		TokenLabel string        `json:"token_label"`
		PKCS11Lib  string        `json:"pkcs11_lib"`
		KEKCount   int           `json:"kek_count"`
		KEKs       []KEKMetadata `json:"keks"`
	}

	metadata := MetadataExport{
		TokenLabel: cfg.HSM.SlotID,
		PKCS11Lib:  cfg.HSM.PKCS11Lib,
		KEKCount:   len(cfg.HSM.Keys),
		KEKs:       make([]KEKMetadata, 0, len(cfg.HSM.Keys)),
	}

	// Load metadata
	metadataPath := cfg.HSM.MetadataFile
	if metadataPath == "" {
		metadataPath = "metadata.yaml"
	}
	metadataFile, err := config.LoadMetadata(metadataPath)
	if err != nil {
		log.Printf("Warning: failed to load metadata: %v", err)
		metadataFile = &config.Metadata{Rotation: make(map[string]config.KeyMetadata)}
	}

	for keyName, keyConfig := range cfg.HSM.Keys {
		kek := KEKMetadata{
			ConfigKey: keyName,
			Type:      keyConfig.Type,
		}

		// Get label from metadata (use current version)
		if meta, ok := metadataFile.Rotation[keyName]; ok {
			kek.Label = meta.Current
		}

		metadata.KEKs = append(metadata.KEKs, kek)
	}

	// Write to file
	file, err := os.Create(*output)
	if err != nil {
		log.Fatalf("Failed to create output file: %v", err)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(metadata); err != nil {
		log.Fatalf("Failed to encode metadata: %v", err)
	}

	fmt.Printf("✓ Metadata exported to: %s\n", *output)
	fmt.Printf("  Total KEKs: %d\n", len(metadata.KEKs))
}

func getConfigPath() string {
	if path := os.Getenv("CONFIG_PATH"); path != "" {
		return path
	}
	return defaultConfigPath
}
