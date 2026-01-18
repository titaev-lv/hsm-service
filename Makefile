# Makefile for HSM Service

VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME := $(shell date -u '+%Y-%m-%d_%H:%M:%S')
LDFLAGS := -s -w -X main.Version=$(VERSION) -X main.BuildTime=$(BUILD_TIME)

# Directories
BUILD_DIR := build
RELEASE_DIR := release

# Binaries
BINARY_SERVICE := $(BUILD_DIR)/hsm-service
BINARY_ADMIN := $(BUILD_DIR)/hsm-admin
BINARY_KEK := $(BUILD_DIR)/create-kek

.PHONY: all build clean test release install help

# Default target
all: build

help:
	@echo "HSM Service Build Commands:"
	@echo ""
	@echo "  make build          - Build all binaries"
	@echo "  make clean          - Clean build artifacts"
	@echo "  make test           - Run tests"
	@echo "  make test-race      - Run tests with race detector"
	@echo "  make test-cover     - Run tests with coverage"
	@echo "  make release        - Create release package"
	@echo "  make install        - Install binaries to /usr/local/bin"
	@echo "  make docker-build   - Build Docker image"
	@echo ""

# Build all binaries
build: $(BINARY_SERVICE) $(BINARY_ADMIN) $(BINARY_KEK)
	@echo "✓ Build complete"

# Build hsm-service
$(BINARY_SERVICE):
	@echo "Building hsm-service..."
	@mkdir -p $(BUILD_DIR)
	@GOOS=linux GOARCH=amd64 go build \
		-ldflags="$(LDFLAGS)" \
		-trimpath \
		-o $(BINARY_SERVICE) \
		main.go

# Build hsm-admin
$(BINARY_ADMIN):
	@echo "Building hsm-admin..."
	@mkdir -p $(BUILD_DIR)
	@GOOS=linux GOARCH=amd64 go build \
		-ldflags="-s -w" \
		-trimpath \
		-o $(BINARY_ADMIN) \
		./cmd/hsm-admin

# Build create-kek
$(BINARY_KEK):
	@echo "Building create-kek..."
	@mkdir -p $(BUILD_DIR)
	@GOOS=linux GOARCH=amd64 go build \
		-ldflags="-s -w" \
		-trimpath \
		-o $(BINARY_KEK) \
		./cmd/create-kek

# Clean build artifacts
clean:
	@echo "Cleaning..."
	@rm -rf $(BUILD_DIR) $(RELEASE_DIR)
	@echo "✓ Cleaned"

# Run tests
test:
	@echo "Running tests..."
	@go test ./...

# Run tests with race detector
test-race:
	@echo "Running tests with race detector..."
	@go test -race ./...

# Run tests with coverage
test-cover:
	@echo "Running tests with coverage..."
	@go test -cover -coverprofile=coverage.out ./...
	@go tool cover -html=coverage.out -o coverage.html
	@echo "✓ Coverage report: coverage.html"

# Create release package
release: build
	@echo "Creating release package..."
	@mkdir -p $(RELEASE_DIR)/hsm-service-$(VERSION)-linux-amd64/{bin,config,scripts}
	@cp $(BUILD_DIR)/* $(RELEASE_DIR)/hsm-service-$(VERSION)-linux-amd64/bin/
	@cp config.yaml $(RELEASE_DIR)/hsm-service-$(VERSION)-linux-amd64/config/config.yaml.example
	@cp metadata.yaml.example $(RELEASE_DIR)/hsm-service-$(VERSION)-linux-amd64/config/
	@cp softhsm2.conf $(RELEASE_DIR)/hsm-service-$(VERSION)-linux-amd64/config/
	@cp scripts/*.sh $(RELEASE_DIR)/hsm-service-$(VERSION)-linux-amd64/scripts/
	@chmod +x $(RELEASE_DIR)/hsm-service-$(VERSION)-linux-amd64/scripts/*.sh
	@cp README.md LICENSE $(RELEASE_DIR)/hsm-service-$(VERSION)-linux-amd64/
	@cd $(RELEASE_DIR) && sha256sum hsm-service-$(VERSION)-linux-amd64/bin/* > hsm-service-$(VERSION)-linux-amd64/CHECKSUMS.txt
	@cd $(RELEASE_DIR) && tar -czf hsm-service-$(VERSION)-linux-amd64.tar.gz hsm-service-$(VERSION)-linux-amd64/
	@echo "✓ Release created: $(RELEASE_DIR)/hsm-service-$(VERSION)-linux-amd64.tar.gz"
	@ls -lh $(RELEASE_DIR)/hsm-service-$(VERSION)-linux-amd64.tar.gz

# Install binaries locally
install: build
	@echo "Installing binaries..."
	@sudo cp $(BINARY_SERVICE) /usr/local/bin/
	@sudo cp $(BINARY_ADMIN) /usr/local/bin/
	@sudo cp $(BINARY_KEK) /usr/local/bin/
	@echo "✓ Installed to /usr/local/bin/"

# Build Docker image
docker-build:
	@echo "Building Docker image..."
	@docker build -t hsm-service:$(VERSION) .
	@docker tag hsm-service:$(VERSION) hsm-service:latest
	@echo "✓ Docker image built: hsm-service:$(VERSION)"
