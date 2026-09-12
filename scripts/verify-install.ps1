# Karkain install verifier (Windows / PowerShell 5.1+)
#
# Validates an installed karkain binary without rebuilding it. Used after
# scripts\install.ps1 and by scripts\verify-rc-journey.ps1.
#
# Usage:
#   .\scripts\verify-install.ps1 [-Prefix <install prefix>]
#
# Exit codes:
#   0  installed binary verified
#   1  binary missing or --version/--help contract failed
#   2  first-program smoke (check/build/run) failed
#   3  invalid arguments

param(
    [string]$Prefix = ""
)

$ErrorActionPreference = "Stop"

function Fail([int]$Code, [string]$Msg) {
    Write-Host "verify-install: $Msg" -ForegroundColor Red
    exit $Code
}

if ([string]::IsNullOrWhiteSpace($Prefix)) {
    $Prefix = Join-Path $env:LOCALAPPDATA "Karkain"
}
$Exe = Join-Path (Join-Path $Prefix "bin") "karkain.exe"

if (-not (Test-Path -LiteralPath $Exe)) {
    Fail 1 "no karkain binary at $Exe"
}

Write-Host "Verifying installed Karkain at $Exe" -ForegroundColor Cyan

# --- version identity ---
$verOut = (& $Exe --version 2>&1) -join "`n"
if ($LASTEXITCODE -ne 0) {
    Fail 1 "karkain --version failed (exit $LASTEXITCODE):`n$verOut"
}
if ($verOut -notmatch "Karkain Compiler v0\.117\.0" -or $verOut -notmatch "Beta 1 Build") {
    Fail 1 "unexpected --version output: $verOut"
}
Write-Host "OK --version: $verOut" -ForegroundColor Green

# --- help contract ---
$helpOut = (& $Exe --help 2>&1) -join "`n"
if ($LASTEXITCODE -ne 0) {
    Fail 1 "karkain --help failed (exit $LASTEXITCODE)"
}
foreach ($cmd in @("run", "build", "check", "test", "prof", "explain", "target", "workspace", "init")) {
    if ($helpOut -notmatch "\b$cmd\b") {
        Fail 1 "--help does not mention required command: $cmd"
    }
}
Write-Host "OK --help command coverage" -ForegroundColor Green

# --- first-program smoke ---
$oldEngine = $env:KARKAIN_ENGINE
$env:KARKAIN_ENGINE = "go"
try {
    $scratch = Join-Path $env:TEMP ("karkain-verify-" + [guid]::NewGuid().ToString("N"))
    New-Item -ItemType Directory -Path $scratch -Force | Out-Null
    $hello = Join-Path $scratch "hello.kark"
    Set-Content -Path $hello -Value "func main() {`n    print(`"Hello, Karkain!`")`n}" -Encoding ASCII

    & $Exe check $hello 2>&1 | Out-Null
    if ($LASTEXITCODE -ne 0) {
        Fail 2 "karkain check hello.kark failed (exit $LASTEXITCODE)"
    }
    & $Exe build $hello 2>&1 | Out-Null
    if ($LASTEXITCODE -ne 0) {
        Fail 2 "karkain build hello.kark failed (exit $LASTEXITCODE)"
    }
    $runOut = (& $Exe run $hello 2>&1) -join "`n"
    if ($LASTEXITCODE -ne 0) {
        Fail 2 "karkain run hello.kark failed (exit $LASTEXITCODE):`n$runOut"
    }
    if ($runOut -notmatch "Hello, Karkain!") {
        Fail 2 "unexpected run output: $runOut"
    }
    Write-Host "OK check + build + run hello.kark" -ForegroundColor Green
    Remove-Item $scratch -Recurse -Force -ErrorAction SilentlyContinue
}
finally {
    $env:KARKAIN_ENGINE = $oldEngine
}

Write-Host "Install verified: PASS" -ForegroundColor Green
exit 0