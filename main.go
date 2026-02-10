package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/titaev-lv/hsm-service/internal/config"
	"github.com/titaev-lv/hsm-service/internal/hsm"
	"github.com/titaev-lv/hsm-service/internal/server"
)

func main() {
	// 1. Load configuration
	configPath := getConfigPath()
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		slog.Error("failed to load config", "path", configPath, "error", err)
		os.Exit(1)
	}
	if err := server.ValidateLogPaths(&cfg.Logging); err != nil {
		slog.Error("log path validation failed", "error", err)
		os.Exit(1)
	}
	if err := server.InitLogger(&cfg.Logging); err != nil {
		slog.Error("failed to initialize logger", "error", err)
		os.Exit(1)
	}

	logServer := slog.With("module", "server")
	logKeyManager := slog.With("module", "keymanager")

	// 2. Load metadata
	metadataPath := cfg.HSM.MetadataFile
	if metadataPath == "" {
		metadataPath = "metadata.yaml" // Default fallback
	}
	metadata, err := config.LoadMetadata(metadataPath)
	if err != nil {
		logServer.Error("failed to load metadata", "error", err)
		os.Exit(1)
	}

	// 3. Get HSM PIN from environment variable
	hsmPIN := os.Getenv("HSM_PIN")
	if hsmPIN == "" {
		logServer.Error("HSM_PIN environment variable not set")
		os.Exit(1)
	}

	// 4. Initialize HSM context
	hsmCtx, err := hsm.InitHSM(&cfg.HSM, metadata, hsmPIN)
	if err != nil {
		logServer.Error("failed to initialize HSM", "error", err)
		os.Exit(1)
	}
	// Note: Close HSM context manually in shutdown handler to avoid panic

	// 4a. Create KeyManager with hot reload capability
	keyManager, err := hsm.NewKeyManager(hsmCtx.GetContext(), cfg, metadata)
	if err != nil {
		logServer.Error("failed to create key manager", "error", err)
		os.Exit(1)
	}

	// 4b. Start auto-reload for metadata.yaml (30 seconds interval)
	keyManager.StartAutoReload(30 * time.Second)
	logKeyManager.Info("metadata hot reload started", "interval_seconds", 30)

	// 4c. Auto-cleanup old key versions (PCI DSS compliance)
	if err := performAutoCleanup(&cfg.HSM, metadata); err != nil {
		logKeyManager.Warn("auto-cleanup failed", "error", err)
	}

	// 4d. Check for keys needing rotation
	keysNeedingRotation := keyManager.GetKeysNeedingRotation()
	if len(keysNeedingRotation) > 0 {
		logKeyManager.Warn("keys need rotation", "count", len(keysNeedingRotation))
		for _, label := range keysNeedingRotation {
			meta, _ := keyManager.GetKeyMetadata(label)
			logKeyManager.Warn("key needs rotation",
				"label", label,
				"created", meta.CreatedAt.Format("2006-01-02"),
				"rotation_interval", meta.RotationInterval,
				"version", meta.Version,
			)
		}
		logKeyManager.Warn("run hsm-admin rotate <label> to rotate keys")
	}

	// 5. Initialize ACL checker
	aclChecker, err := server.NewACLChecker(&cfg.ACL)
	if err != nil {
		logServer.Error("failed to initialize ACL checker", "error", err)
		os.Exit(1)
	}

	// 5. Create rate limiter
	rateLimiter := server.NewRateLimiter(
		cfg.RateLimit.RequestsPerSecond,
		cfg.RateLimit.Burst,
	)

	// 6. Create server with all components
	srv, err := server.NewServer(&cfg.Server, keyManager, aclChecker, rateLimiter)
	if err != nil {
		logServer.Error("failed to create server", "error", err)
		os.Exit(1)
	}

	// 7. Setup graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// 8. Start server in goroutine
	errChan := make(chan error, 1)
	go func() {
		logServer.Info("starting HSM service", "port", cfg.Server.Port)
		if err := srv.Start(); err != nil {
			errChan <- err
		}
	}()

	// 9. Wait for shutdown signal or error
	select {
	case err := <-errChan:
		logServer.Error("server error", "error", err)
		os.Exit(1)
	case sig := <-sigChan:
		logServer.Info("received signal, shutting down", "signal", sig.String())

		// Create shutdown context with timeout
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		// 1. Stop metadata auto-reload
		logKeyManager.Info("stopping metadata auto-reload")
		if err := keyManager.StopAutoReload(shutdownCtx); err != nil {
			logKeyManager.Warn("metadata auto-reload stop timeout", "error", err)
		}

		// 2. Stop ACL auto-reload
		logServer.Info("stopping ACL auto-reload")
		if err := aclChecker.StopAutoReload(shutdownCtx); err != nil {
			logServer.Warn("ACL auto-reload stop timeout", "error", err)
		}

		// 3. Stop HTTP server
		logServer.Info("stopping HTTP server")
		if err := srv.Shutdown(); err != nil {
			logServer.Error("error during shutdown", "error", err)
		}

		// 4. Close KeyManager (which closes HSM context)
		logKeyManager.Info("closing key manager")
		func() {
			defer func() {
				if r := recover(); r != nil {
					logKeyManager.Error("recovered from panic during key manager cleanup", "error", r)
				}
			}()
			if err := keyManager.Close(); err != nil {
				logKeyManager.Error("error closing key manager", "error", err)
			}
		}()
	}

	logServer.Info("HSM service stopped")
}

