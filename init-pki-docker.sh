#!/bin/bash
#
# HSM Service - PKI and Docker Initialization
#
# This script prepares HSM Service for local development:
# 1. Generates PKI infrastructure (CA, server, and client certificates)
# 2. Creates metadata.yaml configuration
# 3. Builds Docker image
# 4. Starts the container
#
# Usage: ./init-pki-docker.sh [--force] [--skip-docker]
#        --force: Regenerate all certificates even if they exist
#        --skip-docker: Skip Docker build and run (only generate PKI)

set -euo pipefail

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
MAGENTA='\033[0;35m'
NC='\033[0m' # No Color

# Script configuration
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$SCRIPT_DIR"
PKI_DIR="$PROJECT_ROOT/pki"
CA_DIR="$PKI_DIR/ca"
SERVER_DIR="$PKI_DIR/server"
CLIENT_DIR="$PKI_DIR/client"

# Certificate configuration
COUNTRY="RU"
STATE="Moscow"
CITY="Moscow"
ORGANIZATION="HSM-Service-Dev"
CERT_VALIDITY_DAYS=825

# Parse arguments
FORCE=0
SKIP_DOCKER=0

for arg in "$@"; do
    case $arg in
        --force)
            FORCE=1
            shift
            ;;
        --skip-docker)
            SKIP_DOCKER=1
            shift
            ;;
        *)
            echo -e "${RED}Unknown argument: $arg${NC}"
            echo "Usage: $0 [--force] [--skip-docker]"
            exit 1
            ;;
    esac
done

# Print functions
print_header() {
    echo ""
    echo -e "${MAGENTA}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo -e "${MAGENTA}$1${NC}"
    echo -e "${MAGENTA}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo ""
}

print_step() {
    echo -e "${BLUE}→${NC} $1"
}

print_success() {
    echo -e "${GREEN}✓${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}⚠${NC} $1"
}

print_error() {
    echo -e "${RED}✗${NC} $1" >&2
}

print_info() {
    echo -e "${BLUE}ℹ${NC} $1"
}

# Check if command exists
command_exists() {
    command -v "$1" >/dev/null 2>&1
}

# Ensure directory exists
ensure_dir() {
    if [ ! -d "$1" ]; then
        mkdir -p "$1"
        print_info "Created directory: $1"
    fi
}

# Banner
clear
echo ""
echo -e "${MAGENTA}╔════════════════════════════════════════════════════════╗${NC}"
echo -e "${MAGENTA}║                                                        ║${NC}"
echo -e "${MAGENTA}║      ${GREEN}HSM Service - PKI & Docker Initialization${MAGENTA}     ║${NC}"
echo -e "${MAGENTA}║                                                        ║${NC}"
echo -e "${MAGENTA}║         Hardware Security Module - Dev Setup          ║${NC}"
echo -e "${MAGENTA}║                                                        ║${NC}"
echo -e "${MAGENTA}╚════════════════════════════════════════════════════════╝${NC}"
echo ""

# Step 1: Check prerequisites
print_header "Step 1: Checking Prerequisites"

if ! command_exists openssl; then
    print_error "OpenSSL is not installed"
    print_info "Install: sudo apt-get install openssl"
    exit 1
fi

print_success "OpenSSL found: $(openssl version)"

if ! command_exists docker; then
    print_error "Docker is not installed"
    exit 1
fi

print_success "Docker found: $(docker --version)"

if ! command_exists docker-compose && ! docker compose version &>/dev/null; then
    print_error "Docker Compose is not installed"
    exit 1
fi

print_success "Docker Compose found"

echo ""
print_success "All prerequisites satisfied!"

# Step 2: Generate PKI Infrastructure
print_header "Step 2: Generating PKI Infrastructure"

# Check if PKI already exists
if [ -f "$CA_DIR/ca.crt" ] && [ -f "$CA_DIR/ca.key" ]; then
    if [ $FORCE -eq 0 ]; then
        print_warning "PKI infrastructure already exists"
        print_info "To regenerate, use: $0 --force"
        echo ""
    else
        print_warning "Force mode: Regenerating PKI infrastructure"
        rm -rf "$PKI_DIR"
        ensure_dir "$CA_DIR"
        ensure_dir "$SERVER_DIR"
        ensure_dir "$CLIENT_DIR"
    fi
