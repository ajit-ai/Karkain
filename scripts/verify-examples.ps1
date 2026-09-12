# verify-examples.ps1 - Smoke-verify the Phase 114 example corpus.
#
# Classifies each .kark example by its `// Status:` and `// Engine:` header
# comment and runs it through the real karkain CLI:
#   Runnable + engine both -> default (kcc) engine
#   Runnable + engine go    -> --engine go
#   Experimental            -> --engine go
#   Planned / test files    -> SKIP
#
# Exit code: 0 when no runnable example failed; 1 otherwise.
#
# Usage:
#   powershell -ExecutionPolicy Bypass -File scripts\verify-examples.ps1
#   powershell -ExecutionPolicy Bypass -File scripts\verify-examples.ps1 -Karkain .\karkain.exe

param(
    [string]$Karkain = ".\karkain.exe",
    [string]$ExamplesDir = "examples",
    [switch]$Quiet
)

$ErrorActionPreference = "Stop"
if (-not (Test-Path -LiteralPath $Karkain)) {
    throw "karkain executable not found at '$Karkain' (pass -Karkain <path>)"
}

$pass = 0
$fail = 0
$skip = 0
$failures = [System.Collections.Generic.List[string]]::new()

function Invoke-Example([string]$kark, [string]$engine, [string]$rel) {
    $args = @("run", $kark, "--engine", $engine)
    $out = & $Karkain @args 2>&1 | Out-String
    $code = $LASTEXITCODE
    if ($code -ne 0) {
        $failures.Add("$rel (engine=$engine) exit=$code`n$out")
        return 1
    }
    if (-not $Quiet) {
        Write-Output "PASS $rel (engine=$engine)"
    }
    return 0
}

Get-ChildItem -LiteralPath $ExamplesDir -Directory | Where-Object { $_.Name -match '^\d{2}-' } |
    Sort-Object Name |
    ForEach-Object {
        $cat = $_
        $files = @(Get-ChildItem -LiteralPath $cat.FullName -Filter *.kark | Sort-Object Name)
        if ($files.Count -eq 0) {
            Write-Output "SKIP $($cat.Name) (planned category, README only)"
            $skip++
        }
        foreach ($f in $files) {
            $rel = "$($cat.Name)/$($f.Name)"
            $lines = Get-Content -LiteralPath $f.FullName -Encoding UTF8
            $status = "Runnable"
            $engine = "both"
            foreach ($line in $lines) {
                if ($line -match '^//\s*Status:\s*(Runnable|Experimental|Planned)') {
                    $status = $Matches[1]
                }
                if ($line -match '^//\s*Engine:\s*(\S+)') {
                    $engine = $Matches[1]
                }
            }
            if ($f.Name -match '_test\.kark$') {
                Write-Output "SKIP $rel (test-mode: run via 'karkain test')"
                $skip++
                continue
            }
            if ($status -eq "Planned") {
                Write-Output "SKIP $rel (planned)"
                $skip++
                continue
            }
            if ($status -eq "Experimental" -or $engine -eq "go") {
                if ((Invoke-Example $f.FullName "go" $rel) -eq 1) { $fail++ } else { $pass++ }
            } else {
                if ((Invoke-Example $f.FullName "kcc" $rel) -eq 1) { $fail++ } else { $pass++ }
            }
        }
    }

Write-Output ""
Write-Output "verify-examples summary: $pass passed, $fail failed, $skip skipped"
if ($fail -gt 0) {
    Write-Output ""
    Write-Output "Failures:"
    $failures | ForEach-Object { Write-Output $_ }
    exit 1
}
exit 0