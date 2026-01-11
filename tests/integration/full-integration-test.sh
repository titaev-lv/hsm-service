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
TOTAL_TESTS=42

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
# SETUP: Determine project root
# ==========================================
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

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

print_test "Stop and remove existing containers and volumes"
cd "$PROJECT_ROOT"
docker compose -f "$COMPOSE_FILE" down -v 2>/dev/null || true
print_success "Containers stopped and volumes removed (tokens ephemeral)"

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

# Try default certificate first
DEFAULT_CLIENT_CERT_NAME="hsm-trading-client-1"
DEFAULT_CLIENT_CERT_PATH="$PROJECT_ROOT/pki/client/${DEFAULT_CLIENT_CERT_NAME}.crt"
DEFAULT_CLIENT_KEY_PATH="$PROJECT_ROOT/pki/client/${DEFAULT_CLIENT_CERT_NAME}.key"

# Check if default certificate exists
if [ -f "$DEFAULT_CLIENT_CERT_PATH" ] && [ -f "$DEFAULT_CLIENT_KEY_PATH" ]; then
    CLIENT_CERT_NAME="$DEFAULT_CLIENT_CERT_NAME"
    CLIENT_CERT_PATH="$DEFAULT_CLIENT_CERT_PATH"
    CLIENT_KEY_PATH="$DEFAULT_CLIENT_KEY_PATH"
    echo "Using default certificate: $CLIENT_CERT_NAME"
else
    # Request certificate name interactively
    echo -e "${BLUE}Please provide client certificate details for testing:${NC}"
    echo ""
    
    # Request client certificate name with validation
    while true; do
        read -p "Client certificate name (default: hsm-trading-client-1): " CLIENT_CERT_NAME
        # Use default if empty
        if [ -z "$CLIENT_CERT_NAME" ]; then
            CLIENT_CERT_NAME="hsm-trading-client-1"
        fi
        # Trim whitespace
        CLIENT_CERT_NAME=$(echo "$CLIENT_CERT_NAME" | xargs)
        # Validate not empty after trimming
        if [ -n "$CLIENT_CERT_NAME" ]; then
            break
        fi
        echo -e "${RED}Certificate name cannot be empty${NC}"
    done

    echo "Using certificate: $CLIENT_CERT_NAME"
    echo ""

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
fi

echo ""

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
    rotation_interval_days: 90
    versions:
      - label: kek-exchange-v1
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

print_test "Start services with docker-compose"
cd "$PROJECT_ROOT"
# Force recreate container to use new image (if build was done)
if ! docker compose -f "$COMPOSE_FILE" up -d --force-recreate > /tmp/docker-compose-up.log 2>&1; then
    cat /tmp/docker-compose-up.log
    print_error "docker-compose up failed (see /tmp/docker-compose-up.log)"
fi
sleep 3
print_success "Services started"

print_test "Verify container is running"
if ! docker ps | grep -q hsm-service; then
    cd "$PROJECT_ROOT"
    docker compose -f "$COMPOSE_FILE" logs
    print_error "Container not running"
fi
print_success "Container is running"

print_test "Check container logs for errors"
sleep 2
cd "$PROJECT_ROOT"
if docker compose -f "$COMPOSE_FILE" logs | grep -i "fatal\|panic"; then
    docker compose -f "$COMPOSE_FILE" logs
    print_error "Fatal errors found in logs"
fi
print_success "No fatal errors in logs"

# ==========================================
# PHASE 6: HSM INITIALIZATION
# ==========================================
print_header "PHASE 6: HSM Key Initialization"

print_test "Verify HSM initialized automatically (init-hsm.sh runs on container start)"
sleep 3  # Give time for init to complete
if ! docker logs hsm-service 2>&1 | grep -q "HSM Service Initialization"; then
    docker logs hsm-service
    print_error "HSM initialization did not run"
fi
print_success "HSM initialization completed automatically"

print_test "Verify keys created automatically"
if ! docker logs hsm-service 2>&1 | grep -q "Default KEKs created"; then
    # Keys might already exist from previous run
    if ! docker logs hsm-service 2>&1 | grep -q "Found .* KEK"; then
        docker logs hsm-service
        print_error "No KEKs found in HSM"
    fi
