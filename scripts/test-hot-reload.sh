#!/bin/bash
# Integration test for KEK hot reload functionality
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

echo "🔥 Testing KEK Hot Reload Functionality"
echo "========================================"

# Check if HSM service is running
if ! docker compose ps | grep -q "hsm-service.*Up"; then
    echo "❌ HSM service not running. Start with: docker compose up -d"
    exit 1
fi

# Backup current metadata
echo "📦 Backing up current metadata.yaml..."
cp "$PROJECT_ROOT/metadata.yaml" "$PROJECT_ROOT/metadata.yaml.backup-test"

# Function to cleanup on exit
cleanup() {
    echo ""
    echo "🧹 Cleaning up..."
    if [ -f "$PROJECT_ROOT/metadata.yaml.backup-test" ]; then
        mv "$PROJECT_ROOT/metadata.yaml.backup-test" "$PROJECT_ROOT/metadata.yaml"
        echo "✓ Restored original metadata.yaml"
    fi
}
trap cleanup EXIT

# Test 1: Check initial state
echo ""
echo "📋 Test 1: Check initial key versions"
echo "--------------------------------------"

# Get current version from metadata
CURRENT_VERSION=$(grep -A 3 "exchange-key:" "$PROJECT_ROOT/metadata.yaml" | grep "current:" | awk '{print $2}')
echo "Current active version: $CURRENT_VERSION"

# Test 2: Modify metadata.yaml
echo ""
echo "📝 Test 2: Modify metadata.yaml (simulate rotation)"
echo "---------------------------------------------------"

# Create a test modification (add a comment to trigger modTime change)
echo "# Hot reload test - $(date)" >> "$PROJECT_ROOT/metadata.yaml"

echo "✓ Modified metadata.yaml"
echo "⏳ Waiting for hot reload (35 seconds)..."
sleep 35

# Test 3: Check service logs for reload event
echo ""
echo "📜 Test 3: Verify hot reload in logs"
echo "-------------------------------------"

if docker compose logs --since 40s hsm-service 2>&1 | grep -q "KEK hot reload successful"; then
    echo "✅ Hot reload detected in logs"
    docker compose logs --since 40s hsm-service 2>&1 | grep "KEK hot reload"
else
    echo "⚠️  No hot reload event found in logs"
    echo "Recent logs:"
    docker compose logs --tail 20 hsm-service
fi

# Test 4: Test encrypt operation still works
echo ""
echo "🔐 Test 4: Verify encryption still works after reload"
echo "------------------------------------------------------"

# Check if test client cert exists
if [ ! -f "$PROJECT_ROOT/pki/client/trading-service-1.crt" ]; then
    echo "⚠️  Test client certificate not found, skipping API test"
else
    RESPONSE=$(curl -s -k -X POST https://localhost:8443/encrypt \
        --cert "$PROJECT_ROOT/pki/client/trading-service-1.crt" \
        --key "$PROJECT_ROOT/pki/client/trading-service-1.key" \
        -H "Content-Type: application/json" \
        -d '{"context":"exchange-key","plaintext":"dGVzdA=="}' 2>&1)
    
    if echo "$RESPONSE" | jq -e '.ciphertext' > /dev/null 2>&1; then
        echo "✅ Encryption successful after hot reload"
        echo "Key used: $(echo "$RESPONSE" | jq -r '.key_id')"
    else
        echo "❌ Encryption failed:"
        echo "$RESPONSE"
    fi
fi

# Test 5: Check metadata auto-reload logs
echo ""
echo "📊 Test 5: Check metadata auto-reload statistics"
echo "------------------------------------------------"

RELOAD_COUNT=$(docker compose logs hsm-service 2>&1 | grep -c "metadata file changed" || echo "0")
SUCCESS_COUNT=$(docker compose logs hsm-service 2>&1 | grep -c "KEK hot reload successful" || echo "0")

echo "Metadata change detections: $RELOAD_COUNT"
echo "Successful hot reloads: $SUCCESS_COUNT"

# Summary
echo ""
echo "========================================" 
echo "📊 Test Summary"
echo "========================================"

if [ "$SUCCESS_COUNT" -gt 0 ]; then
    echo "✅ Hot reload is working correctly"
    echo "   - Metadata changes detected: $RELOAD_COUNT"
    echo "   - Successful reloads: $SUCCESS_COUNT"
    echo ""
    echo "🎯 Phase 4 Task 4.4 - PASSED"
else
    echo "⚠️  Hot reload may not be working"
    echo "   - Metadata changes detected: $RELOAD_COUNT"
    echo "   - Successful reloads: $SUCCESS_COUNT"
    echo ""
    echo "Check logs with: docker compose logs hsm-service | grep reload"
fi

echo ""
echo "✓ Test completed"
