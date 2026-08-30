# Karkain Build Script for Windows (PowerShell equivalent)
# Simulates the POSIX build.sh functionality for Windows testing

$ErrorActionPreference = "Stop"

Write-Host "========================================" -ForegroundColor Cyan
Write-Host "Karkain Build Script (Windows Test)" -ForegroundColor Cyan
Write-Host "========================================" -ForegroundColor Cyan

# Detect operating system
$OS = [System.Environment]::OSVersion.Platform
Write-Host "Detected OS: $OS"

# Detect C compiler
$CC = $null
if (Get-Command gcc -ErrorAction SilentlyContinue) {
    $CC = "gcc"
    Write-Host "Detected C compiler: gcc" -ForegroundColor Green
} elseif (Get-Command clang -ErrorAction SilentlyContinue) {
    $CC = "clang"
    Write-Host "Detected C compiler: clang" -ForegroundColor Green
} else {
    Write-Host "Error: No C compiler found (clang or gcc required)" -ForegroundColor Red
    exit 1
}

# Set environment variable
$env:CC = $CC
Write-Host "Set CC=$CC" -ForegroundColor Yellow

# Check if Go is installed
if (-not (Get-Command go -ErrorAction SilentlyContinue)) {
    Write-Host "Error: Go is not installed or not in PATH" -ForegroundColor Red
    exit 1
}

Write-Host "Go version: $(go version)" -ForegroundColor Green

# Get project root directory
$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$ProjectRoot = Split-Path -Parent $ScriptDir
Write-Host "Project root: $ProjectRoot" -ForegroundColor Yellow

# Change to project root
Set-Location $ProjectRoot

# Create bin directory if it doesn't exist
if (-not (Test-Path "bin")) {
    New-Item -ItemType Directory -Path "bin" -Force | Out-Null
}

Write-Host "========================================" -ForegroundColor Cyan
Write-Host "Building Go-based Karkain compiler..." -ForegroundColor Cyan
Write-Host "========================================" -ForegroundColor Cyan

# Build the Go compiler
go build -o bin/karkain.exe ./cmd/karkain

if ($LASTEXITCODE -eq 0) {
    Write-Host "Go build successful" -ForegroundColor Green
    Write-Host "Output: bin/karkain.exe" -ForegroundColor Green
} else {
    Write-Host "Go build failed" -ForegroundColor Red
    exit 1
}

# Check if a source file was provided
if ($args.Count -eq 0) {
    Write-Host "No source file provided. Build complete." -ForegroundColor Yellow
    Write-Host "Usage: .\scripts\build.ps1 <file.kark>" -ForegroundColor Yellow
    exit 0
}

$SourceFile = $args[0]

# Check if source file exists
if (-not (Test-Path $SourceFile)) {
    Write-Host "Error: Source file '$SourceFile' not found" -ForegroundColor Red
    exit 1
}

# Check if source file has .kark extension
if ($SourceFile -notmatch '\.kark$') {
    Write-Host "Error: Source file must have .kark extension" -ForegroundColor Red
    exit 1
}

Write-Host "========================================" -ForegroundColor Cyan
Write-Host "Compiling Karkain source file..." -ForegroundColor Cyan
Write-Host "========================================" -ForegroundColor Cyan
Write-Host "Source: $SourceFile" -ForegroundColor Yellow
Write-Host "C Compiler: $CC" -ForegroundColor Yellow

# Run the Karkain compiler with the detected C compiler
$env:CC = $CC
Write-Host "Using C compiler: $CC" -ForegroundColor Yellow
.\bin\karkain.exe run $SourceFile

if ($LASTEXITCODE -eq 0) {
    Write-Host "Compilation and execution successful" -ForegroundColor Green
} else {
    Write-Host "Compilation or execution failed" -ForegroundColor Red
    exit 1
}

Write-Host "========================================" -ForegroundColor Cyan
Write-Host "Build complete!" -ForegroundColor Cyan
Write-Host "========================================" -ForegroundColor Cyan