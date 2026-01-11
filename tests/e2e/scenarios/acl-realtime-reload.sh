#!/bin/bash

# E2E Test: ACL Real-time Reload
# Tests dynamic revocation and restoration of client certificates

set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
NC='\033[0m'

print_test() { echo -e "${BLUE}[TEST]${NC} $1"; }
print_success() { echo -e "${GREEN}✓${NC} $1"; }
print_warning() { echo -e "${YELLOW}⚠${NC} $1"; }
print_error() { echo -e "${RED}✗${NC} $1"; exit 1; }

# Configuration
BASE_URL="https://localhost:8443"
CA_CERT="pki/ca/ca.crt"
CLIENT_CERT="pki/client/hsm-trading-client-1.crt"
CLIENT_KEY="pki/client/hsm-trading-client-1.key"
REVOKED_FILE="pki/revoked.yaml"

# Extract client CN from certificate
CLIENT_CN=$(openssl x509 -in "$CLIENT_CERT" -noout -subject | sed 's/.*CN=\([^,]*\).*/\1/')

print_test "Scenario: ACL Real-time Reload"
echo "=========================================="
echo "Client CN: $CLIENT_CN"
echo ""

# Backup original revoked.yaml
cp "$REVOKED_FILE" "${REVOKED_FILE}.backup"

# Step 1: Verify client can connect initially
print_test "Step 1: Verify client can connect (baseline)"
RESPONSE=$(curl -s -w "\n%{http_code}" --cacert "$CA_CERT" --cert "$CLIENT_CERT" --key "$CLIENT_KEY" \
    "$BASE_URL/health" 2>&1)

HTTP_CODE=$(echo "$RESPONSE" | tail -1)
if [ "$HTTP_CODE" != "200" ]; then
    echo "HTTP Code: $HTTP_CODE"
    print_error "Client cannot connect initially"
fi
print_success "Client successfully connected (baseline)"

# Step 2: Add client to revoked list
print_test "Step 2: Add client to revocation list"
cat >> "$REVOKED_FILE" << EOF

revocations:
  - cn: "$CLIENT_CN"
    reason: "E2E test revocation"
    revoked_at: "$(date -u +%Y-%m-%dT%H:%M:%SZ)"
EOF
print_success "Added $CLIENT_CN to revoked.yaml"

# Step 3: Wait for auto-reload (30 seconds + buffer)
print_test "Step 3: Wait for ACL auto-reload (35 seconds)"
for i in {35..1}; do
    echo -ne "\rWaiting... $i seconds   "
    sleep 1
done
echo ""
print_success "Auto-reload period elapsed"

# Step 4: Verify client is now blocked
print_test "Step 4: Verify client is blocked"
BLOCKED_RESPONSE=$(curl -s -w "\n%{http_code}" --cacert "$CA_CERT" --cert "$CLIENT_CERT" --key "$CLIENT_KEY" \
    "$BASE_URL/health" 2>&1)

BLOCKED_CODE=$(echo "$BLOCKED_RESPONSE" | tail -1)
BLOCKED_BODY=$(echo "$BLOCKED_RESPONSE" | head -n -1)

if [ "$BLOCKED_CODE" = "403" ] || echo "$BLOCKED_BODY" | grep -qi "revoked\|forbidden"; then
    print_success "Client correctly blocked (HTTP $BLOCKED_CODE)"
else
    echo "HTTP Code: $BLOCKED_CODE"
    echo "Response: $BLOCKED_BODY"
    # Restore original file before failing
    mv "${REVOKED_FILE}.backup" "$REVOKED_FILE"
    print_error "Client was NOT blocked (ACL reload failed)"
fi

# Step 5: Remove from revoked list
print_test "Step 5: Restore client (remove from revocation list)"
mv "${REVOKED_FILE}.backup" "$REVOKED_FILE"
print_success "Removed $CLIENT_CN from revoked.yaml"

# Step 6: Wait for auto-reload again
print_test "Step 6: Wait for ACL auto-reload (35 seconds)"
for i in {35..1}; do
    echo -ne "\rWaiting... $i seconds   "
    sleep 1
done
echo ""
print_success "Auto-reload period elapsed"

# Step 7: Verify client can connect again
print_test "Step 7: Verify client can connect again"
RESTORED_RESPONSE=$(curl -s -w "\n%{http_code}" --cacert "$CA_CERT" --cert "$CLIENT_CERT" --key "$CLIENT_KEY" \
    "$BASE_URL/health" 2>&1)

RESTORED_CODE=$(echo "$RESTORED_RESPONSE" | tail -1)
if [ "$RESTORED_CODE" != "200" ]; then
    echo "HTTP Code: $RESTORED_CODE"
    print_error "Client still blocked after restoration"
fi
print_success "Client successfully restored (HTTP $RESTORED_CODE)"

# Step 8: Test encryption to ensure full functionality
print_test "Step 8: Verify full functionality (encrypt/decrypt)"
PLAINTEXT="QUNMIFJlbG9hZCBUZXN0"
ENC_RESPONSE=$(curl -s --cacert "$CA_CERT" --cert "$CLIENT_CERT" --key "$CLIENT_KEY" \
    -H "Content-Type: application/json" \
    -d "{\"context\":\"exchange-key\",\"plaintext\":\"$PLAINTEXT\"}" \
    "$BASE_URL/encrypt")

if ! echo "$ENC_RESPONSE" | grep -q "ciphertext"; then
    echo "Response: $ENC_RESPONSE"
    print_error "Encryption failed after restoration"
fi
print_success "Full functionality verified"

echo ""
echo "=========================================="
print_success "✓ ACL Real-time Reload E2E Test PASSED"
echo "=========================================="
echo ""
echo "Summary:"
echo "  1. ✓ Client connected successfully (baseline)"
echo "  2. ✓ Client added to revocation list"
echo "  3. ✓ Auto-reload detected changes (30s)"
echo "  4. ✓ Client correctly blocked (403 Forbidden)"
echo "  5. ✓ Client removed from revocation list"
echo "  6. ✓ Auto-reload detected restoration (30s)"
echo "  7. ✓ Client successfully restored"
echo "  8. ✓ Full functionality verified"
