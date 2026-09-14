# Phase 121 — Compiler Independence Foundation: BASELINE

**Status:** BASELINE VERIFIED (mandatory Phase 121 deliverable, per the
"Compiler Independence Foundation" plan)
**Date:** 2026-09-14
**Verdict at baseline:** self-hosting progress is real but **asymmetric** —
Karkain owns lex/parse/sema/codegen inside the engine, while Go still owns
source **assembly**, import stripping, preflight, and the bootstrap build
trigger on the kcc default path.

## 1. Repository facts (verified, not claimed)

| Fact | Value | Source |
|---|---|---|
| Self-hosted compiler (Karkain) | **10 `.kark` files**, 295,036 chars assembled | `src/compiler/` |
| Files | ast, atypes, checker, codegen, kir, lexer, main, parser, sema, stdlib | `src/compiler/` |
| Go compiler packages (Go files) | lexer 3, parser 13, sema 37, codegen 63, ir 2, source 2, module 2, diagnostics 7, compiler 2, backend 4, npu 10, runtime 7, bootstrap 3, wasm 5, pm 25, lsp 4, target 3, cli 83 | `pkg/*` (counted) |
| Default engine (empty `KARKAIN_ENGINE`) | kcc (`EngineFromEnv`) | `pkg/cli` |
| KIR | Karkain-owned emitter `src/compiler/kir.kark`; CLI gateway only in Go | Phase 120 |

## 2. Pipeline ownership on the default (kcc) path

| Stage | Owner | Evidence (file:line) |
|---|---|---|
| CLI routing / flags | Go | `cmd/karkain/main.go`, `pkg/cli` |
| Syntax **preflight** (re-parse) | Go | `projectSyntaxDiagnostics` (Phase 105), gates BOTH engines |
| Semantic **preflight** (re-resolve) | Go | `runSemanticPreflight` / `kccCheckPreflight` (K002/K102, exit 3) |
| **Source assembly** (import-driven unit OR legacy sibling-join) | **Go** | `kccAssembleSource` (`pkg/cli/kcc_engine.go:387`) → `resolveSourcesRun` (`pkg/cli/module_build.go:61`) → `assembleModuleUnit` (`pkg/module`) or legacy `resolveSources` |
| **import std.x line stripping** | **Go** | regex in `kccAssembleSource` (kcc_engine.go:392) |
| Test-mode dependency assembly | **Go** | `kccTestSource` / `kccMirrorTestDir` (kcc_engine.go:400/419) |
| kcc bootstrap build trigger (staleness) | Go | `kccBinaryPath` / `kccStale` / `buildKCC` (kcc_engine.go:142/170, bootstrap stage-1 = Go front end → gcc) |
| gcc link driver / sandbox | Go | `KCCRunCommand`/`KCCBuildCommand` sandboxes |
| Lex | Karkain | `src/compiler/lexer.kark` |
| Parse / AST | Karkain | `src/compiler/parser.kark` / `ast.kark` |
| Semantic analysis (internal) | Karkain | `src/compiler/checker.kark` + `sema.kark` + `atypes.kark` |
| **KIR v1 text** | **Karkain** (emitter only — CLI-bound) | `src/compiler/kir.kark`; consumption = post-120 boundary |
| C23 emission | Karkain | `src/compiler/codegen.kark` |

## 3. Identified bottleneck (evidence-based)

The **assembly + import-stripping** the kcc engine consumes every run is
produced by Go (`kccAssembleSource`). The Karkain engine is passive:
it never sees the pre-assembly decisions, and the KIR component ends at
the CLI text boundary. Concretely:

- `kccAssembleSource` (Go) is the only producer of the concatenated unit;
  `src/compiler` has no equivalent.
- import stripping (`import std.x`) is a Go regex; the Karkain parser must
  accept the residue.
- KIR (the one Karkain-owned IR) is not consumed by any engine-internal
  pass: a Go gateway runs it, prints text, exits.

Phase 120 already documented KIR consumption as the post-120 candidate.
This baseline therefore selects **Target E** (per the plan's A–F list):
*connect the existing Karkain-written KIR component into the real kcc
pipeline* — turning a CLI-only text dump into a compiler-internal invariant
pass on the default engine, with the successful outcome being "KIR is
genuinely active in the kcc path" (the plan's rule: if KIR is already
active, document it and build on it).

## 4. Go dependency classification (baseline)

| Dependency | Class | Rationale |
|---|---|---|
| CLI routing + flags | KEEP | host/OS boundary, not compiler semantics |
| Syntax preflight (Go re-parse) | BRIDGE→MIGRATE | guards both engines; kcc already re-parses inside → eventually removable |
| Semantic preflight (Go re-resolve) | BRIDGE | kcc checker is conservative-only today; parity needed before removal |
| **Source assembly + import stripping** | **BRIDGE** | Go today; Karkain-owned assembly is the migration (Phase 121+ candidate; do NOT hard-code) |
| kcc bootstrap trigger (stage-1 Go→gcc) | KEEP (documented) | the bootstrap path; self-hosting measured separately (Q5) |
| gcc sandbox link driver | KEEP | native toolchain boundary |
| KIR emitter | REMOVE-class (already Karkain-only) | no Go emitter exists; extend, don't duplicate |
| Lex/Parse/AST/Sema/Codegen internals | Karkain-owned | immutable foundational fact |

## 5. Self-hosting measurements — BASELINE answers (5 questions)

1. **Does the Karkain engine parse its own sources at runtime?** YES —
   `kcc check src/compiler/main.kark` type-checks the assembled compiler.
2. **Does the Karkain engine build/run itself without a Go front end?**
   NO — bootstrap stage-1 is Go→gcc; stage-2/3 identity proven only when
   the 4 GB host can run it (documented environmental limit).
3. **Is the source unit the engine consumes produced in Karkain?**
   NO — assembly is Go (`kccAssembleSource`).
4. **Is any IR pass engine-internal & Karkain-owned?** KIR: NO (CLI-only).
   ← Phase 121 closes this NO.
5. **Is there a competing compiler architecture?** NO — one kcc engine.

## 6. Hard-stop checks at baseline

- No new `lexer2`/`parser2`/`new_kir.kark`: Phase 121 extends
  `kir.kark`/`main.kark` only.
- No Go engine KIR emitter is introduced.
- A smaller verified step (KIR-internal-verification) is chosen over a
  large migration (full Karkain assembly/backend), per the plan's rule.

## 7. Carried-over hardening (uncommitted, already green)

Recursive closure codegen (`fn`) + managed `alloc(T,n)`/`free` on both
engines (Go `pkg/codegen`, `pkg/parser/captures.go`, kcc
`parser.kark`/`codegen.kark`/`ast.kark`/`checker.kark`) — gates
`pkg/codegen/phase121_correctness_test.go`. Retained under Phase 121;
renumbered in the final evidence report per the owner's attached plan.

## 8. Phase 121 success criterion (baseline state)

"At least one existing Karkain-written compiler component becomes
measurably more real, more integrated, and more responsible in the actual
KCC pipeline, with executable evidence and no regression to 1.0.0."
→ Target chosen: **KIR becomes an engine-internal pass on the default
kcc `check` path** (structural verification consumed by the Karkain
compiler itself), plus the honest self-hosting Q4 moves NO → YES.