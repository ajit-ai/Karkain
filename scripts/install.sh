#!/usr/bin/env bash
# Karkain installer (POSIX sh / bash)
#
# Builds the karkain CLI from source and installs it into a prefix directory.
# Today Karkain ships as a source build: pre-built binaries are not published
# yet (see docs/source/development/release.rst).
#
# Exit codes:
#   0  success
#   1  missing prerequisites (Go or a C compiler)
#   2  toolchain build failure
#   3  install smoke-test failure

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

PREFIX="${1:-${PREFIX:-$HOME/.local/karkain}}"
BIN_DIR="$PREFIX/bin"
EXE="$BIN_DIR/karkain"

echo "Karkain installer (source build)"
echo "Project root: $PROJECT_ROOT"
echo "Install prefix: $PREFIX"

# ---- verify prerequisites ----
if command -v go >/dev/null 2>&1; then
    echo "Go: $(go version 2>&1 | head -n 1)"
else
    echo "Go: not found (required to compile Karkain)" >&2
    exit 1
fi

if command -v gcc >/dev/null 2>&1; then
    echo "C compiler: gcc"
elif command -v clang >/dev/null 2>&1; then
    echo "C compiler: clang"
else
    echo "C compiler: none of gcc/clang found (required at runtime)" >&2
    exit 1
fi

# ---- build ----
mkdir -p "$BIN_DIR"
echo "Building karkain from source..."
(cd "$PROJECT_ROOT" && go build -o "$EXE" ./cmd/karkain)
echo "Built: $EXE"

# ---- write version fingerprint ----
REPO_VERSION="$(head -n 1 "$PROJECT_ROOT/VERSION" 2>/dev/null || echo "1.1.0")"
printf 'Karkain Compiler %s (source build)\n' "$REPO_VERSION" > "$PREFIX/VERSION"

# ---- verify --version ----
VER_OUT="$("$EXE" --version 2>&1 || true)"
if ! printf '%s' "$VER_OUT" | grep -q "Stable Build"; then
    echo "installed binary does not identify a 1.1.0 Stable build: $VER_OUT" >&2
    exit 3
fi
echo "Version: $VER_OUT"

# ---- smoke test: first program ----
if [ "${SKIP_SMOKE:-}" != "1" ]; then
    SCRATCH="$(mktemp -d)"
    cat > "$SCRATCH/hello.kark" <<'EOF'
func main() {
    print("Hello, Karkain!")
}
EOF
    (cd "$SCRATCH" && \
        KARKAIN_ENGINE=go "$EXE" check hello.kark && \
        RUN_OUT="$(KARKAIN_ENGINE=go "$EXE" run hello.kark 2>&1)" && \
        printf '%s' "$RUN_OUT" | grep -q "Hello, Karkain!")
    RC=$?
    rm -rf "$SCRATCH"
    if [ $RC -ne 0 ]; then
        echo "smoke: check + run hello.kark FAILED" >&2
        exit 3
    fi
    echo "Smoke test: check + run hello.kark PASS"
fi

echo ""
echo "Install complete."
echo 'Add the bin directory to PATH:'
echo "  export PATH=\"$BIN_DIR:\$PATH\""
exit 0