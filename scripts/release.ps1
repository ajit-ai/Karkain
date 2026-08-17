# Karkain Cross-Compilation & Release Packaging Script
# Produces standalone release archives for multiple platforms
#
# Targets:
#   - windows/amd64  -> karkain-windows-amd64.zip
#   - linux/amd64    -> karkain-linux-amd64.tar.gz
#   - darwin/arm64   -> karkain-darwin-arm64.tar.gz

$ErrorActionPreference = "Stop"

# ============================================================
# Configuration
# ============================================================

$Version = "v0.14.0"
$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$ProjectRoot = Split-Path -Parent $ScriptDir
$ReleaseDir = Join-Path $ProjectRoot "releases"
$TempDir = Join-Path $ReleaseDir "tmp"

Write-Host "========================================" -ForegroundColor Cyan
Write-Host "Karkain Cross-Compilation Release Build" -ForegroundColor Cyan
Write-Host "Version: $Version" -ForegroundColor Yellow
Write-Host "========================================" -ForegroundColor Cyan
Write-Host ""

Set-Location $ProjectRoot

# Ensure Go is available
if (-not (Get-Command go -ErrorAction SilentlyContinue)) {
    Write-Host "Error: Go is not installed or not in PATH" -ForegroundColor Red
    exit 1
}

Write-Host "Go version: $(go version)" -ForegroundColor Green
Write-Host ""

# ============================================================
# Clean previous release artifacts
# ============================================================

Write-Host "Cleaning previous release artifacts..." -ForegroundColor Yellow

if (Test-Path $TempDir) {
    Remove-Item $TempDir -Recurse -Force
}

if (Test-Path $ReleaseDir) {
    Get-ChildItem $ReleaseDir -File -Filter "*.zip" | Remove-Item -Force -ErrorAction SilentlyContinue
    Get-ChildItem $ReleaseDir -File -Filter "*.tar.gz" | Remove-Item -Force -ErrorAction SilentlyContinue
} else {
    New-Item -ItemType Directory -Path $ReleaseDir -Force | Out-Null
}

New-Item -ItemType Directory -Path $TempDir -Force | Out-Null

# ============================================================
# Cross-compile targets
# ============================================================

$targets = @(
    @{ OS = "windows"; Arch = "amd64"; Suffix = ".exe"; ArchiveType = "zip" },
    @{ OS = "linux";   Arch = "amd64"; Suffix = "";     ArchiveType = "tar.gz" },
    @{ OS = "darwin";  Arch = "arm64"; Suffix = "";     ArchiveType = "tar.gz" }
)

$allSucceeded = $true
$builtArchives = @()

