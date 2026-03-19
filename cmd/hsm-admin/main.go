package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/ThalesGroup/crypto11"
	"github.com/titaev-lv/hsm-service/internal/config"
)

const (
	defaultConfigPath = "config.yaml"
)

var (
	exitFunc   = os.Exit
	fatalfFunc = log.Fatalf
	fatalFunc  = log.Fatal
	newHSMCtx  = defaultNewHSMCtx
)

type hsmKey interface {
	Delete() error
}

type hsmContext interface {
	FindKey(id []byte, label []byte) (hsmKey, error)
	Close()
}

type crypto11ContextAdapter struct {
	ctx *crypto11.Context
}

func (a *crypto11ContextAdapter) FindKey(id []byte, label []byte) (hsmKey, error) {
	return a.ctx.FindKey(id, label)
}

func (a *crypto11ContextAdapter) Close() {
	a.ctx.Close()
}

func defaultNewHSMCtx(cfg *config.Config, pin string) (hsmContext, error) {
	p11ctx, err := crypto11.Configure(&crypto11.Config{
		Path:       cfg.HSM.PKCS11Lib,
		TokenLabel: cfg.HSM.SlotID,
		Pin:        pin,
	})
	if err != nil {
		return nil, err
	}
	return &crypto11ContextAdapter{ctx: p11ctx}, nil
}

func main() {
	if len(os.Args) < 2 {
		printUsage()
		exitFunc(1)
	}

	// Parse global flags
	configFlag := flag.String("config", "", "Path to config.yaml")
	configFlagC := flag.String("c", "", "Path to config.yaml (short form)")
	flag.Parse()

	// Determine config path (set as environment variable for subcommands)
	configPath := *configFlag
	if configPath == "" && *configFlagC != "" {
		configPath = *configFlagC
	}
	if configPath != "" {
		if err := os.Setenv("CONFIG_PATH", configPath); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to set CONFIG_PATH: %v\n", err)
		}
	}

	// Get command from remaining arguments
	args := flag.Args()
	if len(args) == 0 {
		printUsage()
		exitFunc(1)
	}

	command := args[0]

	switch command {
	case "create-kek":
		if err := createKEKCommand(args[1:]); err != nil {
			fatalfFunc("Create KEK failed: %v", err)
		}
	case "list-kek":
		listKEK(args[1:])
	case "delete-kek":
		deleteKEK(args[1:])
	case "export-metadata":
		exportMetadata(args[1:])
	case "rotate":
		if err := rotateKeyCommand(args[1:]); err != nil {
			fatalfFunc("Rotation failed: %v", err)
		}
	case "rotation-status":
		if err := checkRotationStatusCommand(); err != nil {
			fatalfFunc("Failed to check rotation status: %v", err)
		}
	case "cleanup-old-versions":
		if err := cleanupOldVersionsCommand(args[1:]); err != nil {
			fatalfFunc("Failed to cleanup old versions: %v", err)
		}
	case "update-checksums":
		if err := updateChecksumsCommand(args[1:]); err != nil {
			fatalfFunc("Failed to update checksums: %v", err)
		}
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", command)
		printUsage()
		exitFunc(1)
	}
}

func printUsage() {
	fmt.Println("HSM Admin Tool - KEK Management")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  hsm-admin [global-options] <command> [command-options]")
	fmt.Println()
	fmt.Println("Global Options:")
	fmt.Println("  -config <path>   Path to config.yaml")
	fmt.Println("  -c <path>        Short form for -config")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  create-kek        Create a new KEK")
	fmt.Println("  list-kek          List all KEKs")
	fmt.Println("  delete-kek        Delete a KEK")
	fmt.Println("  export-metadata   Export KEK metadata to file")
	fmt.Println("  rotate            Rotate a KEK to new version")
	fmt.Println("  rotation-status   Check rotation status for all keys")
	fmt.Println("  cleanup-old-versions  Delete old key versions (PCI DSS compliance)")
	fmt.Println("  update-checksums  Compute and update KEK checksums (integrity verification)")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  hsm-admin --config /etc/hsm-service/config.yaml list-kek")
	fmt.Println("  hsm-admin -c /etc/hsm-service/config.yaml update-checksums")
	fmt.Println("  hsm-admin create-kek --label kek-trading-v1 --context trading")
	fmt.Println("  hsm-admin list-kek")
	fmt.Println("  hsm-admin delete-kek --label kek-old-v1 --confirm")
	fmt.Println("  hsm-admin export-metadata --output metadata.json")
	fmt.Println("  hsm-admin rotate kek-exchange-v1")
	fmt.Println("  hsm-admin rotation-status")
	fmt.Println("  hsm-admin cleanup-old-versions --dry-run")
	fmt.Println("  hsm-admin update-checksums")
	fmt.Println()
	fmt.Println("Environment Variables:")
	fmt.Println("  HSM_PIN          HSM token PIN (required)")
	fmt.Println("  CONFIG_PATH      Path to config.yaml (searched: /etc/hsm-service/config.yaml if not set)")
}