else
    ensure_dir "$CA_DIR"
    ensure_dir "$SERVER_DIR"
    ensure_dir "$CLIENT_DIR"
fi

# Function to generate certificate
generate_cert() {
    local name=$1
    local cert_path=$2
    local key_path=$3
    local subject=$4
    local is_ca=$5
    local ca_cert=$6
    local ca_key=$7

    if [ -f "$cert_path" ] && [ -f "$key_path" ] && [ $FORCE -eq 0 ]; then
        print_info "$name: Already exists (skipping)"
        return 0
    fi

    print_step "Generating $name..."

    if [ "$is_ca" = "true" ]; then
        # Self-signed CA certificate
        openssl req -x509 -newkey rsa:4096 -keyout "$key_path" \
            -out "$cert_path" -days $CERT_VALIDITY_DAYS -nodes \
            -subj "$subject" >/dev/null 2>&1
        print_success "$name generated"
    else
        # Server/Client certificate signed by CA
        # Step 1: Generate private key
        openssl genrsa -out "$key_path" 4096 >/dev/null 2>&1

        # Step 2: Generate CSR
        local csr_path="${cert_path%.crt}.csr"
        openssl req -new -key "$key_path" -out "$csr_path" \
            -subj "$subject" >/dev/null 2>&1

        # Step 3: Sign CSR with CA
        # Create temporary config file for extensions
        local ext_file=$(mktemp)
        echo "subjectAltName=DNS:localhost,DNS:hsm-service,DNS:hsm-service.local,IP:127.0.0.1" > "$ext_file"
        
        openssl x509 -req -in "$csr_path" -CA "$ca_cert" -CAkey "$ca_key" \
            -CAcreateserial -out "$cert_path" -days $CERT_VALIDITY_DAYS \
            -extfile "$ext_file" >/dev/null 2>&1
        
        local sign_exit=$?
        rm -f "$ext_file"
        
        if [ $sign_exit -ne 0 ]; then
            print_error "$name: Failed to sign certificate"
            return 1
        fi

        # Cleanup CSR
        rm -f "$csr_path"
        print_success "$name generated"
    fi
}

# 2.1: Generate Root CA
CA_SUBJECT="/C=$COUNTRY/ST=$STATE/L=$CITY/O=$ORGANIZATION/CN=hsm-service-ca"
generate_cert "Root CA" "$CA_DIR/ca.crt" "$CA_DIR/ca.key" "$CA_SUBJECT" "true" "" ""

# 2.2: Generate Server Certificate
SERVER_SUBJECT="/C=$COUNTRY/ST=$STATE/L=$CITY/O=$ORGANIZATION/CN=hsm-service.local"
generate_cert "Server Certificate (hsm-service.local)" \
    "$SERVER_DIR/hsm-service.local.crt" \
    "$SERVER_DIR/hsm-service.local.key" \
    "$SERVER_SUBJECT" "false" \
    "$CA_DIR/ca.crt" "$CA_DIR/ca.key"

# 2.3: Generate Client Certificates
CLIENT_SUBJECT_1="/C=$COUNTRY/ST=$STATE/L=$CITY/O=$ORGANIZATION/OU=Trading/CN=trading-service-1"
generate_cert "Client Certificate (trading-service-1)" \
    "$CLIENT_DIR/trading-service-1.crt" \
    "$CLIENT_DIR/trading-service-1.key" \
    "$CLIENT_SUBJECT_1" "false" \
    "$CA_DIR/ca.crt" "$CA_DIR/ca.key"

CLIENT_SUBJECT_2="/C=$COUNTRY/ST=$STATE/L=$CITY/O=$ORGANIZATION/OU=2FA/CN=2fa-service-1"
generate_cert "Client Certificate (2fa-service-1)" \
    "$CLIENT_DIR/2fa-service-1.crt" \
    "$CLIENT_DIR/2fa-service-1.key" \
    "$CLIENT_SUBJECT_2" "false" \
    "$CA_DIR/ca.crt" "$CA_DIR/ca.key"

echo ""

# Verify certificates
print_step "Verifying certificates..."
CERT_COUNT=$(find "$PKI_DIR" -name "*.crt" | wc -l)
KEY_COUNT=$(find "$PKI_DIR" -name "*.key" | wc -l)
print_success "Generated: $CERT_COUNT certificates, $KEY_COUNT private keys"

