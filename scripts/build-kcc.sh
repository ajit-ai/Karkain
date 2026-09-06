#!/bin/bash
# Phase 88: Build the self-hosted Karkain compiler (kcc)
# Workflow: bootstrap compiler → C23 → gcc → kcc executable
set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
ROOT_DIR="$(dirname "$SCRIPT_DIR")"
BOOTSTRAP="$ROOT_DIR/karkain.exe"
SRC="$ROOT_DIR/src/compiler/main.kark"
OUT_C="$ROOT_DIR/src/compiler/main.c"
OUT_EXE="$ROOT_DIR/kcc.exe"

echo "=== Phase 88: Building self-hosted Karkain compiler ==="

# Step 1: Build bootstrap compiler if needed
if [ ! -f "$BOOTSTRAP" ]; then
    echo "[1/4] Building bootstrap compiler..."
    cd "$ROOT_DIR"
    go build -o karkain.exe ./cmd/karkain/
else
    echo "[1/4] Bootstrap compiler found: $BOOTSTRAP"
fi

# Step 2: Use bootstrap to compile self-hosted compiler to C23
echo "[2/4] Bootstrap → self-hosted compiler C23 output..."
cd "$ROOT_DIR"
"$BOOTSTRAP" build "$SRC" --target c23

# Step 3: Compile C23 output with gcc
echo "[3/4] Compiling C23 → native executable..."
gcc -std=c99 -o "$OUT_EXE" "$OUT_C" -lm -lgmp

# Step 4: Verify
echo "[4/4] Verifying..."
"$OUT_EXE" --version

echo ""
echo "=== Self-hosted compiler built successfully ==="
echo "Executable: $OUT_EXE"
echo ""
echo "Usage:"
echo "  $OUT_EXE check <file.kark>    — Parse and check a .kark file"
echo "  $OUT_EXE build <file.kark>    — Compile to C23"
echo "  $OUT_EXE run <file.kark>      — Compile and execute"