func listKEK(args []string) {
	fs := flag.NewFlagSet("list-kek", flag.ExitOnError)
	configPath := fs.String("config", getConfigPath(), "Path to config.yaml")
	verbose := fs.Bool("verbose", false, "Show detailed information")

	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing flags: %v\n", err)
		exitFunc(1)
	}

	// Get HSM PIN from environment
	pin := os.Getenv("HSM_PIN")
	if pin == "" {
		fatalFunc("HSM_PIN environment variable not set")
	}

	// Load configuration
	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		fatalfFunc("Failed to load config: %v", err)
	}

	// Initialize PKCS#11 context
	p11ctx, err := newHSMCtx(cfg, pin)
	if err != nil {
		fatalfFunc("Failed to configure PKCS#11: %v", err)
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

	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing flags: %v\n", err)
		exitFunc(1)
	}

	if *label == "" {
		fmt.Println("Error: --label is required")
		fs.Usage()
		exitFunc(1)
	}

	if !*confirm {
		fmt.Println("Error: --confirm flag is required to delete KEK")
		fmt.Println("This operation is irreversible!")
		exitFunc(1)
	}

	// Get HSM PIN from environment
	pin := os.Getenv("HSM_PIN")
	if pin == "" {
		fatalFunc("HSM_PIN environment variable not set")
	}

	// Load configuration
	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		fatalfFunc("Failed to load config: %v", err)
	}

	// Initialize PKCS#11 context
	p11ctx, err := newHSMCtx(cfg, pin)
	if err != nil {
		fatalfFunc("Failed to configure PKCS#11: %v", err)
	}
	defer p11ctx.Close()

	fmt.Printf("Searching for KEK: %s\n", *label)

	// Find key by label
	key, err := p11ctx.FindKey(nil, []byte(*label))
	if err != nil {
		fatalfFunc("Failed to find KEK: %v", err)
	}

	if key == nil {
		fatalfFunc("KEK not found: %s", *label)
	}

	// Delete the key
	err = key.Delete()
	if err != nil {
		fatalfFunc("Failed to delete KEK: %v", err)
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

	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing flags: %v\n", err)
		exitFunc(1)
	}

	// Load configuration (no HSM access needed)
	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		fatalfFunc("Failed to load config: %v", err)
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
		fatalfFunc("Failed to create output file: %v", err)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(metadata); err != nil {
		fatalfFunc("Failed to encode metadata: %v", err)
	}

	fmt.Printf("✓ Metadata exported to: %s\n", *output)
	fmt.Printf("  Total KEKs: %d\n", len(metadata.KEKs))
}

func getConfigPath() string {
	// 1. Check CONFIG_PATH environment variable
	if path := os.Getenv("CONFIG_PATH"); path != "" {
		return path
	}

	// 2. Check in current directory
	if _, err := os.Stat(defaultConfigPath); err == nil {
		return defaultConfigPath
	}

	// 3. Check in /etc/hsm-service/
	etcPath := "/etc/hsm-service/config.yaml"
	if _, err := os.Stat(etcPath); err == nil {
		return etcPath
	}

	// 4. Return default (will cause error if file doesn't exist)
	return defaultConfigPath
}
