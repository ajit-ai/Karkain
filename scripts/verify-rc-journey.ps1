# Karkain external-developer journey simulation (Windows / PowerShell 5.1+)
#
# Simulates a brand-new developer: fresh source build, then the complete
# documented journey (discover -> install/build -> help -> first program ->
# project -> workspace -> test -> debug -> profile -> examples -> docs).
#
# This is Phase 118 section 35. It intentionally does NOT depend on previous
# build artifacts: everything is built into a fresh scratch directory.
#
# Exit codes:
#   0  all journey steps passed
#   1  one or more journey steps failed (details printed per step)

$ErrorActionPreference = "Stop"

$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$ProjectRoot = Split-Path -Parent $ScriptDir

$pass = 0
$fail = 0
$failures = New-Object System.Collections.Generic.List[string]

function Step([string]$Name, [scriptblock]$Body) {
    try {
        & $Body
        $script:pass++
        Write-Host "PASS $Name" -ForegroundColor Green
    }
    catch {
        $script:fail++
        $msg = $_.Exception.Message
        $failures.Add("$Name : $msg")
        Write-Host "FAIL $Name : $msg" -ForegroundColor Red
    }
}

function AssertTrue([object]$Cond, [string]$Msg) {
    $ok = $false
    if ($Cond -is [array]) {
        $ok = ($Cond.Count -gt 0)
    } else {
        $ok = [bool]$Cond
    }
    if (-not $ok) {
        throw $Msg
    }
}

$scratch = Join-Path $env:TEMP ("karkain-journey-" + [guid]::NewGuid().ToString("N"))
New-Item -ItemType Directory -Path $scratch -Force | Out-Null
$bin = Join-Path $scratch "karkain.exe"
$oldEngine = $env:KARKAIN_ENGINE
$env:KARKAIN_ENGINE = "go"

Write-Host "Karkain external-developer journey" -ForegroundColor Cyan
Write-Host "Scratch: $scratch" -ForegroundColor Yellow
Write-Host "Engine: go (deterministic; kcc parity is gated separately)" -ForegroundColor Yellow

