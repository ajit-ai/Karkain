# Karkain Development Conventions

## Branch Workflow (MANDATORY RULE)

After EVERY Phase completion and successful test run:
1. Commit all changes to `develop` branch
2. Merge `develop` into `main` branch
3. Push both branches to origin

This ensures `main` always reflects the latest working state.
NEVER skip this step. This is a hard rule, not optional.

## Roadmap

See `ROADMAP.md` for the complete development plan (Phases 50–79+).
Current phase: **post-97** — Phases 50–97 complete.
Also completed: **97** — Default kcc Engine + Manifest Dependency Resolution
(kcc is now the DEFAULT engine: `EngineFromEnv` returns `EngineKCC` for empty
env/unrelated `KARKAIN_ENGINE` values, Go selected only via explicit
`KARKAIN_ENGINE=go|Go|GO`; `--engine go|kcc` unchanged; manifest dependency
resolution wired into the source-assembly pipeline: new `kccAssembleSource`
(local deps → siblings → root last) feeds `KCCCheckCommand`/`KCCBuildCommand`/
`KCCRunCommand` temp-sandbox builds, `projectModuleSources` prepended to each
test driver in `KCCTestCommand` (dependency-aware test compilation), and
`resolveSources` (check/build/run) + `projectModuleSources` (test) pull
`pm.DependencySources` entries (local & workspace) for all engines; bootstrap
pipeline pins `KARKAIN_ENGINE=go` (`forceGoEngine` in `pkg/bootstrap`) so stage-1
still emits `src/compiler/main.c` through the Go front end; E2E exit-code tests
pin Go deterministically; parity gate `pkg/cli/phase97_parity_test.go` (6 tests:
default-engine flip, explicit Go fallback, local & workspace deps in assembly,
end-to-end kcc run using a dependency function, shared check/build/run/test
dependency-aware pipeline), plus existing parity gates (95/96) green and
`TestBootstrap_BitwiseIdentity` stage2==stage3 bitwise identical; docs under
`docs/audit/PHASE-97-FINAL-REPORT.md`).
Also completed: **96** — Self-Hosted kcc Owns the Test Runner
(kcc is the primary engine for `karkain test`: self-hosted discovery in
`src/compiler/main.kark` (`collectTestFiles` mirrors Go `findTestFiles` —
recursive `*_test.kark` discovery, sorted, single-file fallback, empty
directory reports "No test files found."), per-test driver synthesis
(parse → collect `test_*` → generated `main` calling selected tests),
gcc compile+run with per-test PASS/FAIL, full-file fast path, `--filter`
substring support, and Go-parity summary output (`N passed; M failed;
S skipped; T total`) parsed by `KCCTestCommand` (`karkain test
--engine=kcc` / `KARKAIN_ENGINE=kcc`) which maps `failed>0` → `ExitTest(4)`
and runs kcc in a temp sandbox; root-cause fixes: `INT==BOOL` `values_equal`
mismatch from `endsWith(...) == true` → bare truthiness for INT/BOOL, Windows
`system("./x")` → bare `.exe` name, empty-directory vs single-file
classification via `readFile`/suffix probe; parity gate
`pkg/cli/phase96_parity_test.go` (conformance parity 59 assertions, failing
test, single-file+filter, empty directory); bootstrap identity
`TestBootstrap_BitwiseIdentity` stage2==stage3 bitwise identical
(stage2/stage3 SHA `f39111a2…`) and conformance corpus 59/59 all pass;
docs under `docs/audit/PHASE-96-FINAL-REPORT.md`).
Also completed: **95** — Self-Hosted kcc Owns the Core Pipeline
(self-hosted compiler from `src/compiler` is the primary engine for
lex+parse+sema+C codegen; `karkain check/build/run --engine=kcc` or
`KARKAIN_ENGINE=kcc` decompiled through kcc; `pkg/cli/kcc_engine.go` with
`KCCCheckCommand`/`KCCBuildCommand`/`KCCRunCommand` (temp-sandboxed run), a
staleness check rebuilding `kcc.exe` when any `src/compiler/*.kark` is newer,
and Go fallback for LSP/test-runner/exotic backends; CLI `--engine go|kcc`
flag + env wiring in `cmd/karkain/main.go`; kcc parity fixes for type-keyword
annotations, unparenthesized if, map/struct literals, slices, index/member
assignment, assertions, appendArray Value* and struct-decl comments;
parity gate `pkg/cli/phase95_parity_test.go` proving kcc reproduces all 12
probe goldens + all 11 conformance files (59 assertions); verified
`TestBootstrap_BitwiseIdentity` stage2==stage3 bitwise identical;
docs under `docs/audit/PHASE-95-FINAL-REPORT.md`).
Also completed: **94** — SSA Optimization Pipeline
(Multi-pass optimizer: Mem2Reg, FoldConst with algebraic simplification,
CSE, DCE, LICM for natural loops; Pipeline orchestrator with fixpoint
iteration and stats; 49 passing tests including benchmarks; `pkg/ir/ssa/`).
Also completed: **93** — Typed SSA IR
(TypeRegistry, TypeBits, TypePromote, TypeIsCompatible, dominance tree
(Cooper et al.), liveness analysis, SSA verifier; 30 tests; `pkg/ir/ssa/`).
Also completed: **92** — HIR Infrastructure
(High-Level IR with typed nodes: 20 expression kinds, 16 statement kinds,
19 type kinds, AST→HIR lowering via BuildHIR(), Format() for debug output,
7 passing tests including 5-program round-trip; `pkg/ir/hir/`).
Also completed: **91** — Comprehensive Validation & Release Decision
(audit of all documentation for consistency: SPEC.md version 0.14.0→1.0.0,
self-hosted compiler reference Phase 56→88; stdlib.md import examples updated
for async/gpu modules; full regression suite GREEN: lexer/parser/sema/ssa/pm/
codegen/backend/npu/source/module/diagnostics all pass, CLI passes with 300s
timeout for conformance corpus; release decision: KARKAIN 1.0 — RELEASE READY;
`docs/audit/PHASE-91-FINAL-REPORT.md`).
Also completed: **51** — Borrow Checker Lexical Scoping
(scope-stack identity hardened for shadowing, borrow reversion via declaring scope,
use-after-scope-end diagnostics, escape-analysis flag wiring, Groups A–I tests).
Also completed: **81** — Compiler Correctness (immutable/mutability semantics,
escape-analysis wiring, executable probes under `examples/phase81-probes/`,
diagnostics hardening; unparenthesized `if` + single-path `%`).
Also completed: **82** — Language Conformance, Examples & Developer Tooling
Foundation (native conformance corpus `conformance/` — 9 files, 48 `func test_*`
tests through the real front end + C runtime, self-contained file scope via
`testFileOwnScope`; deterministic probes corpus `examples/probes/` — 11 golden
output programs enforced by `pkg/cli/probes_corpus_test.go`; algorithm corpus
grown to 21 dirs (factorial/fibonacci/gcd/lcm/sieve/power/absolute_value);
toolchain contract `check --format=json` (`karkain-diagnostics-v1`, real parse
columns via `Parser.ErrorCols`) + `fmt`/`fmt --check` (idempotent token-level
canonicalizer); real LSP served by `karkain lsp`/`language-server`; `ide info`
JSON contract; VS Code extension rebuilt (`extension.js` commands check/compile/
run/format, fixed manifest, corrected grammar) validated by
`pkg/cli/vscode_extension_test.go`; CI runs conformance + format checks; docs
under `docs/audit/PHASE-82-*`).
Last completed: **83** — Compiler Symbol Namespacing, True Diagnostic Spans &
LSP↔CLI Pipeline Sharing (deterministic `karkain_user_*` C namespace for user
functions — Go `userFuncC` + self-hosted `codegen.kark` mirror — fixes
C-library collisions like `abs` and makes user functions that shadow builtins
win; `E-K-RES` true columns: lexer 0-based byte columns, parser `Col`/`EndCol`
spans surviving macro expansion, optional `endColumn` + `excerpt` fields in the
`karkain-diagnostics-v1` contract; new `pkg/source` line-index/excerpt package;
single `cli.AnalyzeSource` driver shared by `karkain check` and the LSP with
real-time `didChange` diagnostics sync; conservative undefined-identifier
resolution; array-return semantics; conformance corpus 48→59 tests in 11 files
+ `namespace` probe golden (12 total); docs under
`docs/audit/PHASE-83-FINAL-REPORT.md`).
Last completed: **94** — SSA Optimization Pipeline
(Multi-pass optimizer: Mem2Reg, FoldConst with algebraic simplification,
CSE, DCE, LICM for natural loops; Pipeline orchestrator with fixpoint
iteration and stats; 49 passing tests including benchmarks; `pkg/ir/ssa/`).
Last completed: **93** — Typed SSA IR
(TypeRegistry with 14 primitive types, TypeBits/TypePromote/TypeIsCompatible
type-lattice, Cooper et al. iterative dominance tree (DomTree with IDom,
DomChildren, Dominates/StrictlyDominates/DominanceFrontier/DomTreeDepth),
iterative liveness analysis (LiveRange, Interferences, NumLiveAt),
SSA verifier (undefined register, unterminated block, undefined block target),
30 passing tests across `typed.go`, `dom.go`, `liveness.go`, `typed_test.go`;
`pkg/ir/ssa/`).
Last completed: **90** — Production Release Gate / Karkain 1.0
(production build workflow verified, CLI commands validated: check/build/fmt/lint,
stdlib imports work, runtime type discrepancy resolved as intentional bootstrap
limitation, release acceptance test, 15 focused Go tests, version 1.0.0 consistent,
40+ total tests pass; `docs/audit/PHASE-90-FINAL-REPORT.md`).
Prior completed: **89** — Self-Hosted Runtime & Toolchain
(Karkain-owned runtime boundary with Value type system, container ops,
I/O primitives, platform abstraction, initialization contract;
runtime/ directory with 5 boundary docs; acceptance test proving
self-hosted compiler → C23 → gcc → native executable pipeline;
12 focused Go tests; `docs/runtime.md`; `docs/self-hosted-compiler.md`).
Prior completed: **88** — Self-Hosted Compiler Foundation
(self-hosted Karkain compiler compiles via bootstrap: src/compiler/*.kark
→ C23 → native executable; lexer/parser/AST/sema/codegen in Karkain;
7 representative test programs; 8 focused Go tests; build script;
docs under `docs/self-hosted-compiler.md`).
Prior completed: **87** — Standard Library Foundation
(stdlib structure with core/string/collections/math/io/system modules,
`std.*` import resolution wired into module graph via `findStdlibDir`,
6 focused module-resolution tests; `docs/stdlib.md`).
Prior completed: **86** — Developer Toolchain
(test timeout safety via goroutine+select with 30s default, multi-file
formatter (`karkain fmt .` recursive), exported `Canonicalize` for LSP
reuse, `textDocument/formatting` LSP support, `ExitLint(7)` exit code,
8 focused tests; docs under `docs/audit/PHASE-86-FINAL-REPORT.md`).
Prior completed: **85** — Native Build & Linking Integration
(NativeBuilder orchestrating Object→Linker→Executable pipeline via
`NativeBuilder.Build` and `BuildMultiObject`; CLI integration:
`--target=native-link` flag dispatches `BuildCommand`/`RunCommand` to
`nativeBuildCommand`/`nativeRunCommand`; KOBJ binary artifact format
(sections/symbols); debug address relocation from 0-based counters to
actual linked .text base (0x1000+) via `relocateDebugAddresses`;
SourceAddressMap built from relocated DebugInfo; 8 focused NativeBuilder
tests + 6 CLI integration tests; docs under `docs/native-codegen.md`).
Prior completed: **84** — Native Codegen, Linker & Debug Information
(Karkain-owned object model: sections/symbols/relocations/debug-info;
scope-aware SymbolTable with `karkain_user_*` namespacing via
`CollectSymbolsFromAST`; RelocationManager with Addr32/64, PCRel32/64,
PLT32, GOT32 and byte-level application; Linker (symbol resolution,
section layout, entry-point handling, Executable production);
DebugInfoBuilder + SourceAddressMap bidirectional source↔address mapping;
CodegenDiagnostics wrapper reusing Phase-83 `E-K-CG` diagnostics;
NativeGenerator.GenerateObject AST→Object path; 48 focused tests across
6 new test files; docs under `docs/native-codegen.md`).
Prior completed: **79** — Compiler Integrity, IR Architecture & Self-Hosting
Readiness Audit (evidence-based audit: pipeline, dependency map, Math/Tensor/SSA
IR, CPU/GPU/NPU parity, determinism, optimization boundaries, tests,
BUG-1..8 regression, self-hosting readiness; Phase 80 gate = READY WITH
PREREQUISITES; docs under `docs/audit/PHASE-79-*`).
Prior completed: **78** — NPU Optimization (fusion, memory planning, INT8/INT4 quantization, MLIR codegen);
**70-78** Math/Tensor/NPU chain (see below); **52-69** value/SSA/closures/slices/self-hosting/actors/GPU/quantum/stdlib/borrow/optimizer.

### Completed: Package Manager Hardening (P0)
Deterministic resolver (`pkg/pm/resolver.go`), lockfile-integrated workflows
(`pkg/pm/flow.go`: `ResolveAndLock`, `FetchLocked`, `ResolvedDetails`,
`TreeLines`, `NewProject`), fetch cache safety (atomic temp→rename + checksum in
`FetchModule`).

### Completed: CLI Toolchain (Batches B1–B4) + KTF-001 + KTF-002
Full CLI surface (`bench/lint/explain/clean/workspace/*/target/config/pkg audit
--json/pkg verify --json` + exit-code scheme + `--filter`), the native test
foundation (`karkain test`: language-level `assert/assert_eq/assert_ne`, KTF-001
model, deterministic discovery/execution, `--filter`), and the compile
pass/fail corpus (`karkain test --compile [dir]`: manifest-driven, real lint
pipeline diagnostics, gcc-gated pass cases, exit 4 on failure). Reports:
`docs/audit/KTF-001-REPORT.md`, `docs/audit/KTF-002-REPORT.md`,
`docs/audit/CLI-COMPLETION-IMPLEMENTATION.md`, `docs/audit/CLI-COMPLETION-MATRIX.md`. CLI wiring under single `karkain.exe`: top-level
`new/remove/update/list/tree/fetch` (incl. `karkain pkg ...` aliases);
`update` now re-resolves and writes `karkain.lock`; `fetch` uses the lockfile.
Registry/git fetching remain explicit "not available" errors (not faked).
Tests: `pkg/pm/resolver_test.go`, `pkg/cli/package_cli_test.go` (E2E). Audit:
`docs/audit/PACKAGE-MANAGER-AUDIT.md`,
`docs/audit/PACKAGE-MANAGER-IMPLEMENTATION.md`. Deferred recommended next step:
wire manifest dep resolution into build/run/check (avoided to keep compiler
pipeline stable).

### Completed: Math/Tensor/NPU Chain (Phases 71–78)

```
71 (Math IR) → 72 (Tensor IR) → 73 (CPU Backend)
                                      ↓
                                 74 (Autodiff Integration)
                                      ↓
                                 75 (Backend Abstraction)
                                    ↓        ↓
                               76 (GPU)   77 (NPU) → 78 (NPU Opt)
```
All 8 phases built, tested (full suite green), committed and pushed to `main`.
Backends: CPU reference (`pkg/backend/cpu`, embedded C23 runtime, correct GCC E2E),
GPU/WGSL (`pkg/backend/gpu`), NPU abstraction + 5 vendor adapters (`pkg/npu/*`).
NPU optimization passes: fusion, memory planning, quantization, Karkain-owned MLIR dialect.

### Phase Dependency Chain (Math/Tensor/NPU)

```
71 (Math IR) → 72 (Tensor IR) → 73 (CPU Backend)
                                      ↓
                                 74 (Autodiff Integration)
                                      ↓
                                 75 (Backend Abstraction)
                                    ↓        ↓
                               76 (GPU)   77 (NPU) → 78 (NPU Opt)
```

### Key Design Decisions

1. **No Tensor keyword** — NPU/math ops work on existing `array` types
2. **No Google TPU** — NPU targets Intel/Qualcomm/Apple/AMD/Arm only
3. **Math IR is Karkain-owned** — not ONNX, not MLIR, not vendor-specific
4. **Tensor IR is Karkain-owned** — not NumPy, not PyTorch
5. **CPU is reference backend** — correctness oracle before any accelerator
6. **NPU is a backend** — not the foundation, not the language

## Guiding Principles

1. **Maturity over features** — depth, correctness, performance, verification before expansion
2. **Semantic foundations first** — value representation, ownership, lifetimes, IR before features
3. **Working > ambitious** — fix broken fundamentals before adding new capabilities
4. **Incremental verification** — every phase must compile, pass E2E, pass all tests

## Testing

Run the full test suite before committing:
```
go test ./pkg/lexer/... ./pkg/parser/... ./pkg/codegen/... ./pkg/pm/... -count=1
```

All tests must pass before committing.


