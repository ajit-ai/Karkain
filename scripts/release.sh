#!/usr/bin/env bash
# Karkain Cross-Compilation & Release Packaging Script (POSIX)
# Produces standalone release archives for multiple platforms
#
# Targets:
#   - linux/amd64    -> karkain-linux-amd64.tar.gz
#   - darwin/arm64   -> karkain-darwin-arm64.tar.gz
#   - windows/amd64  -> karkain-windows-amd64.zip (if zip available)

set -euo pipefail

# ============================================================
# Configuration
# ============================================================

VERSION="${VERSION:-v0.14.0}"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
RELEASE_DIR="$PROJECT_ROOT/releases"
TEMP_DIR="$RELEASE_DIR/tmp"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m'

info()  { echo -e "${CYAN}$1${NC}"; }
ok()    { echo -e "${GREEN}$1${NC}"; }
warn()  { echo -e "${YELLOW}$1${NC}"; }
fail()  { echo -e "${RED}$1${NC}"; }

cd "$PROJECT_ROOT"

info "========================================"
info "Karkain Cross-Compilation Release Build"
info "Version: $VERSION"
info "========================================"
echo ""

# Ensure Go is available
if ! command -v go &>/dev/null; then
    fail "Error: Go is not installed or not in PATH"
    exit 1
fi

ok "Go version: $(go version)"
echo ""

# ============================================================
# Clean previous release artifacts
# ============================================================

warn "Cleaning previous release artifacts..."

rm -rf "$TEMP_DIR"
mkdir -p "$TEMP_DIR"

# Remove old archives (keep other files)
find "$RELEASE_DIR" -maxdepth 1 -name "*.zip" -delete 2>/dev/null || true
find "$RELEASE_DIR" -maxdepth 1 -name "*.tar.gz" -delete 2>/dev/null || true

# ============================================================
# Cross-compile targets
# ============================================================

ALL_SUCCEEDED=true
BUILT_ARCHIVES=()

build_target() {
    local os="$1"
    local arch="$2"
    local suffix="$3"
    local archive_type="$4"

    local binary_name="karkain-${os}-${arch}${suffix}"
    local package_name="karkain-${VERSION}-${os}-${arch}"

    echo ""
    info "========================================"
    info "Building: ${os}/${arch}"
    info "========================================"

    # Create package directory
    local pkg_dir="$TEMP_DIR/$package_name"
    rm -rf "$pkg_dir"
    mkdir -p "$pkg_dir/bin"

    # Cross-compile
    warn "  Compiling ${os}/${arch}..."
    local build_start=$(date +%s%N)

    if ! CGO_ENABLED=0 GOOS="$os" GOARCH="$arch" go build -o "$pkg_dir/bin/$binary_name" ./cmd/karkain; then
        fail "  Build FAILED for ${os}/${arch}"
        ALL_SUCCEEDED=false
        return 1
    fi

    local build_end=$(date +%s%N)
    local build_duration=$(( (build_end - build_start) / 1000000 ))

    # Verify binary
    if [ ! -f "$pkg_dir/bin/$binary_name" ]; then
        fail "  Binary not found"
        ALL_SUCCEEDED=false
        return 1
    fi

    local binary_size=$(du -k "$pkg_dir/bin/$binary_name" | cut -f1)
    ok "  Binary: ${binary_size} KB (built in ${build_duration}ms)"

    # ============================================================
    # Bundle standard library
    # ============================================================

    warn "  Bundling stdlib..."

    [ -d "$PROJECT_ROOT/stdlib" ] && cp -r "$PROJECT_ROOT/stdlib" "$pkg_dir/stdlib" && ok "    stdlib/ copied"
    [ -d "$PROJECT_ROOT/std" ]    && cp -r "$PROJECT_ROOT/std"    "$pkg_dir/std"    && ok "    std/ copied"
    [ -d "$PROJECT_ROOT/compiler" ] && cp -r "$PROJECT_ROOT/compiler" "$pkg_dir/compiler" && ok "    compiler/ copied"
    [ -d "$PROJECT_ROOT/examples" ] && cp -r "$PROJECT_ROOT/examples" "$pkg_dir/examples" && ok "    examples/ copied"
    [ -d "$PROJECT_ROOT/runtime" ]  && cp -r "$PROJECT_ROOT/runtime"  "$pkg_dir/runtime"  && ok "    runtime/ copied"
    [ -d "$PROJECT_ROOT/scripts" ]  && cp -r "$PROJECT_ROOT/scripts"  "$pkg_dir/scripts"  && ok "    scripts/ copied"
    [ -d "$PROJECT_ROOT/editors" ]  && cp -r "$PROJECT_ROOT/editors"  "$pkg_dir/editors"  && ok "    editors/ copied"

    # Copy docs
    [ -f "$PROJECT_ROOT/LICENSE" ]   && cp "$PROJECT_ROOT/LICENSE"   "$pkg_dir/"
    [ -f "$PROJECT_ROOT/README.md" ] && cp "$PROJECT_ROOT/README.md" "$pkg_dir/"

    # Create VERSION file
    cat > "$pkg_dir/VERSION" <<EOF
Karkain Compiler $VERSION
Built: $(date -u '+%Y-%m-%d %H:%M:%S UTC')
Platform: ${os}/${arch}
Go: $(go version)
EOF

    # ============================================================
    # Create archive
    # ============================================================

    local archive_name="karkain-${VERSION}-${os}-${arch}.${archive_type}"
    local archive_path="$RELEASE_DIR/$archive_name"

    warn "  Creating archive..."

    if [ "$archive_type" = "zip" ]; then
        if command -v zip &>/dev/null; then
            (cd "$TEMP_DIR" && zip -qr "$archive_path" "$package_name")
        else
            warn "  zip not available, skipping"
            return 1
        fi
    else
        tar -czf "$archive_path" -C "$TEMP_DIR" "$package_name"
    fi

    if [ -f "$archive_path" ]; then
        local archive_size=$(du -k "$archive_path" | cut -f1)
        ok "  Archive: $archive_name (${archive_size} KB)"
        BUILT_ARCHIVES+=("$archive_path")
    else
        fail "  Failed to create archive"
        ALL_SUCCEEDED=false
        return 1
    fi
}

# Build all targets
build_target "linux"   "amd64" ""    "tar.gz" || true
build_target "darwin"  "arm64" ""    "tar.gz" || true
build_target "windows" "amd64" ".exe" "zip"    || true

# ============================================================
# Clean up temp directory
# ============================================================

echo ""
warn "Cleaning up temporary files..."
rm -rf "$TEMP_DIR"

# ============================================================
# Summary
# ============================================================

echo ""
info "========================================"
info "Release Build Summary"
info "========================================"

if [ ${#BUILT_ARCHIVES[@]} -gt 0 ]; then
    ok "Built ${#BUILT_ARCHIVES[@]} release archive(s):"
    for archive in "${BUILT_ARCHIVES[@]}"; do
        local_size=$(du -k "$archive" | cut -f1)
        ok "  $(basename "$archive") (${local_size} KB)"
    done
fi

echo ""
if [ "$ALL_SUCCEEDED" = true ]; then
    ok "All targets built successfully!"
else
    fail "Some targets failed to build"
    exit 1
fi

info "Release artifacts: $RELEASE_DIR"
info "========================================"
