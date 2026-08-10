# Karkain Release Packaging Script v0.10.0
$ErrorActionPreference = "Stop"
$projectRoot = "F:\Codes\karkain"
$Version = "v0.10.0"
$releaseDir = "releases"
$packageName = "karkain-$Version"
$packagePath = "$releaseDir\$packageName.zip"

Write-Host "Karkain Release Packaging Script" -ForegroundColor Cyan
Write-Host "Version: $Version" -ForegroundColor Yellow
Write-Host ""

Set-Location $projectRoot

Write-Host "Checking components..." -ForegroundColor Yellow

if (!(Test-Path "bin/karkain.exe")) {
    go build -o bin/karkain.exe ./cmd/karkain
}

Write-Host "Creating release package..." -ForegroundColor Yellow

$tempDir = "$releaseDir\$packageName"
if (Test-Path $tempDir) {
    Remove-Item $tempDir -Recurse -Force
}
New-Item -ItemType Directory -Path $tempDir -Force | Out-Null

New-Item -ItemType Directory -Path "$tempDir\bin" -Force | Out-Null
Copy-Item "bin\karkain.exe" "$tempDir\bin\" -Force

New-Item -ItemType Directory -Path "$tempDir\compiler" -Force | Out-Null
Copy-Item "compiler\*.kar" "$tempDir\compiler\" -Force

if (Test-Path "std") {
    Copy-Item "std" "$tempDir\" -Recurse -Force
}

New-Item -ItemType Directory -Path "$tempDir\examples" -Force | Out-Null
Copy-Item "examples\*.kar" "$tempDir\examples\" -Force

New-Item -ItemType Directory -Path "$tempDir\scripts" -Force | Out-Null
Copy-Item "scripts\*.ps1" "$tempDir\scripts\" -Force

if (Test-Path "README.md") {
    Copy-Item "README.md" "$tempDir\" -Force
}

$versionContent = "Karkain Compiler $Version"
Set-Content -Path "$tempDir\VERSION" -Value $versionContent

Write-Host "Creating ZIP archive..." -ForegroundColor Yellow
Compress-Archive -Path "$tempDir\*" -DestinationPath $packagePath -Force

if (Test-Path $packagePath) {
    $packageSize = (Get-Item $packagePath).Length / 1KB
    Write-Host "Release package created: $packagePath" -ForegroundColor Green
    Write-Host "Size: $([math]::Round($packageSize, 2)) KB" -ForegroundColor Green
} else {
    Write-Host "Failed to create release package" -ForegroundColor Red
    exit 1
}

Write-Host "Verifying package contents..." -ForegroundColor Yellow

$verifyDir = "$releaseDir\verify"
if (Test-Path $verifyDir) {
    Remove-Item $verifyDir -Recurse -Force
}
New-Item -ItemType Directory -Path $verifyDir -Force | Out-Null

Expand-Archive -Path $packagePath -DestinationPath $verifyDir -Force

$files = @(
    "$verifyDir\bin\karkain.exe",
    "$verifyDir\compiler\lexer.kar",
    "$verifyDir\compiler\ast.kar",
    "$verifyDir\compiler\parser.kar",
    "$verifyDir\compiler\codegen.kar",
    "$verifyDir\compiler\main.kar",
    "$verifyDir\examples\phase11_test.kar",
    "$verifyDir\examples\self_host_codegen_test.kar",
    "$verifyDir\scripts\bootstrap.ps1",
    "$verifyDir\VERSION"
)

$allVerified = $true
foreach ($file in $files) {
    if (Test-Path $file) {
        Write-Host "Verified: $($file.Replace($verifyDir, ''))" -ForegroundColor Green
    } else {
        Write-Host "Missing: $($file.Replace($verifyDir, ''))" -ForegroundColor Red
        $allVerified = $false
    }
}

Remove-Item $verifyDir -Recurse -Force

if ($allVerified) {
    Write-Host "Release package verification: SUCCESS" -ForegroundColor Green
} else {
    Write-Host "Release package verification: FAILED" -ForegroundColor Red
    exit 1
}

Write-Host "Release package created successfully!" -ForegroundColor Green