package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/titaev-lv/hsm-service/internal/config"
	"github.com/titaev-lv/hsm-service/internal/hsm"
	"github.com/titaev-lv/hsm-service/internal/server"
)

func main() {
	// 1. Load configuration
	cfg, err := config.LoadConfig("config.yaml")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// 2. Get HSM PIN from environment variable
	hsmPIN := os.Getenv("HSM_PIN")
	if hsmPIN == "" {
		log.Fatal("HSM_PIN environment variable not set")
	}

	// 3. Initialize HSM context
	hsmCtx, err := hsm.InitHSM(&cfg.HSM, hsmPIN)
	if err != nil {
		log.Fatalf("Failed to initialize HSM: %v", err)
	}
	defer hsmCtx.Close()

	// 4. Initialize ACL checker
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
	srv, err := server.NewServer(&cfg.Server, hsmCtx, aclChecker, rateLimiter)
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
		if err := srv.Shutdown(); err != nil {
			log.Printf("Error during shutdown: %v", err)
		}
	}

	log.Println("HSM service stopped")
}
