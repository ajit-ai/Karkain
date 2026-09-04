# KARKAIN — NEXT-STAGE ENGINEERING REPORT

Baseline commit: `64c598b` — branch `main` — remote `https://github.com/ajit-ai/Karkain.git`

Scope: INSPECT → RECONCILE → ARCHITECT → IMPLEMENT → VERIFY → DOCUMENT → REPORT
This document records repository reality and the justified next engineering work.

---

## A. Repository State

| Item | Value |
|------|-------|
| HEAD | `64c598b` (pre-change baseline) |
| Branch | `main` (== `develop`) |
| Working tree | clean before work; one borrow-checker change + tests applied |
| Tracked files | 279 |
| Extension | `.kark` (never `.kar`) |

## B. Inspection Results (evidence-based inventory)

Production compile pipeline (what `karkain build/run/check/test` actually run):
```
.kark file(s) → Lexer (pkg/lexer) → Parser (pkg/parser)
  → MacroExpansion (parser.ApplyMacroExpansion)
  → Borrow Checker (pkg/sema: only check wired into build/run)
  → Kernel Analyzer (pkg/sema: check only)
  → codegen (pkg/codegen, AST→C23) → gcc/clang/MSVC
```
- **No standalone Core-IR/bytecode pass** in the default compile path. `pkg/ir` bytecode (`.kbc`) + `pkg/jit` are a separate runtime engine.
- **SSA** (`pkg/ir/ssa`) is embedded per-function inside codegen with graceful fallback.
- **Math IR** (`pkg/math`) is imported by **nothing** — fully orphaned.
- **Tensor IR** (`pkg/tensor`) is the IR of the backend execution subsystem (pkg/backend, pkg/npu), not a compiler lowering stage.
- **CPU backend** (`pkg/backend/cpu`) performs **real gcc execution** (reference oracle). GPU = WGSL codegen only. NPU = metadata/stub (5 vendor adapters return fake blobs).
- **Package manager** (`pkg/pm`) is entirely disconnected from `build/run/check/test` — those read only same-directory `.kark` siblings.
- **`src/compiler/`** = Karkain-source self-hosted compiler (`main.kark` driver + lexer/parser/sema/codegen + embedded `runtime.c`).
- **Language has NO module import system** — `import` only accepts literal `"C"` (C interop). Multi-file = sibling concatenation.

## C. Reconciliation Matrix (evidence-based)

