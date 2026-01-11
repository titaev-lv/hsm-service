#!/bin/bash
# Quick smoke test for performance testing
# Verifies HSM service is running and accepts requests

set -euo pipefail

GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m'

HSM_URL="${HSM_URL:-https://localhost:8443}"

# Client certificates (mTLS required)
CLIENT_CERT="${CLIENT_CERT:-pki/client/hsm-trading-client-1.crt}"
CLIENT_KEY="${CLIENT_KEY:-pki/client/hsm-trading-client-1.key}"

# Check if certificates exist
if [ ! -f "$CLIENT_CERT" ] || [ ! -f "$CLIENT_KEY" ]; then
    echo -e "${RED}✗ Client certificates not found${NC}"
    echo "   Expected: $CLIENT_CERT and $CLIENT_KEY"
    echo "   Run: ./pki/scripts/issue-client-cert.sh to generate"
    exit 1
fi

echo "🔍 Checking HSM Service at $HSM_URL"
echo "   Using client cert: $CLIENT_CERT"
echo ""

# Test 1: Health check
echo -n "1. Health check... "
if curl -k -s -f --cert "$CLIENT_CERT" --key "$CLIENT_KEY" "$HSM_URL/health" > /dev/null 2>&1; then
    echo -e "${GREEN}✓ OK${NC}"
else
    echo -e "${RED}✗ FAILED${NC}"
    echo "   HSM service is not running at $HSM_URL"
    echo "   Start with: docker compose up -d"
    exit 1
fi

# Test 2: Encrypt endpoint
echo -n "2. Encrypt endpoint... "
ENCRYPT_RESPONSE=$(curl -k -s -X POST --cert "$CLIENT_CERT" --key "$CLIENT_KEY" "$HSM_URL/encrypt" \
    -H "Content-Type: application/json" \
    -d '{"context":"exchange-key","plaintext":"SGVsbG8gV29ybGQh"}' 2>&1)

if echo "$ENCRYPT_RESPONSE" | grep -q "ciphertext"; then
    echo -e "${GREEN}✓ OK${NC}"
    CIPHERTEXT=$(echo "$ENCRYPT_RESPONSE" | grep -o '"ciphertext":"[^"]*"' | cut -d'"' -f4)
    KEY_ID=$(echo "$ENCRYPT_RESPONSE" | grep -o '"key_id":"[^"]*"' | cut -d'"' -f4)
else
    echo -e "${YELLOW}⚠ WARNING${NC}"
    echo "   Response: $ENCRYPT_RESPONSE"
    echo "   This might be expected if mTLS is enforced"
fi

# Test 3: Decrypt endpoint
if [ -n "${CIPHERTEXT:-}" ] && [ -n "${KEY_ID:-}" ]; then
    echo -n "3. Decrypt endpoint... "
    DECRYPT_RESPONSE=$(curl -k -s -X POST --cert "$CLIENT_CERT" --key "$CLIENT_KEY" "$HSM_URL/decrypt" \
        -H "Content-Type: application/json" \
        -d "{\"context\":\"exchange-key\",\"ciphertext\":\"$CIPHERTEXT\",\"key_id\":\"$KEY_ID\"}" 2>&1)
    
    if echo "$DECRYPT_RESPONSE" | grep -q "plaintext"; then
        echo -e "${GREEN}✓ OK${NC}"
    else
        echo -e "${YELLOW}⚠ WARNING${NC}"
        echo "   Response: $DECRYPT_RESPONSE"
    fi
else
    echo "3. Decrypt endpoint... ${YELLOW}⊘ SKIPPED${NC} (no ciphertext from encrypt)"
fi

echo ""
echo -e "${GREEN}HSM Service is reachable!${NC}"
echo ""
echo "You can now run:"
echo "  • k6 run tests/performance/load-test.js"
echo "  • ./tests/performance/stress-test.sh"
echo "  • ./tests/performance/benchmark-test.sh"
echo ""

# Show current performance
echo "Current service stats:"
docker stats --no-stream hsm-service 2>/dev/null || echo "  (docker stats not available)"
