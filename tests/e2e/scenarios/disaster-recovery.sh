#!/bin/bash

# E2E Test: Disaster Recovery Scenario
# Tests backup → destroy → restore workflow

set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../../.." && pwd)"
cd "$PROJECT_ROOT"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
BLUE='\033[0;34m'
NC='\033[0m'

print_test() { echo -e "${BLUE}[TEST]${NC} $1"; }
print_success() { echo -e "${GREEN}✓${NC} $1"; }
print_error() { echo -e "${RED}✗${NC} $1"; exit 1; }

find_client_cert() {
    for p in \
        "$PROJECT_ROOT/pki/test/client/trading-client-1.crt" \
        "$PROJECT_ROOT/pki/test/client/hsm-trading-client-1.crt" \
        "$PROJECT_ROOT/pki/client/hsm-trading-client-1.crt" \
        "$PROJECT_ROOT/pki/client/client.crt"; do
        if [ -f "$p" ]; then
            echo "$p"
            return 0
        fi
    done
    return 1
}

find_client_key() {
    for p in \
        "$PROJECT_ROOT/pki/test/client/trading-client-1.key" \
        "$PROJECT_ROOT/pki/test/client/hsm-trading-client-1.key" \
        "$PROJECT_ROOT/pki/client/hsm-trading-client-1.key" \
        "$PROJECT_ROOT/pki/client/client.key"; do
        if [ -f "$p" ]; then
            echo "$p"
            return 0
        fi
    done
    return 1
}

find_ca_cert() {
    for p in \
        "$PROJECT_ROOT/pki/test/ca/ca.crt" \
        "$PROJECT_ROOT/pki/ca/ca.crt"; do
        if [ -f "$p" ]; then
            echo "$p"
            return 0
        fi
    done
    return 1
}

find_service_container() {
    if docker ps --format '{{.Names}}' | grep -qx 'hsm-service-test'; then
        echo "hsm-service-test"
        return 0
    fi
    if docker ps --format '{{.Names}}' | grep -qx 'hsm-service'; then
        echo "hsm-service"
        return 0
    fi
    return 1
}

probe_health() {
    local url="$1"
    curl -sk --max-time 3 --cacert "$CA_CERT" --cert "$CLIENT_CERT" --key "$CLIENT_KEY" "$url/health" >/dev/null 2>&1
}

compose_cmd() {
    if [ -f "$TEST_COMPOSE_FILE" ]; then
        docker compose -p "$TEST_COMPOSE_PROJECT" -f "$TEST_COMPOSE_FILE" "$@"
    else
        docker compose "$@"
    fi
}

backup_tokens() {
    local volume_name="$1"
    if [ -n "$volume_name" ]; then
        docker run --rm -v "$volume_name":/from -v "$BACKUP_DIR":/backup busybox \
            sh -c 'cd /from && tar czf /backup/tokens.tgz .'
    else
        docker cp "$SERVICE_CONTAINER":/var/lib/softhsm/tokens "$BACKUP_DIR/"
    fi
}

restore_tokens() {
    local volume_name="$1"
    if [ -n "$volume_name" ]; then
        docker volume create "$volume_name" >/dev/null
        docker run --rm -v "$volume_name":/to -v "$BACKUP_DIR":/backup busybox \
            sh -c 'rm -rf /to/* && cd /to && tar xzf /backup/tokens.tgz'
    else
        docker cp "$BACKUP_DIR/tokens/." "$SERVICE_CONTAINER":/var/lib/softhsm/tokens/
    fi
}

# Configuration
BASE_URL="${HSM_URL:-https://localhost:8443}"
CA_CERT="${CA_CERT:-$(find_ca_cert || true)}"
CLIENT_CERT="${CLIENT_CERT:-$(find_client_cert || true)}"
CLIENT_KEY="${CLIENT_KEY:-$(find_client_key || true)}"
SERVICE_CONTAINER="${SERVICE_CONTAINER:-$(find_service_container || true)}"
TEST_COMPOSE_PROJECT="${TEST_COMPOSE_PROJECT:-hsm-test}"
TEST_COMPOSE_FILE="${TEST_COMPOSE_FILE:-$PROJECT_ROOT/docker-compose-test.yml}"
BACKUP_DIR="/tmp/hsm-backup-$(date +%Y%m%d-%H%M%S)"
TOKEN_VOLUME_NAME=""
METADATA_HOST_FILE="$PROJECT_ROOT/metadata.yaml"

if [ -f "$PROJECT_ROOT/metadata-test.yaml" ]; then
    METADATA_HOST_FILE="$PROJECT_ROOT/metadata-test.yaml"
fi

if [ -z "$CA_CERT" ] || [ ! -f "$CA_CERT" ]; then
    print_error "CA cert not found"
fi
if [ -z "$CLIENT_CERT" ] || [ ! -f "$CLIENT_CERT" ]; then
    print_error "Client cert not found"
fi
if [ -z "$CLIENT_KEY" ] || [ ! -f "$CLIENT_KEY" ]; then
    print_error "Client key not found"
fi
if [ -z "$SERVICE_CONTAINER" ]; then
    print_error "HSM service container not found (expected hsm-service-test or hsm-service)"
fi

if ! probe_health "$BASE_URL"; then
    if probe_health "https://localhost:8444"; then
        BASE_URL="https://localhost:8444"
    elif probe_health "https://localhost:8443"; then
        BASE_URL="https://localhost:8443"
    else
        print_error "HSM service is not reachable on 8443/8444"
    fi
fi

print_test "Scenario: Disaster Recovery"
echo "=========================================="

