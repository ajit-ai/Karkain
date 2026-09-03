# PHASE 79 — KARKAIN COMPILER INTEGRITY, IR ARCHITECTURE & SELF-HOSTING READINESS AUDIT

**Document:** Executive Summary
**Phase:** 79 (audit-first, evidence-first, change-minimizing)
**Date:** 2026-09-03
**Status:** COMPLETE

---

## 1. Scope

Evidence-based audit of the Karkain compiler against the Phase 79 prompt:
repository inventory, dependency map, compiler pipeline, IR architecture
(Core/SSA, Math IR, Tensor IR), CPU/GPU/NPU backend parity, determinism,
optimization boundaries, test architecture, bug regression (BUG-1..8), and
self-hosting readiness.

Audit-first: **no implementation code was changed.** The only edits were
**minimal documentation corrections** to align stale records with verified
implementation facts (Category 2 corrections).

---

## 2. Baseline

```text
Repository: F:\Codes\Git\Karkain
Branch:     main (up to date with origin/main)
Commit:     1c42a5d  (Phase 51: Borrow Checker Lexical Scoping)
            1c42a5d671c5b005adb81f244d586fcb3793d1b0
Working tree: clean at baseline (a regenerated, untracked
              src/compiler/main.c artifact is excluded from commits)
```

Major validation commands:

| Command | Result |
| ------- | ------ |
| `go test ./pkg/... -count=1` | 22 packages ok; **FAIL** only in `pkg/bootstrap` (9 tests: `TestArgs_*`, `TestBootstrap_Stage1`, `TestBootstrap_Stage2`, `TestBootstrap_BitwiseIdentity`) |
| `go test ./pkg/lexer/... ./pkg/parser/... ./pkg/codegen/... ./pkg/pm/... -count=1` (AGENTS.md suite) | PASS |

Note: the prompt's claim "full test suite is currently reported as green"
does **not** match the actual `go test ./pkg/...` result — `pkg/bootstrap`
fails. This is a **pre-existing, documented** self-hosting/bootstrap issue
(stage1/stage2 recompile + bitwise SHA), independent of `pkg/sema`. It is a
real environmental/toolchain issue to report, not a Phase 79 regression.

---

## 3. Overall Architecture Status

```text
HEALTHY WITH GAPS
```

**Why:** The main compiler pipeline (lexer → parser → sema borrow-check →
SSA IR → C23 → gcc) is coherent, well-layered, and deterministic. Backend
abstraction exists and is correctly inverted (backends consume Tensor IR, not
AST). However: (a) **Math IR is disconnected** (imported by no package), (b)
**GPU/NPU backends are codegen/metadata-level stubs, not hardware-executing**,
(c) **no cross-backend differential parity testing** exists, and (d) the
self-hosting bootstrap path is not green.

---

## 4. Critical Findings

1. **`pkg/math` (Math IR) is dead/unconnected** — imported by no package
   anywhere in the module. It duplicates capability that Tensor IR/backends
   provide and its optimizer is unreachable. HIGH confidence (`go list -deps`
   + grep for `"karkain/pkg/math"` = zero non-self importers).
2. **`pkg/bootstrap` test suite fails** — `go test ./pkg/...` is not green.
   `TestBootstrap_Stage1Compilation`, `TestBootstrap_Stage2SelfHosting`, and
   `TestBootstrap_BitwiseIdentity` fail, plus `TestArgs_*`. This is the
   pre-existing self-hosting blocker, not a Phase 79 regression.

---

## 5. Major Risks (ranked)

| # | Risk | Level | Confidence |
|---|------|-------|------------|
| 1 | Self-hosting is not green (stage1/stage2 + bitwise parity) | HIGH | HIGH |
| 2 | GPU/NPU backends do not execute on hardware — parity of *semantics* unproven | HIGH | HIGH |
| 3 | GPL/AGENTS claim of green suite diverges from actual `pkg/bootstrap` failure | MED | HIGH |
| 4 | Math IR disconnected → potential duplicate/confused ownership of math semantics | MED | HIGH |
| 5 | Determinism of temp-file artifacts and unordered sema map diagnostics unverified end-to-end | LOW | MED |

---

## 6. Backend Status

