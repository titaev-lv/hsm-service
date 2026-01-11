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

# Configuration
BASE_URL="https://localhost:8443"
CA_CERT="pki/ca/ca.crt"
CLIENT_CERT="pki/client/hsm-trading-client-1.crt"
CLIENT_KEY="pki/client/hsm-trading-client-1.key"

print_test "Scenario: Complete Key Rotation Workflow"
echo "=========================================="

# Step 1: Encrypt data with v1
print_test "Step 1: Encrypt data with current key (v1)"
PLAINTEXT="SGVsbG8gV29ybGQh"
RESPONSE=$(curl -s --cacert "$CA_CERT" --cert "$CLIENT_CERT" --key "$CLIENT_KEY" \
    -H "Content-Type: application/json" \
    -d "{\"context\":\"exchange-key\",\"plaintext\":\"$PLAINTEXT\"}" \
    "$BASE_URL/encrypt")

CIPHERTEXT=$(echo "$RESPONSE" | grep -o '"ciphertext":"[^"]*"' | cut -d'"' -f4)
KEY_ID_V1=$(echo "$RESPONSE" | grep -o '"key_id":"[^"]*"' | cut -d'"' -f4)

if [ -z "$CIPHERTEXT" ] || [ "$KEY_ID_V1" != "kek-exchange-v1" ]; then
    echo "Response: $RESPONSE"
    print_error "Failed to encrypt with v1"
fi
print_success "Encrypted with $KEY_ID_V1"

# Step 2: Perform rotation
print_test "Step 2: Rotate key to v2"
docker exec hsm-service /app/hsm-admin rotate exchange-key > /dev/null 2>&1
docker cp hsm-service:/app/metadata.yaml metadata.yaml
print_success "Rotation completed"

# Step 3: Restart service to load new key
print_test "Step 3: Restart service to load new key"
docker restart hsm-service > /dev/null 2>&1
sleep 10
print_success "Service restarted"

# Step 4: Verify old data can still be decrypted
print_test "Step 4: Decrypt old data with v1 key"
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

# Step 5: Encrypt new data with v2
print_test "Step 5: Encrypt new data (should use v2)"
NEW_PLAINTEXT="TmV3IERhdGEh"
NEW_RESPONSE=$(curl -s --cacert "$CA_CERT" --cert "$CLIENT_CERT" --key "$CLIENT_KEY" \
    -H "Content-Type: application/json" \
    -d "{\"context\":\"exchange-key\",\"plaintext\":\"$NEW_PLAINTEXT\"}" \
    "$BASE_URL/encrypt")

KEY_ID_V2=$(echo "$NEW_RESPONSE" | grep -o '"key_id":"[^"]*"' | cut -d'"' -f4)
if [ "$KEY_ID_V2" != "kek-exchange-v2" ]; then
    echo "Expected: kek-exchange-v2, Got: $KEY_ID_V2"
    print_error "New encryption not using v2"
fi
print_success "New data encrypted with $KEY_ID_V2"

# Step 6: Verify both versions are available
print_test "Step 6: Verify both key versions are loaded"
KEK_COUNT=$(docker exec hsm-service /app/hsm-admin list-kek 2>/dev/null | grep "kek-exchange-v" | wc -l)
if [ "$KEK_COUNT" -lt 2 ]; then
    print_error "Expected at least 2 versions, got $KEK_COUNT"
fi
print_success "Both v1 and v2 keys are available (overlap period)"

echo ""
echo "=========================================="
print_success "✓ Key Rotation E2E Test PASSED"
echo "=========================================="
