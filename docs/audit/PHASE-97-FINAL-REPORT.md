# Phase 97 — Default kcc Engine + Manifest Dependency Resolution

**Status:** ✅ Complete
**Date:** 2026-09-08
**Branch:** `develop` → merged into `main`, pushed to origin

## Objective

Per `C:\Users\Lenovo\Downloads\Phase 97 Implementation Prompt.md`:

1. Make the self-hosted compiler (**kcc**) the **default** engine, with the Go
   front end available only via explicit selection.
2. Wire manifest dependency resolution (`pm.DependencySources`) into the
   source-assembly pipeline so `check`/`build`/`run`/`test` compile the full
   project (dependencies + siblings + root).
3. Add focused `pkg/cli/phase97_*_test.go` tests.
4. Preserve existing compiler/runtime/package-manager/LSP behavior.
5. Run regressions, fix regressions, commit on `develop`, merge to `main`,
   push both branches.

## Files Changed

| File | Change |
|------|--------|
| `pkg/cli/kcc_engine.go` | `EngineFromEnv` now returns `EngineKCC` for empty env and any env value other than `go\|Go\|GO` (explicit `kcc` still → kcc, `go\|Go\|GO` → Go). New `kccAssembleSource` (local deps → siblings → root last) feeds `KCCCheckCommand`/`KCCBuildCommand`/`KCCRunCommand`; `KCCTestCommand` prepends `projectModuleSources` via new `kccTestSource`/`kccMirrorTestDir`. |
| `pkg/cli/commands.go` | `resolveSources` (check/build/run) and `projectModuleSources` (test) now pull `pm.DependencySources` entries (local & workspace) into the assembled source. |
| `pkg/cli/phase97_parity_test.go` | **New.** 6 focused tests (see Tests). |
| `pkg/cli/phase95_parity_test.go` | `TestPhase95_EngineSelection` updated: empty env → `EngineKCC`; `KARKAIN_ENGINE=go` → `EngineGo`. |
| `pkg/cli/exitcodes_test.go` | `TestCLI_ExitCodes_E2E` and `TestCLI_TestFailure_ExitCode_E2E` pin `KARKAIN_ENGINE=go` so they keep asserting the Go engine's exit-code contract deterministically. |
| `pkg/bootstrap/bootstrap.go` | New `forceGoEngine` helper; `runCmdOutput` pins `KARKAIN_ENGINE=go` so stage-1 (Go front end) still emits `src/compiler/main.c` instead of routing into kcc after the default flip. |
| `AGENTS.md` | "Current phase: post-96" → "post-97" + Phase 97 summary block. |
| `docs/ROADMAP-PRODUCTION.md` | Phase 97 row marked re-purposed & complete with delivered summary. |
| `docs/audit/PHASE-97-FINAL-REPORT.md` | **New.** This report. |

## Implementation Summary

### Engine default flip

`EngineFromEnv()` in `pkg/cli/kcc_engine.go:EngineFromEnv`:

- empty `KARKAIN_ENGINE` → `EngineKCC`
- any value other than `go|Go|GO` (e.g. `kcc`, or unrelated) → `EngineKCC`
- `go|Go|GO` → `EngineGo`
- `--engine go|kcc` (`EngineFlag`) unchanged.

### Manifest dependency resolution in source assembly

- **check/build/run:** `resolveSources(file)` now returns
  `pm.DependencySources` (local & workspace deps, each module source) +
  sibling project files + the root file last. kcc's `kccAssembleSource(file)`
  uses it to build a temp sandbox that `KCCCheckCommand`, `KCCBuildCommand`,
  `KCCRunCommand` compile.
- **test:** `projectModuleSources(testFile)` prepends dependency sources to each
  test driver; `KCCTestCommand` mirrors project dirs into a sandbox
  (`kccMirrorTestDir`) and stages single-file tests with `kccTestSource`.
  Flat/non-project corpora are copied verbatim (no behavior change).

### Bootstrap pin

`pkg/bootstrap:runCmdOutput` calls new `forceGoEngine` (adds
`KARKAIN_ENGINE=go`) so `RunStage1`/`runCompileStage` transpilation still uses
the Go front end to produce `src/compiler/main.c`; the self-hosting pipeline is
definitionally the Go bootstrap and must not recurse into the now-default kcc.

## Tests

New parity gate `pkg/cli/phase97_parity_test.go` — 6 tests, all pass:

| Test | Proves |
|------|--------|
| `TestPhase97_DefaultEngineIsKCC` | Empty env + unrelated env + explicit `kcc` → `EngineKCC` |
| `TestPhase97_GoEngineExplicitFallback` | `KARKAIN_ENGINE=go` and `--engine go` → `EngineGo`; `--engine kcc` → kcc |
| `TestPhase97_ManifestDepsInCompile` | `kccAssembleSource` includes local dep source, root last |
| `TestPhase97_WorkspaceDepInCompile` | workspace-source deps included in assembly |
| `TestPhase97_LocalDepRunsThroughKCC` | end-to-end `KCCRunCommand` of a project whose `main` calls a dependency function → `answer=42` |
| `TestPhase97_CheckBuildRunTestShareDependencyAwarePipeline` | `resolveSources` (check/build/run) + `projectModuleSources` (test) both include dependency sources |

### Regression results

- `pkg/cli` full suite: **ok** (442.765s) — includes 95/96 parity gates and the
  two E2E exit-code tests (now Go-pinned).
- `pkg/lexer`, `pkg/parser`, `pkg/codegen`, `pkg/pm`: **ok**
- `pkg/module`, `pkg/source`, `pkg/sema`: **ok**
- `go build ./...`: **ok**; `go vet ./pkg/cli ./pkg/bootstrap`: **clean**
- `TestBootstrap_BitwiseIdentity` stage2==stage3 **bitwise identical** (ok,
  255.212s) after the `forceGoEngine` pin.

### Regressions fixed

1. `TestCLI_ExitCodes_E2E` / `TestCLI_TestFailure_ExitCode_E2E` — after the
   default flip these invoked kcc in a temp dir with no kcc available. Pin the
   Go engine (they assert Go's exit-code contract; kcc's runner path is covered
   by the 95/96 parity gates).
2. `TestBootstrap_BitwiseIdentity` — stage-1 CLI routed to kcc (default),
   emitting `.c23` into a temp sandbox instead of `src/compiler/main.c`; fixed
   with `forceGoEngine`.

## Commit

- `develop`: `git commit` (Phase 97) → `git merge main` (fast-forward) → push both.
- Final commit message: **"Phase 97: default kcc engine + manifest dependency resolution"**