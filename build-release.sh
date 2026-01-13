#!/bin/bash
set -e

# Build Release Script for HSM Service
# Usage: ./build-release.sh

echo "========================================"
echo "HSM Service - Release Builder"
echo "========================================"
echo ""

# Detect version
VERSION=$(git describe --tags --always --dirty 2>/dev/null || echo "dev")
echo "Version: $VERSION"

# Build using Makefile
echo ""
echo "Building binaries..."
make clean
make build

# Create release package
echo ""
echo "Creating release package..."
make release

echo ""
echo "========================================"
echo "✓ Release built successfully!"
echo "========================================"
echo ""
echo "Release package: release/hsm-service-${VERSION}-linux-amd64.tar.gz"
echo ""
echo "Next steps:"
echo "  1. Test the binaries:"
echo "     tar -xzf release/hsm-service-${VERSION}-linux-amd64.tar.gz"
echo "     cd hsm-service-${VERSION}-linux-amd64"
echo "     sha256sum -c CHECKSUMS.txt"
echo ""
echo "  2. Copy to production server:"
echo "     scp release/hsm-service-${VERSION}-linux-amd64.tar.gz prod-server:/tmp/"
echo ""
echo "  3. Follow PRODUCTION_DEBIAN.md for setup"
echo ""
