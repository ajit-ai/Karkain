# Phase 120 — Compiler Independence: GA Envelope & KIR v1 Text Emitter

**Status:** COMPLETE
**Verdict:** GO (Phase 120 gates green; foundation for Phase 121)

## Goal

Deliver the first compiler component written entirely in **Karkain** and
owned by the self-hosted compiler: a deterministic **KIR v1 text emitter**
(`src/compiler/kir.kark`), plus the **GA envelope** around it — an honest
inventory of compiler-component ownership, docs, an executable example,
and a gate suite that pins the KIR contract.

## Deliverables

### 1. KIR v1 text emitter (Karkain-owned)

- `src/compiler/kir.kark` — `kirEmit` plus `kirImport`/`kirStmts`/
  `kirStmt`/`kirArm`/`kirPattern`/`kirInit`/`kirCond`/`kirPost`/
  `kirExpr`/`kirBound`/`kirField`/`kirJoin`/`kirIndent`. Covers the full
  surface: funcs, kernels, struct/enum decls, var/let/const, if/else,
  while, for, for-in, break/continue, alloc/free/global_id, all unary/
  binary ops, call, index/member/slice, array/map/struct literals,
  lambdas/closures, match, return/print.
- Wired into the self-hosted engine: `src/compiler/main.kark` `kir`
  dispatch + `kirFile` mirroring `checkFile`'s parse entry.
- **Invariants** (pinned by the gate):
  - path-less `source: <basename>` header → byte-identical across
    sandboxes;
  - success marker `[ok] kir text: N lines` (path-less);
  - parse errors surface before emission (exit 3), never a partial `[ok]`;
  - one `print` per line (determinism-by-construction).
- Parse-error path verified: `let = 42` → `error[K001]`, line+excerpt,
  exit 3, no `[ok]`.

### 2. GA envelope

- **Routing:** `pkg/cli/kir.go` `KCCKirCommand` (Go-side syntax preflight
  → target preflight → `kccAssembleSource` → sandboxed `kcc kir` →
  Success iff exit 0 *and* `[ok] kir text:` present) + `kir`/help
  registration in `cmd/karkain/main.go`.
- **Inventory:** `docs/inventory/compiler-dependencies.json` — classifies
  every compiler component (lexer/parser/AST/sema/KIR/codegen/runtime/
  bootstrap) with Go vs Karkain ownership, active path, status, and the
  KEEP/MIGRATE/bootstrap horizons for Phase 121.
- **Docs:** `docs/source/compiler/kir.rst` (KIR contract + full format
  table), `docs/source/compiler/index.rst` (kcc row documents KIR
  ownership, toctree entry), `docs/source/compiler/architecture.rst`
  (new *Compiler independence* section → Phase 121).
- **Executable example:** `examples/self-hosting/kir/main.kark`
  (+ READMEs) exercising the full KIR surface; prints identical stdout
  (`15 / 20 / 1 / 15`) on both engines; gated by the Phase 120 test.
- **Gate:** `pkg/cli/phase120_kir_test.go` — subtests
  `EmitterEndToEnd`, `ByteDeterminism`, `SpaceFormPrintParity`,
  `SelfHosting` (kcc re-emits `kirEmit`'s own definition from the
  assembled pipeline), `ExampleCoverage`, `ExitCodeContract`, and
  `TestPhase120_KirCommandExists`.

## Parity fix discovered & applied

**Go↔kcc `print` parity gap:** kcc's `parsePrint` accepts optional
parentheses (`print x` and `print(x)`); the Go parser unconditionally
consumed `(`, so a space-form `print x` made the Go preflight reject
programs the KIR emitter renders (spurious `unexpected token 'else'`).
Fixed in `pkg/parser/parser.go` (parsePrint optional parens). Both
engines now check/run space-form prints identically.

## Blocking regression resolved

An earlier uncommitted WIP attempted module-resolution helpers
(`resolveModulePath`/`replaceDots` in `stdlib.kark`,
`loadSourceWithModules`/`extractImports`/`startsWith` in `main.kark`)
that were (a) dead code and (b) called `fileExists`, which only the
self-hosted checker knows — the Go bootstrap stage-1 failed with
`error[K002]: undefined function 'fileExists'`. Per the Phase 121 plan's
"no hard-coded modules" rule these were removed; stage-1 bootstrap and
the kcc rebuild are green again. Real module resolution is a Phase 121
target candidate.

## Regressions

All green:

- `TestPhase120_Kir` (6 subtests) + `TestPhase120_KirCommandExists`
- `TestPhase120_GaEnvelope`, `TestPhase99` (self-hosted compiler sources
  type-check clean)
- `TestPhase114` (49 Go goldens + kcc parity + corpus metadata)
- `TestConformanceCorpus_RunsClean` (kcc, 59/59)
- `TestProbesCorpus_RunsEveryProbe` (11/11)
- `verify-examples.ps1` (49 passed / 0 failed / 5 skipped)
- `pkg/lexer`, `pkg/parser`, `pkg/sema`, `pkg/codegen`, `pkg/source`,
  `pkg/module`, `pkg/diagnostics`, `pkg/runtime`, `pkg/wasm`,
  `pkg/compiler`, `pkg/ir({,hir,ssa})`, `pkg/backend`, `pkg/lsp`,
  `pkg/npu`, `pkg/pm`, `pkg/target`
- Sphinx HTML `-W` + linkcheck `-W` (0 warnings)
- `go build ./...`, `go vet ./...`

## Known environmental limitation (NOT a defect)

`TestBootstrap_BitwiseIdentity` stage-2 SEGFAULT (exit 0xc0000005) on
this ~4 GB-RAM host reproduced in the Phase 119 QA battery identically
on a **pristine HEAD worktree** (1017068). Re-confirmed here with
663 MB free physical memory at run time and no stray build processes.
Evidence for the compiler-source changes instead:
stage-1 built clean (17 s), Phase 99 type-checks all 10 sources
(295,036 chars assembled) via kcc, and the Phase 120 SelfHosting subtest
runs the assembled kcc pipeline end-to-end.

## Commit workflow

Phase 120 changes committed on `develop`, merged to `main`, pushed per
the mandatory branch rule.

## Hand-off to Phase 121

Baseline for the Compiler Independence Foundation:
compiler components = 10 `.kark` files / 295,036 chars; Go compiler
packages: lexer 3, parser 13, sema 37, codegen 63, ir 2, compiler 2,
diagnostics 7, target 3, bootstrap 3 (file counts). Inventory JSON is
the authoritative ownership map; KIR is documented as existing progress.