try {
    # 1. fresh build from source
    Step "fresh source build" {
        $out = & go build -o $bin ./cmd/karkain 2>&1
        AssertTrue ($LASTEXITCODE -eq 0) "go build failed: $out"
        AssertTrue (Test-Path -LiteralPath $bin) "binary not produced at $bin"
    }

    # 2. version identity
    Step "--version identifies Beta 1" {
        $out = & $bin --version 2>&1
        AssertTrue ($LASTEXITCODE -eq 0) "--version failed: $out"
        AssertTrue ($out -match "Karkain Compiler v0\.117\.0" -and $out -match "Beta 1 Build") "unexpected --version: $out"
    }

    # 3. help coverage
    Step "--help covers the CLI contract" {
        $out = (& $bin --help 2>&1) -join "`n"
        AssertTrue ($LASTEXITCODE -eq 0) "--help failed"
        foreach ($c in @("run", "build", "check", "test", "prof", "lint", "explain", "fmt", "clean", "target", "config", "workspace", "init")) {
            AssertTrue ($out -match "\b$c\b") "--help missing command: $c"
        }
        AssertTrue ($out -notmatch "Developer Preview") "--help still mentions Developer Preview"
    }

    # 4. first program: hello.kark -> check -> build -> run
    Step "first program hello.kark" {
        $hello = Join-Path $scratch "hello.kark"
        Set-Content -Path $hello -Value "func main() {`n    print(`"Hello, Karkain!`")`n}" -Encoding ASCII
        & $bin check $hello 2>&1 | Out-Null
        AssertTrue ($LASTEXITCODE -eq 0) "karkain check hello.kark failed (exit $LASTEXITCODE)"
        & $bin build $hello 2>&1 | Out-Null
        AssertTrue ($LASTEXITCODE -eq 0) "karkain build hello.kark failed (exit $LASTEXITCODE)"
        $out = & $bin run $hello 2>&1
        AssertTrue ($LASTEXITCODE -eq 0) "karkain run hello.kark failed (exit $LASTEXITCODE)"
        AssertTrue ($out -match "Hello, Karkain!") "unexpected run output: $out"
    }

    # 5. multi-file project: main.kark + math.kark (sibling module)
    Step "multi-file project (module)" {
        $proj = Join-Path $scratch "project"
        New-Item -ItemType Directory -Path $proj -Force | Out-Null
        Set-Content -Path (Join-Path $proj "math.kark") -Value "public func twice(n) {`n    return n * 2`n}" -Encoding ASCII
        Set-Content -Path (Join-Path $proj "main.kark") -Value "import math`n`nfunc main() {`n    print(math.twice(21))`n}" -Encoding ASCII
        & $bin check (Join-Path $proj "main.kark") 2>&1 | Out-Null
        AssertTrue ($LASTEXITCODE -eq 0) "check failed (exit $LASTEXITCODE)"
        $out = & $bin run (Join-Path $proj "main.kark") 2>&1
        AssertTrue ($LASTEXITCODE -eq 0) "run failed (exit $LASTEXITCODE)"
        AssertTrue ($out -match "42") "expected 42, got: $out"
    }

    # 6. workspace dependency: app consumes sibling library
    Step "workspace app consumes sibling library" {
        $ws = Join-Path $scratch "ws"
        Copy-Item (Join-Path $ProjectRoot "examples\workspace") $ws -Recurse -Force
        Push-Location $ws
        try {
            & $bin workspace build 2>&1 | Out-Null
            AssertTrue ($LASTEXITCODE -eq 0) "workspace build failed (exit $LASTEXITCODE)"
            $out = & $bin workspace run 2>&1
            AssertTrue ($LASTEXITCODE -eq 0) "workspace run failed (exit $LASTEXITCODE): $out"
            AssertTrue ($out -match "hi from library api") "library api output missing: $out"
            AssertTrue ($out -match "hi from app") "app output missing: $out"
        }
        finally {
            Pop-Location
        }
    }

    # 7. native tests
    Step "karkain test" {
        $tf = Join-Path $ProjectRoot "examples\15-developer-tools\02_assertions_test.kark"
        $out = & $bin test $tf 2>&1
        AssertTrue ($LASTEXITCODE -eq 0) "karkain test failed (exit $LASTEXITCODE): $out"
        AssertTrue ($out -match "passed") "no pass summary in output: $out"
    }

    # 8. debug build
    Step "debug build (--debug)" {
        $hello = Join-Path $scratch "hello.kark"
        & $bin build $hello --debug -o (Join-Path $scratch "hello.debug.exe") 2>&1 | Out-Null
        AssertTrue ($LASTEXITCODE -eq 0) "karkain build --debug failed (exit $LASTEXITCODE)"
    }

    # 9. profiling
    Step "karkain prof" {
        $hello = Join-Path $scratch "hello.kark"
        $out = & $bin prof $hello 2>&1
        AssertTrue ($LASTEXITCODE -eq 0) "karkain prof failed (exit $LASTEXITCODE): $out"
    }

    # 10. selected examples (goldens)
    Step "example corpus spot checks" {
        $h = Join-Path $ProjectRoot "examples\01-fundamentals\01_hello_world.kark"
        $outH = & $bin run $h 2>&1
        AssertTrue ($LASTEXITCODE -eq 0) "hello example failed (exit $LASTEXITCODE)"
        AssertTrue ($outH -match "Hello, Karkain!") "hello example output: $outH"
        $ms = Join-Path $ProjectRoot "examples\module_system\main.kark"
        $outM = & $bin run $ms 2>&1
        AssertTrue ($LASTEXITCODE -eq 0) "module_system example failed (exit $LASTEXITCODE): $outM"
        AssertTrue ($outM -match "42") "module_system expected 42, got: $outM"
        AssertTrue ($outM -match "9") "module_system expected 9, got: $outM"
    }

    # 11. documentation + issue/security metadata
    Step "docs, issue and security metadata present" {
        $expect = @(
            "SECURITY.md",
            "docs\source\status\compatibility.rst",
            "docs\source\status\migration-beta1.rst",
            "docs\source\status\scope.rst",
            "docs\source\status\rc-checklist.rst",
            "docs\source\reference\stable-api.rst",
            "docs\source\development\feature-freeze.rst",
            "docs\source\development\release.rst",
            "docs\source\development\reporting-bugs.rst",
            "docs\source\getting-started\first-project.rst",
            "docs\source\getting-started\workspace.rst",
            ".github\ISSUE_TEMPLATE\bug_report.yml",
            "examples\workspace\README.md"
        )
        foreach ($f in $expect) {
            AssertTrue (Test-Path -LiteralPath (Join-Path $ProjectRoot $f)) "missing $f"
        }
    }
}
finally {
    $env:KARKAIN_ENGINE = $oldEngine
}

Write-Host ""
Write-Host "Journey result: $pass passed, $fail failed" -ForegroundColor Cyan
if ($fail -gt 0) {
    Write-Host "Failed steps:" -ForegroundColor Red
    $failures | ForEach-Object { Write-Host "  - $_" -ForegroundColor Red }
    Remove-Item $scratch -Recurse -Force -ErrorAction SilentlyContinue
    exit 1
}
Write-Host "External developer journey: PASS" -ForegroundColor Green
Remove-Item $scratch -Recurse -Force -ErrorAction SilentlyContinue
exit 0