// performAutoCleanup performs automatic cleanup of old key versions on startup
func performAutoCleanup(hsmCfg *config.HSMConfig, metadata *config.Metadata) error {
	logKeyManager := slog.With("module", "keymanager")
	maxVersions := hsmCfg.MaxVersions
	if maxVersions == 0 {
		maxVersions = 3 // Default
	}
	cleanupAfterDays := hsmCfg.CleanupAfterDays
	if cleanupAfterDays == 0 {
		cleanupAfterDays = 30 // Default
	}

	logKeyManager.Info("auto-cleanup",
		"max_versions", maxVersions,
		"cleanup_after_days", cleanupAfterDays,
	)

	// For auto-cleanup, we only check version count limits, not age
	// Age-based cleanup is done manually via hsm-admin for safety
	deleted := 0
	for contextName, keyMeta := range metadata.Rotation {
		if len(keyMeta.Versions) <= maxVersions {
			continue
		}

		// Too many versions - need cleanup
		excessCount := len(keyMeta.Versions) - maxVersions
		logKeyManager.Warn("context has excess versions",
			"context", contextName,
			"versions", len(keyMeta.Versions),
			"limit", maxVersions,
		)
		logKeyManager.Warn("run hsm-admin cleanup-old-versions --dry-run")
		deleted += excessCount
	}

	if deleted > 0 {
		logKeyManager.Warn("excess versions detected",
			"count", deleted,
			"command", "hsm-admin cleanup-old-versions",
		)
	} else {
		logKeyManager.Info("all key versions within limits")
	}

	return nil
}

// getConfigPath returns the path to config.yaml, searching in multiple locations
// Priority:
// 1. CONFIG_PATH environment variable
// 2. ./config.yaml (current directory)
// 3. /etc/hsm-service/config.yaml (production location)
func getConfigPath() string {
	// Check CONFIG_PATH environment variable first
	if path := os.Getenv("CONFIG_PATH"); path != "" {
		return path
	}

	// Check in current directory
	if _, err := os.Stat("config.yaml"); err == nil {
		return "config.yaml"
	}

	// Check in /etc/hsm-service/ (production location)
	etcPath := "/etc/hsm-service/config.yaml"
	if _, err := os.Stat(etcPath); err == nil {
		return etcPath
	}

	// Return default (will cause error if file doesn't exist)
	return "config.yaml"
}
