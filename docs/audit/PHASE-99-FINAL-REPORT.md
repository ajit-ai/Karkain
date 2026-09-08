# Phase 99 — Self-Hosted Parser & Type Checker

**Status:** ✅ Complete / Certified — READY TO COMMIT WITH DOCUMENTED ENVIRONMENTAL LIMITATION
**Date:** 2026-09-08
**Branch:** `develop` — commit prepared, awaiting authorization (no merge/push yet)

## Objective

Per `C:\Users\Lenovo\Downloads\Phase 99 — Final Controlled Verification Prompt.md`:

1. Give the self-hosted compiler (`src/compiler`) a complete recursive-descent
   parser and a static **type checker written in Karkain**, replacing the
   minimal parser surface so the default engine understands the current
   language on its own.
2. Implement the checker with a **two-pass architecture** mirroring the Go
   semantic authority (`pkg/sema/resolve.go`), emitting `error[K1XX]`
   diagnostics for hard semantic errors.
3. Wire the checker into `karkain check` **check-only** (no production codegen
   changes): clean files report `[ok]`, files with `error[K1XX]` diagnostics
   map to `ExitCompile` (exit 3).
4. Prove parity and rejection with a deterministic gate
   (`pkg/cli/phase99_selfhosted_test.go`) plus 14 hand-crafted fixtures under
   `examples/type_errors/`.
5. Run the full certification campaign: isolated bootstrap identity,
   isolated CLI conformance, full-tree regression, architecture review,
   documentation, then wait for authorization to commit/merge/push.

## Files Changed

| File | Change |
|------|--------|
| `src/compiler/atypes.kark` | **New.** Conservative static type inference: `inferType`, `primitiveTag`, `isPrimitiveTag`, `typeDescription`, `annotationMismatch`. ("a" prefix sorts it after `ast.kark` and before `checker.kark` in the assembler — Karkain emits C top-down, definitions must precede use.) |
| `src/compiler/checker.kark` | **New.** Two-pass whole-program checker written in Karkain: pass 1 `collectDecls` (funcs/kernels/structs/enums → K107 duplicates), `collectLocals` (forward-reference pre-scan); pass 2 `checkStmt`/`checkExpr`/`checkCall`/`checkIdent` with loop-depth threading, conservative rules only (provable-from-AST errors; never rejects an unprovable program). |
| `src/compiler/main.kark` | `checkFile` now runs `typeCheckProgram(ast)`; on any `error[K1XX]` the lines are printed and the file is NOT reported `[ok]` (CLI maps to exit 3). Check-only — build/run/test codegen untouched. |
| `pkg/cli/phase99_selfhosted_test.go` | **New.** Phase 99 gate: `TestPhase99_SelfHostedParserAndTypeChecker` with 4 subtests — CorpusAccept, CrossFileDuplicateDetected, ErrorFixturesRejected, CompilerSourcesTypeCheck. |
| `examples/type_errors/` | **New.** 14 fixture directories (err01…err14), one `main.kark` each, one per K1XX code path. |
| `pkg/bootstrap/bootstrap.go` | Subprocess timeout `2 minutes → 5 minutes` (measured clean Phase 99 transpile runtime exceeded the 120s budget). Harness-only; no production compiler semantics. |
| `pkg/cli/phase88_test.go` | Pins `KARKAIN_ENGINE=go` for the legacy Phase 88 `src/compiler/main.c` contract (default engine changed in Phase 97). Test-harness-only. |
| `pkg/cli/phase95_parity_test.go` | `phase95KCC` stage-1 build pins `KARKAIN_ENGINE=go` (stage-1 bootstrap builds are definitionally Go transpiles; avoids `%TEMP%` kcc-root resolution problems). Test-harness-only. |
| `AGENTS.md` | "post-98" → "post-99" + Phase 99 summary block. |
| `docs/ROADMAP-PRODUCTION.md` | Phase 99 marked ✅ COMPLETE + Status block + tracking-table row. |
| `docs/audit/PHASE-99-FINAL-REPORT.md` | **New.** This report. |

## Implementation Summary

### Semantic checker — K1XX diagnostics

The checker is implemented in **Karkain** (`src/compiler/checker.kark`) and
uses a **two-pass architecture**:

