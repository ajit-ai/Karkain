# run-full-qa.ps1 - Karkain 1.2.0 master QA gate
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
#              (monolithic gcc-gated; -p 1 to avoid the host's OOM class).
#              The stage's -timeout is 90m because it bundles EIGHT gate
#              families and the Phase-114 corpus alone runs in BOTH engines:
#              measured 1925 s for the bundle before the version-flip release,
#              and the bundle exceeded Go's 10-minute default and then a
#              40-minute one on this ~4 GB host. A timeout here is a HARNESS
#              artifact, not a regression -- the CI runs these gates as
#              separate steps with their own per-step timeouts.
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

# Best-effort scratch-directory cleanup. See scripts\install.ps1 for why this
# is not `Remove-Item`: on some hosts Remove-Item cannot delete a DIRECTORY
# under %TEMP% (measured here: %TEMP% = C:\Windows\Temp, "Access is denied" on
# an empty directory -- the FILES inside delete fine, the directory does not),
# and a cleanup failure must not fail a stage whose work already succeeded.
function Remove-ScratchDir([string]$Path) {
    if ([string]::IsNullOrWhiteSpace($Path)) { return }
    if (-not (Test-Path -LiteralPath $Path)) { return }
    for ($i = 0; $i -lt 5; $i++) {
        try {
            [System.IO.Directory]::Delete($Path, $true)
            return
        } catch {
            Start-Sleep -Milliseconds 400
        }
    }
    try { cmd /c rmdir /s /q "$Path" 2>&1 | Out-Null } catch { }
}

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
Run-Stage cligates   { go test ./pkg/cli -run "TestPhase114_|TestPhase115_|TestPhase116_|TestPhase117_|TestPhase118_|TestPhase119_|TestConformanceCorpus_RunsClean|TestProbesCorpus_RunsEveryProbe" -count=1 -p 1 -timeout 90m }
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
        Remove-ScratchDir $tmp
    }
}
Run-Stage docs {
    if (-not $Failed) {
        $py = Get-Command python -ErrorAction SilentlyContinue
        if (-not $py) { Log "docs SKIP (python not found)"; return }
        # PS 5.1 escalates a NATIVE command's stderr into a TERMINATING error
        # when $ErrorActionPreference is "Stop" -- the same trap the
        # fresh-checkout script documents. The probe for sphinx therefore
        # KILLED the whole battery instead of skipping the stage, and a
        # sphinx run that warns (which is what -W is for) would have done
        # the same. The exit code is the authority here, so relax the
        # preference around every python invocation and restore it after.
        $prevEAP = $ErrorActionPreference
        $ErrorActionPreference = "Continue"
        & python -m sphinx --version 2>&1 | Out-Null
        $sphinxExit = $LASTEXITCODE
        $ErrorActionPreference = $prevEAP
        if ($sphinxExit -ne 0) {
            Log "docs SKIP (sphinx not installed)"
            # The stage runner reads $LASTEXITCODE, and the probe above left
            # it at the probe's own non-zero code -- so a DELIBERATE skip was
            # reported as a stage FAILURE. Resetting it is the idiomatic way
            # to say "the last native command succeeded", and it is what makes
            # the skip path work at all on a host that has python but not
            # sphinx.
            $global:LASTEXITCODE = 0
            return
        }
        $ErrorActionPreference = "Continue"
        & python -m sphinx -b html -W --keep-going docs/source docs/build/html 2>&1 | Select-Object -Last 20
        $htmlExit = $LASTEXITCODE
        & python -m sphinx -b linkcheck -W docs/source docs/build/linkcheck 2>&1 | Select-Object -Last 20
        $linkExit = $LASTEXITCODE
        $ErrorActionPreference = $prevEAP
        if ($htmlExit -ne 0) { throw "sphinx html failed" }
        if ($linkExit -ne 0) { throw "sphinx linkcheck failed" }
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
        Remove-ScratchDir $prefix
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