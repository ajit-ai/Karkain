# Karkain Release Packaging Script
$ErrorActionPreference = "Stop"

# Force-kill any running instances locking karkain.exe
Stop-Process -Name "karkain" -Force -ErrorAction SilentlyContinue

$Version = "v0.9.0"
$DistDir = "dist"
$StageDir = Join-Path $DistDir "karkain-$Version-windows-amd64"
$ZipPath = Join-Path $DistDir "karkain-$Version-windows-amd64.zip"

Write-Host "[+] Building Karkain Compiler $Version binary..." -ForegroundColor Green
if (-not (Test-Path "bin")) { 
    New-Item -ItemType Directory -Path "bin" | Out-Null 
}

go build -o bin/karkain.exe ./cmd/karkain

if ($LASTEXITCODE -ne 0) {
    Write-Host "[-] Build failed!" -ForegroundColor Red
    exit 1
}

Write-Host "[+] Creating release distribution structure..." -ForegroundColor Green
if (Test-Path $StageDir) { Remove-Item -Recurse -Force $StageDir }
if (Test-Path $ZipPath) { Remove-Item -Force $ZipPath }

New-Item -ItemType Directory -Path $StageDir | Out-Null

Copy-Item "bin/karkain.exe" -Destination "$StageDir\"
if (Test-Path "README.md") { Copy-Item "README.md" -Destination "$StageDir\" }
if (Test-Path "examples") { Copy-Item -Recurse "examples" -Destination "$StageDir\examples" }

Write-Host "[+] Compressing to $ZipPath..." -ForegroundColor Green
Compress-Archive -Path "$StageDir\*" -DestinationPath $ZipPath -Force

Remove-Item -Recurse -Force $StageDir

$msg = "[+] Release package created successfully at: $ZipPath"
Write-Host $msg -ForegroundColor Cyan