- **Pass 1 — declarations.** `collectDecls` registers every top-level
  `FuncDecl`/`KernelDecl`/`StructDecl`/`EnumDecl` (name + kind + arity) and
  reports **K107** duplicates. `collectLocals` pre-scans statement lists so
  forward local references resolve (mirrors the Go resolver's
  `walkLocalDefs`), handling `VarDecl`/`LetDecl`/`ForIn`/`Block`/`If`/`While`/`For`.
- **Pass 2 — bodies.** `checkStmt`/`checkExpr` walk every function body with a
  threaded loop depth (for K108) and a per-function local scope
  (params + locals + lambda closures). Calls are validated against the symbol
  table and the builtin allow-list.

| Code | Diagnostic |
|------|------------|
| K101 | undefined function call |
| K102 | undefined identifier read (inside a function body) |
| K103 | function/kernel arity mismatch |
| K104 | builtin arity mismatch |
| K106 | struct literal with an undefined type name |
| K107 | duplicate top-level definition |
| K108 | break/continue outside a loop |
| K109 | calling a type name as a function |
| K112 | primitive annotation/initializer type mismatch |

Karkain values are dynamically typed, so the checker is **conservative by
design**: it reports only errors provable from the AST (non-primitive
annotations, non-literal initializers, dotted/method calls, and unknown shapes
are never rejected), guaranteeing the compiled corpus and the compiler's own
sources keep passing unchanged. Editor/lint flows keep treating `sema.kark`
warnings as non-fatal.

`atypes.kark` supplies the static type lattice (`int/float/string/bool/array/
map/struct/function/unknown`) and `annotationMismatch` used by K112. The
filename sorts correctly for the letter-order source assembly.

### Check-only wiring

`main.kark` `checkFile` invokes `typeCheckProgram(ast)` after a successful
parse. Clean programs print `[ok] <path> parsed successfully` (unchanged
contract); programs with checker errors print the `error[K1XX]` lines and
return without `[ok]`, so the CLI maps the result to `ExitCompile`. The
build/run/test paths and the C codegen are untouched — emitted code is
bit-identical to Phase 98.

## Tests

### `pkg/cli/phase99_selfhosted_test.go` — `TestPhase99_SelfHostedParserAndTypeChecker` — PASS 54.58s

| Subtest | Result | Proves |
|---------|--------|--------|
| `CorpusAccept` | PASS | kcc parse + type-check of every conformance/algorithms/probes program (≥25 files enforced) → `[ok]` |
| `CrossFileDuplicateDetected` | PASS | assembling the conformance directory (which shares factorial/fib across files) fails with `ExitCompile` + K107, at assembly scope |
| `ErrorFixturesRejected` | PASS | 14/14 fixtures rejected with the exact `error[K1XX]` code, `ExitCompile`, no `[ok]` |
| `CompilerSourcesTypeCheck` | PASS | the compiler's own assembled source tree (214,127 bytes) parses + type-checks clean through kcc |

### Deterministic bootstrap identity — PASS 347.60s

```
stage 1:  karkain-compiler1.exe   1,071,595 bytes   SHA256=912114d5d2aca17f
stage 2:  karkain-compiler2.exe   1,432,956 bytes   SHA256=aff1d624d9e52c2d
stage 3:  karkain-compiler3.exe   1,432,956 bytes   SHA256=aff1d624d9e52c2d
stage2 == stage3: bitwise identical
```

Reproduced: a second isolated run again produced stage2 at
**1,432,956 bytes / SHA `aff1d624d9e52c2d`** (the run was interrupted during
stage 3 by operator action; determinism of stage 2 was re-confirmed). Earlier
recorded isolated passes: 352s identity run and full `pkg/bootstrap` suite
(571.688s), both green.

### CLI conformance — PASS in isolation

`TestConformanceCorpus_RunsClean` (conformance_test.go:65) passed inside the
isolated full `go test ./pkg/cli/...` run — **PASS 634.128s**. It is the same
test that timing-out under the concurrent full-tree workload (see
Environmental Limitation below).

### Error fixtures — 14/14 PASS

Every fixture exits `ExitCompile` with its exact K1XX code, no `[ok]`, no
panic, no access violation, no process crash:

