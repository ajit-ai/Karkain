#!/bin/bash
# Karkain Build Script for POSIX Environments (Linux/macOS)
# Automatically detects C compiler and builds Karkain compiler

set -e  # Exit on error

# Color output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo "========================================"
echo "Karkain Build Script (POSIX)"
echo "========================================"

# Detect operating system
OS=$(uname -s)
echo "Detected OS: $OS"

# Detect C compiler
if command -v clang &> /dev/null; then
    CC=clang
    echo "Detected C compiler: clang"
elif command -v gcc &> /dev/null; then
    CC=gcc
    echo "Detected C compiler: gcc"
else
    echo -e "${RED}Error: No C compiler found (clang or gcc required)${NC}"
    exit 1
fi

# Export CC for downstream processes
export CC=$CC
echo "Exported CC=$CC"

# Check if Go is installed
if ! command -v go &> /dev/null; then
    echo -e "${RED}Error: Go is not installed or not in PATH${NC}"
    exit 1
fi

echo "Go version: $(go version)"

# Get project root directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
echo "Project root: $PROJECT_ROOT"

# Change to project root
cd "$PROJECT_ROOT"

# Create bin directory if it doesn't exist
mkdir -p bin

echo "========================================"
echo "Building Go-based Karkain compiler..."
echo "========================================"

# Build the Go compiler
go build -o bin/karkain ./cmd/karkain

if [ $? -eq 0 ]; then
    echo -e "${GREEN}✓ Go build successful${NC}"
    echo "Output: bin/karkain"
else
    echo -e "${RED}✗ Go build failed${NC}"
    exit 1
fi

# Check if a source file was provided
if [ $# -eq 0 ]; then
    echo -e "${YELLOW}No source file provided. Build complete.${NC}"
    echo "Usage: $0 <file.kar>"
    exit 0
fi

SOURCE_FILE=$1

# Check if source file exists
if [ ! -f "$SOURCE_FILE" ]; then
    echo -e "${RED}Error: Source file '$SOURCE_FILE' not found${NC}"
    exit 1
fi

# Check if source file has .kar extension
if [[ "$SOURCE_FILE" != *.kar ]]; then
    echo -e "${RED}Error: Source file must have .kar extension${NC}"
    exit 1
fi

echo "========================================"
echo "Compiling Karkain source file..."
echo "========================================"
echo "Source: $SOURCE_FILE"
echo "C Compiler: $CC"

# Run the Karkain compiler with the detected C compiler
# The karkain compiler will use the CC environment variable
echo "Using C compiler: $CC"
CC=$CC ./bin/karkain run "$SOURCE_FILE"

if [ $? -eq 0 ]; then
    echo -e "${GREEN}✓ Compilation and execution successful${NC}"
else
    echo -e "${RED}✗ Compilation or execution failed${NC}"
    exit 1
fi

echo "========================================"
echo "Build complete!"
echo "========================================"