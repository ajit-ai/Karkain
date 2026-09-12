# beta-fresh-checkout.ps1 - Phase 117 Beta-1 fresh-checkout validation.
#
# Simulates a first-time developer cloning the Karkain repository and walking
# the documented Beta workflow without any local state:
#
#   checkout -> go build ./... -> go vet ./... -> rebuild karkain.exe
#   -> help -> hello world (check/build/run) -> test -> debug -> profile
#   -> target -> example corpus (verify-examples) -> Sphinx documentation
#
# The script is deliberately self-contained: it compiles the CLI from source,
# works in repository-relative paths only, and creates no artifacts outside a
# scratch directory it removes before exiting. It does not depend on any
# private absolute path, personal environment variable or stale binary.
#
# Prerequisites (documented): Go toolchain, a C compiler on PATH (gcc/clang;
# the C-transpile pipeline and the self-hosted kcc engine both need one),
# and - optionally - Python with Sphinx for the documentation step (that
# step logs SKIP when unavailable rather than failing).
#
# Usage:
#   powershell -ExecutionPolicy Bypass -File scripts\beta-fresh-checkout.ps1
#   powershell -ExecutionPolicy Bypass -File scripts\beta-fresh-checkout.ps1 -RepoRoot <path>
#
# Exit code: 0 when every step passes; 1 on the first failure.

param(
    [string]$RepoRoot = ".",
    [switch]$SkipCorpus
)

$ErrorActionPreference = "Stop"

function Fail([string]$msg) {
    Write-Output "FAIL $msg"
    exit 1
}

foreach ($tool in @("go", "gcc")) {
    if (-not (Get-Command $tool -ErrorAction SilentlyContinue)) {
        Fail ("required tool '$tool' not found on PATH")
    }
}

Push-Location $RepoRoot
try {
    $repo = (Get-Location).Path
    if (-not (Test-Path -LiteralPath (Join-Path $repo "cmd\karkain\main.go"))) {
        Fail "not a Karkain repository root: $repo"
    }
    Write-Output "ROOT $repo"
    Write-Output "ENGINE default = kcc (self-hosted), gcc = $((Get-Command gcc).Source)"

    Write-Output "OK go build ./..."
    go build ./...
    if ($LASTEXITCODE -ne 0) { Fail ("go build ./... exit " + $LASTEXITCODE) }

    Write-Output "OK go vet ./..."
    go vet ./...
    if ($LASTEXITCODE -ne 0) { Fail ("go vet ./... exit " + $LASTEXITCODE) }

    Write-Output "OK rebuild karkain.exe"
    go build -o karkain.exe ./cmd/karkain
    if ($LASTEXITCODE -ne 0) { Fail ("go build -o karkain.exe exit " + $LASTEXITCODE) }
    $bin = Join-Path $repo "karkain.exe"

    $help = (& $bin --help 2>&1 | Out-String)
    if (-not ($help -match "Usage")) { Fail "help output missing Usage banner`n$help" }
    Write-Output "OK help"

    $version = (& $bin --version 2>&1 | Out-String)
    if (-not ($version -match "Karkain Compiler")) { Fail "--version output missing compiler banner" }
    Write-Output "OK version $version"

    $target = (& $bin target 2>&1 | Out-String)
    if (-not ($target -match "native")) { Fail "target output missing native target" }
    Write-Output "OK target"

    $scratch = Join-Path $repo ".karkain-fresh-checkout"
    if (Test-Path -LiteralPath $scratch) { Remove-Item -LiteralPath $scratch -Recurse -Force }
    New-Item -ItemType Directory -Path $scratch | Out-Null
    try {
        # Native tools (karkain debug traces to stderr; gcc/compilers may warn)
        # write to stderr; under $ErrorActionPreference="Stop" PS 5.1 escalates
        # any NativeCommandError into a terminating error. The explicit
        # $LASTEXITCODE checks below are authoritative, so relax EAP here.
        $oldEAP = $ErrorActionPreference
        $ErrorActionPreference = "Continue"
        $hello = Join-Path $scratch "hello.kark"
        Set-Content -LiteralPath $hello -Value @"
// Status: Runnable
// Engine: both
func main() {
    print("hello beta 1")
}
"@ -Encoding ASCII -NoNewline

        $check = (& $bin check $hello 2>&1 | Out-String)
        if (-not ($check -match "\[ok\]")) { Fail "check did not report [ok]`n$check" }
        Write-Output "OK check hello"

        $build = (& $bin build $hello 2>&1 | Out-String)
        if ($LASTEXITCODE -ne 0) { Fail "build hello exit $LASTEXITCODE`n$build" }
        Write-Output "OK build hello"

        & $bin run $hello 2>&1 | Out-Null
        if ($LASTEXITCODE -ne 0) { Fail "run hello exit $LASTEXITCODE" }
        Write-Output "OK run hello"

        $t = Join-Path $scratch "t.kark"
        Set-Content -LiteralPath $t -Value @"
func test_alpha() { assert_eq(2, 2) }
func test_beta() { assert_eq(1, 1) }
"@ -Encoding ASCII -NoNewline
        $tout = (& $bin test $t 2>&1 | Out-String)
        if ($LASTEXITCODE -ne 0 -or -not ($tout -match "2 passed")) {
            Fail "test runner: expected 2 passed`n$tout"
        }
        Write-Output "OK test runner"

        $dbg = (& $bin debug $hello 2>&1 | Out-String)
        if (-not ($dbg -match "karkain:hello.kark:enter main")) {
            Fail "debug trace missing enter marker`n$dbg"
        }
        Write-Output "OK debug trace"

        $prof = (& $bin prof $hello 2>&1 | Out-String)
        if ($LASTEXITCODE -ne 0 -or -not ($prof -match "Functions \(by total time\):")) {
            Fail "prof report missing function table`n$prof"
        }
        Write-Output "OK prof report"
    }
    finally {
        $ErrorActionPreference = $oldEAP
        if (Test-Path -LiteralPath $scratch) { Remove-Item -LiteralPath $scratch -Recurse -Force }
    }

    if (-not $SkipCorpus) {
        $corpus = & powershell -ExecutionPolicy Bypass -File (Join-Path $repo "scripts\verify-examples.ps1") -Karkain $bin -Quiet 2>&1 | Out-String
        if ($LASTEXITCODE -ne 0) {
            Fail ("example corpus exit $LASTEXITCODE`n" + $corpus)
        }
        Write-Output "OK example corpus (verify-examples.ps1)"
    }

    if (Get-Command python -ErrorAction SilentlyContinue) {
        python -c "import sphinx" 2>$null | Out-Null
        if ($LASTEXITCODE -eq 0) {
            python -m sphinx -b html -W --keep-going (Join-Path $repo "docs\source") (Join-Path $repo "docs\build\html") 2>&1 | Out-Null
            if ($LASTEXITCODE -ne 0) { Fail "sphinx strict build failed" }
            Write-Output "OK sphinx strict html"

            python -m sphinx -b linkcheck -W (Join-Path $repo "docs\source") (Join-Path $repo "docs\build\linkcheck") 2>&1 | Out-Null
            if ($LASTEXITCODE -ne 0) { Fail "sphinx linkcheck failed" }
            Write-Output "OK sphinx linkcheck"
        }
        else {
            Write-Output "SKIP sphinx (not installed)"
        }
    }
    else {
        Write-Output "SKIP sphinx (python not installed)"
    }

    Write-Output ("BETA FRESH-CHECKOUT: PASS")
}
finally {
    Pop-Location
}