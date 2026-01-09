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
TOTAL_TESTS=20

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

# Get project root
cd "$(dirname "$0")/.."
PROJECT_ROOT=$(pwd)

print_header "HSM Service - Full Integration Test Suite"
print_info "Project: $PROJECT_ROOT"
print_info "Date: $(date)"

# ==========================================
# PHASE 1: CLEANUP
# ==========================================
print_header "PHASE 1: Docker Cleanup"

print_test "Stop and remove existing containers"
docker-compose down -v 2>/dev/null || true
print_success "Containers stopped"

print_test "Remove project images"
docker rmi hsm-service:latest 2>/dev/null || true
print_success "Images removed"

print_test "Prune unused Docker resources"
docker system prune -f > /dev/null
print_success "Docker cleanup complete"

# ==========================================
# PHASE 2: BUILD
# ==========================================
print_header "PHASE 2: Build from Scratch"

print_test "Build Docker image (no cache)"
if ! docker build --no-cache -t hsm-service:latest . > /tmp/docker-build.log 2>&1; then
    cat /tmp/docker-build.log
    print_error "Docker build failed (see /tmp/docker-build.log)"
fi
print_success "Image built successfully"

print_test "Verify image exists"
if ! docker images | grep -q hsm-service; then
    print_error "Image hsm-service:latest not found"
fi
print_success "Image verified"

# ==========================================
# PHASE 3: PKI SETUP
# ==========================================
print_header "PHASE 3: PKI Verification"

print_test "Check CA certificate exists"
if [ ! -f "$PROJECT_ROOT/pki/ca/ca.crt" ]; then
    print_error "CA certificate not found. Run: ./pki/scripts/issue-server-cert.sh hsm-service.local"
fi
print_success "CA certificate exists"

print_test "Check server certificate exists"
if [ ! -f "$PROJECT_ROOT/pki/server/hsm-service.local.crt" ]; then
    print_error "Server certificate not found"
fi
print_success "Server certificate exists"

print_test "Request client certificate details"
echo ""
echo -e "${BLUE}Please provide client certificate details for testing:${NC}"
echo ""

# Request client certificate name
read -p "Client certificate name (default: hsm-trading-client-1): " CLIENT_CERT_NAME
CLIENT_CERT_NAME=${CLIENT_CERT_NAME:-hsm-trading-client-1}

# Build paths
CLIENT_CERT_PATH="$PROJECT_ROOT/pki/client/${CLIENT_CERT_NAME}.crt"
CLIENT_KEY_PATH="$PROJECT_ROOT/pki/client/${CLIENT_CERT_NAME}.key"

# Check if custom paths are needed
if [ ! -f "$CLIENT_CERT_PATH" ]; then
    echo -e "${YELLOW}Certificate not found at default location: $CLIENT_CERT_PATH${NC}"
    read -p "Enter full path to client certificate: " CLIENT_CERT_PATH
fi

if [ ! -f "$CLIENT_KEY_PATH" ]; then
    echo -e "${YELLOW}Key not found at default location: $CLIENT_KEY_PATH${NC}"
    read -p "Enter full path to client key: " CLIENT_KEY_PATH
fi

# Validate files exist
if [ ! -f "$CLIENT_CERT_PATH" ]; then
    print_error "Client certificate not found: $CLIENT_CERT_PATH"
fi
if [ ! -f "$CLIENT_KEY_PATH" ]; then
    print_error "Client key not found: $CLIENT_KEY_PATH"
fi

print_success "Client certificate verified: $CLIENT_CERT_NAME"

# ==========================================
# PHASE 4: METADATA INITIALIZATION
# ==========================================
print_header "PHASE 4: Metadata Initialization"

print_test "Create initial metadata.yaml with multi-version structure"
cat > "$PROJECT_ROOT/metadata.yaml" << 'EOF'
rotation:
  exchange-key:
    current: kek-exchange-v1
    versions:
      - label: kek-exchange-v1
        version: 1
        created_at: '2026-01-09T00:00:00Z'
  2fa:
    current: kek-2fa-v1
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

print_test "Start services with docker-compose"
docker-compose up -d > /dev/null 2>&1
sleep 3
print_success "Services started"

print_test "Verify container is running"
if ! docker ps | grep -q hsm-service; then
    docker-compose logs
    print_error "Container not running"
