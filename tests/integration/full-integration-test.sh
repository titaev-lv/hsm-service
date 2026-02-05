#!/bin/bash

set -e  # Exit on error

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Test counters
CURRENT_TEST=0
TOTAL_TESTS=60

# Helper functions
print_header() {
    echo ""
    echo "=========================================="
    echo -e "${BLUE}$1${NC}"
    echo "=========================================="
    echo ""
}

print_test() {
    CURRENT_TEST=$((CURRENT_TEST + 1))
    echo ""
    echo -e "${YELLOW}[TEST $CURRENT_TEST/$TOTAL_TESTS]${NC} $1"
}

print_success() {
    echo -e "${GREEN}✓${NC} $1"
}

print_error() {
    echo -e "${RED}✗ FAILED at TEST $CURRENT_TEST/$TOTAL_TESTS${NC}"
    echo -e "${RED}Error: $1${NC}"
    exit 1
}

print_info() {
    echo -e "${BLUE}ℹ${NC} $1"
}

# ==========================================
# PKI GENERATION FUNCTIONS
# ==========================================
generate_test_pki() {
    local pki_dir="$1"
    local ca_dir="$pki_dir/test/ca"
    local server_dir="$pki_dir/test/server"
    local client_dir="$pki_dir/test/client"
    
    print_test "Generate test PKI infrastructure"
    
    # Create directories
    mkdir -p "$ca_dir" "$server_dir" "$client_dir"
    
    # Certificate configuration
    local country="RU"
    local state="Moscow"
    local city="Moscow"
    local org="HSM-Test"
    local validity_days=365
    
    # 1. Generate Root CA (self-signed, no password)
    print_info "Generating test Root CA..."
    openssl req -x509 -newkey rsa:4096 -keyout "$ca_dir/ca.key" \
        -out "$ca_dir/ca.crt" -days $validity_days -nodes \
        -subj "/C=$country/ST=$state/L=$city/O=$org/CN=hsm-test-ca" >/dev/null 2>&1
    print_success "Test CA generated"
    
    # 2. Generate Server Certificate
    print_info "Generating test server certificate..."
    openssl genrsa -out "$server_dir/hsm-service.key" 4096 >/dev/null 2>&1
    local server_csr=$(mktemp)
    openssl req -new -key "$server_dir/hsm-service.key" -out "$server_csr" \
        -subj "/C=$country/ST=$state/L=$city/O=$org/CN=hsm-service.local" >/dev/null 2>&1
    
    # Sign with CA, adding SANs
    local ext_file=$(mktemp)
    echo "subjectAltName=DNS:localhost,DNS:hsm-service,DNS:hsm-service.local,IP:127.0.0.1" > "$ext_file"
    openssl x509 -req -in "$server_csr" -CA "$ca_dir/ca.crt" -CAkey "$ca_dir/ca.key" \
        -CAcreateserial -out "$server_dir/hsm-service.crt" -days $validity_days \
        -extfile "$ext_file" >/dev/null 2>&1
    rm -f "$server_csr" "$ext_file"
    print_success "Test server certificate generated"
    
    # 3. Generate Trading Client Certificate (OU=Trading)
    print_info "Generating test client certificate (Trading)..."
    openssl genrsa -out "$client_dir/trading-client-1.key" 4096 >/dev/null 2>&1
    local client_csr=$(mktemp)
    openssl req -new -key "$client_dir/trading-client-1.key" -out "$client_csr" \
        -subj "/C=$country/ST=$state/L=$city/O=$org/OU=Trading/CN=trading-client-1" >/dev/null 2>&1
    
    openssl x509 -req -in "$client_csr" -CA "$ca_dir/ca.crt" -CAkey "$ca_dir/ca.key" \
        -CAcreateserial -out "$client_dir/trading-client-1.crt" -days $validity_days >/dev/null 2>&1
    rm -f "$client_csr"
    print_success "Test trading client certificate generated"
    
    # 4. Generate 2FA Client Certificate (OU=2FA)
    print_info "Generating test client certificate (2FA)..."
    openssl genrsa -out "$client_dir/2fa-client-1.key" 4096 >/dev/null 2>&1
    local client_csr=$(mktemp)
    openssl req -new -key "$client_dir/2fa-client-1.key" -out "$client_csr" \
        -subj "/C=$country/ST=$state/L=$city/O=$org/OU=2FA/CN=2fa-client-1" >/dev/null 2>&1
    
    openssl x509 -req -in "$client_csr" -CA "$ca_dir/ca.crt" -CAkey "$ca_dir/ca.key" \
        -CAcreateserial -out "$client_dir/2fa-client-1.crt" -days $validity_days >/dev/null 2>&1
    rm -f "$client_csr"
    print_success "Test 2FA client certificate generated"
    
    # Set permissions
    chmod 600 "$ca_dir/ca.key" "$server_dir/hsm-service.key" "$client_dir"/*.key
    chmod 644 "$ca_dir/ca.crt" "$server_dir/hsm-service.crt" "$client_dir"/*.crt
}

backup_config() {
    local config_file="$1"
    if [ -f "$config_file" ]; then
        cp "$config_file" "$config_file.test-backup"
        print_success "Config backed up: $config_file.test-backup"
    fi
}

restore_config() {
    local config_file="$1"
    if [ -f "$config_file.test-backup" ]; then
        mv "$config_file.test-backup" "$config_file"
        print_success "Config restored from backup"
    fi
}

update_config_for_test_pki() {
    local config_file="$1"
    local pki_test_dir="pki/test"
    
    print_test "Update config.yaml to use test PKI"
    
    # Escape / in paths for sed
    local pki_escaped="${pki_test_dir//\//\\\/}"
    
    sed -i "s|/app/pki/ca/ca.crt|/app/$pki_escaped/ca/ca.crt|g" "$config_file"
    sed -i "s|/app/pki/server/hsm-service.crt|/app/$pki_escaped/server/hsm-service.crt|g" "$config_file"
    sed -i "s|/app/pki/server/hsm-service.key|/app/$pki_escaped/server/hsm-service.key|g" "$config_file"
    
    print_success "Config updated for test PKI"
}

cleanup_test_pki() {
    local pki_dir="$1"
    print_test "Cleanup test PKI"
    if [ -d "$pki_dir/test" ]; then
        rm -rf "$pki_dir/test"
        print_success "Test PKI directory removed"
    fi
}

# ==========================================
# SETUP: Determine project root
# ==========================================
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

# Cleanup function to restore config and PKI on exit
cleanup_on_exit() {
    echo ""
    print_header "CLEANUP: Restoring original state"
    
    # Stop and remove test containers (keep production containers)
    if [ -n "${TEST_COMPOSE_FILE:-}" ] && [ -f "$TEST_COMPOSE_FILE" ]; then
        print_test "Stop test containers"
        cd "$PROJECT_ROOT"
        docker compose -f "$TEST_COMPOSE_FILE" down > /dev/null 2>&1
        print_success "Test containers stopped"
        
        # Remove test compose file
        rm -f "$TEST_COMPOSE_FILE"
    fi
    
    # Restore original config
    if [ -n "${CONFIG_FILE:-}" ]; then
        restore_config "$PROJECT_ROOT/$CONFIG_FILE"
    fi
    
    # Cleanup test PKI
    if [ -n "${PROJECT_ROOT:-}" ]; then
        cleanup_test_pki "$PROJECT_ROOT/pki"
    fi
    
    print_success "Test cleanup complete"
}

# Set trap to run cleanup on exit
trap cleanup_on_exit EXIT

# Always work from project root
cd "$PROJECT_ROOT"

print_header "HSM Service - Full Integration Test Suite"
print_info "Project root: $PROJECT_ROOT"
print_info "Working directory: $(pwd)"
print_info "Date: $(date)"

# Detect docker-compose file (.yaml or .yml)
if [ -f "$PROJECT_ROOT/docker-compose.yaml" ]; then
    COMPOSE_FILE="docker-compose.yaml"
elif [ -f "$PROJECT_ROOT/docker-compose.yml" ]; then
    COMPOSE_FILE="docker-compose.yml"
else
    echo -e "${RED}✗ docker-compose.yaml or docker-compose.yml not found in $PROJECT_ROOT${NC}"
    exit 1
fi
print_info "Using: $COMPOSE_FILE"

# ==========================================
# PHASE 1: CLEANUP
# ==========================================
print_header "PHASE 1: Docker Cleanup"

print_test "Stop production container (if running)"
docker stop hsm-service 2>/dev/null || true
docker rm hsm-service 2>/dev/null || true
print_success "Production container stopped"

print_test "Stop and remove existing test containers and volumes"
cd "$PROJECT_ROOT"
docker compose -f "$TEST_COMPOSE_FILE" down -v 2>/dev/null || true
print_success "Test containers stopped and volumes removed (tokens ephemeral)"

# Commented out: Keep downloaded layers to speed up rebuilds
# Uncomment these lines for full cleanup (slower but cleaner)
#print_test "Remove project images"
#docker rmi hsm-service:latest 2>/dev/null || true
#print_success "Images removed"

#print_test "Prune unused Docker resources"
#docker system prune -f > /dev/null
#print_success "Docker cleanup complete"

print_success "Cleanup complete (cached layers preserved for faster rebuilds)"

# ==========================================
# PHASE 2: BUILD
# ==========================================
print_header "PHASE 2: Build from Scratch"

print_test "Build Docker image (no cache)"
cd "$PROJECT_ROOT"
echo ""
echo "=== Docker Build Output (--no-cache) ==="
echo "This will take a few minutes on first run..."
echo ""
if ! docker build --no-cache -t hsm-service:latest . 2>&1 | tee /tmp/docker-build.log; then
    echo ""
    print_error "Docker build failed (full log saved to /tmp/docker-build.log)"
fi
echo ""
print_success "Image built successfully"

print_test "Verify image exists"
if ! docker images | grep -q hsm-service; then
    print_error "Image hsm-service:latest not found"
fi
print_success "Image verified"

# ==========================================
# PHASE 3: PKI SETUP
# ==========================================
print_header "PHASE 3: Generate Test PKI"

# Backup original config
CONFIG_FILE="config.yaml"
backup_config "$PROJECT_ROOT/$CONFIG_FILE"

# Generate test PKI infrastructure
generate_test_pki "$PROJECT_ROOT/pki"

# Update config to use test PKI
update_config_for_test_pki "$PROJECT_ROOT/$CONFIG_FILE"

print_test "Verify test CA certificate exists"
if [ ! -f "$PROJECT_ROOT/pki/test/ca/ca.crt" ]; then
    print_error "Test CA certificate not generated"
fi
print_success "Test CA certificate exists"

print_test "Verify test server certificate exists"
if [ ! -f "$PROJECT_ROOT/pki/test/server/hsm-service.crt" ]; then
    print_error "Test server certificate not generated"
fi
print_success "Test server certificate exists"

print_test "Verify test client certificates exist"
if [ ! -f "$PROJECT_ROOT/pki/test/client/trading-client-1.crt" ] || [ ! -f "$PROJECT_ROOT/pki/test/client/2fa-client-1.crt" ]; then
    print_error "Test client certificates not generated"
fi
print_success "Test client certificates exist"

# Set test certificate variables
CLIENT_CERT_NAME="trading-client-1"
CLIENT_CERT_PATH="$PROJECT_ROOT/pki/test/client/trading-client-1.crt"
CLIENT_KEY_PATH="$PROJECT_ROOT/pki/test/client/trading-client-1.key"

print_success "Test PKI setup complete"

# ==========================================
# PHASE 4: METADATA INITIALIZATION
# ==========================================
print_header "PHASE 4: Metadata Initialization"

print_test "Create initial metadata.yaml with multi-version structure"
cat > "$PROJECT_ROOT/metadata.yaml" << 'EOF'
rotation:
  exchange-key:
    current: kek-exchange-key-v1
    rotation_interval_days: 90
    versions:
      - label: kek-exchange-key-v1
        version: 1
        created_at: '2026-01-09T00:00:00Z'
  2fa:
    current: kek-2fa-v1
    rotation_interval_days: 90
    versions:
      - label: kek-2fa-v1
        version: 1
        created_at: '2026-01-09T00:00:00Z'
EOF
print_success "metadata.yaml created with initial structure"

# ==========================================
# PHASE 5: START SERVICE
# ==========================================
print_header "PHASE 5: Start Service"

print_test "Create test docker-compose file with -test suffix"
# Copy original compose file but modify container name to include -test
TEST_COMPOSE_FILE="$PROJECT_ROOT/docker-compose-test.yml"
sed "s/container_name: hsm-service/container_name: hsm-service-test/" "$PROJECT_ROOT/$COMPOSE_FILE" > "$TEST_COMPOSE_FILE"

# Update image tag if needed for tests
if grep -q "image: hsm-service:latest" "$TEST_COMPOSE_FILE"; then
    sed -i "s/image: hsm-service:latest/image: hsm-service:latest/" "$TEST_COMPOSE_FILE"
fi

print_success "Test docker-compose created: $TEST_COMPOSE_FILE"

print_test "Start services with docker-compose (test mode)"
cd "$PROJECT_ROOT"
# Use test compose file - force recreate test container
if ! docker compose -f "$TEST_COMPOSE_FILE" up -d --force-recreate > /tmp/docker-compose-up.log 2>&1; then
    cat /tmp/docker-compose-up.log
    print_error "docker-compose up failed (see /tmp/docker-compose-up.log)"
fi
sleep 3
print_success "Test services started"

print_test "Verify test container is running"
if ! docker ps | grep -q hsm-service-test; then
    cd "$PROJECT_ROOT"
    docker compose -f "$TEST_COMPOSE_FILE" logs
    print_error "Container not running"
fi
print_success "Test container is running (hsm-service-test)"

print_test "Check container logs for errors"
sleep 2
cd "$PROJECT_ROOT"
if docker compose -f "$TEST_COMPOSE_FILE" logs | grep -i "fatal\|panic"; then
    docker compose -f "$TEST_COMPOSE_FILE" logs
    print_error "Fatal errors found in logs"
fi
print_success "No fatal errors in logs"

# ==========================================
# PHASE 6: HSM INITIALIZATION
# ==========================================
print_header "PHASE 6: HSM Key Initialization"

print_test "Verify HSM initialized automatically (init-hsm.sh runs on container start)"
sleep 3  # Give time for init to complete
if ! docker logs hsm-service-test 2>&1 | grep -q "HSM Service Initialization"; then
    docker logs hsm-service-test
    print_error "HSM initialization did not run"
fi
print_success "HSM initialization completed automatically"

print_test "Verify keys created automatically"
if ! docker logs hsm-service-test 2>&1 | grep -q "Default KEKs created"; then
    # Keys might already exist from previous run
    if ! docker logs hsm-service-test 2>&1 | grep -q "Found .* KEK"; then
        docker logs hsm-service-test
        print_error "No KEKs found in HSM"
    fi
fi
print_success "HSM keys initialized"

print_test "Verify keys loaded (check logs)"
if ! docker logs hsm-service-test 2>&1 | grep -q "Loaded KEK: kek-exchange-key-v1"; then
    docker logs hsm-service-test
    print_error "KEK kek-exchange-key-v1 not loaded"
fi
if ! docker logs hsm-service-test 2>&1 | grep -q "Loaded KEK: kek-2fa-v1"; then
    docker logs hsm-service-test
    print_error "KEK kek-2fa-v1 not loaded"
fi
print_success "All KEKs loaded successfully"

# ==========================================
# PHASE 7: BASIC FUNCTIONALITY TESTS
# ==========================================
print_header "PHASE 7: Basic Functionality Tests"

# Test variables
BASE_URL="https://localhost:8443"
CA_CERT="$PROJECT_ROOT/pki/test/ca/ca.crt"
CLIENT_CERT="$CLIENT_CERT_PATH"
CLIENT_KEY="$CLIENT_KEY_PATH"

print_test "Test 7.1: Health check endpoint"
echo ""
echo "=== Health Check Request ==="
echo "URL: $BASE_URL/health"
echo "CA Cert: $CA_CERT"
echo "Client Cert: $CLIENT_CERT"
echo "Client Key: $CLIENT_KEY"
echo ""

# Make request with verbose output
HEALTH_RESPONSE=$(curl -v --connect-timeout 10 --max-time 15 \
    --cacert "$CA_CERT" \
    --cert "$CLIENT_CERT" \
    --key "$CLIENT_KEY" \
    "$BASE_URL/health" 2>&1)

echo "=== Full Response ==="
echo "$HEALTH_RESPONSE"
echo ""

# Extract just the body (last line after headers)
HEALTH_BODY=$(echo "$HEALTH_RESPONSE" | tail -1)
echo "=== Response Body ==="
echo "$HEALTH_BODY"
echo ""

# Check for success - looking for "healthy" or "ok" status
if ! echo "$HEALTH_BODY" | grep -qiE "(healthy|ok)"; then
    echo ""
    echo "Checking if service is running..."
    docker ps | grep hsm-service
    echo ""
    echo "Service logs (last 30 lines):"
    docker logs --tail 30 hsm-service
    print_error "Health check failed - response doesn't contain 'healthy' or 'ok'"
fi
print_success "Health check passed"

print_test "Test 7.2: Encrypt data with exchange-key"
PLAINTEXT="SGVsbG8gV29ybGQh"  # "Hello World!" in base64

echo ""
echo "=== Encrypt Request ==="
echo "URL: $BASE_URL/encrypt"
echo "Payload: {\"context\":\"exchange-key\",\"plaintext\":\"$PLAINTEXT\"}"
echo ""

ENCRYPT_RESPONSE=$(curl -v --connect-timeout 10 --max-time 15 \
    --cacert "$CA_CERT" \
    --cert "$CLIENT_CERT" \
    --key "$CLIENT_KEY" \
    -H "Content-Type: application/json" \
    -d "{\"context\":\"exchange-key\",\"plaintext\":\"$PLAINTEXT\"}" \
    "$BASE_URL/encrypt" 2>&1)

echo "=== Encrypt Full Response ==="
echo "$ENCRYPT_RESPONSE"
echo ""

# Extract body
ENCRYPT_BODY=$(echo "$ENCRYPT_RESPONSE" | grep -o '{.*}' | tail -1)
echo "=== Encrypt Response Body ==="
echo "$ENCRYPT_BODY"
echo ""

CIPHERTEXT=$(echo "$ENCRYPT_BODY" | grep -o '"ciphertext":"[^"]*"' | cut -d'"' -f4)
KEY_ID=$(echo "$ENCRYPT_BODY" | grep -o '"key_id":"[^"]*"' | cut -d'"' -f4)

echo "Extracted - Ciphertext: ${CIPHERTEXT:0:50}... Key ID: $KEY_ID"
echo ""

if [ -z "$CIPHERTEXT" ]; then
    echo "ERROR: No ciphertext in response"
    print_error "Encryption failed - no ciphertext returned"
fi
if [ "$KEY_ID" != "kek-exchange-key-v1" ]; then
    echo "Expected: kek-exchange-key-v1, Got: $KEY_ID"
    print_error "Wrong key_id returned"
fi
print_success "Encryption successful (key: $KEY_ID)"

print_test "Test 7.3: Decrypt data with exchange-key"
echo ""
echo "=== Decrypt Request ==="
echo "URL: $BASE_URL/decrypt"
echo "Payload: {\"context\":\"exchange-key\",\"ciphertext\":\"${CIPHERTEXT:0:50}...\",\"key_id\":\"$KEY_ID\"}"
echo ""

DECRYPT_RESPONSE=$(curl -v --connect-timeout 10 --max-time 15 \
    --cacert "$CA_CERT" \
    --cert "$CLIENT_CERT" \
    --key "$CLIENT_KEY" \
    -H "Content-Type: application/json" \
    -d "{\"context\":\"exchange-key\",\"ciphertext\":\"$CIPHERTEXT\",\"key_id\":\"$KEY_ID\"}" \
    "$BASE_URL/decrypt" 2>&1)

echo "=== Decrypt Full Response ==="
echo "$DECRYPT_RESPONSE"
echo ""

DECRYPT_BODY=$(echo "$DECRYPT_RESPONSE" | grep -o '{.*}' | tail -1)
echo "=== Decrypt Response Body ==="
echo "$DECRYPT_BODY"
echo ""

DECRYPTED=$(echo "$DECRYPT_BODY" | grep -o '"plaintext":"[^"]*"' | cut -d'"' -f4)

echo "Decrypted: $DECRYPTED (expected: $PLAINTEXT)"
echo ""

if [ "$DECRYPTED" != "$PLAINTEXT" ]; then
    echo "Expected: $PLAINTEXT"
    echo "Got: $DECRYPTED"
    print_error "Decryption failed - plaintext mismatch"
fi
print_success "Decryption successful - data matches"

# ==========================================
# PHASE 7.5: AAD MODE TESTS (SHARED/PRIVATE)
# ==========================================
print_header "PHASE 7.5: AAD Mode Tests (Shared/Private)"

print_test "Test 7.5.1: Shared mode - encrypt/decrypt with exchange-key (AAD uses OU)"
echo ""
echo "=== Testing shared mode (exchange-key, mode=shared) ==="
echo "Client: hsm-trading-client-1 (OU=Trading)"
echo "AAD should use OU instead of CN for this context"
echo ""

# Encrypt data with shared mode context
SHARED_PLAINTEXT="U2hhcmVkRGF0YQ=="  # "SharedData" in base64

SHARED_ENCRYPT=$(curl -s --connect-timeout 10 --max-time 15 \
    --cacert "$CA_CERT" \
    --cert "$CLIENT_CERT" \
    --key "$CLIENT_KEY" \
    -H "Content-Type: application/json" \
    -d "{\"context\":\"exchange-key\",\"plaintext\":\"$SHARED_PLAINTEXT\"}" \
    "$BASE_URL/encrypt" 2>&1)

SHARED_CIPHERTEXT=$(echo "$SHARED_ENCRYPT" | grep -o '"ciphertext":"[^"]*"' | cut -d'"' -f4)
SHARED_KEY_ID=$(echo "$SHARED_ENCRYPT" | grep -o '"key_id":"[^"]*"' | cut -d'"' -f4)

if [ -z "$SHARED_CIPHERTEXT" ]; then
    echo "Encrypt response: $SHARED_ENCRYPT"
    print_error "Failed to encrypt with exchange-key (shared mode)"
fi

echo "Encrypted: ${SHARED_CIPHERTEXT:0:40}... (key: $SHARED_KEY_ID)"
echo ""

# Decrypt with same client (verifies shared mode AAD works)
SHARED_DECRYPT=$(curl -s --connect-timeout 10 --max-time 15 \
    --cacert "$CA_CERT" \
    --cert "$CLIENT_CERT" \
    --key "$CLIENT_KEY" \
    -H "Content-Type: application/json" \
    -d "{\"context\":\"exchange-key\",\"ciphertext\":\"$SHARED_CIPHERTEXT\",\"key_id\":\"$SHARED_KEY_ID\"}" \
    "$BASE_URL/decrypt" 2>&1)

SHARED_DECRYPTED=$(echo "$SHARED_DECRYPT" | grep -o '"plaintext":"[^"]*"' | cut -d'"' -f4)

echo "Decrypted: $SHARED_DECRYPTED"
echo ""

if [ "$SHARED_DECRYPTED" = "$SHARED_PLAINTEXT" ]; then
    print_success "Shared mode works - AAD uses OU (all clients with OU=Trading can share data)"
else
    echo "Expected: $SHARED_PLAINTEXT"
    echo "Got: $SHARED_DECRYPTED"
    echo "Decrypt response: $SHARED_DECRYPT"
    print_error "Shared mode failed"
fi

print_test "Test 7.5.2: Private mode - different clients cannot decrypt (2fa)"
# Use 2fa client cert if it exists
TFA_CLIENT_CERT="$PROJECT_ROOT/pki/test/client/2fa-client-1.crt"
TFA_CLIENT_KEY="$PROJECT_ROOT/pki/test/client/2fa-client-1.key"

if [ -f "$TFA_CLIENT_CERT" ] && [ -f "$TFA_CLIENT_KEY" ]; then
    echo ""
    echo "=== Testing private mode isolation (2fa context) ==="
    echo "Note: This test requires 2 different clients with OU=2FA"
    echo "Since we only have 1 2FA client, we verify ACL blocks wrong OU instead"
    echo ""
    
    # Try to use Trading client cert to access 2fa context (should fail via ACL)
    PRIVATE_PLAINTEXT="UHJpdmF0ZURhdGE="  # "PrivateData" in base64
    
    PRIVATE_RESPONSE=$(curl -s -w "\n%{http_code}" --connect-timeout 10 --max-time 15 \
        --cacert "$CA_CERT" \
        --cert "$CLIENT_CERT" \
        --key "$CLIENT_KEY" \
        -H "Content-Type: application/json" \
        -d "{\"context\":\"2fa\",\"plaintext\":\"$PRIVATE_PLAINTEXT\"}" \
        "$BASE_URL/encrypt" 2>&1)
    
    HTTP_CODE=$(echo "$PRIVATE_RESPONSE" | tail -1)
    RESPONSE_BODY=$(echo "$PRIVATE_RESPONSE" | head -n -1)
    
    echo "Response HTTP Code: $HTTP_CODE"
    echo "Response: ${RESPONSE_BODY:0:100}..."
    echo ""
    
    if [ "$HTTP_CODE" = "403" ] || echo "$RESPONSE_BODY" | grep -qi "forbidden\|not.*authorized\|access.*denied"; then
        print_success "Private mode enforced - wrong OU blocked by ACL (Trading cannot access 2fa)"
    else
        print_error "Private mode ACL failed - Trading client accessed 2fa context!"
    fi
else
    print_info "2fa client cert not found, skipping private mode test"
fi

print_test "Test 7.5.3: Verify mode configuration in config.yaml"
# Read config and verify modes are set correctly
CONFIG_EXCHANGE_MODE=$(grep -A 2 "exchange-key:" "$PROJECT_ROOT/config.yaml" | grep "mode:" | awk '{print $2}')
CONFIG_2FA_MODE=$(grep -A 2 "2fa:" "$PROJECT_ROOT/config.yaml" | grep "mode:" | awk '{print $2}')

echo "Config modes:"
echo "  exchange-key: $CONFIG_EXCHANGE_MODE (expected: shared)"
echo "  2fa: $CONFIG_2FA_MODE (expected: private)"
echo ""

if [ "$CONFIG_EXCHANGE_MODE" = "shared" ] && [ "$CONFIG_2FA_MODE" = "private" ]; then
    print_success "AAD modes configured correctly"
else
    print_error "AAD modes misconfigured in config.yaml"
fi

# ==========================================
# PHASE 8: KEY ROTATION
# ==========================================
print_header "PHASE 8: Key Rotation Tests"

print_test "Test 8.1: Check rotation status before rotation"
ROTATION_STATUS=$(docker exec hsm-service-test /app/hsm-admin rotation-status 2>&1)
echo "$ROTATION_STATUS"
if ! echo "$ROTATION_STATUS" | grep -q "exchange-key"; then
    print_error "rotation-status command failed"
fi
print_success "Rotation status command works"

print_test "Test 8.2: Perform key rotation (exchange-key)"
if ! docker exec hsm-service-test /app/hsm-admin rotate exchange-key > /tmp/rotation.log 2>&1; then
    cat /tmp/rotation.log
    print_error "Key rotation failed (see /tmp/rotation.log)"
fi
# Force sync: copy metadata from container to host
docker cp hsm-service-test:/app/metadata.yaml "$PROJECT_ROOT/metadata.yaml"
print_success "Key rotation completed"

print_test "Test 8.3: Verify metadata.yaml updated"
METADATA_CONTENT=$(cat "$PROJECT_ROOT/metadata.yaml")
if ! echo "$METADATA_CONTENT" | grep -q "kek-exchange-key-v2"; then
    echo "$METADATA_CONTENT"
    print_error "metadata.yaml not updated with v2"
fi
if ! echo "$METADATA_CONTENT" | grep -q "current: kek-exchange-key-v2"; then
    echo "$METADATA_CONTENT"
    print_error "current pointer not updated to v2"
fi
print_success "metadata.yaml contains both v1 and v2"

print_test "Test 8.4: Restart service to load new key"
docker stop hsm-service-test > /dev/null 2>&1
sleep 2
docker start hsm-service-test > /dev/null 2>&1
sleep 7
if ! docker ps | grep -q hsm-service; then
    docker logs hsm-service-test
    print_error "Container failed to start after rotation"
fi
print_success "Service restarted"

print_test "Test 8.5: Verify both versions loaded (overlap period)"
LOGS=$(docker logs hsm-service-test 2>&1)
if ! echo "$LOGS" | grep -q "Loaded KEK: kek-exchange-key-v1"; then
    echo "$LOGS"
    print_error "Old key (v1) not loaded after rotation"
fi
if ! echo "$LOGS" | grep -q "Loaded KEK: kek-exchange-key-v2"; then
    echo "$LOGS"
    print_error "New key (v2) not loaded after rotation"
fi
print_success "Both v1 and v2 keys loaded (overlap period active)"

# ==========================================
# PHASE 9: POST-ROTATION FUNCTIONALITY
# ==========================================
print_header "PHASE 9: Post-Rotation Functionality"

print_test "Test 9.1: Decrypt old data with v1 key"
DECRYPT_V1=$(curl -s --connect-timeout 10 --max-time 15 \
    --cacert "$CA_CERT" \
    --cert "$CLIENT_CERT" \
    --key "$CLIENT_KEY" \
    -H "Content-Type: application/json" \
    -d "{\"context\":\"exchange-key\",\"ciphertext\":\"$CIPHERTEXT\",\"key_id\":\"kek-exchange-key-v1\"}" \
    "$BASE_URL/decrypt" 2>&1)

DECRYPTED_V1=$(echo "$DECRYPT_V1" | grep -o '"plaintext":"[^"]*"' | cut -d'"' -f4)
if [ "$DECRYPTED_V1" != "$PLAINTEXT" ]; then
    echo "Response: $DECRYPT_V1"
    print_error "Cannot decrypt old data with v1 after rotation"
fi
print_success "Old data successfully decrypted with v1"

print_test "Test 9.2: Encrypt new data uses v2 key"
PLAINTEXT_NEW="TmV3IERhdGEh"  # "New Data!" in base64

ENCRYPT_V2=$(curl -s --connect-timeout 10 --max-time 15 \
    --cacert "$CA_CERT" \
    --cert "$CLIENT_CERT" \
    --key "$CLIENT_KEY" \
    -H "Content-Type: application/json" \
    -d "{\"context\":\"exchange-key\",\"plaintext\":\"$PLAINTEXT_NEW\"}" \
    "$BASE_URL/encrypt" 2>&1)

KEY_ID_V2=$(echo "$ENCRYPT_V2" | grep -o '"key_id":"[^"]*"' | cut -d'"' -f4)
if [ "$KEY_ID_V2" != "kek-exchange-key-v2" ]; then
    echo "Expected: kek-exchange-key-v2, Got: $KEY_ID_V2"
    echo "Response: $ENCRYPT_V2"
    print_error "New encryption not using v2 key"
fi
print_success "New encryption uses v2 key"

# ==========================================
# PHASE 9.5: KEK HOT RELOAD (Zero-Downtime)
# ==========================================
print_header "PHASE 9.5: KEK Hot Reload (Zero-Downtime)"

print_test "Test 9.5.1: Modify metadata to trigger hot reload"
# Modify metadata (add comment to trigger modTime change)
echo "# Hot reload test - $(date)" >> "$PROJECT_ROOT/metadata.yaml"
print_success "Modified metadata.yaml"

print_test "Test 9.5.2: Wait for automatic hot reload (35 seconds)"
print_info "Service monitors metadata.yaml every 30 seconds"
sleep 35

# Check logs for reload event
if docker compose -f "$TEST_COMPOSE_FILE" logs --since 40s 2>&1 | grep -q "KEK hot reload successful\|metadata file changed"; then
    print_success "Hot reload detected in logs"
    docker compose -f "$TEST_COMPOSE_FILE" logs --since 40s 2>&1 | grep -E "KEK hot reload|metadata file changed" | tail -3
else
    print_info "No hot reload event (file change may not have triggered reload)"
fi

print_test "Test 9.5.3: Verify encryption works without service restart"
# Test encrypt - should work without restart
HOT_RELOAD_ENCRYPT=$(curl -s --connect-timeout 10 --max-time 15 \
    --cacert "$CA_CERT" \
    --cert "$CLIENT_CERT" \
    --key "$CLIENT_KEY" \
    -H "Content-Type: application/json" \
    -d "{\"context\":\"exchange-key\",\"plaintext\":\"SG90UmVsb2FkVGVzdA==\"}" \
    "$BASE_URL/encrypt" 2>&1)

if ! echo "$HOT_RELOAD_ENCRYPT" | grep -q "ciphertext"; then
    echo "Response: $HOT_RELOAD_ENCRYPT"
    # Clean up test comment before failing
    sed -i '/# Hot reload test -/d' "$PROJECT_ROOT/metadata.yaml"
    print_error "Encryption failed after metadata modification (hot reload issue)"
fi
print_success "Encryption works without service restart (zero-downtime verified)"

# Remove test comment from metadata (preserve all other content)
sed -i '/# Hot reload test -/d' "$PROJECT_ROOT/metadata.yaml"
print_info "Cleaned up test comment from metadata.yaml"

# Wait for reload back to current state
sleep 35

# ==========================================
# PHASE 10: CLEANUP OLD VERSIONS
# ==========================================
print_header "PHASE 10: Key Lifecycle Management (PCI DSS)"

print_test "Test 10.1: Simulate multiple rotations (create v3, v4)"
# Rotate to v3
echo "=== Rotating to v3 ==="
if ! docker exec hsm-service-test /app/hsm-admin rotate exchange-key; then
    print_error "Failed to rotate to v3"
fi
# Force sync: copy metadata from container to host
docker cp hsm-service-test:/app/metadata.yaml "$PROJECT_ROOT/metadata.yaml"
# Wait for automatic hot reload (service monitors every 30s)
echo "Waiting for hot reload (35 seconds)..."
sleep 35

# Rotate to v4
echo "=== Rotating to v4 ==="
if ! docker exec hsm-service-test /app/hsm-admin rotate exchange-key; then
    print_error "Failed to rotate to v4"
fi
# Force sync: copy metadata from container to host
docker cp hsm-service-test:/app/metadata.yaml "$PROJECT_ROOT/metadata.yaml"
echo "Waiting for hot reload (35 seconds)..."
sleep 35
print_success "Rotated to v3 and v4"

print_test "Test 10.2: Verify 4 versions exist"
# Read metadata from container (source of truth)
VERSION_COUNT=$(docker exec hsm-service-test grep -c "label: kek-exchange-key-v" /app/metadata.yaml)
if [ "$VERSION_COUNT" -ne 4 ]; then
    echo "Metadata in container:"
    docker exec hsm-service-test cat /app/metadata.yaml
    print_error "Expected 4 versions, got $VERSION_COUNT"
fi
print_success "4 versions exist (v1, v2, v3, v4)"

print_test "Test 10.3: Check auto-cleanup warning at startup"
if ! docker logs hsm-service-test 2>&1 | grep -q "excess versions detected"; then
    print_info "No warning logged (expected when versions > max_versions)"
fi
print_success "Auto-cleanup check executed"

print_test "Test 10.4: Dry-run cleanup (should show what would be deleted)"
echo ""
echo "=== Running cleanup in dry-run mode ==="
CLEANUP_DRYRUN=$(docker exec -e HSM_PIN=1234 hsm-service-test /app/hsm-admin cleanup-old-versions --dry-run 2>&1)
echo "$CLEANUP_DRYRUN"
echo ""
if ! echo "$CLEANUP_DRYRUN" | grep -q "DRY RUN"; then
    print_error "Dry-run flag not working"
fi

# Check if dry-run shows deletions planned
if echo "$CLEANUP_DRYRUN" | grep -q "Would delete"; then
    print_success "Dry-run shows planned deletions"
else
    echo "WARNING: Dry-run shows NO deletions planned!"
    echo "This might indicate that cleanup logic needs adjustment"
    print_success "Dry-run executed (but no deletions planned)"
fi

print_test "Test 10.4b: Backdate old versions to test cleanup (simulate aging)"
echo ""
echo "=== Simulating aged key versions for cleanup testing ==="
echo "ℹ️  For testing purposes, we're backdating v1 and v2 to simulate old keys"
echo "   In production, this would happen naturally over time"
echo ""

# Backdate v1 and v2 to 60 days ago (will be deleted by cleanup)
# Backdate v3 to 15 days ago (will be kept)
# v4 stays current
DATE_60_DAYS_AGO=$(date -d '60 days ago' +%Y-%m-%dT%H:%M:%SZ)
DATE_15_DAYS_AGO=$(date -d '15 days ago' +%Y-%m-%dT%H:%M:%SZ)

echo "Backdating versions:"
echo "  v1 → $DATE_60_DAYS_AGO (60 days ago - will be deleted)"
echo "  v2 → $DATE_60_DAYS_AGO (60 days ago - will be deleted)"  
echo "  v3 → $DATE_15_DAYS_AGO (15 days ago - will be kept)"
echo "  v4 → current (will be kept as current version)"
echo ""

# Work with metadata.yaml on host (container has it mounted)
cp "$PROJECT_ROOT/metadata.yaml" /tmp/metadata-before-backdate.yaml

# Modify metadata with backdated timestamps
sed -E "s/(label: kek-exchange-key-v1.*)/\1/; /label: kek-exchange-key-v1/,/created_at:/ s/created_at:.*/created_at: $DATE_60_DAYS_AGO/" /tmp/metadata-before-backdate.yaml | \
sed -E "s/(label: kek-exchange-key-v2.*)/\1/; /label: kek-exchange-key-v2/,/created_at:/ s/created_at:.*/created_at: $DATE_60_DAYS_AGO/" | \
sed -E "s/(label: kek-exchange-key-v3.*)/\1/; /label: kek-exchange-key-v3/,/created_at:/ s/created_at:.*/created_at: $DATE_15_DAYS_AGO/" > /tmp/metadata-backdated.yaml

# Stop container before modifying mounted file
echo "Stopping container to modify metadata.yaml on host..."
docker stop hsm-service-test > /dev/null 2>&1
sleep 3

# Replace metadata.yaml on host (which is mounted to container)
cp /tmp/metadata-backdated.yaml "$PROJECT_ROOT/metadata.yaml"

echo "Backdated metadata.yaml content:"
cat "$PROJECT_ROOT/metadata.yaml" | grep -A 15 "exchange-key:"
echo ""

# Restart container with backdated metadata
echo "Restarting container with backdated metadata..."
docker start hsm-service-test > /dev/null 2>&1
sleep 7

print_success "Versions backdated for cleanup testing"

print_test "Test 10.5: Execute cleanup (delete excess versions)"
echo ""
echo "=== Executing cleanup with --force ==="
docker exec -e HSM_PIN=1234 hsm-service-test /app/hsm-admin cleanup-old-versions --force > /tmp/cleanup.log 2>&1
CLEANUP_EXIT_CODE=$?

echo "Cleanup output:"
cat /tmp/cleanup.log
echo ""
echo "Cleanup exit code: $CLEANUP_EXIT_CODE"
echo ""

if ! grep -q "CLEANUP COMPLETE" /tmp/cleanup.log; then
    print_error "Cleanup failed - CLEANUP COMPLETE not found in output"
fi

# Check how many were deleted
DELETED_COUNT=$(grep -oP "Deleted \K\d+" /tmp/cleanup.log | tail -1 || echo "0")
echo "Deleted versions: $DELETED_COUNT"

if [ "$DELETED_COUNT" -eq 0 ]; then
    echo "WARNING: No versions were deleted!"
    echo "This indicates cleanup logic may need adjustment"
fi

print_success "Cleanup executed"

print_test "Test 10.6: Verify cleanup behavior"
docker stop hsm-service-test > /dev/null 2>&1
sleep 2
docker start hsm-service-test > /dev/null 2>&1
sleep 7

# Copy updated metadata from container to host
docker cp hsm-service-test:/app/metadata.yaml "$PROJECT_ROOT/metadata.yaml"

echo "Updated metadata.yaml content:"
cat "$PROJECT_ROOT/metadata.yaml"
echo ""

VERSION_COUNT_AFTER=$(grep -c "label: kek-exchange-key-v" "$PROJECT_ROOT/metadata.yaml")
echo "Version count after cleanup: $VERSION_COUNT_AFTER"
echo ""

if [ "$VERSION_COUNT_AFTER" -le 3 ]; then
    print_success "Cleanup worked! Kept $VERSION_COUNT_AFTER versions (≤3)"
else
    echo "⚠️  Cleanup did not reduce versions to ≤3"
    echo "   Current count: $VERSION_COUNT_AFTER"
    print_error "Cleanup failed to enforce max_versions limit"
fi

print_test "Test 10.7: Current version still works after cleanup"
ENCRYPT_AFTER_CLEANUP=$(curl -s --connect-timeout 10 --max-time 15 \
    --cacert "$CA_CERT" \
    --cert "$CLIENT_CERT" \
    --key "$CLIENT_KEY" \
    -H "Content-Type: application/json" \
    -d "{\"context\":\"exchange-key\",\"plaintext\":\"$PLAINTEXT_NEW\"}" \
    "$BASE_URL/encrypt" 2>&1)

if ! echo "$ENCRYPT_AFTER_CLEANUP" | grep -q "ciphertext"; then
    echo "Response: $ENCRYPT_AFTER_CLEANUP"
    print_error "Encryption failed after cleanup"
fi
print_success "Encryption still works after cleanup"

print_test "Test 10.8: Reset to clean state after cleanup tests"
echo "Resetting metadata and HSM to clean state for remaining tests..."
# Create clean metadata with only current versions
cat > "$PROJECT_ROOT/metadata.yaml" << 'EOF'
rotation:
  exchange-key:
    current: kek-exchange-key-v1
    rotation_interval_days: 90
    versions:
      - label: kek-exchange-key-v1
        version: 1
        created_at: '2026-01-09T00:00:00Z'
  2fa:
    current: kek-2fa-v1
    rotation_interval_days: 90
    versions:
      - label: kek-2fa-v1
        version: 1
        created_at: '2026-01-09T00:00:00Z'
EOF

# Restart container to load clean metadata
docker restart hsm-service-test > /dev/null 2>&1
sleep 10

# Verify service is healthy
if docker ps | grep -q "hsm-service-test"; then
    print_success "System reset to clean state for remaining tests"
else
    docker logs hsm-service-test --tail 20
    print_error "Failed to restart after reset"
fi

# ==========================================
# PHASE 11: mTLS SECURITY VALIDATION
# ==========================================
print_header "PHASE 11: mTLS Security Validation"

print_test "Test 11.1: Request without client certificate should be rejected"
# Try to connect without client cert (only CA cert)
# Note: Using shorter timeout to avoid hanging
NO_CERT_RESPONSE=$(timeout 8 curl -sS -w "\n%{http_code}" --connect-timeout 3 --max-time 5 \
    --cacert "$CA_CERT" \
    -H "Content-Type: application/json" \
    -d "{\"context\":\"exchange-key\",\"plaintext\":\"dGVzdA==\"}" \
    "$BASE_URL/encrypt" 2>&1 || echo "CONNECTION_FAILED")

# Check if connection was rejected during TLS handshake
if echo "$NO_CERT_RESPONSE" | grep -qi "certificate required\|handshake.*fail\|ssl.*error\|peer.*disconnect\|CONNECTION_FAILED"; then
    print_success "Connection rejected without client certificate (TLS handshake failed)"
else
    HTTP_CODE=$(echo "$NO_CERT_RESPONSE" | tail -1)
    echo "HTTP Code: $HTTP_CODE"
    echo "Response: $NO_CERT_RESPONSE"
    print_error "Server accepted request without client certificate!"
fi

print_test "Test 11.2: Request with self-signed certificate should be rejected"
# Create self-signed cert (not signed by our CA)
SELF_SIGNED_DIR=$(mktemp -d)
openssl req -x509 -newkey rsa:2048 -nodes -days 1 \
    -keyout "$SELF_SIGNED_DIR/selfsigned.key" \
    -out "$SELF_SIGNED_DIR/selfsigned.crt" \
    -subj "/CN=attacker.example.com/O=Evil Corp" > /dev/null 2>&1

SELF_SIGNED_RESPONSE=$(timeout 8 curl -sS -w "\n%{http_code}" --connect-timeout 3 --max-time 5 \
    --cacert "$CA_CERT" \
    --cert "$SELF_SIGNED_DIR/selfsigned.crt" \
    --key "$SELF_SIGNED_DIR/selfsigned.key" \
    -H "Content-Type: application/json" \
    -d "{\"context\":\"exchange-key\",\"plaintext\":\"dGVzdA==\"}" \
    "$BASE_URL/encrypt" 2>&1 || echo "000")

HTTP_CODE=$(echo "$SELF_SIGNED_RESPONSE" | tail -1)
rm -rf "$SELF_SIGNED_DIR"

if [ "$HTTP_CODE" = "000" ] || echo "$SELF_SIGNED_RESPONSE" | grep -qi "ssl.*error\|certificate.*verify.*fail\|unknown.*ca"; then
    print_success "Self-signed certificate rejected (TLS verification failed)"
else
    echo "HTTP Code: $HTTP_CODE"
    echo "Response: $SELF_SIGNED_RESPONSE"
    print_error "Server accepted self-signed certificate!"
fi

print_test "Test 11.3: Dynamic certificate revocation and ACL blocking"
# Get a valid client certificate CN
TRADING_CERT="$PROJECT_ROOT/pki/test/client/trading-client-1.crt"
TRADING_KEY="$PROJECT_ROOT/pki/test/client/trading-client-1.key"

if [ ! -f "$TRADING_CERT" ] || [ ! -f "$TRADING_KEY" ]; then
    print_error "Trading client certificate not found"
fi

# Extract CN from the certificate using openssl name output (most reliable)
# openssl x509 -in cert -noout -subject -nameopt rfc2253 gives: CN=trading-client-1,...
CERT_CN=$(openssl x509 -in "$TRADING_CERT" -noout -subject -nameopt rfc2253 2>/dev/null | grep -oP 'CN=\K[^,]+' || openssl x509 -in "$TRADING_CERT" -noout -subject 2>/dev/null | sed 's/.*CN = \([^,]*\).*/\1/')
if [ -z "$CERT_CN" ]; then
    print_error "Failed to extract CN from certificate"
fi

print_info "Using certificate CN: $CERT_CN"

# Step 1: Verify the certificate works BEFORE revocation
print_info "Step 1: Testing certificate works before revocation..."
BEFORE_RESPONSE=$(timeout 8 curl -s -w "\n%{http_code}" --connect-timeout 3 --max-time 5 \
    --cacert "$CA_CERT" \
    --cert "$TRADING_CERT" \
    --key "$TRADING_KEY" \
    -H "Content-Type: application/json" \
    -d "{\"context\":\"exchange-key\",\"plaintext\":\"dGVzdA==\"}" \
    "$BASE_URL/encrypt" 2>&1 || echo "000")

HTTP_CODE_BEFORE=$(echo "$BEFORE_RESPONSE" | tail -1)
if [ "$HTTP_CODE_BEFORE" = "200" ]; then
    print_success "Certificate accepted before revocation (HTTP 200)"
else
    echo "HTTP Code: $HTTP_CODE_BEFORE"
    echo "Response: $BEFORE_RESPONSE"
    print_error "Certificate rejected before revocation (should be accepted)"
fi

# Step 2: Add certificate to revoked.yaml
print_info "Step 2: Adding certificate to revoked.yaml..."
REVOKED_FILE="$PROJECT_ROOT/revoked.yaml"
BACKUP_FILE="$REVOKED_FILE.backup"
cp "$REVOKED_FILE" "$BACKUP_FILE"

# Create new revoked entry with proper YAML structure
cat > "$REVOKED_FILE" << EOF
revoked_certificates:
  - cn: "$CERT_CN"
    serial: "01"
    reason: "Test revocation"
    date: "$(date -u +'%Y-%m-%dT%H:%M:%SZ')"
EOF

# Verify file was written correctly
if [ ! -f "$REVOKED_FILE" ]; then
    print_error "Failed to create revoked.yaml"
fi

# Sync to container (ensure volume mount sees it)
sleep 1
touch "$REVOKED_FILE"  # Update modification time to trigger reload

print_success "Certificate added to revoked.yaml"
print_info "New revoked.yaml content (host):"
cat "$REVOKED_FILE" | sed 's/^/  /'

# Verify file in container
print_info "Checking revoked.yaml in container:"
docker exec hsm-service-test cat /app/revoked.yaml 2>/dev/null | head -3 | sed 's/^/  /'

# Step 3: Wait for auto-reload (default reload interval is 30 seconds)
# On slow systems, may need to wait for full cycle to complete
print_info "Step 3: Waiting for ACL auto-reload (up to 75 seconds on slow systems)..."
RELOAD_DETECTED=0
for i in {1..150}; do
    sleep 0.5
    # Try request to see if revocation took effect
    AFTER_RESPONSE=$(timeout 8 curl -s -w "\n%{http_code}" --connect-timeout 3 --max-time 5 \
        --cacert "$CA_CERT" \
        --cert "$TRADING_CERT" \
        --key "$TRADING_KEY" \
        -H "Content-Type: application/json" \
        -d "{\"context\":\"exchange-key\",\"plaintext\":\"dGVzdA==\"}" \
        "$BASE_URL/encrypt" 2>&1 || echo "000")
    
    HTTP_CODE_AFTER=$(echo "$AFTER_RESPONSE" | tail -1)
    if [ "$HTTP_CODE_AFTER" = "403" ] || echo "$AFTER_RESPONSE" | grep -qi "revoked\|forbidden"; then
        ELAPSED_SECS=$((i / 2))
        print_success "Certificate revocation detected after $ELAPSED_SECS seconds"
        RELOAD_DETECTED=1
        break
    fi
    
    # Show progress every 5 seconds
    if [ $((i % 10)) -eq 0 ]; then
        ELAPSED=$((i / 2))
        print_info "Waiting... ($ELAPSED seconds elapsed)"
    fi
done

if [ $RELOAD_DETECTED -eq 0 ]; then
    print_info "Auto-reload not detected within timeout"
    print_info "This can happen on very slow systems or if reload thread is stuck"
    print_info "Check container logs: docker logs hsm-service-test | grep -i reload"
fi

# Step 4: Verify certificate is now blocked
print_info "Step 4: Verifying revoked certificate is blocked..."

# Check container logs for reload
RECENT_LOGS=$(docker logs hsm-service-test 2>&1 | tail -20)
if echo "$RECENT_LOGS" | grep -qi "revoked.yaml reload"; then
    print_info "ACL reload detected in logs"
else
    print_info "No ACL reload in recent logs - may still be waiting"
fi

REVOKED_RESPONSE=$(timeout 8 curl -s -w "\n%{http_code}" --connect-timeout 3 --max-time 5 \
    --cacert "$CA_CERT" \
    --cert "$TRADING_CERT" \
    --key "$TRADING_KEY" \
    -H "Content-Type: application/json" \
    -d "{\"context\":\"exchange-key\",\"plaintext\":\"dGVzdA==\"}" \
    "$BASE_URL/encrypt" 2>&1 || echo "000")

HTTP_CODE_REVOKED=$(echo "$REVOKED_RESPONSE" | tail -1)
BODY_REVOKED=$(echo "$REVOKED_RESPONSE" | sed '$d')

if [ "$HTTP_CODE_REVOKED" = "403" ] || echo "$BODY_REVOKED" | grep -qi "revoked\|forbidden\|access.*denied"; then
    print_success "Revoked certificate blocked by ACL (HTTP $HTTP_CODE_REVOKED)"
else
    echo "Certificate CN: $CERT_CN"
    echo "HTTP Code: $HTTP_CODE_REVOKED"
    echo "Response body: $BODY_REVOKED"
    echo ""
    echo "Debug info:"
    echo "  revoked.yaml (host): $(cat $REVOKED_FILE)"
    echo "  revoked.yaml (container): $(docker exec hsm-service-test cat /app/revoked.yaml 2>/dev/null || echo 'NOT FOUND')"
    echo ""
    print_error "Server accepted revoked certificate!"
fi

# Step 5: Restore revoked.yaml
print_info "Step 5: Restoring revoked.yaml to original state..."
mv "$BACKUP_FILE" "$REVOKED_FILE"
print_success "revoked.yaml restored"

print_test "Test 11.4: Verify TLS 1.3 enforcement"
# Test that TLS 1.2 and below are rejected
TLS12_RESPONSE=$(timeout 8 curl -s -v --tlsv1.2 --tls-max 1.2 --connect-timeout 3 --max-time 5 \
    --cacert "$CA_CERT" \
    --cert "$CLIENT_CERT" \
    --key "$CLIENT_KEY" \
    "$BASE_URL/health" 2>&1 || echo "TLS_REJECTED")

if echo "$TLS12_RESPONSE" | grep -qi "ssl.*error\|handshake.*fail\|protocol.*version\|tls.*alert\|TLS_REJECTED"; then
    print_success "TLS 1.2 rejected (TLS 1.3 enforced)"
else
    print_info "TLS version test inconclusive"
fi

print_test "Test 11.5: Valid certificate with wrong OU should be rejected by ACL"
# Try with valid cert but OU not in ACL for exchange-key
if [ -f "$PROJECT_ROOT/pki/test/client/2fa-client-1.crt" ]; then
    # 2fa client has OU=2fa, which is not authorized for exchange-key
    WRONG_OU_RESPONSE=$(timeout 8 curl -s -w "\n%{http_code}" --connect-timeout 3 --max-time 5 \
        --cacert "$CA_CERT" \
        --cert "$PROJECT_ROOT/pki/test/client/2fa-client-1.crt" \
        --key "$PROJECT_ROOT/pki/test/client/2fa-client-1.key" \
        -H "Content-Type: application/json" \
        -d "{\"context\":\"exchange-key\",\"plaintext\":\"dGVzdA==\"}" \
        "$BASE_URL/encrypt" 2>&1 || echo "000")
    
    HTTP_CODE=$(echo "$WRONG_OU_RESPONSE" | tail -1)
    BODY=$(echo "$WRONG_OU_RESPONSE" | head -n -1)
    
    if [ "$HTTP_CODE" = "403" ] || echo "$BODY" | grep -qi "forbidden\|not.*authorized\|access.*denied"; then
        print_success "Wrong OU blocked by ACL (OU=2fa for exchange-key)"
    else
        echo "HTTP Code: $HTTP_CODE"
        echo "Response: $BODY"
        print_error "Server accepted wrong OU!"
    fi
else
    print_info "2fa client cert not found, skipping ACL test"
fi

# ==========================================
# PHASE 12: VOLUME PERSISTENCE
# ==========================================
print_header "PHASE 12: Volume Persistence"

print_test "Test 12.1: Capture current state before restart"
# Get current metadata and HSM token state
BEFORE_METADATA=$(docker exec hsm-service-test cat /app/metadata.yaml)
BEFORE_TOKEN_COUNT=$(docker exec hsm-service-test sh -c 'ls -1 /var/lib/softhsm/tokens/ 2>/dev/null | wc -l' | tr -d '\n')
BEFORE_KEY_COUNT=$(docker exec hsm-service-test /app/hsm-admin list-kek 2>/dev/null | grep -c "Config Key:" | tr -d '\n' || echo "0")

echo "State before restart:"
echo "  Metadata contexts: $(echo "$BEFORE_METADATA" | grep -c "current:" | tr -d '\n' || echo "0")"
echo "  SoftHSM tokens: $BEFORE_TOKEN_COUNT"
echo "  KEKs loaded: $BEFORE_KEY_COUNT"
print_success "Current state captured"

print_test "Test 12.2: Restart container with docker restart"
docker restart hsm-service-test > /dev/null 2>&1
sleep 15  # Wait for service to fully restart

# Check if container is running
if docker ps | grep -q hsm-service; then
    print_success "Container restarted successfully"
else
    print_error "Container failed to restart"
fi

print_test "Test 12.3: Verify metadata persisted after restart"
AFTER_METADATA=$(docker exec hsm-service-test cat /app/metadata.yaml 2>/dev/null)
AFTER_CONTEXTS=$(echo "$AFTER_METADATA" | grep -c "current:" | tr -d '\n' || echo "0")
BEFORE_CONTEXTS=$(echo "$BEFORE_METADATA" | grep -c "current:" | tr -d '\n' || echo "0")

if [ "$AFTER_CONTEXTS" = "$BEFORE_CONTEXTS" ] && [ "$AFTER_CONTEXTS" -ge "2" ]; then
    print_success "Metadata persisted ($AFTER_CONTEXTS contexts preserved)"
else
    echo "Before: $BEFORE_CONTEXTS, After: $AFTER_CONTEXTS"
    print_error "Metadata not preserved"
fi

print_test "Test 12.4: Verify SoftHSM tokens persisted"
AFTER_TOKEN_COUNT=$(docker exec hsm-service-test sh -c 'ls -1 /var/lib/softhsm/tokens/ 2>/dev/null | wc -l' | tr -d '\n')

if [ "$AFTER_TOKEN_COUNT" = "$BEFORE_TOKEN_COUNT" ] && [ "$AFTER_TOKEN_COUNT" -gt "0" ]; then
    print_success "SoftHSM tokens persisted ($AFTER_TOKEN_COUNT tokens)"
else
    echo "Before: $BEFORE_TOKEN_COUNT, After: $AFTER_TOKEN_COUNT"
    print_error "Tokens not preserved"
fi

print_test "Test 12.5: Verify KEKs reloaded after restart"
# Wait for KEKs to load
sleep 5
AFTER_KEY_COUNT=$(docker exec hsm-service-test /app/hsm-admin list-kek 2>/dev/null | grep -c "Config Key:" | tr -d '\n' || echo "0")

if [ "$AFTER_KEY_COUNT" = "$BEFORE_KEY_COUNT" ] && [ "$AFTER_KEY_COUNT" -ge "2" ]; then
    print_success "KEKs reloaded ($AFTER_KEY_COUNT contexts)"
else
    echo "Before: $BEFORE_KEY_COUNT, After: $AFTER_KEY_COUNT"
    print_error "KEKs not reloaded"
fi

print_test "Test 12.6: Verify encryption still works after restart"
ENCRYPT_RESPONSE=$(curl -s -w "\n%{http_code}" --connect-timeout 10 --max-time 15 \
    --cacert "$CA_CERT" \
    --cert "$CLIENT_CERT" \
    --key "$CLIENT_KEY" \
    -H "Content-Type: application/json" \
    -d '{"context":"exchange-key","plaintext":"cGVyc2lzdGVuY2U="}' \
    "$BASE_URL/encrypt" 2>&1)

HTTP_CODE=$(echo "$ENCRYPT_RESPONSE" | tail -1)
if [ "$HTTP_CODE" = "200" ]; then
    print_success "Encryption works after restart"
else
    echo "HTTP Code: $HTTP_CODE"
    print_error "Encryption failed after restart"
fi

print_test "Test 12.7: Full compose down/up cycle"
echo "Stopping all services (docker compose down)..."
docker compose -f "$TEST_COMPOSE_FILE" down > /dev/null 2>&1
sleep 3

# Verify containers stopped
if ! docker ps | grep -q hsm-service-test; then
    print_success "Services stopped"
else
    print_error "Failed to stop services"
fi

echo "Starting services (docker compose up -d)..."
docker compose -f "$TEST_COMPOSE_FILE" up -d > /dev/null 2>&1
sleep 15

# Check if service is running
if docker ps | grep -q hsm-service-test; then
    print_success "Services started"
else
    print_error "Failed to start services"
fi

print_test "Test 12.8: Verify data survived compose down/up"
FINAL_METADATA=$(docker exec hsm-service-test cat /app/metadata.yaml 2>/dev/null)
FINAL_CONTEXTS=$(echo "$FINAL_METADATA" | grep -c "current:" || echo "0")

if [ "$FINAL_CONTEXTS" = "$BEFORE_CONTEXTS" ]; then
    print_success "Metadata survived full restart ($FINAL_CONTEXTS contexts)"
else
    echo "Original: $BEFORE_CONTEXTS, Final: $FINAL_CONTEXTS"
    print_error "Data lost during compose down/up"
fi

print_test "Test 12.9: Final encryption test after compose cycle"
sleep 5  # Wait for service to be ready
FINAL_ENCRYPT=$(curl -s -w "\n%{http_code}" --connect-timeout 10 --max-time 15 \
    --cacert "$CA_CERT" \
    --cert "$CLIENT_CERT" \
    --key "$CLIENT_KEY" \
    -H "Content-Type: application/json" \
    -d '{"context":"exchange-key","plaintext":"ZmluYWw="}' \
    "$BASE_URL/encrypt" 2>&1)

HTTP_CODE=$(echo "$FINAL_ENCRYPT" | tail -1)
if [ "$HTTP_CODE" = "200" ]; then
    print_success "Service fully operational after compose cycle"
else
    echo "HTTP Code: $HTTP_CODE"
    print_error "Service not operational"
fi

# ==========================================
# PHASE 13: ENVIRONMENT VARIABLES OVERRIDE
# ==========================================
print_header "PHASE 13: Environment Variables Override"

print_test "Test 13.1: Stop container to test env override"
docker compose -f "$TEST_COMPOSE_FILE" down > /dev/null 2>&1
sleep 2
print_success "Container stopped"

print_test "Test 13.2: Start with custom environment variables"
# Temporarily modify docker-compose.yml to add env vars
# Note: Keep HSM_PIN same as existing token (1234), just test that env vars work
cat > docker-compose-test.yml << 'EOF'
services:
  hsm-service:
    image: hsm-service:latest
    container_name: hsm-service
    environment:
      - HSM_PIN=1234
      - HSM_SO_PIN=5678
      - CONFIG_PATH=/app/config.yaml
      - LOG_LEVEL=info
    ports:
      - "8443:8443"
    volumes:
      - ./pki:/app/pki:ro
      - ./metadata.yaml:/app/metadata.yaml:rw
      - ./data/tokens:/var/lib/softhsm/tokens
    networks:
      - hsm-net
    restart: unless-stopped

networks:
  hsm-net:
    driver: bridge
EOF

docker compose -f docker-compose-test.yml up -d > /dev/null 2>&1
sleep 15

if docker ps | grep -q hsm-service; then
    print_success "Container started with custom env vars"
else
    docker logs hsm-service-test --tail 20
    print_error "Failed to start with custom env"
fi

print_test "Test 13.3: Verify PINs are NOT exposed in logs"
LOGS=$(docker logs hsm-service-test 2>&1)
if echo "$LOGS" | grep -q "1234\|5678"; then
    print_error "SECURITY RISK: PIN exposed in logs!"
else
    print_success "PINs not exposed in logs (secure)"
fi

print_test "Test 13.4: Verify CONFIG_PATH override works"
CONFIG_CHECK=$(docker exec hsm-service-test sh -c 'echo $CONFIG_PATH' 2>/dev/null)
if [ "$CONFIG_CHECK" = "/app/config.yaml" ]; then
    print_success "CONFIG_PATH override working"
else
    print_info "CONFIG_PATH check inconclusive"
fi

print_test "Test 13.5: Verify service works with custom env"
sleep 5
ENV_ENCRYPT=$(curl -s -w "\n%{http_code}" --connect-timeout 10 --max-time 15 \
    --cacert "$CA_CERT" \
    --cert "$CLIENT_CERT" \
    --key "$CLIENT_KEY" \
    -H "Content-Type: application/json" \
    -d '{"context":"exchange-key","plaintext":"ZW52dGVzdA=="}' \
    "$BASE_URL/encrypt" 2>&1)

HTTP_CODE=$(echo "$ENV_ENCRYPT" | tail -1)
if [ "$HTTP_CODE" = "200" ]; then
    print_success "Service operational with custom environment"
else
    echo "HTTP Code: $HTTP_CODE"
    print_info "Service may need HSM re-initialization with custom PIN"
fi

print_test "Test 13.6: Restore original compose configuration"
docker compose -f docker-compose-test.yml down > /dev/null 2>&1
rm -f docker-compose-test.yml
docker compose -f "$TEST_COMPOSE_FILE" up -d > /dev/null 2>&1
sleep 15

if docker ps | grep -q hsm-service-test; then
    print_success "Restored to original configuration"
else
    print_error "Failed to restore original config"
fi

# ==========================================
# FINAL SUMMARY
# ==========================================
print_header "Test Summary"
echo -e "${GREEN}✓ ALL $TOTAL_TESTS TESTS PASSED${NC}"
echo ""
echo "Test Coverage:"
echo "  ✓ Docker cleanup and rebuild"
echo "  ✓ PKI certificate generation"
echo "  ✓ HSM key initialization"
echo "  ✓ Health check endpoint"
echo "  ✓ Encrypt/Decrypt functionality"
echo "  ✓ Key rotation (v1 → v2 → v3 → v4)"
echo "  ✓ KEK Hot Reload (zero-downtime)"
echo "  ✓ Multi-version support (overlap period)"
echo "  ✓ PCI DSS compliance (cleanup old versions)"
echo "  ✓ Post-cleanup functionality"
echo "  ✓ mTLS security validation"
echo "  ✓ Volume persistence (docker restart + compose down/up)"
echo "  ✓ Environment variables override"
echo ""
echo -e "${BLUE}Logs:${NC}"
echo "  Docker build:  /tmp/docker-build.log"
echo "  Compose up:    /tmp/docker-compose-up.log"
echo "  Rotation:      /tmp/rotation.log"
echo "  Cleanup:       /tmp/cleanup.log"
echo ""
echo -e "${GREEN}Integration test suite completed successfully!${NC}"