| Backend | Execution | Gap |
| ------- | --------- | --- |
| CPU | **Real** — compiles embedded C23 with gcc, runs, captures output (`pkg/backend/cpu/runtime.go`) | reference oracle |
| GPU | **Codegen/metadata** — generates WGSL compute shader + metadata; no hardware execution | parity unverified |
| NPU (Intel/Qualcomm/Apple/AMD/Arm) | **Stub** — `Execute` returns metadata-only result | parity unverified |

---

## 7. IR Status

- **SSA IR** (`pkg/ir/ssa`): the compiler middle-end. Coherent; AST→SSA→C23
  with fallback. Constant folding + DCE. Well-formedness `Verify`.
- **Math IR** (`pkg/math`): exists, Karkain-owned, tested internally, but
  **disconnected** (no callers).
- **Tensor IR** (`pkg/tensor`): Karkain-owned, correctly consumed by all
  backends; reverse-mode autodiff `BuildGradient`; lowers to C.
- **Boundary finding:** Tensor IR is the single shared IR the backends consume
  (correct direction). Math IR is not in that flow.

---

## 8. Determinism Status

```text
Status:   Deterministic output-emission path (CPU pipeline)
Confidence: HIGH for C-emission ordering; MEDIUM overall
```

- Map iteration in the primary C emitter is **sorted before output**:
  `pkg/codegen/emit_ir.go` collects into `decls` map then `sort.Strings(names)`.
- Top-level struct/enum/function emission iterates the **ordered** `prog.Statements` slice.
- No `rand` usage anywhere in `pkg/`.
- `time` usage in `pkg/codegen` is only a compile **timeout**, not output.
- Remaining risk: runtime temp filenames (`os.CreateTemp` in CPU backend) and
  unordered map iteration inside **sema diagnostic aggregation** are
  nondeterministic ordering artifacts, but do not reorder emitted code.

---

## 9. Test Confidence

```text
Confidence: MEDIUM
```

- **Strengths:** 67 test files; strong per-package unit coverage (codegen 15,
  sema 14, parser 6, cli 5); real CPU E2E (`TestExecuteAdd/MatMul/Relu`,
  autodiff gradient E2E); AGENTS.md suite green; Phase 51 borrow Groups A-I;
  BUG-4/7/8 E2E (`pkg/cli/bugfix_e2e_test.go`).
- **Gaps:** no golden/snapshot tests; **no cross-backend differential/parity
  test** (CPU vs GPU vs NPU numeric comparison); GPU/NPU hardware unverified;
  `pkg/bootstrap` failing.

---

## 10. Self-Hosting Readiness

```text
Score:   Not reproducible as a single percentage — the methodology below.
Confidence: MEDIUM on component portability, HIGH on current non-green status
```

Partial Karkain-written compiler exists (`src/*.kark`: lexer 525, parser 668,
codegen 1477, sema 407, ast 796 lines). Stage1/Stage2/bitwise tests **fail**,
so the bootstrap is NOT currently operational. See
`PHASE-79-SELF-HOSTING-READINESS.md`.

---

## 11. Changes Made

```text
NO IMPLEMENTATION CODE CHANGES.
```

Documentation-only minimal corrections (Category 2):

| File | Reason | Category |
| ---- | ------ | -------- |
| `ROADMAP.md` | BUG-1..8 status table marked OPEN though all are fixed with regression coverage | 2 (stale facts) |

Planned (this doc-set): new audit documents under `docs/audit/`, and README/
AGENTS phase-status updates (52–79 completion). These are documentation
records, not compiler code.

---

## 12. Recommendation

```text
READY WITH PREREQUISITES
```

Karkain is architecturally coherent enough to begin **Phase 80 — SIMD / Vector
Execution Architecture**, provided these small, targeted prerequisites are
acknowledged/staged first:

1. **Reconcile Math IR** (connect to the pipeline or explicitly mark it as the
   reference caller-free evaluator for now).
2. **Document the `pkg/bootstrap` failures** as a known self-hosting blocker
   with an exit criterion (they are pre-existing, not Phase 79 regressions).
3. **Establish the parity baseline** for SIMD by confirming the CPU backend as
   the cross-backend oracle and, before hardware validation, define a
   differential (tolerance) comparison harness for CPU vs GPU vs NPU.

No **major architectural blocker** prevents SIMD design. Proceeding with SIMD
does not require resolving self-hosting first, but the bootstrap failure and
Math-IR ownership should be recorded as tracked risks.