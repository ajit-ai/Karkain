# PHASE 121 — COMPILER INDEPENDENCE FOUNDATION — FINAL EVIDENCE REPORT

- **Date**: 2026-09-14
- **Milestone**: Karkain 1.0.0 (Stable Build)
- **Verdict**: **COMPLETE** — Target E delivered, baseline gap closed (self-hosting Q4 NO → YES), all regressions green. Evidence below.
- **Baseline**: `docs/audit/PHASE-121-BASELINE-COMPILER-INDEPENDENCE.md`

---

## 1. Plan compliance

| Plan element | Requirement | Status |
|---|---|---|
| Mandatory BASELINE | Repo/pipeline facts before implementation | DONE (`PHASE-121-BASELINE-COMPILER-INDEPENDENCE.md`) |
| ONE target A–F | Choose and ship exactly one self-hosting migration | **Target E** (below) |
| Extend existing `.kark` only | No `lexer2`/`parser2`/`new_kir.kark` | DONE — only `src/compiler/kir.kark` + `main.kark` edited |
| Real-implementation bar | Code, not scaffolding | DONE — `kirVerify` is a real structural verifier wired into `checkFile` |
| Go dependency decision | KEEP / MIGRATE / BRIDGE / REMOVE stated | DONE — KEEP + BRIDGE (section 4) |
| Self-hosting measured 5× yes/no | Updated at end of phase | DONE (section 5) |
| E2E proof | Executable demonstration through the real pipeline | DONE (section 6) |
| Honest docs | inventory/JSON + Sphinx + report reflect reality | DONE |
| Hard stop | No second compiler architecture | Respecting — verifier is a pass over the existing single pipeline |

## 2. Target E — decision summary

The baseline proved the real independence bottleneck is that **KIR was CLI-only**
(baseline pipeline ownership table). The Go layer still owns project assembly
/import stripping (`kccAssembleSource`, kcc_engine.go:387 → `resolveSourcesRun`,
module_build.go:61 → `assembleModuleUnit`), so a full module-resolution migration
was deferred (earlier WIP was rejected as hard-coded/non-real). Target E makes
the Karkain-owned compiler component **self-verifying inside the default engine
path** — the smallest real step that removes a hard-coded CLI boundary and is
enforceable forever after.

## 3. Implementation (all files, real code)

### `src/compiler/kir.kark` — `kirVerify` structural verifier
- `kirVerify(lines, path)` — returns a (possible empty) problem array:
  - header line must be exactly `KIR v1`;
  - `lines[1]` must be `source: ` + `fileBaseName(path)`;
  - two-space indentation discipline per depth level;
  - nesting depth never jumps forward by more than one level;
  - every statement line ends ` line: <decimal>` except bare `block`
    introducers and the unknown-node fallback `stmt <type>`.
- Helpers: `kirIndentCount`, `kirIsDigits`, `kirIsStructureLine`,
  `kirHasLineNo` (scans the **final** ` line: ` occurrence so
  expression-embedded match-arm markers `(case … line: N)` cannot confuse the
  statement-level suffix check).
- Uses only language builtins (`len`, `str`, `substr`, `trim`, `contains`,
  `appendArray`) and `fileBaseName` from `main.kark` — written in Karkain,
  verified by the Phase 99 self-hosted checker, zero Go participation.

### `src/compiler/main.kark` — wiring
- `checkFile`: after the Phase 99 `typeCheckProgram` gate, emits KIR and runs
  `kirVerify`; **silent on success**, `error[K121]` + problem lines + no `[ok]`
  on drift → the CLI maps it to `ExitCompile` (3). This runs on the DEFAULT
  kcc check path for every accepted file.
- New `verifykir` command + `verifyFile`: standalone form printing
  `[ok] kir text: N lines` and `[ok] kir verify: N lines ok`; parse errors and
  drift print `error[K121]`/parser errors and no `[ok]`.

### Go side (routing only — no IR logic)
- `pkg/cli/kir.go`: `KCCKirVerifyCommand` — mirrors `KCCKirCommand` (syntax
  preflight → assemble → target preflight → temp sandbox → `kcc verifykir`),
  success requires the `[ok] kir verify:` marker.
- `cmd/karkain/main.go`: `--verify` flag (rejected for non-kir commands) and
  kir-dispatch routing to `KCCKirVerifyCommand`; help text updated.
- Tests: `pkg/cli/phase121_compiler_independence_test.go` (6 subtests +
  command-exists). The orphaned `pkg/cli/phase121_module_resolution_test.go`
  (guarding the rejected, hard-coded WIP funcs `resolveModulePath`/
  `replaceDots`/`loadSourceWithModules`/`extractImports`/`startsWith`) was
  deleted and replaced.

## 4. Go dependency classification (final)

