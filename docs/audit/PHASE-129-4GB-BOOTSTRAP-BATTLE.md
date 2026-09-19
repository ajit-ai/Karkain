# Phase 129 — 4GB Bootstrap Battle

**Status:** COMPLETE (documentation + hardening; live RSA measurement on the
4GB host is verification-pending — see Verification)

**Milestone:** GA-1 — Ship & Harden (Phases 128–129)
**Predecessor:** Phase 127 (bootstrap memory guard `error[K127]`)

## Problem space

The development host is a Windows 11 x86_64 machine with **4,101,260 KB
(3.9 GiB) of physical RAM** and a small system-managed page file. Three
recurring failure classes were traced to this constraint and re-proven
non-regressions in Phases 117–127:

1. **Bootstrap stage 2/3 SEGFAULT** — `TestBootstrap_Stage2SelfHosting` and
   `TestBootstrap_BitwiseIdentity` crash with `exit code 0xc0000005` when the
   native self-hosted compiler (built by kcc from ~214 KB of Karkain source)
   exhausts the 4GB address space mid-transpile. Stage 1 (Go front end) always
   succeeds.
2. **Combined-CLI-gate OOM** — running every CLI phase gate in one `go test
   ./pkg/...` invocation intermittently OOM-stalls a constituent test
   (e.g. `TestPhase107_CodegenSpawnJoin`), even though each gate passes in
   isolation. Causes the "combined-suite flake" class.
3. **Toolchain cannot launch** — when Windows commit charge hits ~100%
   (17.08/17.08 GB commit, ~185–345 MB free physical), even `go.exe` fails to
   start: `The paging file is too small for this operation to complete` /
   `runtime: cannot allocate memory` / `can't allocate more heap arena map`.
   This is the Go toolchain itself OOMing *before* any compile work begins.

The battle is not one fix: it is a layered set of defenses (guard the failure,
make the stall visible, bound CI memory, give the user actionable recovery
steps) plus honest diagnostics so that on small hosts the toolchain fails
**cleanly and explainably** rather than as a SIGSEGV or a hung console.

## Delivered defenses

### 1. Bootstrap memory guard (Phase 127, shipped)

`CheckBootstrapMemory(stage)` in `pkg/bootstrap/memcheck.go` probes available
RAM via `GlobalMemoryStatusEx` (Windows) / `/proc/meminfo` `MemAvailable`
(Linux) at the top of `runCompileStage` for **stages 2/3 only** (stage 1 =
Go front end, always-working, unguarded). Below the default **1.5 GiB** free
it returns a clean `error[K127]` naming the stage and the MiB shortfall —
never a child-process exhaust, never a SEGFAULT. Overrides:
`KARKAIN_BOOTSTRAP_MIN_MEM` (plain bytes or `K/M/G/KiB/MiB/GiB`; `0` disables).
Deterministic unit coverage in `memcheck_test.go`.

### 2. Progress heartbeat (Phase 129, new)

Bootstrap transpile/compile steps routinely take 2–3 minutes (Phase 99 raised
the subprocess timeout 2min→5min because a clean transpile measured
~145–174 s), and `runCmdOutput` buffers the child's output — so on a small
host the console sits silent for minutes and reads as "hung". `startProgress`
(`pkg/bootstrap/bootstrap.go`) spawns a liveness goroutine that prints
`[<label>] still running after <Ns>` every **45 s** until the child exits or
the 5-minute `context.WithTimeout` fires. It is wired into every
`runCmd`/`runCmdOutput` call site (stage-1 `go build`, transpile steps for
stages 1/2/3, and the gcc link steps) with a label derived from the child
binary. Self-terminating on context cancellation; a stall now reads as a
liveness report, and a genuine timeout still yields the deterministic
`context deadline exceeded` error.

### 3. CI memory cap (Phase 129, new)

The `test` job in `.github/workflows/ci.yml` now exports `GOMEMLIMIT: 8GiB`
job-wide (Go ≥1.19; builders run Go 1.21). Combined with the pre-existing
`-p 1` sequential package execution (documented Phase 107 flake fix), the
Go runtime heap is bounded CI-wide so the kcc-build-mode OOM class cannot grow
unmanaged on small runners.

## Windows 11 memory guidance (4GB hosts)

For a 4GB Windows host that runs the Karkain bootstrap/CLI gates, in order of
impact:

1. **Enlarge the page file.** The single most effective change. The failing
   `The paging file is too small` error is Windows commit-charge exhaustion,
   and Phase 129's own measurements showed commit at 17.08/17.08 GB with a
   system-managed page file. Set a fixed size:
   ```powershell
   # Admin PowerShell — set a 16GB fixed page file on C: (restart required)
   $cs = Get-CimInstance Win32_ComputerSystem
   $cs.AutomaticManagedPagefile = $false
   Set-CimInstance -InputObject $cs -Property @{ AutomaticManagedPagefile = $false }
   New-Item -Path 'HKLM:\SYSTEM\CurrentControlSet\Control\Session Manager\Memory Management' -Force | Out-Null
   Set-ItemProperty 'HKLM:\SYSTEM\CurrentControlSet\Control\Session Manager\Memory Management' `
     -Name PagingFiles -Value 'C:\pagefile.sys 16384 16384'
   # Restart, then verify:
   Get-CimInstance Win32_PageFileUsage
   ```
   With a large committed page file, the Go toolchain and kcc can run (paged)
   even with 0.4 GB physical free.
2. **Close idle memory hogs** (typical 4GB-host offenders): browser processes
   (Chrome ~150 MB+ per heavy tab), the OpenCode/editor sessions hosting this
   work, and `MsMpEng.exe` (Windows Defender, ~420 MB). On this host, freeing
   ~1 GB is enough for `go build`/`go vet` to launch.
3. **Run the checks that matter in isolation.** The documented rule from
   Phases 99–127: full-tree `go test ./pkg/...` OOM-mixes at 4GB — every gate
   passes in isolation. Prefer targeted runs:
   ```powershell
   go test ./pkg/bootstrap/ -run 'TestParseMemBytes|TestCheckBootstrapMemory' -count=1
   go test ./pkg/bootstrap/ -run 'TestBootstrap_Stage1GoBuild_Vertical' -count=1   # stage-1 only
   ```
4. **Quick memory status** (used for the Phase 129 grounding measurements):
   ```powershell
   $os = Get-CimInstance Win32_OperatingSystem
   "MEM total={0}GB free={1}GB" -f [math]::Round($os.TotalVisibleMemorySize/1MB,1), [math]::Round($os.FreePhysicalMemory/1MB,1)
   Get-CimInstance Win32_PageFileUsage | Select-Object Name,CurrentUsage,PeakUsage
   ```

## kcc build RSS measurement plan (verification-pending)

To target the stage-2/3 SEGFAULT root cause (emit a reduced-kcc build under
~2.5 GB peak RSS) we first must *measure* peak RSS per stage. On this host the
build cannot currently launch; on a healthy host (or CI, `-p 1`) the owner
should:

1. Patch `KARKAIN_BOOTSTRAP_MIN_MEM=0` (disable the guard) in the run env.
2. Run only `TestBootstrap_Stage2SelfHosting` in isolation.
3. Sample RSS during the transpile via a 1-second poll loop while the test runs:
   ```powershell
   while ($true) { Get-Process karkain-compiler1,kcc,cc1,gcc -ErrorAction SilentlyContinue | Select-Object Name,@{n='RSS_MB';e={[math]::Round($_.WorkingSet64/1MB)}}; Start-Sleep -Seconds 1 }
   ```
4. Record the peak `WorkingSet64` for the kcc compiler process and the gcc/cc1
   process separately; record it in this report (table below).

Target: **stage-2/3 total peak RSS ≤ 2.5 GB** (compiler + cc1 together) with
a 1.5 GiB free-memory guard, leaving headroom under the 3.9 GB ceiling.

| Stage | process | peak RSS (MB) | host | date |
|-------|---------|---------------|------|------|
| 2     | (measure) | — | — | — |
| 2     | cc1 | — | — | — |
| 3     | (measure) | — | — | — |
| 3     | cc1 | — | — | — |

If RSS exceeds target, the levers are: `GOGC` on the kcc process, `-O0`/`-x%
c` flags already defaulted, splitting the C translation unit, or raising the
guard threshold. This remains open.

## Verification

- **This host (live, 2026-09-19):** memory freed transiently (0.6 GB free) and
  the toolchain launched. Results confirm the defense end-to-end:
  - `go vet ./pkg/bootstrap/` — **PASS**.
  - `go test ./pkg/bootstrap/ -run 'TestParseMemBytes|TestCheckBootstrapMemory' -count=1`
    — **PASS** (3/3). Deterministic memcheck gates are host-independent.
  - `go test ./pkg/bootstrap/ -run 'TestArgs_|TestBootstrap_Stage1Compilation' -count=1`
    — **PASS** (7/7). Stage-1 Go front end + transpile + gcc succeeded in
    18.7 s; the heartbeat wiring did not disturb the pipeline.
  - `go test ./pkg/bootstrap/ -run 'TestBootstrap_Stage2SelfHosting' -count=1`
    — **clean `error[K127]` abort**: `cannot run bootstrap stage 2: only 642
    MiB memory available, 1536 MiB required: insufficient memory for bootstrap
    stage (close memory-heavy apps, raise the page file, or set
    KARKAIN_BOOTSTRAP_MIN_MEM)`. **This replaces the historical
    `0xc0000005` SEGFAULT class** with an actionable diagnostic — the battle's
    core goal. Stage 2's post-guard behavior on a ≥1.5 GiB-free host remains to
    be re-verified (open item; apply the page-file guidance).
- **Static checks:** `gofmt -e` parses all `pkg/bootstrap` files without syntax
  errors (remaining `gofmt -l` flags are the repo-wide CRLF artifact, confirmed
  identical for committed files like `args_test.go`).
- **CI:** `GOMEMLIMIT` change is declarative; the workflow lints itself at
  parse time on push.

## Related records

- `docs/audit/PHASE-127-BOOTSTRAP-MEMORY-GUARD.md` — the memory guard itself.
- `docs/audit/GENERAL-AVAILABILITY-ROADMAP.md` — GA-1/2/3 milestone plan.
- `.github/workflows/ci.yml` — `-p 1` (Phase 107 flake fix) + `GOMEMLIMIT`
  (Phase 129).