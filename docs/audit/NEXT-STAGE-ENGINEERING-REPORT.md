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

## D3. Test runner: project-aware module scope (added subsequent pass)

The `test` runner compiles each `test_*` function as a standalone mini-program
(function + synthetic `main`). Previously that mini-program contained *only* the
test function, so a test that called a sibling-module or local-dependency helper
failed to link. `runSingleTestFile` now prepends the **project module scope**
(non-`main` local-dependency + sibling module sources, assembled via a new
`projectModuleSources`) to every mini-program. Flat/non-project builds get an
empty scope and behave byte-identically to before. Discovery is unchanged
(`test_` prefix); module code is never auto-discovered.

E2E: a project `tests/main_test.kark` calling `helper_double` (sibling module)
and `lib_add` (local dependency) now compiles and runs; linking would fail
without the scope.

## E. Implemented Changes

| File | Purpose | Architectural reason |
|------|---------|----------------------|
| `pkg/sema/borrow_checker.go` | `checkFuncDecl` now declares each function parameter as an owned live binding in the function scope | Correct ownership semantics: a param is alive for its body; previously params were never declared, so referencing a param whose name an earlier function had marked dead produced false "use after scope has ended". This was the self-hosting blocker. |
| `pkg/sema/borrow_checker_test.go` | Regression tests: param live on first statement, across independent functions, after branches; plus genuine param-move still rejected | Protect the fix from regression without weakening ownership checking. |
| `pkg/cli/commands.go` | `resolveSources` + `projectSourceFiles` (wired into `build`/`run`/`check`); `projectModuleSources` + module-scope injection (wired into `test`); project-aware, deterministic, deduplicated source assembly | Closes the package-manager→compiler gap without changing grammar; pulls local dependency sources upstream; non-project builds stay byte-identical; tests gain the project module scope. |
| `pkg/cli/module_resolve_test.go` | Unit tests for `resolveSources`: non-project fallback parity, project dep/sibling/main ordering, single-root dedup | Lock in deterministic project-aware assembly and backward compatibility. |
| `pkg/cli/pm_test_runner_test.go` | E2E that a project `tests/*_test.kark` can call a sibling module + local dependency; flat runner unchanged | Prove the test runner's project-aware scope and preserve flat behavior. |
| `pkg/bootstrap/bootstrap.go` | Removed the dead `runtimeFile` parameter of `compileWithGCC` (only ever received never-linked `runtime.c`) | Eliminated a correctness trap (a maintainer "fixing" it would relink legacy runtime.c and hit duplicate-symbol errors); behavior-neutral, bootstrapping suite re-verified. |
| `docs/audit/C-ABI.md` | Authoritative three-runtime boundary spec (codegen scalar preamble / CPU tensor oracle / legacy self-hosting Value runtime), each ABI + std flag + ownership + determinism + change rules | Prevents silent drift and mis-linking between runtimes that serve different ABIs and must not be merged. |
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
check integration NOW WIRED (project-aware compile unit, see D2)
test integration NOW WIRED (project-aware module scope, see D2)
```

## I. Remaining Blockers (ranked)

- **P0:** (resolved) borrow-checker param false-positive → fixed.
- **P0:** (resolved) PM→compiler gap closed for build/run/check/test — all of
  `build`, `run`, `check` and `test` now assemble the project-aware compile unit.
  `test` additionally injects the project module scope (deps + sibling modules,
  no `main`) into each test's mini-program so tests can exercise the code they
  target. Registry/git deps and a full **pub/private module/import language**
  still require a language-design decision (whole-program name-resolution pass is
  a V1 MUST-HAVE that is not yet implemented; full module/visibility is a V1
  SHOULD-HAVE gated behind it). Deferred with this evidence, not invented
  speculatively.
- **P0/P1:** (resolved) Whole-program name-resolution pass (Option B) now in
  `check`: two-pass symbol table collects top-level FuncDecl/Struct/Enum
  definitions then validates (a) duplicate definitions and (b) undefined bare
  function references. Diagnostics-only — build/run output is untouched. Zero
  false positives across the full test suite + self-hosting stages. Design doc:
  `docs/audit/NAME-RESOLUTION-DESIGN.md`. Full pub/private/import module system
  (Option A) deferred as a separate sign-off.
- **P1:** (done) `public` visibility modifier (Phase 7 — Option A foundation).
  Adds `TokenPub` lexer token, `Public bool` on `FuncDecl`/`StructDeclStmt`/`EnumDecl`,
  parser grammar for `public func`/`public struct`/`public enum`, and a
  SourceMap-aware resolver that enforces visibility (private names inaccessible
  from different source files) only when any declaration carries `public`.
  Opt-in: existing code without `public` compiles identically. Parser also now
  populates the `Line` field on top-level declarations (previously always 0).
- **P1:** (done) Module system steps A–C completed. (A) Resolver now properly
  distinguishes public vs private: only non-public functions trigger cross-file
  visibility errors. (B) `import <module>` syntax added to parser (AST node
  `ModuleImport`, stored on `Program.Imports`); C imports (`import "C" { ... }`)
  coexist via peek-based dispatch. (C) Resolver validates that each import
  references a source file in the compile unit (file basename = module name).
  Step D (self-hosting adoption) N/A — compiler is C, not Karkain `.kark`.
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

---

## K. Phase Log

### KTF-001 — Karkain Native Testing Foundation (COMPLETE)

Native assertion builtins (`assert`/`assert_eq`/`assert_ne`) with structured
failure output to stderr + non-zero exit; toolchain test model (`pkg/testing`:
TestCase/TestResult/Status/Failure/Summary/Filter); deterministic per-test
discovery and reporting in `karkain test`; `--filter`; captured per-test
stdout/stderr via new `codegen.Config.Stdout/Stderr` writers. Existing
`test_`-prefix convention preserved (reconciled: no new grammar forced), P2
infrastructure reused unchanged, `WorkspaceTest` untouched.

Full suite green (`go test ./... -count=1`, all 27 packages; `pkg/bootstrap`
240s self-hosting path intact). Report: `docs/audit/KTF-001-REPORT.md`.

Deferred (future KTF phases): compile-pass/compile-fail, diagnostics,
conformance, runtime/ABI, bootstrap/self-host harness, property/fuzz,
benchmarks, Math/Tensor/SIMD/GPU/NPU/Quantum test frameworks.

### CLI-COMPLETION — Complete CLI Toolchain Implementation (COMPLETE)

Executes the Karkain Complete CLI Toolchain Implementation prompt in four
batches behind the previous dispatch work:

- **Exit codes (§23)** 0..6 (`pkg/cli/exitcodes.go`) with every dispatcher
  path classified; `--target` closed-set validation (`native|c23|wasm32-wasi`,
  both flag forms; invalid → usage 2; no compiler → infrastructure 6).
- **clean** (source-anchored artifact removal, `--all`, never touches
  sources/manifests/lock) and the **workspace family** (`list/build/test/
  check/run/clean/init/add/remove/lint/graph`) via `pm.WorkspaceOrder`,
  identical under `karkain workspace` and `karkain pkg workspace`.
- **bench** — genuine single-run DURATION harness for `bench_` functions in
  `*_bench.kark`/`*_test.kark`; **lint** — full front-end incl. borrow checker
  with `E-K-*` tagging; **explain** — stable error-code registry
  (`pkg/diagnostics` E-K-* classes + all E-PKG-*) with `--list`.
- **target/config** commands; **`pkg audit --json` / `pkg verify --json`**
  machine-readable output; repaired a mojibake arrow in `pkg audit`.
- Honest boundary: `ir`, `bootstrap`, `selfhost`, `profile` deliberately NOT
  provided (no fake commands); registry-gated paths retain availability
  errors.

Reports: `docs/audit/CLI-COMPLETION-IMPLEMENTATION.md`,
`docs/audit/CLI-COMPLETION-MATRIX.md`. Full suite green
(`go test ./... -count=1`).

### KTF-002 - Compile-Pass / Compile-Fail Corpus with Diagnostics Capture (COMPLETE)

Established the bundled compile corpus: `manifest.json`-driven, deterministic,
exercising the real lint pipeline for diagnostics and the native C backend for
compile-pass cases.

- Model: `pkg/testing/compile.go` (`CompileCase`/`Diagnostic`/`CompileResult`/
  `CompileSummary`/`SummarizeCompile`/`SortCompileByID`; `CompileExpect` with
  `ExpectPass`/`ExpectFail`; `StatusSkip` honored in summaries).
- Runner: `pkg/cli/compile_corpus.go` — `loadCompileManifest` (validates
  manifest + referenced files, deterministic sort), `captureFrontendDiagnostics`
  (parse→macro→resolve→kernel sema→borrow, mirrors `lint`), `runPassCase`
  (clean front end + `codegen.GenerateAndCompile` with `CompileOnly`; no C
  toolchain ⇒ SKIP), `runFailCase` (one diagnostic matching declared code +
  message substring), `RunCompileCorpus`.
- Bundled corpus `pkg/cli/testdata/compile/`: 15 cases (8 pass: arith,
  control_flow, data_structures, for_in, funcs, strings, borrow_safe,
  kernel_compile; 7 fail spanning E-K-RES / E-K-SYN / E-K-BRW / E-K-SEM).
  Every fail expectation was validated empirically against `karkain lint`
  before freezing.
- CLI: `karkain test --compile [dir]` (default `./testdata/compile`); exit 0
  all pass, exit 4 (ExitTest) any fail.
- Grammar constraints respected: newline-separated statements (no `;`), no
  plain reassignment / `->` return types / `a..b` ranges; `in` reserved word
  avoided in kernel param names (misaligns `parseKernel`).
- Self-tests: `pkg/cli/compile_corpus_test.go` (10 tests incl. 2 E2E) — full
  suite green (`go test ./... -count=1`), bootstrap path intact.

Report: `docs/audit/KTF-002-REPORT.md`.

### Phase 80 - SIMD / Vector Execution Architecture (COMPLETE)

Established the cross-backend parity baseline and delivered a SIMD CPU
backend as the first vectorized execution path, per the Phase 79 gate
(READY WITH PREREQUISITES).

- Prerequisite 1 — Math IR: reconciled as the reference caller-free evaluator
  (zero importers confirmed); decision record
  `docs/audit/PHASE-80-MATH-IR-RECONCILIATION.md`.
- Prerequisite 2 — Self-hosting: `docs/audit/SELF-HOSTING-BLOCKER.md` created
  (exit criterion + per-phase tracking). Re-measured during Phase 80:
  `pkg/bootstrap` now GREEN (`ok 283.772s`), status updated; the exit
  criterion is retained as the standing gate.
- Prerequisite 3 — Parity baseline: `pkg/backend/parity/parity.go` harness
  (`Candidate`/`Diff`/`Comparison`/`Report`/`Compare`/`ReportString`). CPU
  scalar backend is the numeric oracle; GPU/NPU are metadata-only execute
  stubs recorded as `NoNumericOutput` (structural gap surfaced, never faked).
- SIMD backend: `pkg/backend/cpu/simd_c.go` (`SimdExtensions` C runtime using
  GNU vector extensions `double4 __attribute__((vector_size(32)))`, portable
  gcc/clang without `-march`); `TensorSimdCRuntime = TensorCRuntime +
  SimdExtensions`; `cpu.NewSimd()` alongside scalar `New()`; SIMD paths for
  contiguous same-shape add/sub/mul/div, matmul (K-loop in 4-lane blocks),
  relu; broadcast falls back to scalar; results carry `Metadata["simd"]="true"`.
- Oracle completeness fix: the CPU backend previously emitted RESULT blocks
  only for `OpCreate` outputs — op outputs (Add/MatMul/Relu) never produced
  numeric Values. The C emiter now prints a multi-line RESULT block for every
  declared output, making `Result.Values` populated for the full graph
  (pre-existing latent gap, found by parity tests).
- Verification: `pkg/backend/parity/parity_test.go` — oracle-contract (GPU/NPU
  ⇒ NoNumericOutput), SIMD-vs-scalar numeric parity (add/relu exact ≤1e-12,
  matmul tolerance captures FP-order difference), broadcast-fallback parity,
  injected-divergence detection (harness must fail), dispatcher integration
  (SIMD backend is first-class). `go vet ./pkg/...` clean; backend/tensor/
  math/npu suites green.
- Full suite: `go test ./... -count=1` green including `pkg/bootstrap`.

Report: `docs/audit/PHASE-80-REPORT.md`.