| Capability | Exists | Partial | Stub | Missing | Notes |
|------------|--------|---------|------|---------|-------|
| Lexer | ✅ | | | | pkg/lexer, tests |
| Parser | ✅ | | | | pkg/parser, arena, macro, captures |
| Borrow checker | ✅ | | | | pkg/sema; **had P0 bug** (params not declared) |
| Lifetime/ownership model | ✅ | | | | scope stack, dead-name tracking |
| Core IR (bytecode) | ✅ | | | | pkg/ir (runtime path, not default pipeline) |
| SSA | ✅ | | | | pkg/ir/ssa, embedded in codegen |
| Math IR | | ⚠️ | | | pkg/math — orphaned (imported by nothing) |
| Tensor IR | ✅ | | | | pkg/tensor — backend-execution IR |
| SIMD | | ⚠️ | | | real SSE/AVX type map; arithmetic NOT vectorized; no opt pass |
| CPU backend | ✅ | | | | real gcc execution + E2E tests |
| GPU backend | | ⚠️ | | | WGSL codegen only; no runtime |
| NPU backend | | | ✅ | | adapters return fake blobs; planner/fusion real |
| Runtime (memory/panic/startup/atomic) | ✅ | | | | embedded C23 Value runtime in codegen + src/compiler/runtime.c |
| Std | ⚠️ | | | | `std/` contains only 2 placeholder comments |
| Stdlib | | ⚠️ | | | io/math/gpu/async handwritten, real |
| System library | | ⚠️ | | | file/socket/process via generated C + libc |
| Formal ABI | | ⚠️ | | | implicit in codegen; no spec doc |
| Package manager | ✅ | | | | resolver/lock/cache/integrity/flow |
| Manifest integration | | ⚠️ | | | pm has it; compiler ignores it |
| Lockfile | ✅ | | | | karkain.lock read/write |
| **Build/run/check/test ← package graph** | | ❌ | | | **P0 gap: disconnected** |
| Differential tests (CPU vs other) | | ⚠️ | | | only hosted in backend cpu/gpu tests |
| Golden tests infra | | ⚠️ | | | phase/*_test.go ad-hoc, not a harness |
| Self-hosting | | ❌ | | | **fixed in this phase (Stage1/Stage2/Args PASS)** |
| Atomics | ✅ | | | | C11 stdatomic lowering |
| Actors/channels/async | | | ✅ | | parsed → placeholder C comments |
| Generics | | ⚠️ | | | AST only — no parse/codegen/monomorphization |
| Traits | | ⚠️ | | | AST + derive stub, no dispatch |
| Option/Result/Match/`?`/Enum | ✅ | | | | full pipeline |
| Closures/captures | ✅ | | | | env-struct conversion |
| Defer / const / pub / methods / operators | | | | ❌ | absent |
| Raw pointers / `@raw` | ✅ | | | | thin, no unsafe-block gate |
| C interop | ✅ | | | | `C.func()`, `import "C" {}` |

## D. Architecture Decisions

1. **IR ownership (reconciled, no rewrite):** Core/SSA IR lives in `pkg/ir`(+`/ssa`); Tensor IR is the backend-execution graph (`pkg/tensor` → `pkg/backend` → `pkg/npu`); Math IR (`pkg/math`) is a free-standing scalar/expression IR owned by **no one** — documented as orphaned. No component is rewritten; ownership is recorded so no IR remains architecturally orphaned.
2. **CPU is the reference oracle.** Only `pkg/backend/cpu` performs real execution; GPU/NPU are codegen/metadata and must not claim parity.
3. **No fake features:** registry publish, Git fetch, NPU execution, GPU execution remain explicit stubs/errors, not faked.
4. **Do not weaken correctness:** the borrow-checker fix restores correct treatment of function parameters; genuine ownership violations (e.g. move-then-use of a parameter) are still rejected (regression-tested).

## D2. PM→compiler integration: project-aware source resolution (added subsequent pass)

The package manager and the compiler were disconnected: `build`/`run` joined only
same-directory sibling `.kark` files and ignored `karkain.toml`/deps. This pass
wired the compiler to the package graph *without changing the language grammar*:

- New `resolveSources` (used by `RunCommand` and `BuildCommand`): if the root
  file is inside a project (`karkain.toml` found), it emits, in deterministic
  order, the sources of resolved **local** dependencies (upstream), then the
  project's own sibling modules, then the root file last, deduplicating by path.
- Non-project builds fall back to the classic sibling-join — byte-identical to
  prior behaviour (verified by test).
- `projectSourceFiles` refuses `go.mod`-only roots (so Go-tree dirs are not
  misread as Karkain projects).
- Registry/git dependencies keep their existing "not yet available" semantics
  and contribute no sources (their module layout is not yet formalized).

E2E verified: a temp project `main` → `helper` (sibling) → `lib_add` (local dep)
compiles and runs, printing `42`.

### Bonus fix: UTF-8 BOM handling (found while verifying)

Concatenating sibling files that started with a UTF-8 BOM (common on Windows
editors) silently **dropped the first declaration** — the lexer treated the
BOM-prefixed `func` as an unknown identifier, so the first function never made
it into `prog.Statements`. `lexer.New` now strips a leading BOM. Regression
tests added. This was a latent pre-existing bug, exposed by the new project
assembler feeding scope-checked files.

## E. Implemented Changes

| File | Purpose | Architectural reason |
|------|---------|----------------------|
| `pkg/sema/borrow_checker.go` | `checkFuncDecl` now declares each function parameter as an owned live binding in the function scope | Correct ownership semantics: a param is alive for its body; previously params were never declared, so referencing a param whose name an earlier function had marked dead produced false "use after scope has ended". This was the self-hosting blocker. |
| `pkg/sema/borrow_checker_test.go` | Regression tests: param live on first statement, across independent functions, after branches; plus genuine param-move still rejected | Protect the fix from regression without weakening ownership checking. |
| `pkg/cli/commands.go` | `resolveSources` + `projectSourceFiles`: project-aware, deterministic, deduplicated source assembly wired into `build`/`run` | Closes the package-manager→compiler gap without changing grammar; pulls local dependency sources upstream; non-project builds stay byte-identical. |
| `pkg/cli/module_resolve_test.go` | Unit tests for `resolveSources`: non-project fallback parity, project dep/sibling/main ordering, single-root dedup | Lock in deterministic project-aware assembly and backward compatibility. |
| `pkg/lexer/lexer.go` | Strip a leading UTF-8 BOM in `New` | A BOM-prefixed first line corrupted the first token and silently dropped the first declaration when sibling files were concatenated. |
| `pkg/lexer/lexer_test.go` | BOM regression tests (`TestLeadingBOMSkipped`, `TestBOMDoesNotDropLeadingFunc`) | Guard the BOM fix. |

No fake implementations introduced.

## F. Tests

Commands:
```
go test ./pkg/lexer/... ./pkg/parser/... ./pkg/codegen/... ./pkg/pm/... ./pkg/cli/... -count=1
go test ./pkg/sema/... -count=1
```
Results: PASS (all). Borrow-checker regression suite (incl. new param tests): PASS.

## G. Self-Hosting (after fix)

```
Stage 1 (Go compiler → main.kark → C → gcc):   PASS
Stage 2 (self-hosted stage1 builds stage2):     PASS
Argument propagation (run karkain build/run):   PASS
Bitwise identity (stage2 == stage3, byte-identical): PASS
```
Self-hosting was unblocked by fixing the borrow checker, not by weakening it.
Determinism was completed by making the gcc/MinGW link step reproducible.

### Bitwise-identity root cause & fix (added subsequent pass)

The bitwise-identity test (`pkg/bootstrap`) asserts stage2 and stage3 are byte
identical. After the borrow-checker fix, all stages built and the generated C
reached a **fixed point** (stage2-C == stage3-C), but the **binaries differed**.

Root cause (empirically proven): the failure was NOT in Karkain codegen. gcc on
MSYS2/MinGW (GNU ld, binutils >= 2.40) embeds the wall-clock time into the PE
TimeDateStamp of each linked binary unless `SOURCE_DATE_EPOCH` is set, so
compiling the *identical* C file twice (default and `-O0`) produced
byte-different executables. Karkain's codegen was already a deterministic fixed
point.

Fix (`pkg/bootstrap/bootstrap.go`): inject a fixed `SOURCE_DATE_EPOCH`
(`1072915200` = 2004-01-01) into every child process (gcc link, go build) unless
the caller already exports one. Result: stage2 SHA == stage3 SHA
(`4fb8ec32dee06982`), and the full bootstrap suite (args, Stage 1, Stage 2,
Bitwise Identity) is green.

## H. Package Manager

```
resolver        PASS (implemented)
lockfile        PASS (implemented)
fetch           PASS (implemented, atomic)
cache           PASS (implemented, checksum)
build integration NOW WIRED (local deps + sibling modules, see D2)
run integration  NOW WIRED (same resolver as build)
check integration PARTIAL (still single-file; see I)
test integration MISSING (see I)
```

## I. Remaining Blockers (ranked)

- **P0:** (resolved) borrow-checker param false-positive → fixed.
- **P0:** (resolved) PM→compiler gap partially closed — `build`/`run` now include local dependency + sibling sources via `resolveSources`; `check` now validates the same project-aware compile unit. Registry/git deps and a full **pub/private module/import language** still require a language-design decision (whole-program name-resolution pass is a V1 MUST-HAVE that is not yet implemented; full module/visibility is a V1 SHOULD-HAVE gated behind it). Deferred with this evidence, not invented speculatively.
- **P1:** Wire `test`'s per-file runner to a mechanism for test-only sibling/dependency sources (orthogonal to `check`; discovery is per-file); consider registry/git module layout.
- **P1:** (doc-written) `docs/audit/C-ABI.md` formalizes the three-runtime boundary; legacy `src/compiler/runtime.c` confirmed dead/unlinked and the misleading bootstrap param removed.
- **P2:** Registry/JSON + publish tarball; Git fetch; workspace build/test (currently stubs).
- **P3:** Math IR wiring or explicit retirement; SIMD arithmetic vectorization + optimizer; GPU/NPU real execution.
- **P4:** async/await, defer, const, pub, methods/overloading, macros expansion runtime.

## J. Recommended Next Sequence (evidence-based)

1. **P0** — (done) correct borrow checker parameter handling.
2. **P0/P1** — Design a minimal language-level module/import mechanism (automatic dependency discovery from `karkain.toml`/lock without inventing incompatible syntax), then wire `build/run/check/test` to load resolved dependency sources + stdlib + cache. This is the bridge between the package manager and the compiler.
3. **P1** — Formalize runtime + ABI contracts; fold `src/compiler/runtime.c` and codegen's embedded runtime under one documented boundary.
4. **P1** — Build a golden/differential test harness with CPU as oracle before any accelerator parity claim.
5. **P2/P3** — Only after correctness + determinism are locked: registry, git, SIMD arithmetic, GPU/NPU.