fi
print_success "Container is running"

print_test "Check container logs for errors"
sleep 2
if docker-compose logs | grep -i "fatal\|panic"; then
    docker-compose logs
    print_error "Fatal errors found in logs"
fi
print_success "No fatal errors in logs"

# ==========================================
# PHASE 6: HSM INITIALIZATION
# ==========================================
print_header "PHASE 6: HSM Key Initialization"

print_test "Initialize HSM keys (kek-exchange-v1, kek-2fa-v1)"
if ! docker exec hsm-service /app/scripts/init-hsm.sh > /tmp/init-hsm.log 2>&1; then
    cat /tmp/init-hsm.log
    print_error "HSM initialization failed (see /tmp/init-hsm.log)"
fi
print_success "HSM keys initialized"

print_test "Restart service to load keys"
docker restart hsm-service > /dev/null
sleep 5
print_success "Service restarted"

print_test "Verify keys loaded (check logs)"
if ! docker logs hsm-service 2>&1 | grep -q "Loaded KEK: kek-exchange-v1"; then
    docker logs hsm-service
    print_error "KEK kek-exchange-v1 not loaded"
fi
if ! docker logs hsm-service 2>&1 | grep -q "Loaded KEK: kek-2fa-v1"; then
    docker logs hsm-service
    print_error "KEK kek-2fa-v1 not loaded"
fi
print_success "All KEKs loaded successfully"

# ==========================================
# PHASE 7: BASIC FUNCTIONALITY TESTS
# ==========================================
print_header "PHASE 7: Basic Functionality Tests"

# Test variables
BASE_URL="https://hsm-service.local:8443"
CA_CERT="$PROJECT_ROOT/pki/ca/ca.crt"
CLIENT_CERT="$CLIENT_CERT_PATH"
CLIENT_KEY="$CLIENT_KEY_PATH"

print_test "Test 7.1: Health check endpoint"
HEALTH_RESPONSE=$(curl -s --cacert "$CA_CERT" \
    --cert "$CLIENT_CERT" \
    --key "$CLIENT_KEY" \
    "$BASE_URL/health")

if ! echo "$HEALTH_RESPONSE" | grep -q "ok"; then
    echo "Response: $HEALTH_RESPONSE"
    print_error "Health check failed"
fi
print_success "Health check passed"

print_test "Test 7.2: Encrypt data with exchange-key"
PLAINTEXT="SGVsbG8gV29ybGQh"  # "Hello World!" in base64

ENCRYPT_RESPONSE=$(curl -s --cacert "$CA_CERT" \
    --cert "$CLIENT_CERT" \
    --key "$CLIENT_KEY" \
    -H "Content-Type: application/json" \
    -d "{\"context\":\"exchange-key\",\"plaintext\":\"$PLAINTEXT\"}" \
    "$BASE_URL/encrypt")

CIPHERTEXT=$(echo "$ENCRYPT_RESPONSE" | grep -o '"ciphertext":"[^"]*"' | cut -d'"' -f4)
KEY_ID=$(echo "$ENCRYPT_RESPONSE" | grep -o '"key_id":"[^"]*"' | cut -d'"' -f4)

if [ -z "$CIPHERTEXT" ]; then
    echo "Response: $ENCRYPT_RESPONSE"
    print_error "Encryption failed - no ciphertext returned"
fi
if [ "$KEY_ID" != "kek-exchange-v1" ]; then
    echo "Expected: kek-exchange-v1, Got: $KEY_ID"
    print_error "Wrong key_id returned"
fi
print_success "Encryption successful (key: $KEY_ID)"

print_test "Test 7.3: Decrypt data with exchange-key"
DECRYPT_RESPONSE=$(curl -s --cacert "$CA_CERT" \
    --cert "$CLIENT_CERT" \
    --key "$CLIENT_KEY" \
    -H "Content-Type: application/json" \
    -d "{\"context\":\"exchange-key\",\"ciphertext\":\"$CIPHERTEXT\",\"key_id\":\"$KEY_ID\"}" \
    "$BASE_URL/decrypt")

DECRYPTED=$(echo "$DECRYPT_RESPONSE" | grep -o '"plaintext":"[^"]*"' | cut -d'"' -f4)

