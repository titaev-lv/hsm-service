package main

import (
	"context"
	"io"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/titaev-lv/hsm-service/internal/config"
	"github.com/titaev-lv/hsm-service/internal/hsm"
	"github.com/titaev-lv/hsm-service/internal/server"
	"gopkg.in/natefinch/lumberjack.v2"
)

func main() {
	// 0. Setup log rotation (A09:2021 security requirement)
	logWriter := &lumberjack.Logger{
		Filename:   "/var/log/hsm-service/hsm-service.log",
		MaxSize:    100, // MB
		MaxBackups: 10,  // Keep 10 old log files
		MaxAge:     30,  // Keep logs for 30 days
		Compress:   true,
	}

	// Use both file and stdout for logs
	multiWriter := io.MultiWriter(os.Stdout, logWriter)
	logger := slog.New(slog.NewJSONHandler(multiWriter, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	// Also configure standard log package for compatibility
	log.SetOutput(multiWriter)

	// 1. Load configuration
	cfg, err := config.LoadConfig("config.yaml")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// 2. Load metadata
	metadataPath := cfg.HSM.MetadataFile
	if metadataPath == "" {
		metadataPath = "metadata.yaml" // Default fallback
	}
	metadata, err := config.LoadMetadata(metadataPath)
	if err != nil {
		log.Fatalf("Failed to load metadata: %v", err)
	}

	// 3. Get HSM PIN from environment variable
	hsmPIN := os.Getenv("HSM_PIN")
	if hsmPIN == "" {
		log.Fatal("HSM_PIN environment variable not set")
	}

	// 4. Initialize HSM context
	hsmCtx, err := hsm.InitHSM(&cfg.HSM, metadata, hsmPIN)
	if err != nil {
		log.Fatalf("Failed to initialize HSM: %v", err)
	}
	// Note: Close HSM context manually in shutdown handler to avoid panic

	// 4a. Create KeyManager with hot reload capability
	keyManager, err := hsm.NewKeyManager(hsmCtx.GetContext(), &cfg.HSM, metadata)
	if err != nil {
		log.Fatalf("Failed to create key manager: %v", err)
	}

	// 4b. Start auto-reload for metadata.yaml (30 seconds interval)
	keyManager.StartAutoReload(30 * time.Second)
	log.Println("✓ Started metadata hot reload (30s interval)")

	// 4c. Auto-cleanup old key versions (PCI DSS compliance)
	if err := performAutoCleanup(&cfg.HSM, metadata); err != nil {
		log.Printf("⚠️  Warning: auto-cleanup failed: %v", err)
	}

	// 4d. Check for keys needing rotation
	keysNeedingRotation := keyManager.GetKeysNeedingRotation()
	if len(keysNeedingRotation) > 0 {
		log.Printf("⚠️  WARNING: The following keys need rotation:")
		for _, label := range keysNeedingRotation {
			meta, _ := keyManager.GetKeyMetadata(label)
			log.Printf("  - %s (created: %s, rotation interval: %s, version: %d)",
				label, meta.CreatedAt.Format("2006-01-02"), meta.RotationInterval, meta.Version)
		}
		log.Printf("⚠️  Run 'hsm-admin rotate <label>' to rotate keys")
	}

	// 5. Initialize ACL checker
	aclChecker, err := server.NewACLChecker(&cfg.ACL)
	if err != nil {
		log.Fatalf("Failed to initialize ACL checker: %v", err)
	}

	// 5. Create rate limiter
	rateLimiter := server.NewRateLimiter(
		cfg.RateLimit.RequestsPerSecond,
		cfg.RateLimit.Burst,
	)

	// 6. Create server with all components
	srv, err := server.NewServer(&cfg.Server, keyManager, aclChecker, rateLimiter)
	if err != nil {
		log.Fatalf("Failed to create server: %v", err)
	}

	// 7. Setup graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// 8. Start server in goroutine
	errChan := make(chan error, 1)
	go func() {
		log.Printf("Starting HSM service on port %s", cfg.Server.Port)
		if err := srv.Start(); err != nil {
			errChan <- err
		}
	}()

	// 9. Wait for shutdown signal or error
	select {
	case err := <-errChan:
		log.Fatalf("Server error: %v", err)
	case sig := <-sigChan:
		log.Printf("Received signal %v, shutting down gracefully...", sig)

		// Create shutdown context with timeout
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		// 1. Stop metadata auto-reload
		log.Println("Stopping metadata auto-reload...")
		if err := keyManager.StopAutoReload(shutdownCtx); err != nil {
			log.Printf("Warning: metadata auto-reload stop timeout: %v", err)
		}

		// 2. Stop ACL auto-reload
		log.Println("Stopping ACL auto-reload...")
		if err := aclChecker.StopAutoReload(shutdownCtx); err != nil {
			log.Printf("Warning: ACL auto-reload stop timeout: %v", err)
		}

		// 3. Stop HTTP server
		log.Println("Stopping HTTP server...")
		if err := srv.Shutdown(); err != nil {
			log.Printf("Error during shutdown: %v", err)
		}

		// 4. Close KeyManager (which closes HSM context)
		log.Println("Closing KeyManager...")
		func() {
			defer func() {
				if r := recover(); r != nil {
					log.Printf("Recovered from panic during KeyManager cleanup: %v", r)
				}
			}()
			if err := keyManager.Close(); err != nil {
				log.Printf("Error closing KeyManager: %v", err)
			}
		}()
	}

	log.Println("HSM service stopped")
}

// performAutoCleanup performs automatic cleanup of old key versions on startup
func performAutoCleanup(hsmCfg *config.HSMConfig, metadata *config.Metadata) error {
	maxVersions := hsmCfg.MaxVersions
	if maxVersions == 0 {
		maxVersions = 3 // Default
	}
	cleanupAfterDays := hsmCfg.CleanupAfterDays
	if cleanupAfterDays == 0 {
		cleanupAfterDays = 30 // Default
	}

	log.Printf("🧹 Auto-cleanup: max_versions=%d, cleanup_after_days=%d", maxVersions, cleanupAfterDays)

	// For auto-cleanup, we only check version count limits, not age
	// Age-based cleanup is done manually via hsm-admin for safety
	deleted := 0
	for contextName, keyMeta := range metadata.Rotation {
		if len(keyMeta.Versions) <= maxVersions {
			continue
		}

		// Too many versions - need cleanup
		excessCount := len(keyMeta.Versions) - maxVersions
		log.Printf("⚠️  Context '%s' has %d versions (limit: %d) - manual cleanup recommended",
			contextName, len(keyMeta.Versions), maxVersions)
		log.Printf("   Run: hsm-admin cleanup-old-versions --dry-run")
		deleted += excessCount
	}

	if deleted > 0 {
		log.Printf("⚠️  Total %d excess versions detected - use 'hsm-admin cleanup-old-versions'", deleted)
	} else {
		log.Printf("✓ All key versions within limits")
	}

	return nil
}
