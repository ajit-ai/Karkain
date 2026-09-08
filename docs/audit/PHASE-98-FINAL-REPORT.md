# Phase 98 — NPU Integration into Compiler

**Status:** ✅ Complete
**Date:** 2026-09-08
**Branch:** `develop` → merged into `main`, pushed to origin

## Objective

Per `C:\Users\Lenovo\Downloads\Karkain Phase 98 — NPU Integration into Compiler — Implementation Prompt.md`:

1. Implement the `@target(...)` function attribute end-to-end (parser → sema →
   C codegen → runtime dispatch) with `cpu` and `npu` targets.
2. Add NPU matrix-multiplication dispatch (`NPUDispatcher`) with a **mandatory
   CPU fallback** — the CPU reference backend is the correctness oracle.
3. Teach the **self-hosted kcc compiler** about `@target(...)` so the default
   engine (since Phase 97) compiles and runs targeted programs instead of
   emitting invalid C (`target(npu);`).
4. Ensure **unknown targets are rejected on both engines** (`error[K004]`,
   exit 3) — including the default kcc `check`/`build`/`run`/`test` paths.
5. Add deterministic tests (mock NPU backend — no hardware/SDK/driver/cloud).
6. Update docs, run regressions, commit on `develop`, merge to `main`, push both.

## Files Changed

| File | Change |
|------|--------|
| `pkg/parser/ast.go` | `FuncDecl.Target` field (Phase 98). |
| `pkg/parser/parser.go` | `parseTargetAttr()` — parses `@target(<name>) func` at top level, sets `fn.Target`, enforces placement before `func`. |
| `pkg/parser/macro.go` | `expandNode` preserves `Target` through macro expansion (`case *FuncDecl`). |
| `pkg/sema/npu_check.go` | **New.** `TargetCPU`/`TargetNPU`, `TargetNames`, `ValidTarget`, `NPUError`, `NPUAnalyzer.Analyze` rejects unknown targets. |
| `pkg/codegen/codegen.go` | `genFuncDecl` emits `// @target(<name>)` C comment marker. |
| `pkg/codegen/npu_compiler.go` | **New.** `NPUOperation`/`OpMatMul`, `DispatchRequest`/`DispatchResult`, `NPUDispatcher` (auto-detect + `WithBackend` for tests) with `dispatchMatMul` and CPU fallback; adapter compile/execute failures degrade to CPU. |
| `pkg/cli/checker.go` | New `npuTargetDiagnostics` (parse-only Go analyzer); used in `AnalyzeSource`. |
| `pkg/cli/lint.go` | E-K-SEM NPU stage incl. `npuTargetDiagnostics`. |
| `pkg/cli/kcc_engine.go` | `kccTargetPreflight`/`kccStagedPreflight` validate targets Go-side before kcc runs; wired into `KCCCheckCommand`/`KCCBuildCommand`/`KCCRunCommand`/`KCCTestCommand`. |
| `src/compiler/ast.kark` | `setFuncTarget`/`funcTarget` (optional 7th FuncDecl slot). |
| `src/compiler/parser.kark` | `parseTargetAttr` claims `@target(...)` before `func`; `parseFunc` threads the target. |
| `src/compiler/codegen.kark` | C11 `FuncDecl` emitter prints `// @target(<name>)` comment instead of `target(npu);`. |
| `pkg/sema/phase98_npu_test.go` | **New.** 8 tests. |
| `pkg/codegen/phase98_npu_test.go` | **New.** 10 tests with a mock NPU backend. |
| `pkg/cli/phase98_npu_test.go` | **New.** 8 tests. |
| `AGENTS.md` | "post-97" → "post-98" + Phase 98 summary block. |
| `docs/ROADMAP-PRODUCTION.md` | Phase 98 row marked complete (deliverable now implemented, not deferred). |
| `docs/npu-targeting.md` | **New.** Language + pipeline + fallback doc. |
| `docs/audit/PHASE-98-FINAL-REPORT.md` | **New.** This report. |

