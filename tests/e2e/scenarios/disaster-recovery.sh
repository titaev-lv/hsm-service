#!/bin/bash

# E2E Test: Disaster Recovery Scenario
# Tests backup → destroy → restore workflow

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
BACKUP_DIR="/tmp/hsm-backup-$(date +%Y%m%d-%H%M%S)"

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

# Backup metadata
docker cp hsm-service:/app/metadata.yaml "$BACKUP_DIR/metadata.yaml"
print_success "Backed up metadata.yaml"

# Backup HSM tokens
docker cp hsm-service:/var/lib/softhsm/tokens "$BACKUP_DIR/"
print_success "Backed up HSM tokens"

# Backup config
cp config.yaml "$BACKUP_DIR/config.yaml" 2>/dev/null || true
print_success "Backed up config.yaml"

echo "Backup location: $BACKUP_DIR"

# Step 3: Simulate disaster (destroy container and volumes)
print_test "Step 3: Simulate disaster (destroy container)"
docker compose down -v > /dev/null 2>&1
print_success "Container and volumes destroyed"

# Verify everything is gone
if docker ps -a | grep -q hsm-service; then
    print_error "Container still exists!"
fi
print_success "Verified: complete destruction"

sleep 3

# Step 4: Restore from backup
print_test "Step 4: Restore from backup"

# Create data directory
mkdir -p data/tokens

# Restore metadata
cp "$BACKUP_DIR/metadata.yaml" metadata.yaml
print_success "Restored metadata.yaml"

# Restore HSM tokens
cp -r "$BACKUP_DIR/tokens/"* data/tokens/
print_success "Restored HSM tokens"

# Start service with restored data
print_test "Step 5: Start service with restored data"
docker compose up -d > /dev/null 2>&1
sleep 15

if ! docker ps | grep -q hsm-service; then
    docker logs hsm-service --tail 30
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
