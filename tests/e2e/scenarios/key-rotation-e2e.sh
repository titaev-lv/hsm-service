#!/bin/bash

# E2E Test: Complete Key Rotation Workflow
# Tests the full lifecycle: encrypt with v1 → rotate → decrypt old data → encrypt new data

set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
BLUE='\033[0;34m'
NC='\033[0m'

print_test() { echo -e "${BLUE}[TEST]${NC} $1"; }
print_success() { echo -e "${GREEN}✓${NC} $1"; }
print_error() { echo -e "${RED}✗${NC} $1"; exit 1; }

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../../.." && pwd)"
cd "$PROJECT_ROOT"

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

# Configuration
BASE_URL="${HSM_URL:-https://localhost:8443}"
CA_CERT="${CA_CERT:-$(find_ca_cert || true)}"
CLIENT_CERT="${CLIENT_CERT:-$(find_client_cert || true)}"
CLIENT_KEY="${CLIENT_KEY:-$(find_client_key || true)}"
SERVICE_CONTAINER="${SERVICE_CONTAINER:-$(find_service_container || true)}"

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

print_test "Scenario: Complete Key Rotation Workflow"
echo "=========================================="

# Step 1: Encrypt data with current key
print_test "Step 1: Encrypt data with current key"
PLAINTEXT="SGVsbG8gV29ybGQh"
RESPONSE=$(curl -s --cacert "$CA_CERT" --cert "$CLIENT_CERT" --key "$CLIENT_KEY" \
    -H "Content-Type: application/json" \
    -d "{\"context\":\"exchange-key\",\"plaintext\":\"$PLAINTEXT\"}" \
    "$BASE_URL/encrypt")

CIPHERTEXT=$(echo "$RESPONSE" | grep -o '"ciphertext":"[^"]*"' | cut -d'"' -f4)
KEY_ID_V1=$(echo "$RESPONSE" | grep -o '"key_id":"[^"]*"' | cut -d'"' -f4)

if [ -z "$CIPHERTEXT" ] || [ -z "$KEY_ID_V1" ]; then
    echo "Response: $RESPONSE"
    print_error "Failed to encrypt with current key"
fi
print_success "Encrypted with $KEY_ID_V1"

# Step 2: Perform rotation
print_test "Step 2: Rotate key to v2"
docker exec "$SERVICE_CONTAINER" /app/hsm-admin rotate exchange-key > /dev/null 2>&1
docker exec "$SERVICE_CONTAINER" /app/hsm-admin update-checksums > /dev/null 2>&1
docker cp "$SERVICE_CONTAINER":/app/metadata.yaml metadata.yaml
print_success "Rotation completed"

# Step 3: Restart service to load new key
print_test "Step 3: Restart service to load new key"
docker restart "$SERVICE_CONTAINER" > /dev/null 2>&1
sleep 10
print_success "Service restarted"

# Step 4: Verify old data can still be decrypted
print_test "Step 4: Decrypt old data with previous key"
DECRYPT_RESPONSE=$(curl -s --cacert "$CA_CERT" --cert "$CLIENT_CERT" --key "$CLIENT_KEY" \
    -H "Content-Type: application/json" \
    -d "{\"context\":\"exchange-key\",\"ciphertext\":\"$CIPHERTEXT\",\"key_id\":\"$KEY_ID_V1\"}" \
    "$BASE_URL/decrypt")

DECRYPTED=$(echo "$DECRYPT_RESPONSE" | grep -o '"plaintext":"[^"]*"' | cut -d'"' -f4)
if [ "$DECRYPTED" != "$PLAINTEXT" ]; then
    echo "Expected: $PLAINTEXT, Got: $DECRYPTED"
    print_error "Failed to decrypt old data"
fi
print_success "Old data successfully decrypted"

# Step 5: Encrypt new data with rotated key
print_test "Step 5: Encrypt new data (should use rotated key)"
NEW_PLAINTEXT="TmV3IERhdGEh"
NEW_RESPONSE=$(curl -s --cacert "$CA_CERT" --cert "$CLIENT_CERT" --key "$CLIENT_KEY" \
    -H "Content-Type: application/json" \
    -d "{\"context\":\"exchange-key\",\"plaintext\":\"$NEW_PLAINTEXT\"}" \
    "$BASE_URL/encrypt")

KEY_ID_V2=$(echo "$NEW_RESPONSE" | grep -o '"key_id":"[^"]*"' | cut -d'"' -f4)
if [ -z "$KEY_ID_V2" ]; then
    echo "Response: $NEW_RESPONSE"
    print_error "Failed to encrypt with rotated key"
fi
if [ "$KEY_ID_V2" = "$KEY_ID_V1" ]; then
    echo "Previous key: $KEY_ID_V1, New key: $KEY_ID_V2"
    print_error "Rotation did not switch active key"
fi
print_success "New data encrypted with $KEY_ID_V2"

# Step 6: Verify both versions are available
print_test "Step 6: Verify both key versions are loaded"
KEK_COUNT=$(docker exec "$SERVICE_CONTAINER" /app/hsm-admin list-kek 2>/dev/null | grep "kek-exchange-key-v" | wc -l)
if [ "$KEK_COUNT" -lt 2 ]; then
    print_error "Expected at least 2 versions, got $KEK_COUNT"
fi
print_success "Both v1 and v2 keys are available (overlap period)"

echo ""
echo "=========================================="
print_success "✓ Key Rotation E2E Test PASSED"
echo "=========================================="