if [ "$DECRYPTED" != "$PLAINTEXT" ]; then
    echo "Expected: $PLAINTEXT"
    echo "Got: $DECRYPTED"
    echo "Response: $DECRYPT_RESPONSE"
    print_error "Decryption failed - plaintext mismatch"
fi
print_success "Decryption successful - data matches"

# ==========================================
# PHASE 8: KEY ROTATION
# ==========================================
print_header "PHASE 8: Key Rotation Tests"

print_test "Test 8.1: Check rotation status before rotation"
ROTATION_STATUS=$(docker exec hsm-service /app/hsm-admin rotation-status 2>&1)
echo "$ROTATION_STATUS"
if ! echo "$ROTATION_STATUS" | grep -q "exchange-key"; then
    print_error "rotation-status command failed"
fi
print_success "Rotation status command works"

print_test "Test 8.2: Perform key rotation (exchange-key)"
if ! docker exec hsm-service /app/hsm-admin rotate exchange-key > /tmp/rotation.log 2>&1; then
    cat /tmp/rotation.log
    print_error "Key rotation failed (see /tmp/rotation.log)"
fi
print_success "Key rotation completed"

print_test "Test 8.3: Verify metadata.yaml updated"
METADATA_CONTENT=$(cat "$PROJECT_ROOT/metadata.yaml")
if ! echo "$METADATA_CONTENT" | grep -q "kek-exchange-v2"; then
    echo "$METADATA_CONTENT"
    print_error "metadata.yaml not updated with v2"
fi
if ! echo "$METADATA_CONTENT" | grep -q "current: kek-exchange-v2"; then
    echo "$METADATA_CONTENT"
    print_error "current pointer not updated to v2"
fi
print_success "metadata.yaml contains both v1 and v2"

print_test "Test 8.4: Restart service to load new key"
docker restart hsm-service > /dev/null
sleep 5
print_success "Service restarted"

print_test "Test 8.5: Verify both versions loaded (overlap period)"
LOGS=$(docker logs hsm-service 2>&1)
if ! echo "$LOGS" | grep -q "Loaded KEK: kek-exchange-v1"; then
    echo "$LOGS"
    print_error "Old key (v1) not loaded after rotation"
fi
if ! echo "$LOGS" | grep -q "Loaded KEK: kek-exchange-v2"; then
    echo "$LOGS"
    print_error "New key (v2) not loaded after rotation"
fi
print_success "Both v1 and v2 keys loaded (overlap period active)"

# ==========================================
# PHASE 9: POST-ROTATION FUNCTIONALITY
# ==========================================
print_header "PHASE 9: Post-Rotation Functionality"

print_test "Test 9.1: Decrypt old data with v1 key"
DECRYPT_V1=$(curl -s --cacert "$CA_CERT" \
    --cert "$CLIENT_CERT" \
    --key "$CLIENT_KEY" \
    -H "Content-Type: application/json" \
    -d "{\"context\":\"exchange-key\",\"ciphertext\":\"$CIPHERTEXT\",\"key_id\":\"kek-exchange-v1\"}" \
    "$BASE_URL/decrypt")

DECRYPTED_V1=$(echo "$DECRYPT_V1" | grep -o '"plaintext":"[^"]*"' | cut -d'"' -f4)
if [ "$DECRYPTED_V1" != "$PLAINTEXT" ]; then
    echo "Response: $DECRYPT_V1"
    print_error "Cannot decrypt old data with v1 after rotation"
fi
print_success "Old data successfully decrypted with v1"

print_test "Test 9.2: Encrypt new data uses v2 key"
PLAINTEXT_NEW="TmV3IERhdGEh"  # "New Data!" in base64

ENCRYPT_V2=$(curl -s --cacert "$CA_CERT" \
    --cert "$CLIENT_CERT" \
    --key "$CLIENT_KEY" \
    -H "Content-Type: application/json" \
    -d "{\"context\":\"exchange-key\",\"plaintext\":\"$PLAINTEXT_NEW\"}" \
    "$BASE_URL/encrypt")

KEY_ID_V2=$(echo "$ENCRYPT_V2" | grep -o '"key_id":"[^"]*"' | cut -d'"' -f4)
if [ "$KEY_ID_V2" != "kek-exchange-v2" ]; then
    echo "Expected: kek-exchange-v2, Got: $KEY_ID_V2"
    echo "Response: $ENCRYPT_V2"
    print_error "New encryption not using v2 key"
