#!/usr/bin/env bash
# Karkain Cross-Platform Release Builder
# Builds for all supported platforms and creates release archives.
#
# Usage:
#   ./scripts/release-all.sh [version]
#
# Prerequisites:
#   - Go 1.21+
#   - zip (for Windows archives)
#   - tar + gzip (for Linux/macOS/BSD archives)

set -euo pipefail

VERSION="${1:-v0.115.0}"
PROJECT_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
OUTPUT_DIR="$PROJECT_ROOT/releases"
BUILD_DIR="$PROJECT_ROOT/.build"
BINARY_NAME="karkain"

# Clean previous builds
rm -rf "$BUILD_DIR" "$OUTPUT_DIR"
mkdir -p "$BUILD_DIR" "$OUTPUT_DIR"

echo "=============================================="
echo "  Karkain Release Builder $VERSION"
echo "=============================================="
echo ""

# Define all targets: GOOS/GOARCH/EXT/ARCHIVE_TYPE
TARGETS=(
    "windows/amd64/.exe/zip"
    "windows/arm64/.exe/zip"
    "linux/amd64//tar.gz"
    "linux/arm64//tar.gz"
    "linux/armv7//tar.gz"
    "linux/i386//tar.gz"
    "linux/ppc64le//tar.gz"
    "linux/s390x//tar.gz"
    "darwin/amd64//tar.gz"
    "darwin/arm64//tar.gz"
    "freebsd/amd64//tar.gz"
    "netbsd/amd64//tar.gz"
    "openbsd/amd64//tar.gz"
)

BUILT=0
FAILED=0

for target in "${TARGETS[@]}"; do
    IFS='/' read -r GOOS GOARCH EXT ARCHIVE_TYPE <<< "$target"

    echo -n "Building $GOOS/$GOARCH... "

    # Skip js/wasm if no EXT
    OUT_NAME="$BINARY_NAME$EXT"
    STAGE_DIR="$BUILD_DIR/$BINARY_NAME-$VERSION-$GOOS-$GOARCH"
    OUT_PATH="$STAGE_DIR/$OUT_NAME"

    mkdir -p "$STAGE_DIR"

    # Set env and build
    if GOOS="$GOOS" GOARCH="$GOARCH" CGO_ENABLED=0 \
        go build -ldflags="-s -w -X main.versionString=Karkain Compiler $VERSION ($GOOS/$GOARCH)" \
        -o "$OUT_PATH" "$PROJECT_ROOT/cmd/karkain" 2>/dev/null; then

        # Copy supporting files into stage directory
        cp "$PROJECT_ROOT/README.md" "$STAGE_DIR/" 2>/dev/null || true
        cp "$PROJECT_ROOT/LICENSE" "$STAGE_DIR/" 2>/dev/null || true
        echo "$VERSION" > "$STAGE_DIR/VERSION"

        # Copy stdlib
        if [ -d "$PROJECT_ROOT/stdlib" ]; then
            cp -r "$PROJECT_ROOT/stdlib" "$STAGE_DIR/stdlib"
        elif [ -d "$PROJECT_ROOT/std" ]; then
            cp -r "$PROJECT_ROOT/std" "$STAGE_DIR/stdlib"
        fi

        # Create archive
        ARCHIVE_NAME="$BINARY_NAME-$VERSION-$GOOS-$GOARCH"
        cd "$BUILD_DIR"

        if [ "$ARCHIVE_TYPE" = "zip" ]; then
            zip -qr "$OUTPUT_DIR/$ARCHIVE_NAME.zip" "$ARCHIVE_NAME/"
        else
            tar czf "$OUTPUT_DIR/$ARCHIVE_NAME.tar.gz" "$ARCHIVE_NAME/"
        fi

        cd "$PROJECT_ROOT"
        SIZE=$(du -sh "$OUTPUT_DIR/$ARCHIVE_NAME.$ARCHIVE_TYPE" | cut -f1)
        echo "OK ($SIZE)"
        BUILT=$((BUILT + 1))
    else
        echo "FAILED"
        FAILED=$((FAILED + 1))
    fi
done

echo ""
echo "=============================================="
echo "  Results: $BUILT built, $FAILED failed"
echo "  Output:  $OUTPUT_DIR/"
echo "=============================================="
echo ""

# List all archives
echo "Release archives:"
ls -lh "$OUTPUT_DIR/"*.$ARCHIVE_TYPE 2>/dev/null || ls -lh "$OUTPUT_DIR/"* 2>/dev/null

echo ""
echo "Checksums (SHA-256):"
cd "$OUTPUT_DIR"
for f in *; do
    if command -v sha256sum &>/dev/null; then
        sha256sum "$f"
    elif command -v shasum &>/dev/null; then
        shasum -a 256 "$f"
    fi
done