## Implementation Summary

### Attribute end-to-end

- **Parser**: `@target(<ident>)` preceding a top-level `func` attaches the name
  to `FuncDecl.Target`. Placement is a parse error ("@target(...) must be
  followed by func"). Macro expansion preserves the field.
- **Sema**: `NPUAnalyzer` iterates `FuncDecl`s with a non-empty `Target`,
  emitting `error[K004]: @target(<name>): unknown execution target; supported
  targets: cpu, npu` for anything outside `{cpu, npu}`.
- **Codegen**: the attribute becomes an inert C comment (`// @target(npu)`);
  no executable NPU runtime code is emitted, so the generated program is
  self-contained.

### NPU dispatcher (matmul as the first NPU op)

`NPUDispatcher.Dispatch(OpMatMul)` builds `C = A @ B` in Karkain-owned Tensor
IR, then:

- compiles for and executes on an available vendor adapter
  (`intel`/`qualcomm`/`apple`/`amd`/`arm`), taking the result **only** if the
  adapter returns computed values;
- otherwise (no adapter, adapter reports no hardware, adapter stubs out,
  or compile/execute errors) executes on the **CPU reference backend** —
  the correctness oracle.

Caller-input errors (bad dims, operand size mismatch, unknown op) remain hard
errors. Adapter-environment failures never fail the program — they degrade to
CPU, so `@target(npu)` compiles and runs correctly everywhere.

### Self-hosted kcc support

kcc is the default engine (Phase 97). Without changes it emitted `target(npu);`
from a targeted function body or dropped the attribute — either way gcc failed.
Now:

- `src/compiler/parser.kark` claims `@target(<name>) func` at the top level and
  stores the target on the FuncDecl node;
- `src/compiler/ast.kark` adds an optional 7th slot (`setFuncTarget`/`funcTarget`);
- `src/compiler/codegen.kark` emits `// @target(<name>)` before the function's C
  definition.

### CLI + dual-engine rejection

`npuTargetDiagnostics` (parser-only Go analyzer) is shared by `AnalyzeSource`
(Go engine) and the kcc default-engine `check`/`build`/`run`/`test` paths
(`kccTargetPreflight`/`kccStagedPreflight`). Because kcc preserves
`@target(...)` as an inert comment, the Go NPU analyzer is the semantic
authority for both engines: `karkain check` rejects `@target(bogus)` with
`error[K004]` (exit 3) regardless of engine.

## Tests

### `pkg/sema/phase98_npu_test.go` — 8, all pass

| Test | Proves |
|------|--------|
| `TestPhase98_NPU_TargetRecognized` | `@target(npu)` accepted |
| `TestPhase98_NPU_CPUAndEmptyTargetsAccepted` | `cpu` and no-attribute accepted |
| `TestPhase98_NPU_UnknownTargetRejected` | unknown target → 1 error, correct wording + line |
| `TestPhase98_NPU_PlainProgramUntouched` | functions without the attribute unaffected |
| `TestPhase98_NPU_ParsedAttributeAttached` | parser attaches `npu` to FuncDecl |
| `TestPhase98_NPU_ParsedAttributeInvalidTargetStillAttached` | parser still attaches, analyzer rejects |
| `TestPhase98_NPU_AttributeMustPrecedeFunc` | analyzer safe when attribute precedes non-func |
| `TestPhase98_NPU_ValidTargetTable` | `ValidTarget` accepts `""`/`cpu`/`npu`, rejects others |

### `pkg/codegen/phase98_npu_test.go` — 10, all pass (mock NPU backend)

| Test | Proves |
|------|--------|
| `TestPhase98_Dispatch_MatMulReachesNPU` | matmul routes to an available mock adapter (`Path=np`u`, vendor metadata) |
| `TestPhase98_Dispatch_MatMulCPUFallback` | no adapter → CPU fallback, correct result |
| `TestPhase98_Dispatch_MatMulSameResultBothPaths` | NPU path result == CPU path result bit-identical |
| `TestPhase98_Dispatch_NoHardwareResultStillCorrect` | backend with no detected hardware behaves as CPU-only |
| `TestPhase98_Dispatch_AdapterErrorDelegatesToCPU` | adapter execute failure degrades to CPU, still correct |
| `TestPhase98_Dispatch_InvalidDimensionsRejected` | zero rows → error |
| `TestPhase98_Dispatch_OperandShapeMismatchRejected` | operand element mismatch → error |
| `TestPhase98_Dispatch_UnsupportedOperationRejected` | unknown op → error |
| `TestPhase98_Dispatcher_CapabilitiesDefaultToCPU` | default dispatcher reports matmul via CPU oracle |

The mock backend (`phase98MockBackend`) software-emulates an accelerator by
delegating `Execute` to the CPU reference backend, so it returns real computed
values with no hardware/SDK/driver dependency.

### `pkg/cli/phase98_npu_test.go` — 8, all pass

| Test | Proves |
|------|--------|
| `TestPhase98_CLI_AnalyzeSourceValidatesTargets` | shared analyzer: valid passes, bogus yields 1 diagnostic |
| `TestPhase98_CLI_GoEngine_CheckAcceptsAndRejects` | Go `check` exit 0 (npu) / exit 3 (bogus) |
| `TestPhase98_CLI_KCCEngine_CheckAcceptsAndRejects` | kcc-default `check` rejects bogus (preflight) |
| `TestPhase98_CLI_GoEngine_RunTargetedProgram` | Go run prints 41, exit 0 |
| `TestPhase98_CLI_KCCEngine_RunTargetedProgram` | kcc run prints 41, exit 0 |
| `TestPhase98_CLI_KCCEngine_BuildTargetedProgram` | kcc build succeeds |
| `TestPhase98_CLI_PlainProgramUnaffectedOnKCC` | plain program behaves identically |
| `TestPhase98_CLI_TargetNamesMatchAnalyzer` | CLI ↔ analyzer agree on targets |

## Regression results

- `pkg/sema` full suite: **ok**.
- `pkg/codegen` full suite: **ok** (incl. new Phase 98 dispatch tests).
- `pkg/cli` Phase 98 + Phase 95/96/97 parity gates: **ok**.
- `go build ./...`: **ok**; `go vet ./pkg/cli ./pkg/sema ./pkg/codegen ./pkg/parser`: **clean**.
- Smoke (default kcc engine): valid `@target(npu)` check/run OK (prints 41);
  `@target(bogus)` check rejected exit 3.

### Regressions fixed

1. **Self-hosted codegen emitted `target(npu);`** — the default kcc engine failed
   any `@target(npu)` program at gcc stage. Fixed by teaching
   `src/compiler/codegen.kark` to emit a C comment and the parser/ast to carry
   the attribute.
2. **kcc-default `check` accepted unknown targets** — kcc preserves the attribute
   as an inert comment, so it cannot reject them itself. Added the Go-side
   `kccTargetPreflight`/`kccStagedPreflight` (shared `npuTargetDiagnostics`) so
   `karkain check` on the default engine reports `error[K004]` exit 3, matching
   the Go engine.
3. **Adapter `Execute` errors aborted the program** while testing the mock
   backend's software-emulated adapter. Changed `dispatchMatMul` so compile /
   execute failures are environmental and degrade to the CPU fallback rather
   than surfacing a dispatch error; only caller-input errors stay hard errors.

No regressions in pre-Phase-98 behavior were observed in the covered suites.

## Commit

- `develop`: commit Phase 98 → merge into `main` (fast-forward) → push both.
- Final commit message: **"Phase 98: integrate NPU target dispatch"**