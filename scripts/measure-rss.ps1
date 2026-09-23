# Measures peak working-set RSS of a command and its child processes.
#
# Phase 141: kcc self-build memory table. Usage:
#   pwsh scripts/measure-rss.ps1 -- <command> [args...]
# Example:
#   pwsh scripts/measure-rss.ps1 -- ./karkain.exe check --engine kcc src/compiler/main.kark
#
# Samples every 500 ms: for the child process tree rooted at <command>,
# sums WorkingSet64 and tracks the peak. Prints PEAK_RSS_MB=<n> at the end.
# Child processes that exit between polls are simply missed for that poll;
# for minute-scale builds the sampling error is negligible.

$ErrorActionPreference = "Stop"

if ($args.Count -lt 2 -or $args[0] -ne "--") {
    Write-Host "usage: measure-rss.ps1 -- <command> [args...]"
    exit 2
}

$exe = $args[1]
$rest = @()
if ($args.Count -gt 2) { $rest = $args[2..($args.Count - 1)] }

$proc = Start-Process -FilePath $exe -ArgumentList $rest -NoNewWindow -PassThru
$rootId = $proc.Id
$peak = 0

function TreeRss($parentId, $seen, $depth) {
    # $seen guards against PID-reuse cycles (a dead child's PID recycled
    # into an unrelated subtree would otherwise recurse forever); $depth
    # is a second backstop.
    if ($depth -gt 32 -or $seen.Contains($parentId)) { return 0 }
    [void]$seen.Add($parentId)
    $sum = 0
    try {
        $kids = Get-CimInstance Win32_Process -Filter "ParentProcessId = $parentId" -ErrorAction SilentlyContinue
    } catch { return 0 }
    foreach ($k in $kids) {
        try {
            $p = Get-Process -Id $k.ProcessId -ErrorAction SilentlyContinue
            if ($p) { $sum += $p.WorkingSet64 }
        } catch { }
        $sum += TreeRss $k.ProcessId $seen ($depth + 1)
    }
    return $sum
}

while (-not $proc.HasExited) {
    Start-Sleep -Milliseconds 500
    try {
        $p = Get-Process -Id $rootId -ErrorAction SilentlyContinue
        $rss = 0
        if ($p) { $rss = $p.WorkingSet64 }
        $seen = New-Object System.Collections.Generic.HashSet[int]
        $rss += TreeRss $rootId $seen 0
        if ($rss -gt $peak) { $peak = $rss }
    } catch { }
}

$mb = [math]::Round($peak / 1MB, 1)
Write-Host "PEAK_RSS_MB=$mb"
