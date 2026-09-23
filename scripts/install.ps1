# Karkain installer (Windows / PowerShell 5.1+)
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
#   4  invalid arguments

param(
    [string]$Prefix = "",
    [switch]$SkipSmoke
)

$ErrorActionPreference = "Stop"

function Fail([int]$Code, [string]$Msg) {
    Write-Host "install: $Msg" -ForegroundColor Red
    exit $Code
}

$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$ProjectRoot = Split-Path -Parent $ScriptDir

# ---- resolve install prefix ----
if ([string]::IsNullOrWhiteSpace($Prefix)) {
    $Prefix = Join-Path $env:LOCALAPPDATA "Karkain"
}
$BinDir = Join-Path $Prefix "bin"
$Exe = Join-Path $BinDir "karkain.exe"

Write-Host "Karkain installer (source build)" -ForegroundColor Cyan
Write-Host "Project root: $ProjectRoot" -ForegroundColor Yellow
Write-Host "Install prefix: $Prefix" -ForegroundColor Yellow

# ---- verify prerequisites ----
$haveGo = $false
if (Get-Command go -ErrorAction SilentlyContinue) {
    $haveGo = $true
    Write-Host "Go: $(go version)" -ForegroundColor Green
} else {
    Write-Host "Go: not found (required to compile Karkain)" -ForegroundColor Red
}

$haveC = $false
if ($env:CC) {
    if (Get-Command $env:CC -ErrorAction SilentlyContinue) {
        $haveC = $true
        Write-Host "C compiler: $env:CC (CC)" -ForegroundColor Green
    }
} else {
    foreach ($c in @("gcc", "clang")) {
        if (Get-Command $c -ErrorAction SilentlyContinue) {
            $haveC = $true
            Write-Host "C compiler: $c" -ForegroundColor Green
            break
        }
    }
}
if (-not $haveC) {
    Write-Host "C compiler: none of gcc/clang found (required at runtime)" -ForegroundColor Red
}

if (-not $haveGo) {
    Fail 1 "A Go toolchain (Go 1.21+) is required to build Karkain from source."
}
if (-not $haveC) {
    Fail 1 "A C compiler (GCC or Clang) is required on PATH for check/build/run."
}

# ---- build ----
Write-Host "Building karkain.exe from source..." -ForegroundColor Cyan
New-Item -ItemType Directory -Path $BinDir -Force | Out-Null
$buildOut = & go build -o $Exe ./cmd/karkain 2>&1
if ($LASTEXITCODE -ne 0) {
    Fail 2 "go build failed:`n$buildOut"
}
Write-Host "Built: $Exe" -ForegroundColor Green

# ---- write version fingerprint ----
$repoVersion = Get-Content (Join-Path $ProjectRoot "VERSION") -ErrorAction SilentlyContinue
if (-not $repoVersion) { $repoVersion = "1.1.0" }
$fp = "Karkain Compiler $repoVersion`r`nBuild: from source (Go `$(go version))`r`nInstall date: $(Get-Date -Format 'yyyy-MM-dd')"
Set-Content -Path (Join-Path $Prefix "VERSION") -Value $fp -Encoding ASCII

# ---- verify --version ----
$verOut = & $Exe --version 2>&1
if ($LASTEXITCODE -ne 0) {
    Fail 3 "installed binary failed --version:`n$verOut"
}
if ($verOut -notmatch "Stable Build") {
    Fail 3 "installed binary does not identify a 1.1.0 Stable build: $verOut"
}
Write-Host "Version: $verOut" -ForegroundColor Green

# ---- smoke test: first program ----
if (-not $SkipSmoke) {
    $oldEngine = $env:KARKAIN_ENGINE
    $env:KARKAIN_ENGINE = "go"
    try {
        $scratch = Join-Path $env:TEMP ("karkain-install-" + [guid]::NewGuid().ToString("N"))
        New-Item -ItemType Directory -Path $scratch -Force | Out-Null
        $hello = Join-Path $scratch "hello.kark"
        Set-Content -Path $hello -Value "func main() {`n    print(`"Hello, Karkain!`")`n}" -Encoding ASCII

        & $Exe check $hello 2>&1 | Out-Null
        if ($LASTEXITCODE -ne 0) {
            Fail 3 "smoke: karkain check hello.kark failed (exit $LASTEXITCODE)"
        }
        $runOut = & $Exe run $hello 2>&1
        if ($LASTEXITCODE -ne 0) {
            Fail 3 "smoke: karkain run hello.kark failed (exit $LASTEXITCODE):`n$runOut"
        }
        if ($runOut -notmatch "Hello, Karkain!") {
            Fail 3 "smoke: unexpected output from hello.kark: $runOut"
        }
        Write-Host "Smoke test: check + run hello.kark PASS" -ForegroundColor Green
        Remove-Item $scratch -Recurse -Force -ErrorAction SilentlyContinue
    }
    finally {
        $env:KARKAIN_ENGINE = $oldEngine
    }
}

# ---- next steps ----
Write-Host ""
Write-Host "Install complete." -ForegroundColor Cyan
Write-Host "Add the bin directory to your PATH to use karkain from anywhere:" -ForegroundColor Yellow
Write-Host "  `$env:PATH += `";$BinDir`"" -ForegroundColor Yellow
exit 0