| Dependency | Class | Why |
|---|---|---|
| `pkg/cli` routing (`KCCKirCommand`, `KCCKirVerifyCommand`) | **KEEP** | Thin gateway (preflight + assemble + dispatch); owns no IR/compiler logic |
| `kccAssembleSource` / module resolution | **BRIDGE** | Stays Go this phase; documented as the next independence bottleneck |
| `cmd/karkain` main.go (`--verify`, kir dispatch) | **KEEP** | CLI contract only |
| KIR emitter + verifier | **REMOVE (Go)** | No Go KIR exists — 100% Karkain-owned, now self-verified |

## 5. Self-hosting questions (baseline → final)

| # | Question | Baseline | Final |
|---|---|---|---|
| Q1 | Does kcc lex itself with the Go engine? | NO (self) | NO (self) |
| Q2 | Does kcc parse itself with the Go engine? | NO (self) | NO (self) |
| Q3 | Does kcc check itself with the Go engine? | NO (self) | NO (self) |
| Q4 | Does kcc emit AND verify its own IR on the default path? | **NO** (CLI-only) | **YES** — `kirVerify` in `checkFile` | 
| Q5 | Does kcc regenerate/deploy itself without Go-stage-1? | NO (bootstrap) | NO (bootstrap — unchanged) |

## 6. E2E proof (evidenced by the gate, 164.3s)

- `karkain kir --verify` on the shipped surface fixture: identical repeated runs
  (`source: main.kark` header, `[ok] kir text:` count == `[ok] kir verify:`
  count, no `error[K121]`).
- `karkain kir --verify src/compiler/kir.kark`: the compiler verifies the file
  that **contains the emitter and verifier themselves** (assembled unit = all
  10 compiler sources; 6399 KIR lines verified).
- `karkain check src/compiler/main.kark` (DEFAULT engine): the full compiler
  assembly passes through `checkFile`'s internal `kirEmit`+`kirVerify` (the
  checkFile hook, silent success).
- Exit contract: missing file → usage error; unparseable file → failure with
  **no** `[ok]` verdict markers; semantic failure → exit 3 no `[ok]`.

## 7. Regression battery (all green on this change set)

| Suite | Result |
|---|---|
| Phase 121 gate (`TestPhase121*`) | PASS 164.3s |
| Phase 120 kir + GA envelope (`TestPhase120*`) | PASS 85.8s |
| Phase 99/117/118 (`TestPhase99`, `TestPhase117*`, `TestPhase118*`) | PASS 204.7s (incl. CompilerSourcesTypeCheck 71.5s) |
| Phase 114 (`TestPhase114`, 55 golden corpus incl. 49 KIR byte-identical) | PASS 497.2s |
| Conformance 59/59 + probes 11/11 | PASS 132.5s |
| Phase 115/116 | PASS 157.6s |
| verify-examples.ps1 | 49 passed / 0 failed / 5 skipped |
| All package suites (lexer, parser, sema, module, source, diagnostics, pm, target, runtime, runtime/gpu, runtime/concurrency-battery, compiler, wasm, ir, hir, ssa, backend, backend/cpu, backend/gpu, backend/parity, lsp, npu/all) | PASS |
| `pkg/codegen` (incl. carried-over correctness-sweep gate `phase121_correctness_test.go`) | PASS 39.6s |
| `go vet ./...`, `go build ./...` | clean |

### Notes
- **Emitter perf**: whole-tree KIR emit+verify ≈ 66–95 s. Measured per-file:
  every compiler root assembles the whole tree (6399 KIR lines), so the gate
  verifies one representative root + the whole-assembly `check` rather than ten
  redundant full-tree runs. Cost is pre-existing Phase-120 emitter behavior
  (verified by comparing with/without verify: +~5% on top of emission), NOT a
  Phase-121 regression.
- **Environmental (documented, not a defect)**: the ~4GB host cannot run two
  heavy kcc processes concurrently; all heavy gates were run sequentially.
  Stage-2 bootstrap SEGFAULT remains the documented kcc-build OOM class
  (reproduced in Phase 119 QA on pristine HEAD); this phase exercised stage-1 +
  check-type passes only.

## 8. Numbering reconciliation

The plan doc driving this phase is titled *"Karkain Phase 121 - Compiler
Independence Foundation"*. An untracked local draft `ROADMAP-PLAN.md` numbered
120 = GA envelope and 121 = correctness sweep. Resolution (documented here and
in AGENTS.md):
- Phase 120 = GA Envelope + KIR v1 text emitter (done, report
  `PHASE-120-KIR-FINAL-REPORT.md`);
- Phase 121 = **Compiler Independence Foundation** (this report);
- the correctness sweep (closures/`fn`, `alloc(T,n)`/`free` hardening) is
  retained as carried-over hardening under the attached plan's principle
  ("smaller verified step > large fake migration") and keeps its own green gate
  `pkg/codegen/phase121_correctness_test.go`.

## 9. Docs updated

- `docs/source/compiler/kir.rst` — `--verify`/`verifykir` interface, new
  "Structural verification (Phase 121)" section, boundaries amended.
- `docs/inventory/compiler-dependencies.json` — KIR component now records
  `kirVerify` + engine-invariant status; `structural_rules` added.
- Reports: this report + the Phase 121 baseline (pre-existing) + Phase 120 KIR
  report.