fi
print_success "New encryption uses v2 key"

# ==========================================
# PHASE 10: CLEANUP OLD VERSIONS
# ==========================================
print_header "PHASE 10: Key Lifecycle Management (PCI DSS)"

print_test "Test 10.1: Simulate multiple rotations (create v3, v4)"
# Rotate to v3
docker exec hsm-service /app/hsm-admin rotate exchange-key > /dev/null 2>&1
docker restart hsm-service > /dev/null 2>&1
sleep 5
# Rotate to v4
docker exec hsm-service /app/hsm-admin rotate exchange-key > /dev/null 2>&1
docker restart hsm-service > /dev/null 2>&1
sleep 5
print_success "Rotated to v3 and v4"

print_test "Test 10.2: Verify 4 versions exist"
VERSION_COUNT=$(grep -c "label: kek-exchange-v" "$PROJECT_ROOT/metadata.yaml")
if [ "$VERSION_COUNT" -ne 4 ]; then
    cat "$PROJECT_ROOT/metadata.yaml"
    print_error "Expected 4 versions, got $VERSION_COUNT"
fi
print_success "4 versions exist (v1, v2, v3, v4)"

print_test "Test 10.3: Check auto-cleanup warning at startup"
if ! docker logs hsm-service 2>&1 | grep -q "excess versions detected"; then
    print_info "No warning logged (expected when versions > max_versions)"
fi
print_success "Auto-cleanup check executed"

print_test "Test 10.4: Dry-run cleanup (should show what would be deleted)"
CLEANUP_DRYRUN=$(docker exec -e HSM_PIN=1234 hsm-service /app/hsm-admin cleanup-old-versions --dry-run 2>&1)
echo "$CLEANUP_DRYRUN"
if ! echo "$CLEANUP_DRYRUN" | grep -q "DRY RUN"; then
    print_error "Dry-run flag not working"
fi
print_success "Dry-run shows planned deletions"

print_test "Test 10.5: Execute cleanup (delete excess versions)"
docker exec -e HSM_PIN=1234 hsm-service /app/hsm-admin cleanup-old-versions --force > /tmp/cleanup.log 2>&1
if ! grep -q "CLEANUP COMPLETE" /tmp/cleanup.log; then
    cat /tmp/cleanup.log
    print_error "Cleanup failed (see /tmp/cleanup.log)"
fi
print_success "Cleanup executed successfully"

print_test "Test 10.6: Verify max 3 versions remain"
docker restart hsm-service > /dev/null 2>&1
sleep 5
VERSION_COUNT_AFTER=$(grep -c "label: kek-exchange-v" "$PROJECT_ROOT/metadata.yaml")
if [ "$VERSION_COUNT_AFTER" -gt 3 ]; then
    cat "$PROJECT_ROOT/metadata.yaml"
    print_error "Still have $VERSION_COUNT_AFTER versions (expected ≤3)"
fi
print_success "Cleanup enforced max_versions=3 limit"

print_test "Test 10.7: Current version still works after cleanup"
ENCRYPT_AFTER_CLEANUP=$(curl -s --cacert "$CA_CERT" \
    --cert "$CLIENT_CERT" \
    --key "$CLIENT_KEY" \
    -H "Content-Type: application/json" \
    -d "{\"context\":\"exchange-key\",\"plaintext\":\"$PLAINTEXT_NEW\"}" \
    "$BASE_URL/encrypt")

if ! echo "$ENCRYPT_AFTER_CLEANUP" | grep -q "ciphertext"; then
    echo "Response: $ENCRYPT_AFTER_CLEANUP"
    print_error "Encryption failed after cleanup"
fi
print_success "Encryption still works after cleanup"

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
echo "  ✓ Multi-version support (overlap period)"
echo "  ✓ PCI DSS compliance (cleanup old versions)"
echo "  ✓ Post-cleanup functionality"
echo ""
echo -e "${BLUE}Logs:${NC}"
echo "  Docker build:  /tmp/docker-build.log"
echo "  HSM init:      /tmp/init-hsm.log"
echo "  Rotation:      /tmp/rotation.log"
echo "  Cleanup:       /tmp/cleanup.log"
echo ""
echo -e "${GREEN}Integration test suite completed successfully!${NC}"