foreach ($target in $targets) {
    $os = $target.OS
    $arch = $target.Arch
    $suffix = $target.Suffix
    $archiveType = $target.ArchiveType

    $binaryName = "karkain-$os-$arch$suffix"
    $packageName = "karkain-$Version-$os-$arch"

    Write-Host ""
    Write-Host "========================================" -ForegroundColor Cyan
    Write-Host "Building: $os/$arch" -ForegroundColor Cyan
    Write-Host "========================================" -ForegroundColor Cyan

    # Set environment variables for cross-compilation
    $env:GOOS = $os
    $env:GOARCH = $arch
    $env:CGO_ENABLED = "0"

    $buildStart = Get-Date
    $outputPath = Join-Path $TempDir $packageName $binaryName

    # Create package directory
    $pkgDir = Join-Path $TempDir $packageName
    if (Test-Path $pkgDir) {
        Remove-Item $pkgDir -Recurse -Force
    }
    New-Item -ItemType Directory -Path $pkgDir -Force | Out-Null

    # Create bin directory inside package
    New-Item -ItemType Directory -Path (Join-Path $pkgDir "bin") -Force | Out-Null

    # Build
    Write-Host "  Compiling $os/$arch..." -ForegroundColor Yellow
    $env:CGO_ENABLED = "0"
    $buildOutput = go build -o $outputPath ./cmd/karkain 2>&1

    $buildEnd = Get-Date
    $buildDuration = ($buildEnd - $buildStart).TotalSeconds

    if ($LASTEXITCODE -ne 0) {
        Write-Host "  Build FAILED for $os/$arch" -ForegroundColor Red
        Write-Host "  Error: $buildOutput" -ForegroundColor Red
        $allSucceeded = $false
        continue
    }

    # Verify binary was created
    if (-not (Test-Path $outputPath)) {
        Write-Host "  Binary not found at $outputPath" -ForegroundColor Red
        $allSucceeded = $false
        continue
    }

    $binarySize = (Get-Item $outputPath).Length / 1KB
    Write-Host "  Binary: $([math]::Round($binarySize, 2)) KB" -ForegroundColor Green
    Write-Host "  Build time: $([math]::Round($buildDuration, 2))s" -ForegroundColor Green

    # ============================================================
    # Bundle standard library
    # ============================================================

    Write-Host "  Bundling stdlib..." -ForegroundColor Yellow

    # Copy stdlib/ directory
    if (Test-Path (Join-Path $ProjectRoot "stdlib")) {
        Copy-Item (Join-Path $ProjectRoot "stdlib") (Join-Path $pkgDir "stdlib") -Recurse -Force
        Write-Host "    stdlib/ copied" -ForegroundColor Green
    }

    # Copy std/ directory (legacy)
    if (Test-Path (Join-Path $ProjectRoot "std")) {
        Copy-Item (Join-Path $ProjectRoot "std") (Join-Path $pkgDir "std") -Recurse -Force
        Write-Host "    std/ copied" -ForegroundColor Green
    }

    # Copy compiler/ directory (self-hosting .kar sources)
    if (Test-Path (Join-Path $ProjectRoot "compiler")) {
        Copy-Item (Join-Path $ProjectRoot "compiler") (Join-Path $pkgDir "compiler") -Recurse -Force
        Write-Host "    compiler/ copied" -ForegroundColor Green
    }

    # Copy examples/
    if (Test-Path (Join-Path $ProjectRoot "examples")) {
        Copy-Item (Join-Path $ProjectRoot "examples") (Join-Path $pkgDir "examples") -Recurse -Force
        Write-Host "    examples/ copied" -ForegroundColor Green
    }

    # Copy runtime/
    if (Test-Path (Join-Path $ProjectRoot "runtime")) {
        Copy-Item (Join-Path $ProjectRoot "runtime") (Join-Path $pkgDir "runtime") -Recurse -Force
        Write-Host "    runtime/ copied" -ForegroundColor Green
    }

    # Copy scripts/
    if (Test-Path (Join-Path $ProjectRoot "scripts")) {
        Copy-Item (Join-Path $ProjectRoot "scripts") (Join-Path $pkgDir "scripts") -Recurse -Force
        Write-Host "    scripts/ copied" -ForegroundColor Green
    }

    # Copy editors/
    if (Test-Path (Join-Path $ProjectRoot "editors")) {
        Copy-Item (Join-Path $ProjectRoot "editors") (Join-Path $pkgDir "editors") -Recurse -Force
        Write-Host "    editors/ copied" -ForegroundColor Green
    }

    # Copy LICENSE and README
    if (Test-Path (Join-Path $ProjectRoot "LICENSE")) {
        Copy-Item (Join-Path $ProjectRoot "LICENSE") (Join-Path $pkgDir "LICENSE") -Force
    }
    if (Test-Path (Join-Path $ProjectRoot "README.md")) {
        Copy-Item (Join-Path $ProjectRoot "README.md") (Join-Path $pkgDir "README.md") -Force
    }

    # Create VERSION file
    $versionContent = "Karkain Compiler $Version`nBuilt: $(Get-Date -Format 'yyyy-MM-dd HH:mm:ss')`nPlatform: $os/$arch`nGo: $(go version)"
    Set-Content -Path (Join-Path $pkgDir "VERSION") -Value $versionContent

    # ============================================================
    # Create archive
    # ============================================================

    $archiveName = "karkain-$Version-$os-$arch.$archiveType"
    $archivePath = Join-Path $ReleaseDir $archiveName

    Write-Host "  Creating $archiveType archive..." -ForegroundColor Yellow

    if ($archiveType -eq "zip") {
        Compress-Archive -Path (Join-Path $pkgDir "*") -DestinationPath $archivePath -Force
    } else {
        # tar.gz - use tar if available, else use Go
        if (Get-Command tar -ErrorAction SilentlyContinue) {
            Push-Location $TempDir
            tar -czf $archivePath $packageName
            Pop-Location
        } else {
            Write-Host "  tar not available, creating zip instead" -ForegroundColor Yellow
            $archivePath = Join-Path $ReleaseDir "karkain-$Version-$os-$arch.zip"
            Compress-Archive -Path (Join-Path $pkgDir "*") -DestinationPath $archivePath -Force
        }
    }

    if (Test-Path $archivePath) {
        $archiveSize = (Get-Item $archivePath).Length / 1KB
        Write-Host "  Archive: $archiveName ($([math]::Round($archiveSize, 2)) KB)" -ForegroundColor Green
        $builtArchives += $archivePath
    } else {
        Write-Host "  Failed to create archive" -ForegroundColor Red
        $allSucceeded = $false
    }
}

# ============================================================
# Clean up temp directory
# ============================================================

Write-Host ""
Write-Host "Cleaning up temporary files..." -ForegroundColor Yellow
if (Test-Path $TempDir) {
    Remove-Item $TempDir -Recurse -Force
}

# ============================================================
# Summary
# ============================================================

Write-Host ""
Write-Host "========================================" -ForegroundColor Cyan
Write-Host "Release Build Summary" -ForegroundColor Cyan
Write-Host "========================================" -ForegroundColor Cyan

if ($builtArchives.Count -gt 0) {
    Write-Host "Built $($builtArchives.Count) release archive(s):" -ForegroundColor Green
    foreach ($archive in $builtArchives) {
        $size = [math]::Round((Get-Item $archive).Length / 1KB, 2)
        Write-Host "  $([System.IO.Path]::GetFileName($archive)) ($size KB)" -ForegroundColor Green
    }
}

# Restore native build environment
$env:GOOS = ""
$env:GOARCH = ""
$env:CGO_ENABLED = "1"

Write-Host ""
if ($allSucceeded) {
    Write-Host "All targets built successfully!" -ForegroundColor Green
} else {
    Write-Host "Some targets failed to build" -ForegroundColor Red
    exit 1
}

Write-Host "Release artifacts: $ReleaseDir" -ForegroundColor Yellow
Write-Host "========================================" -ForegroundColor Cyan
