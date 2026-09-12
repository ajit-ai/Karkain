# run-full-qa.ps1 - Karkain 1.0.0 master QA gate
#
# Orchestrates the existing test machinery (no duplicated suites) for the
# Phase 119 release preparation. Stages run in order and abort at the first
# failure; the failing stage is printed last. Exit code 0 = all stages green.
#
# Stages:
#   build      go build ./...
#   vet        go vet ./...
#   units      go test over all non-CLI packages (-count=1 -p 1)
#   cligates   phase-114/115/116/117/118 CLI gates + conformance + probes
#              (monolithic gcc-gated; -p 1 to avoid the host's OOM class)
#   conformance conformance corpus (Go gate)
#   probes     probes corpus (Go gate)
#   examples   build karkain.exe into a temp dir, run verify-examples.ps1
#   docs       Sphinx html -W --keep-going + linkcheck -W (SKIPs w/o python)
#   fresh      scripts/beta-fresh-checkout.ps1 (fresh-checkout gate)
#   install    scripts/install.ps1 + scripts/verify-install.ps1 (temp prefix)
#   journey    scripts/verify-rc-journey.ps1 (11-step external journey)
#
# Usage:
#   powershell -ExecutionPolicy Bypass -File scripts\qa\run-full-qa.ps1
#   powershell -ExecutionPolicy Bypass -File scripts\qa\run-full-qa.ps1 -Stage cligates
#   powershell -ExecutionPolicy Bypass -File scripts\qa\run-full-qa.ps1 -Skip docs,fresh
#
# Prerequisites: Go toolchain, C compiler (gcc/clang) on PATH for the
# C-transpile/kcc stages; Python+Sphinx for the docs stage (SKIP otherwise).

param(
    [string]$Stage = "",
    [string[]]$Skip = @()
)

$ErrorActionPreference = "Stop"
$Visible = $env:KARKAIN_QA_VERBOSE

$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$Root = Split-Path -Parent (Split-Path -Parent $ScriptDir)

$Passed = @()
$Failed = $null

function Log([string]$Msg) { Write-Host "[qa] $Msg" -ForegroundColor Cyan }

function Run-Stage {
    param([string]$Name, [scriptblock]$Body)
    if ($Failed) { return }
    if ($Stage -and $Stage -ne $Name) { return }
    if ($Skip -contains $Name) { Log "skip $Name"; return }
    Log ">> stage: $Name"
    $sw = [System.Diagnostics.Stopwatch]::StartNew()
    $code = 0
    & $Body
    $code = $LASTEXITCODE
    if (-not $?) { $code = 1 }
    $sw.Stop()
    if ($code -eq 0) {
        Log "   stage: $Name PASS ($($sw.Elapsed.TotalSeconds.ToString('0.0'))s)"
        $script:Passed += $Name
    } else {
        Log "   stage: $Name FAIL ($($sw.Elapsed.TotalSeconds.ToString('0.0'))s) exit=$code"
        $script:Failed = "$Name (exit $code)"
    }
}

$UnitPkgs = @(
    "./pkg/lexer/...", "./pkg/parser/...", "./pkg/sema/...", "./pkg/ir/...",
    "./pkg/codegen/...", "./pkg/backend/...", "./pkg/npu/...", "./pkg/pm/...",
    "./pkg/module/...", "./pkg/runtime/...", "./pkg/wasm/...",
    "./pkg/compiler/...", "./pkg/source/...", "./pkg/diagnostics/...",
    "./pkg/target/...", "./pkg/lsp/...", "./pkg/bootstrap/..."
)

if (-not $Stage) {
    Run-Stage build  { go build ./... }
    Run-Stage vet    { go vet ./... }
}
Run-Stage units      { go test @UnitPkgs -count=1 -p 1 }
Run-Stage cligates   { go test ./pkg/cli -run "TestPhase114_|TestPhase115_|TestPhase116_|TestPhase117_|TestPhase118_|TestConformanceCorpus_RunsClean|TestProbesCorpus_RunsEveryProbe" -count=1 -p 1 }
Run-Stage conformance{ go test ./pkg/cli -run "TestConformanceCorpus_RunsClean" -count=1 -p 1 }
Run-Stage probes     { go test ./pkg/cli -run "TestProbesCorpus_RunsEveryProbe" -count=1 -p 1 }
Run-Stage examples   {
    if (-not $Failed) {
        $tmp = Join-Path $env:TEMP ("karkain-qa-" + [guid]::NewGuid().ToString("N"))
        New-Item -ItemType Directory -Path $tmp | Out-Null
        go build -o (Join-Path $tmp "karkain.exe") ./cmd/karkain
        if ($LASTEXITCODE -ne 0) { throw "karkain.exe build failed" }
        & powershell -NoProfile -ExecutionPolicy Bypass -File (Join-Path $Root "scripts\verify-examples.ps1") -Karkain (Join-Path $tmp "karkain.exe")
        if ($LASTEXITCODE -ne 0) { throw "verify-examples failed" }
        Remove-Item -LiteralPath $tmp -Recurse -Force
    }
}
Run-Stage docs {
    if (-not $Failed) {
        $py = Get-Command python -ErrorAction SilentlyContinue
        if (-not $py) { Log "docs SKIP (python not found)"; return }
        & python -m sphinx --version 2>$null | Out-Null
        if ($LASTEXITCODE -ne 0) { Log "docs SKIP (sphinx not installed)"; return }
        & python -m sphinx -b html -W --keep-going docs/source docs/build/html 2>&1 | Select-Object -Last 20
        if ($LASTEXITCODE -ne 0) { throw "sphinx html failed" }
        & python -m sphinx -b linkcheck -W docs/source docs/build/linkcheck 2>&1 | Select-Object -Last 20
        if ($LASTEXITCODE -ne 0) { throw "sphinx linkcheck failed" }
    }
}
Run-Stage fresh      {
    if (-not $Failed) {
        & powershell -NoProfile -ExecutionPolicy Bypass -File (Join-Path $Root "scripts\beta-fresh-checkout.ps1")
        if ($LASTEXITCODE -ne 0) { throw "fresh-checkout failed" }
    }
}
Run-Stage install    {
    if (-not $Failed) {
        $prefix = Join-Path $env:TEMP ("karkain-qa-inst-" + [guid]::NewGuid().ToString("N"))
        & powershell -NoProfile -ExecutionPolicy Bypass -File (Join-Path $Root "scripts\install.ps1") -Prefix $prefix
        if ($LASTEXITCODE -ne 0) { throw "install.ps1 failed" }
        & powershell -NoProfile -ExecutionPolicy Bypass -File (Join-Path $Root "scripts\verify-install.ps1") -Prefix $prefix
        if ($LASTEXITCODE -ne 0) { throw "verify-install failed" }
        Remove-Item -LiteralPath $prefix -Recurse -Force -ErrorAction SilentlyContinue
    }
}
Run-Stage journey    {
    if (-not $Failed) {
        & powershell -NoProfile -ExecutionPolicy Bypass -File (Join-Path $Root "scripts\verify-rc-journey.ps1")
        if ($LASTEXITCODE -ne 0) { throw "rc-journey failed" }
    }
}

Write-Host ""
Log "passed: $($Passed -join ', ')"
if ($Failed) {
    Log "FAILED STAGE: $Failed"
    exit 1
}
Log "ALL STAGES GREEN"
exit 0