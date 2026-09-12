# Karkain Cross-Platform Release Builder (Windows)
# Builds for all supported platforms and creates release archives.
#
# Usage:
#   .\scripts\release-all.ps1 [version]
#
# Prerequisites:
#   - Go 1.21+
#   - 7z or PowerShell built-in Compress-Archive

param(
    [string]$Version = "v0.117.0"
)

$ErrorActionPreference = "Stop"
$ProjectRoot = Split-Path -Parent $PSScriptRoot
$OutputDir = Join-Path $ProjectRoot "releases"
$BuildDir = Join-Path $ProjectRoot ".build"
$BinaryName = "karkain"

# Clean previous builds
if (Test-Path $BuildDir) { Remove-Item -Recurse -Force $BuildDir }
if (Test-Path $OutputDir) { Remove-Item -Recurse -Force $OutputDir }
New-Item -ItemType Directory -Path $BuildDir, $OutputDir -Force | Out-Null

Write-Host "==============================================" -ForegroundColor Cyan
Write-Host "  Karkain Release Builder $Version" -ForegroundColor Cyan
Write-Host "==============================================" -ForegroundColor Cyan
Write-Host ""

# Define all targets
$Targets = @(
    @{ OS = "windows"; Arch = "amd64";   Ext = ".exe"; Format = "zip" }
    @{ OS = "windows"; Arch = "arm64";   Ext = ".exe"; Format = "zip" }
    @{ OS = "linux";   Arch = "amd64";   Ext = "";     Format = "tar.gz" }
    @{ OS = "linux";   Arch = "arm64";   Ext = "";     Format = "tar.gz" }
    @{ OS = "linux";   Arch = "armv7";   Ext = "";     Format = "tar.gz" }
    @{ OS = "linux";   Arch = "i386";    Ext = "";     Format = "tar.gz" }
    @{ OS = "linux";   Arch = "ppc64le"; Ext = "";     Format = "tar.gz" }
    @{ OS = "linux";   Arch = "s390x";   Ext = "";     Format = "tar.gz" }
    @{ OS = "darwin";  Arch = "amd64";   Ext = "";     Format = "tar.gz" }
    @{ OS = "darwin";  Arch = "arm64";   Ext = "";     Format = "tar.gz" }
    @{ OS = "freebsd"; Arch = "amd64";   Ext = "";     Format = "tar.gz" }
    @{ OS = "netbsd";  Arch = "amd64";   Ext = "";     Format = "tar.gz" }
    @{ OS = "openbsd"; Arch = "amd64";   Ext = "";     Format = "tar.gz" }
)

$Built = 0
$Failed = 0

foreach ($Target in $Targets) {
    $GOOS = $Target.OS
    $GOARCH = $Target.Arch
    $Ext = $Target.Ext
    $Format = $Target.Format

    Write-Host -NoNewline "Building $GOOS/$GOARCH... "

    $OutName = "$BinaryName$Ext"
    $StageDir = Join-Path $BuildDir "$BinaryName-$Version-$GOOS-$GOARCH"
    $OutPath = Join-Path $StageDir $OutName

    New-Item -ItemType Directory -Path $StageDir -Force | Out-Null

    try {
        $env:GOOS = $GOOS
        $env:GOARCH = $GOARCH
        $env:CGO_ENABLED = "0"

        & go build -ldflags="-s -w" -o $OutPath (Join-Path $ProjectRoot "cmd\karkain") 2>$null

        if ($LASTEXITCODE -ne 0) { throw "build failed" }

        # Copy supporting files
        Copy-Item (Join-Path $ProjectRoot "README.md") $StageDir -ErrorAction SilentlyContinue
        Copy-Item (Join-Path $ProjectRoot "LICENSE") $StageDir -ErrorAction SilentlyContinue
        Set-Content (Join-Path $StageDir "VERSION") $Version

        # Copy stdlib
        $StdDir = Join-Path $ProjectRoot "stdlib"
        if (Test-Path $StdDir) {
            Copy-Item $StdDir (Join-Path $StageDir "stdlib") -Recurse -ErrorAction SilentlyContinue
        }

        # Create archive
        $ArchiveName = "$BinaryName-$Version-$GOOS-$GOARCH"

        if ($Format -eq "zip") {
            $ArchivePath = Join-Path $OutputDir "$ArchiveName.zip"
            Compress-Archive -Path $StageDir -DestinationPath $ArchivePath -Force
        } else {
            # For tar.gz on Windows, use tar if available
            $ArchivePath = Join-Path $OutputDir "$ArchiveName.tar.gz"
            Push-Location $BuildDir
            & tar czf $ArchivePath "$ArchiveName/"
            Pop-Location
        }

        $Size = (Get-Item $ArchivePath).Length / 1MB
        Write-Host "OK ($([math]::Round($Size, 1)) MB)" -ForegroundColor Green
        $Built++
    } catch {
        Write-Host "FAILED" -ForegroundColor Red
        $Failed++
    } finally {
        $env:GOOS = ""
        $env:GOARCH = ""
        $env:CGO_ENABLED = ""
    }
}

Write-Host ""
Write-Host "==============================================" -ForegroundColor Cyan
Write-Host "  Results: $Built built, $Failed failed" -ForegroundColor Cyan
Write-Host "  Output:  $OutputDir" -ForegroundColor Cyan
Write-Host "==============================================" -ForegroundColor Cyan
Write-Host ""

# List archives
Write-Host "Release archives:" -ForegroundColor Yellow
Get-ChildItem $OutputDir | ForEach-Object {
    $SizeMB = [math]::Round($_.Length / 1MB, 1)
    Write-Host "  $($_.Name) ($SizeMB MB)"
}

# SHA-256 checksums
Write-Host ""
Write-Host "Checksums (SHA-256):" -ForegroundColor Yellow
Get-ChildItem $OutputDir | ForEach-Object {
    $Hash = (Get-FileHash $_.FullName -Algorithm SHA256).Hash.ToLower()
    Write-Host "  $Hash  $($_.Name)"
}
