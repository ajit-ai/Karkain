# Phase 127 — Bootstrap Memory Guard (low-RAM OOM/SEGFAULT class)

**Verdict**: IMPLEMENTED — verification pending on the low-RAM host (see
"Verification" section). No regressions expected: the guard is additive and
only fires when available system memory drops below a threshold.

## Problem Statement

On ~4GB-RAM hosts, the self-hosted bootstrap stages crash:

1. **`TestBootstrap_Stage2SelfHosting` / `TestBootstrap_BitwiseIdentity`** —
   SEGFAULT (exit 0xc0000005) during stage-2/3 *native* kcc self-build. The
   native compiler process building the full `src/compiler` tree exhausts the
   address space; gcc has no room to link. Stage 1 (Go front end) always
   succeeds, so the crash class is exactly the native self-hosting stages.
2. **Combined-CLI-gate OOM** — `go test ./pkg/...` under full concurrency
   stalls even though every constituent gate passes individually; the Go build
   toolchain itself also OOMs (observed: `fatal error: runtime: cannot allocate
   memory` / `out of memory allocating heap arena map`) when free memory is
   ~200 MB and Windows commit charge is at 100%.

Both are **environmental, not defects** — but a "clean, actionable error"
contract is far better than a crash. That is this phase's deliverable.

## Implementation

New dependency-free memory probe + guard in `pkg/bootstrap`:

| File | Purpose |
|------|---------|
| `memcheck.go` | `CheckBootstrapMemory(stage)` — threshold logic, `error[K127]` message, `parseMemBytes` for `KARKAIN_BOOTSTRAP_MIN_MEM`, default 1.5 GiB minimum |
| `memcheck_windows.go` | `platformAvailableRAM()` via `GlobalMemoryStatusEx` (stdlib `syscall` only, no cgo) |
| `memcheck_linux.go` | `platformAvailableRAM()` by parsing `MemAvailable` from `/proc/meminfo` |
| `memcheck_other.go` | Probe absent → guard disabled (no guessing about unobservable memory) |
| `memcheck_test.go` | Deterministic tests (injected 512 MiB probe; no dependence on host RAM) |
| `bootstrap.go` | `CheckBootstrapMemory(stage)` runs at the top of `runCompileStage` (stages 2/3 only) |

Design decisions:

- **Stage 1 is NOT guarded**: it drives the Go front end (light, definitionally
  working). Stages 2/3 run the native self-hosted compiler — those are the
  OOM class.
- **Default threshold 1.5 GiB**: the native compiler approaches ~2 GB RSS plus
  ~500 MB gcc; 1.5 GiB headroom is effective on 4 GB hosts while healthy
  CI/host machines (typically > 1.5 GiB free) pass through.
- **`KARKAIN_BOOTSTRAP_MIN_MEM`** env override: plain bytes or K/M/G/KiB/MiB/GiB
  suffix; `0` disables the guard. Invalid values are reported, never swallowed.
- **No new dependencies**: stdlib `syscall` + `unsafe` on Windows, `bufio` etc.
  on Linux.
- The guard converts the SEGFAULT into:
  ```
  error[K127]: cannot run bootstrap stage 2: only 203 MiB memory available, 1536 MiB required: ...
  ```

## Related Observations (this phase's analysis)

- Existing 124-A (subprocess 5-min timeout) was **already present**
  (`runCmd`/`runCmdOutput` in `bootstrap.go`).
- Existing 124-C (CI `-p 1`) was **already present** in `.github/workflows/ci.yml`.
- `pkg/parser/parser.go` was found **emptied (0 bytes)** in the working tree
  (WIP accident alongside the in-flight `ClosureExpr` work in
  `arena.go`/`ast.go`/`captures.go`), which broke the whole build; restored from
  HEAD. The WIP closure AST additions are additive and compile against the
  restored parser.
- Phase 124 in the repo is actually the **compute-target model** (GPU/NPU/quantum,
  commits `a5f762e`/`0234bb2`) — it was never recorded in AGENTS.md; recorded
  there as part of this phase's bookkeeping.

## Verification (run on enough RAM)

```
go build ./pkg/bootstrap/... ./cmd/karkain/...
go vet ./pkg/bootstrap/...
go test ./pkg/bootstrap/ -run 'TestParseMemBytes|TestCheckBootstrapMemory' -count=1
```

On a healthy host the existing bootstrap gates keep passing unchanged
(`go test ./pkg/bootstrap/ -run TestBootstrap`). On the 4 GB host
`TestBootstrap_Stage2SelfHosting`/`BitwiseIdentity` now fail with a clean
`error[K127]` (when free RAM < 1.5 GiB) instead of a 0xc0000005 SEGFAULT;
lowering the guard via `KARKAIN_BOOTSTRAP_MIN_MEM` lets them proceed if enough
headroom exists.

Note: the Go toolchain itself may OOM at < 500 MB free even for `go build` of a
single package; free ~1 GB (close browsers, cap OpenCode sessions) before
verifying on this host.

## Out of Scope

- The hard fix (making kcc stage-2/3 build within 4 GB) is a later-phase
  compiler-memory project.
- Combined-suite test parallelism tuning for `go test ./pkg/...` (initiative
  `-p 1`/`GOMEMLIMIT`) is a CI concern, not a code defect, and CI already runs
  `-p 1`.

## Deliverables

- `pkg/bootstrap/memcheck.go`, `memcheck_windows.go`, `memcheck_linux.go`,
  `memcheck_other.go`, `memcheck_test.go`
- `pkg/bootstrap/bootstrap.go` guard hook in `runCompileStage`
- AGENTS.md: Phase 124 (compute-target) + Phase 127 records
- Phase 127+ post-GA roadmap: `docs/audit/PHASE-127-POST-GA-ROADMAP.md`