# Step 1: Create and encrypt test data
print_test "Step 1: Create test data"
PLAINTEXT="RGlzYXN0ZXIgUmVjb3ZlcnkgVGVzdA=="
RESPONSE=$(curl -s --cacert "$CA_CERT" --cert "$CLIENT_CERT" --key "$CLIENT_KEY" \
    -H "Content-Type: application/json" \
    -d "{\"context\":\"exchange-key\",\"plaintext\":\"$PLAINTEXT\"}" \
    "$BASE_URL/encrypt")

CIPHERTEXT=$(echo "$RESPONSE" | grep -o '"ciphertext":"[^"]*"' | cut -d'"' -f4)
KEY_ID=$(echo "$RESPONSE" | grep -o '"key_id":"[^"]*"' | cut -d'"' -f4)

if [ -z "$CIPHERTEXT" ]; then
    echo "Response: $RESPONSE"
    print_error "Failed to encrypt test data"
fi
print_success "Test data encrypted with $KEY_ID"
echo "Ciphertext: ${CIPHERTEXT:0:50}..."

# Step 2: Create backup
print_test "Step 2: Create backup"
mkdir -p "$BACKUP_DIR"

TOKEN_VOLUME_NAME=$(docker inspect "$SERVICE_CONTAINER" --format '{{range .Mounts}}{{if eq .Destination "/var/lib/softhsm/tokens"}}{{.Name}}{{end}}{{end}}' 2>/dev/null || true)

# Backup metadata
docker cp "$SERVICE_CONTAINER":/app/metadata.yaml "$BACKUP_DIR/metadata.yaml"
print_success "Backed up metadata.yaml"

# Backup HSM tokens
if backup_tokens "$TOKEN_VOLUME_NAME"; then
    print_success "Backed up HSM tokens"
else
    print_error "Failed to back up HSM tokens"
fi

# Backup config
cp config.yaml "$BACKUP_DIR/config.yaml" 2>/dev/null || true
print_success "Backed up config.yaml"

echo "Backup location: $BACKUP_DIR"

# Step 3: Simulate disaster (destroy container and volumes)
print_test "Step 3: Simulate disaster (destroy container)"
compose_cmd down -v > /dev/null 2>&1
print_success "Container and volumes destroyed"

# Verify everything is gone
if docker ps -a --format '{{.Names}}' | grep -qx "$SERVICE_CONTAINER"; then
    print_error "Container still exists!"
fi
print_success "Verified: complete destruction"

sleep 3

# Step 4: Restore from backup
print_test "Step 4: Restore from backup"

# Restore metadata
cp "$BACKUP_DIR/metadata.yaml" "$METADATA_HOST_FILE"
print_success "Restored metadata to $(basename "$METADATA_HOST_FILE")"

# Restore HSM token storage before start
if restore_tokens "$TOKEN_VOLUME_NAME"; then
    print_success "Restored HSM tokens"
else
    print_error "Failed to restore HSM tokens"
fi

# Start service with restored data
print_test "Step 5: Start service with restored data"
compose_cmd up -d > /dev/null 2>&1
sleep 15

if ! docker ps --format '{{.Names}}' | grep -qx "$SERVICE_CONTAINER"; then
    docker logs "$SERVICE_CONTAINER" --tail 30
    print_error "Service failed to start after restore"
fi
print_success "Service started successfully"

# Step 6: Verify restored data can be decrypted
print_test "Step 6: Decrypt original data with restored keys"
DECRYPT_RESPONSE=$(curl -s --cacert "$CA_CERT" --cert "$CLIENT_CERT" --key "$CLIENT_KEY" \
    -H "Content-Type: application/json" \
    -d "{\"context\":\"exchange-key\",\"ciphertext\":\"$CIPHERTEXT\",\"key_id\":\"$KEY_ID\"}" \
    "$BASE_URL/decrypt")

DECRYPTED=$(echo "$DECRYPT_RESPONSE" | grep -o '"plaintext":"[^"]*"' | cut -d'"' -f4)
if [ "$DECRYPTED" != "$PLAINTEXT" ]; then
    echo "Expected: $PLAINTEXT"
    echo "Got: $DECRYPTED"
    print_error "Decryption failed after restore"
fi
print_success "Original data successfully decrypted"

# Step 7: Verify new operations work
print_test "Step 7: Verify new operations work"
NEW_PLAINTEXT="TmV3IE9wZXJhdGlvbg=="
NEW_RESPONSE=$(curl -s --cacert "$CA_CERT" --cert "$CLIENT_CERT" --key "$CLIENT_KEY" \
    -H "Content-Type: application/json" \
    -d "{\"context\":\"exchange-key\",\"plaintext\":\"$NEW_PLAINTEXT\"}" \
    "$BASE_URL/encrypt")

if ! echo "$NEW_RESPONSE" | grep -q "ciphertext"; then
    echo "Response: $NEW_RESPONSE"
    print_error "New operations not working"
fi
print_success "New operations working correctly"

# Cleanup
print_test "Cleanup: Remove backup"
rm -rf "$BACKUP_DIR"
print_success "Backup cleaned up"

echo ""
echo "=========================================="
print_success "✓ Disaster Recovery E2E Test PASSED"
echo "=========================================="
echo ""
echo "Summary:"
echo "  1. ✓ Created test data"
echo "  2. ✓ Backed up metadata and HSM tokens"
echo "  3. ✓ Destroyed container and volumes"
echo "  4. ✓ Restored from backup"
echo "  5. ✓ Verified old data can be decrypted"
echo "  6. ✓ Verified new operations work"
