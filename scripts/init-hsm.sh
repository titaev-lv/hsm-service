#!/bin/sh
set -e

# Configuration from environment variables
TOKEN_LABEL="${HSM_TOKEN_LABEL:-hsm-token}"
TOKEN_PIN="${HSM_PIN:-1234}"
TOKEN_SO_PIN="${HSM_SO_PIN:-12345678}"

echo "========================================="
echo "HSM Service Initialization"
echo "========================================="

# Create token directory if it doesn't exist
mkdir -p /var/lib/softhsm/tokens

# Check if token is already initialized
if ! softhsm2-util --show-slots | grep -q "$TOKEN_LABEL"; then
    echo "⏳ Initializing SoftHSM token: $TOKEN_LABEL"
    softhsm2-util --init-token \
        --free \
        --label "$TOKEN_LABEL" \
        --pin "$TOKEN_PIN" \
        --so-pin "$TOKEN_SO_PIN"
    echo "✓ Token initialized successfully"
else
    echo "✓ Token '$TOKEN_LABEL' already initialized"
fi

# Display token information
echo ""
echo "Token slots:"
softhsm2-util --show-slots

echo ""
echo "========================================="
echo "KEK Management"
echo "========================================="
echo "Note: Use 'hsm-admin' to manage KEKs"
echo "Example:"
echo "  docker exec <container> /app/hsm-admin list-kek"
echo "  docker exec <container> /app/hsm-admin export-metadata"
echo ""

# Export HSM_PIN for the service
export HSM_PIN="$TOKEN_PIN"

echo "========================================="
echo "KEK Setup"
echo "========================================="

refresh_kek_cache() {
    KEK_LIST_OUTPUT=$(/app/hsm-admin list-kek --verbose 2>/dev/null || true)
}

kek_exists() {
    label="$1"
    echo "$KEK_LIST_OUTPUT" | grep -Fq "$label: ✓ Available in HSM"
}

refresh_kek_cache
KEK_COUNT=$(echo "$KEK_LIST_OUTPUT" | grep -c "Available in HSM" || true)
if [ -z "$KEK_COUNT" ]; then
    KEK_COUNT=0
fi

echo "✓ Found $KEK_COUNT KEK(s) in HSM token"

# Create missing KEKs
CREATED_ANY=false

if ! kek_exists "kek-exchange-key-v1"; then
    echo "⚠️  kek-exchange-key-v1 not found. Creating..."
    /app/hsm-admin create-kek --label kek-exchange-key-v1 --context exchange-key --version 1 || echo "Failed to create kek-exchange-key-v1"
    refresh_kek_cache
    CREATED_ANY=true
else
    echo "✓ kek-exchange-key-v1 already exists"
fi

if ! kek_exists "kek-2fa-v1"; then
    echo "⚠️  kek-2fa-v1 not found. Creating..."
    /app/hsm-admin create-kek --label kek-2fa-v1 --context 2fa --version 1 || echo "Failed to create kek-2fa-v1"
    refresh_kek_cache
    CREATED_ANY=true
else
    echo "✓ kek-2fa-v1 already exists"
fi

if [ "$CREATED_ANY" = true ]; then
    echo ""
    echo "✓ Default KEKs created"
    echo ""
    echo "⏳ Computing KEK checksums..."
    /app/hsm-admin update-checksums || echo "⚠️  Failed to update checksums"
    echo "✓ Checksums computed and saved to metadata.yaml"
fi

# Reconcile metadata-declared KEK versions with actual HSM objects.
# This prevents startup failure when metadata current points to v2+ while token only has v1.
if [ -f /app/metadata.yaml ]; then
    echo ""
    echo "⏳ Reconciling KEKs from metadata.yaml..."

    META_LABELS=$(grep -E '^[[:space:]]*-[[:space:]]*label:[[:space:]]*kek-.*-v[0-9]+[[:space:]]*$' /app/metadata.yaml \
        | sed -E 's/.*label:[[:space:]]*//')

    for label in $META_LABELS; do
        case "$label" in
            kek-*-v*)
                ;;
            *)
                echo "⚠️  Skipping unsupported label format from metadata: $label"
                continue
                ;;
        esac

                if kek_exists "$label"; then
            continue
        fi

        context=$(echo "$label" | sed -E 's/^kek-(.*)-v[0-9]+$/\1/')
        version=$(echo "$label" | sed -E 's/^kek-.*-v([0-9]+)$/\1/')

        if [ -z "$context" ] || [ -z "$version" ] || [ "$context" = "$label" ]; then
            echo "⚠️  Failed to parse context/version from label: $label"
            continue
        fi

        echo "⚠️  $label declared in metadata but missing in HSM. Creating..."
        if /app/hsm-admin create-kek --label "$label" --context "$context" --version "$version"; then
            refresh_kek_cache
            CREATED_ANY=true
        else
            echo "⚠️  Failed to create metadata-declared key: $label"
        fi
    done

    if [ "$CREATED_ANY" = true ]; then
        echo ""
        echo "⏳ Recomputing KEK checksums after metadata reconciliation..."
        /app/hsm-admin update-checksums || echo "⚠️  Failed to update checksums"
        echo "✓ Metadata reconciliation complete"
    else
        echo "✓ Metadata reconciliation complete (no missing KEKs)"
    fi
fi

# Ensure revoked.yaml exists to avoid spurious reload logs
if [ ! -f /app/revoked.yaml ]; then
    echo "# ACL revocation list"
    echo "revoked_certificates: []"
fi > /app/revoked.yaml
echo "✓ Revocation list initialized"

echo ""
echo "========================================="
echo "Starting HSM Service..."
echo "========================================="
echo ""

# Start the HSM service
exec /app/hsm-service