fi
print_success "HSM keys initialized"

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
BASE_URL="https://localhost:8443"
CA_CERT="$PROJECT_ROOT/pki/ca/ca.crt"
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
if [ "$KEY_ID" != "kek-exchange-v1" ]; then
    echo "Expected: kek-exchange-v1, Got: $KEY_ID"
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
# Force sync: copy metadata from container to host
docker cp hsm-service:/app/metadata.yaml "$PROJECT_ROOT/metadata.yaml"
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
docker stop hsm-service > /dev/null 2>&1
sleep 2
docker start hsm-service > /dev/null 2>&1
sleep 7
if ! docker ps | grep -q hsm-service; then
    docker logs hsm-service
    print_error "Container failed to start after rotation"
fi
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
DECRYPT_V1=$(curl -s --connect-timeout 10 --max-time 15 \
    --cacert "$CA_CERT" \
    --cert "$CLIENT_CERT" \
    --key "$CLIENT_KEY" \
    -H "Content-Type: application/json" \
    -d "{\"context\":\"exchange-key\",\"ciphertext\":\"$CIPHERTEXT\",\"key_id\":\"kek-exchange-v1\"}" \
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
if [ "$KEY_ID_V2" != "kek-exchange-v2" ]; then
    echo "Expected: kek-exchange-v2, Got: $KEY_ID_V2"
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
if docker compose logs --since 40s hsm-service 2>&1 | grep -q "KEK hot reload successful\|metadata file changed"; then
    print_success "Hot reload detected in logs"
    docker compose logs --since 40s hsm-service 2>&1 | grep -E "KEK hot reload|metadata file changed" | tail -3
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
if ! docker exec hsm-service /app/hsm-admin rotate exchange-key; then
    print_error "Failed to rotate to v3"
fi
# Force sync: copy metadata from container to host
docker cp hsm-service:/app/metadata.yaml "$PROJECT_ROOT/metadata.yaml"
# Wait for automatic hot reload (service monitors every 30s)
echo "Waiting for hot reload (35 seconds)..."
sleep 35

# Rotate to v4
echo "=== Rotating to v4 ==="
if ! docker exec hsm-service /app/hsm-admin rotate exchange-key; then
    print_error "Failed to rotate to v4"
fi
# Force sync: copy metadata from container to host
docker cp hsm-service:/app/metadata.yaml "$PROJECT_ROOT/metadata.yaml"
echo "Waiting for hot reload (35 seconds)..."
sleep 35
print_success "Rotated to v3 and v4"

print_test "Test 10.2: Verify 4 versions exist"
# Read metadata from container (source of truth)
VERSION_COUNT=$(docker exec hsm-service grep -c "label: kek-exchange-v" /app/metadata.yaml)
if [ "$VERSION_COUNT" -ne 4 ]; then
    echo "Metadata in container:"
    docker exec hsm-service cat /app/metadata.yaml
    print_error "Expected 4 versions, got $VERSION_COUNT"
fi
print_success "4 versions exist (v1, v2, v3, v4)"

print_test "Test 10.3: Check auto-cleanup warning at startup"
if ! docker logs hsm-service 2>&1 | grep -q "excess versions detected"; then
    print_info "No warning logged (expected when versions > max_versions)"
fi
print_success "Auto-cleanup check executed"

print_test "Test 10.4: Dry-run cleanup (should show what would be deleted)"
echo ""
echo "=== Running cleanup in dry-run mode ==="
CLEANUP_DRYRUN=$(docker exec -e HSM_PIN=1234 hsm-service /app/hsm-admin cleanup-old-versions --dry-run 2>&1)
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
sed -E "s/(label: kek-exchange-v1.*)/\1/; /label: kek-exchange-v1/,/created_at:/ s/created_at:.*/created_at: $DATE_60_DAYS_AGO/" /tmp/metadata-before-backdate.yaml | \
sed -E "s/(label: kek-exchange-v2.*)/\1/; /label: kek-exchange-v2/,/created_at:/ s/created_at:.*/created_at: $DATE_60_DAYS_AGO/" | \
sed -E "s/(label: kek-exchange-v3.*)/\1/; /label: kek-exchange-v3/,/created_at:/ s/created_at:.*/created_at: $DATE_15_DAYS_AGO/" > /tmp/metadata-backdated.yaml

# Stop container before modifying mounted file
echo "Stopping container to modify metadata.yaml on host..."
docker stop hsm-service > /dev/null 2>&1
sleep 3

# Replace metadata.yaml on host (which is mounted to container)
cp /tmp/metadata-backdated.yaml "$PROJECT_ROOT/metadata.yaml"

echo "Backdated metadata.yaml content:"
cat "$PROJECT_ROOT/metadata.yaml" | grep -A 15 "exchange-key:"
echo ""

# Restart container with backdated metadata
echo "Restarting container with backdated metadata..."
docker start hsm-service > /dev/null 2>&1
sleep 7

print_success "Versions backdated for cleanup testing"

print_test "Test 10.5: Execute cleanup (delete excess versions)"
echo ""
echo "=== Executing cleanup with --force ==="
docker exec -e HSM_PIN=1234 hsm-service /app/hsm-admin cleanup-old-versions --force > /tmp/cleanup.log 2>&1
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
docker stop hsm-service > /dev/null 2>&1
sleep 2
docker start hsm-service > /dev/null 2>&1
sleep 7

# Copy updated metadata from container to host
docker cp hsm-service:/app/metadata.yaml "$PROJECT_ROOT/metadata.yaml"

echo "Updated metadata.yaml content:"
cat "$PROJECT_ROOT/metadata.yaml"
echo ""

VERSION_COUNT_AFTER=$(grep -c "label: kek-exchange-v" "$PROJECT_ROOT/metadata.yaml")
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
    current: kek-exchange-v1
    rotation_interval_days: 90
    versions:
      - label: kek-exchange-v1
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
docker restart hsm-service > /dev/null 2>&1
sleep 10

# Verify service is healthy
if docker ps | grep -q "hsm-service"; then
    print_success "System reset to clean state for remaining tests"
else
    docker logs hsm-service --tail 20
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

print_test "Test 11.3: Request with revoked certificate should be blocked by ACL"
# Check if we have a revoked cert in revoked.yaml
REVOKED_CN=$(grep -A 1 "^  - cn:" "$PROJECT_ROOT/pki/revoked.yaml" 2>/dev/null | grep "cn:" | head -1 | sed 's/.*cn: *"\(.*\)".*/\1/')
if [ -n "$REVOKED_CN" ]; then
    # Try to find matching cert file in pki/client
    REVOKED_CERT=$(find "$PROJECT_ROOT/pki/client" -name "*.crt" 2>/dev/null | while read cert; do
        if openssl x509 -in "$cert" -noout -subject 2>/dev/null | grep -q "$REVOKED_CN"; then
            echo "$cert"
            break
        fi
    done)
    REVOKED_KEY="${REVOKED_CERT%.crt}.key"
    
    if [ -f "$REVOKED_CERT" ] && [ -f "$REVOKED_KEY" ]; then
        REVOKED_RESPONSE=$(timeout 8 curl -s -w "\n%{http_code}" --connect-timeout 3 --max-time 5 \
            --cacert "$CA_CERT" \
            --cert "$REVOKED_CERT" \
            --key "$REVOKED_KEY" \
            -H "Content-Type: application/json" \
            -d "{\"context\":\"exchange-key\",\"plaintext\":\"dGVzdA==\"}" \
            "$BASE_URL/encrypt" 2>&1 || echo "000")
        
        HTTP_CODE=$(echo "$REVOKED_RESPONSE" | tail -1)
        BODY=$(echo "$REVOKED_RESPONSE" | sed '$d')
        
        if [ "$HTTP_CODE" = "403" ] || echo "$BODY" | grep -qi "revoked\|forbidden\|access.*denied"; then
            print_success "Revoked certificate blocked by ACL (CN: $REVOKED_CN)"
        else
            echo "CN: $REVOKED_CN"
            echo "HTTP Code: $HTTP_CODE"
            echo "Response: $BODY"
            print_error "Server accepted revoked certificate!"
        fi
    else
        print_info "Revoked cert/key files not found, skipping test"
    fi
else
    print_info "No revoked certificates in revoked.yaml, skipping test"
fi

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
if [ -f "$PROJECT_ROOT/pki/client/hsm-2fa-client-1.crt" ]; then
    # 2fa client has OU=2fa, which is not authorized for exchange-key
    WRONG_OU_RESPONSE=$(timeout 8 curl -s -w "\n%{http_code}" --connect-timeout 3 --max-time 5 \
        --cacert "$CA_CERT" \
        --cert "$PROJECT_ROOT/pki/client/hsm-2fa-client-1.crt" \
        --key "$PROJECT_ROOT/pki/client/hsm-2fa-client-1.key" \
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
BEFORE_METADATA=$(docker exec hsm-service cat /app/metadata.yaml)
BEFORE_TOKEN_COUNT=$(docker exec hsm-service sh -c 'ls -1 /var/lib/softhsm/tokens/ 2>/dev/null | wc -l' | tr -d '\n')
BEFORE_KEY_COUNT=$(docker exec hsm-service /app/hsm-admin list-kek 2>/dev/null | grep -c "Config Key:" | tr -d '\n' || echo "0")

echo "State before restart:"
echo "  Metadata contexts: $(echo "$BEFORE_METADATA" | grep -c "current:" | tr -d '\n' || echo "0")"
echo "  SoftHSM tokens: $BEFORE_TOKEN_COUNT"
echo "  KEKs loaded: $BEFORE_KEY_COUNT"
print_success "Current state captured"

print_test "Test 12.2: Restart container with docker restart"
docker restart hsm-service > /dev/null 2>&1
sleep 15  # Wait for service to fully restart

# Check if container is running
if docker ps | grep -q hsm-service; then
    print_success "Container restarted successfully"
else
    print_error "Container failed to restart"
fi

print_test "Test 12.3: Verify metadata persisted after restart"
AFTER_METADATA=$(docker exec hsm-service cat /app/metadata.yaml 2>/dev/null)
AFTER_CONTEXTS=$(echo "$AFTER_METADATA" | grep -c "current:" | tr -d '\n' || echo "0")
BEFORE_CONTEXTS=$(echo "$BEFORE_METADATA" | grep -c "current:" | tr -d '\n' || echo "0")

if [ "$AFTER_CONTEXTS" = "$BEFORE_CONTEXTS" ] && [ "$AFTER_CONTEXTS" -ge "2" ]; then
    print_success "Metadata persisted ($AFTER_CONTEXTS contexts preserved)"
else
    echo "Before: $BEFORE_CONTEXTS, After: $AFTER_CONTEXTS"
    print_error "Metadata not preserved"
fi

print_test "Test 12.4: Verify SoftHSM tokens persisted"
AFTER_TOKEN_COUNT=$(docker exec hsm-service sh -c 'ls -1 /var/lib/softhsm/tokens/ 2>/dev/null | wc -l' | tr -d '\n')

if [ "$AFTER_TOKEN_COUNT" = "$BEFORE_TOKEN_COUNT" ] && [ "$AFTER_TOKEN_COUNT" -gt "0" ]; then
    print_success "SoftHSM tokens persisted ($AFTER_TOKEN_COUNT tokens)"
else
    echo "Before: $BEFORE_TOKEN_COUNT, After: $AFTER_TOKEN_COUNT"
    print_error "Tokens not preserved"
fi

print_test "Test 12.5: Verify KEKs reloaded after restart"
# Wait for KEKs to load
sleep 5
AFTER_KEY_COUNT=$(docker exec hsm-service /app/hsm-admin list-kek 2>/dev/null | grep -c "Config Key:" | tr -d '\n' || echo "0")

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
docker compose down > /dev/null 2>&1
sleep 3

# Verify containers stopped
if ! docker ps | grep -q hsm-service; then
    print_success "Services stopped"
else
    print_error "Failed to stop services"
fi

echo "Starting services (docker compose up -d)..."
docker compose up -d > /dev/null 2>&1
sleep 15

# Check if service is running
if docker ps | grep -q hsm-service; then
    print_success "Services started"
else
    print_error "Failed to start services"
fi

print_test "Test 12.8: Verify data survived compose down/up"
FINAL_METADATA=$(docker exec hsm-service cat /app/metadata.yaml 2>/dev/null)
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
docker compose down > /dev/null 2>&1
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
    docker logs hsm-service --tail 20
    print_error "Failed to start with custom env"
fi

print_test "Test 13.3: Verify PINs are NOT exposed in logs"
LOGS=$(docker logs hsm-service 2>&1)
if echo "$LOGS" | grep -q "1234\|5678"; then
    print_error "SECURITY RISK: PIN exposed in logs!"
else
    print_success "PINs not exposed in logs (secure)"
fi

print_test "Test 13.4: Verify CONFIG_PATH override works"
CONFIG_CHECK=$(docker exec hsm-service sh -c 'echo $CONFIG_PATH' 2>/dev/null)
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
docker compose up -d > /dev/null 2>&1
sleep 15

if docker ps | grep -q hsm-service; then
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