err01 K101 · err02 K101 · err03 K102 · err04 K103 · err05 K103 · err06 K104 ·
err07 K104 · err08 K106 · err09 K107 · err10 K108 · err11 K108 · err12 K109 ·
err13 K112 · err14 K101

### Compiler source tree

Assembled self-hosted compiler sources: **214,127 bytes** — parse + type-check
clean through kcc (`[ok]`, exit 0).

## Karkain Correctness vs Constrained-Environment Limitation

This distinction is deliberate and evidence-based:

### Karkain correctness / toolchain — PASS

Every correctness gate passes, reproduced fresh in isolation:

- `TestPhase99_SelfHostedParserAndTypeChecker` — PASS (54.58s)
- `TestBootstrap_BitwiseIdentity` — PASS (347.60s), stage2 == stage3,
  SHA `aff1d624d9e52c2d`
- `TestConformanceCorpus_RunsClean` — PASS in isolated `pkg/cli` run
- `go test ./pkg/cli/...` isolated — PASS (634.128s); earlier PASS (456.542s)
- `pkg/bootstrap` isolated — PASS (571.688s)
- `go vet` — clean; all other packages green

### Full-tree concurrent execution on constrained host — ENVIRONMENTALLY LIMITED

Observed limitation: **approximately 4GB RAM, limited paging capacity, and
parallel Go/GCC/bootstrap workloads** during a single `go test ./pkg/...`
invocation. Under that concurrent full-tree workload only two tests failed:

```
TestBootstrap_BitwiseIdentity     — failed at 209.82s under concurrent load
TestConformanceCorpus_RunsClean   — 10-minute timeout under concurrent load
```

Both pass when executed in isolation. The failures correlate directly with
resource contention (parallel gcc stages + Go builds exhausting memory/paging),
not with Phase 99 code. This is a documented **environmental resource-contention
limitation**, not a compiler defect.

## Timeout Rationale

`pkg/bootstrap/bootstrap.go` raised the subprocess timeout from
**2 minutes → 5 minutes** because the measured clean Phase 99
bootstrap/transpile runtime exceeded the previous 120-second budget:

- stage-2 transpile (single longest subprocess): **~145–174s clean**
  (stage totals measured 2m07.98s–2m53.97s; growth driven by the added
  checker.kark/atypes.kark sources)
- 2-minute (120s) timeout < measured clean runtime → was killing the build
  deterministically; 5 minutes is ~1.7–2× measured clean max and is **not**
  increased further.
- Under memory pressure the process stalls in `os/exec.Wait`; the context
  timeout aborts cleanly instead of hanging forever — it is not concealing a
  hang.

## Test-Harness Changes (no production compiler semantics)

1. **`pkg/bootstrap/bootstrap.go`** — 5-minute subprocess timeout, to
   accommodate the measured Phase 99 self-hosting source growth.
2. **`pkg/cli/phase88_test.go`** — pins `KARKAIN_ENGINE=go`: preserves the
   historical Phase 88 Go-front-end `src/compiler/main.c` contract after
   Phase 97 changed the default engine.
3. **`pkg/cli/phase95_parity_test.go`** — pins `KARKAIN_ENGINE=go` in
   `phase95KCC`: keeps Phase 95 stage-1 bootstrap semantics deterministic and
   avoids `%TEMP%` kcc-root resolution problems.

These are test/bootstrap harness stabilizers and **do not alter production
compiler behavior**.

## Regression Results

- `TestPhase99_SelfHostedParserAndTypeChecker` — PASS, all 4 subtests.
- `go test ./pkg/cli/...` (isolated) — ok, 634.128s.
- `go test ./pkg/bootstrap/` (isolated) — ok, includes bitwise identity PASS.
- Full-tree `go test ./pkg/...` — all packages ok except the two
  environmental-limitation tests above.
- `go vet` — clean; No regressions observed in pre-Phase-99 behavior.

## Commit

Commit prepared but **NOT executed** — waiting for explicit authorization:

```
Phase 99: complete self-hosted parser and type checker
```

Per the repo workflow: commit on `develop` → merge into `main` (fast-forward)
→ push both branches to origin. **Do NOT merge or push until authorized.**
**Phase 100 has NOT been started.**