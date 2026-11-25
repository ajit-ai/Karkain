# run-full-qa.ps1 - Karkain v1.2.x master QA gate (v1.2.16 closeout)
#
# Orchestrates the existing test machinery (no duplicated suites). Stages run
# in order and abort at the first failure; the failing stage is printed last.
# Exit code 0 = all selected stages green. This script is the SOURCE of the
# QA battery, not a claim that every stage passed on this host: v1.2.16 was
# measured stage-by-stage and the exact PASS/SKIP/blocked verdicts are in
# docs/audit/V1.2.16-QA-BATTERY.md.
#
# Stages:
#   build      go build ./...
#   vet        go vet ./...
#   units      go test over all non-CLI packages (-count=1 -p 1 -timeout 20m)
#   cli114     TestPhase114_ (BOTH engines; -timeout 30m, measured ~1553 s)
#   cli115     TestPhase115_ (-timeout 20m)
#   cli116     TestPhase116_ (-timeout 20m)
#   cli117     TestPhase117_ (-timeout 20m)
#   cli118     TestPhase118_ (-timeout 20m)
#   cli119     TestPhase119_ (-timeout 20m)
#   conformance conformance corpus (Go gate)
#   probes     probes corpus (Go gate)
#   examples   build karkain.exe into a temp dir, run verify-examples.ps1
#   docs       Sphinx html -W --keep-going + linkcheck -W (SKIPs w/o python)
#   fresh      scripts/beta-fresh-checkout.ps1 (fresh-checkout gate)
#   install    scripts/install.ps1 + scripts/verify-install.ps1 (temp prefix)
#   journey    scripts/verify-rc-journey.ps1 (11-step external journey)
#
# The former monolithic `cligates` stage is REPLACED, not renamed. On this
# host the bundle ended as `Exit code: 143 (SIGTERM)` from the 590 s shell cap,
# and the isolated families are what carry evidence. Heavy gates are never
# combined: CI already runs each family as its own step, and the local runner
# now does the same. `-Stage cligates` is an explicit error rather than a
# silent no-op.
#
# Usage (one long family per invocation on capacity-limited hosts):
#   powershell -ExecutionPolicy Bypass -File scripts\qa\run-full-qa.ps1 -Stage build
#   powershell -ExecutionPolicy Bypass -File scripts\qa\run-full-qa.ps1 -Stage cli114
#   powershell -ExecutionPolicy Bypass -File scripts\qa\run-full-qa.ps1 -Stage conformance
#   powershell -ExecutionPolicy Bypass -File scripts\qa\run-full-qa.ps1 -Skip docs,fresh
#
# Prerequisites: Go toolchain, C compiler (gcc/clang) on PATH for the
# C-transpile/kcc stages; Python+Sphinx for the docs stage (SKIP otherwise).
# The PATH prelude below adds Git, WindowsPowerShell and MinGW when present;
# without it this host reports `git: executable file not found in %PATH%`
# and `no cross-linker available for target 'x86_64-windows'`.

param(
    [string]$Stage = "",
    [string[]]$Skip = @()
)

$ErrorActionPreference = "Stop"
$Visible = $env:KARKAIN_QA_VERBOSE

if ($Stage -eq "cligates") {
    Write-Host "[qa] -Stage cligates was replaced by cli114..cli119 plus conformance/probes; run one family at a time (see the header)." -ForegroundColor Red
    exit 2
}

$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$Root = Split-Path -Parent (Split-Path -Parent $ScriptDir)

$Passed = @()
$Failed = $null

function Log([string]$Msg) { Write-Host "[qa] $Msg" -ForegroundColor Cyan }

# v1.2.16 PATH prelude. Measured on this host: Git, WindowsPowerShell and
# MinGW all exist, but the default PATH exposed none of them to the QA
# children, so pm tests failed with `git ... executable file not found` and
# the CLI gates failed with `no cross-linker available`. Add each directory
# only when it exists and only once, so a machine with a different toolchain
# layout is not given a hard-coded prerequisite.
function Add-QAPath([string]$Path) {
    if ([string]::IsNullOrWhiteSpace($Path)) { return }
    if (-not (Test-Path -LiteralPath $Path)) { return }
    if (($env:PATH -split ';') -contains $Path) { return }
    $env:PATH = "$Path;$env:PATH"
    Log "PATH += $Path"
}
Add-QAPath 'C:\Program Files\Git\cmd'
Add-QAPath 'C:\Windows\System32\WindowsPowerShell\v1.0'
Add-QAPath 'C:\msys64\mingw64\bin'

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

# build and vet are deliberately NOT wrapped in `if (-not $Stage)`: Run-Stage
# already filters by -Stage, and the old wrapper made `-Stage build` and
# `-Stage vet` report "ALL STAGES GREEN" without executing anything.
Run-Stage build     { go build ./... }
Run-Stage vet       { go vet ./... }
Run-Stage units      { go test @UnitPkgs -count=1 -p 1 -timeout 20m }
# CLI families are separate stages, never one monolithic bundle. The 590 s
# shell cap is a harness limit, not a compiler result: the full Phase-114
# family was measured at 1553 s, so a combined invocation cannot complete
# here. CI already executes these as separate steps.
Run-Stage cli114     { go test ./pkg/cli -run "^TestPhase114_" -count=1 -p 1 -timeout 30m }
Run-Stage cli115     { go test ./pkg/cli -run "^TestPhase115_" -count=1 -p 1 -timeout 20m }
Run-Stage cli116     { go test ./pkg/cli -run "^TestPhase116_" -count=1 -p 1 -timeout 20m }
Run-Stage cli117     { go test ./pkg/cli -run "^TestPhase117_" -count=1 -p 1 -timeout 20m }
Run-Stage cli118     { go test ./pkg/cli -run "^TestPhase118_" -count=1 -p 1 -timeout 20m }
Run-Stage cli119     { go test ./pkg/cli -run "^TestPhase119_" -count=1 -p 1 -timeout 20m }
Run-Stage conformance{ go test ./pkg/cli -run "^TestConformanceCorpus_RunsClean$" -count=1 -p 1 -timeout 20m }
Run-Stage probes     { go test ./pkg/cli -run "^TestProbesCorpus_RunsEveryProbe$" -count=1 -p 1 -timeout 20m }
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