echo ""

# Step 3: Create metadata.yaml
print_header "Step 3: Creating Configuration Files"

if [ -f "$PROJECT_ROOT/metadata.yaml" ] && [ $FORCE -eq 0 ]; then
    print_info "metadata.yaml: Already exists (skipping)"
else
    print_step "Creating metadata.yaml from example..."
    if [ -f "$PROJECT_ROOT/metadata.yaml.example" ]; then
        cp "$PROJECT_ROOT/metadata.yaml.example" "$PROJECT_ROOT/metadata.yaml"
        print_success "metadata.yaml created"
    else
        print_warning "metadata.yaml.example not found (skipping)"
    fi
fi

echo ""

# Step 4: Docker Build and Run
if [ $SKIP_DOCKER -eq 0 ]; then
    print_header "Step 4: Building and Starting Docker Container"

    print_step "Building Docker image..."
    cd "$PROJECT_ROOT"

    if docker compose build --no-cache 2>&1 | tail -5; then
        print_success "Docker image built successfully"
    else
        print_error "Failed to build Docker image"
        exit 1
    fi

    echo ""
    print_step "Starting HSM Service container..."

    if docker compose up -d; then
        echo ""
        print_success "HSM Service started successfully!"
        print_info "Container: hsm-service"
        print_info "Port: https://localhost:8443"
    else
        echo ""
        print_error "Failed to start HSM Service"
        exit 1
    fi

    echo ""
    print_step "Waiting for service to initialize..."
    sleep 3

    print_header "Step 5: Verifying Service Health"

    # Check container status
    if docker compose ps | grep -q "hsm-service.*Up"; then
        print_success "HSM Service container is running"
    else
        print_error "HSM Service container is not running"
        print_info "Check logs: docker compose logs"
        exit 1
    fi

    # Check health
    if curl -sk https://localhost:8443/health \
        --cert "$CLIENT_DIR/trading-service-1.crt" \
        --key "$CLIENT_DIR/trading-service-1.key" \
        --cacert "$CA_DIR/ca.crt" >/dev/null 2>&1; then
        print_success "HSM Service is healthy"
    else
        print_warning "HSM Service health check failed (may still be initializing)"
        print_info "Check logs: docker compose logs"
    fi

    echo ""
else
    print_header "Step 4: Skipping Docker Build and Run"
    print_info "To start Docker later, run:"
    echo "  cd $PROJECT_ROOT"
    echo "  docker compose up -d"
fi

# Summary
print_header "🎉 Initialization Complete!"

echo ""
print_success "HSM Service is ready for development!"
echo ""

print_info "Summary:"
echo "  ✓ PKI infrastructure generated"
echo "    • CA certificate: $CA_DIR/ca.crt"
echo "    • Server certificates: $SERVER_DIR/"
echo "    • Client certificates: $CLIENT_DIR/"
echo "  ✓ Configuration files prepared"
echo "    • metadata.yaml created"

if [ $SKIP_DOCKER -eq 0 ]; then
    echo "  ✓ Docker container running"
fi

echo ""
print_info "Next steps:"
echo "  1. Test the API:"
echo "     curl -k https://localhost:8443/health \\"
echo "       --cert pki/client/trading-service-1.crt \\"
echo "       --key pki/client/trading-service-1.key \\"
echo "       --cacert pki/ca/ca.crt"
echo ""
echo "  2. View logs:"
echo "     docker compose logs -f"
echo ""
echo "  3. Run encryption test:"
echo "     curl -k -X POST https://localhost:8443/encrypt \\"
echo "       --cert pki/client/trading-service-1.crt \\"
echo "       --key pki/client/trading-service-1.key \\"
echo "       --cacert pki/ca/ca.crt \\"
echo "       -H 'Content-Type: application/json' \\"
echo "       -d '{\"context\":\"exchange-key\",\"plaintext\":\"SGVsbG8gV29ybGQh\"}'"

echo ""
print_info "Documentation:"
echo "  • QUICKSTART_DOCKER.md - Quick start guide"
echo "  • API.md - Complete API reference"
echo "  • README.md - Full documentation"

echo ""
print_success "Happy HSM'ing! 🔐"
echo ""
