# Karkain 1.1.0 — Maturity & Capability Gap Matrix

## 1. Audit Scope

| Field | Value |
|---|---|
| Repository | `F:/Codes/Git/Karkain` |
| Branch | **`main`** (audit brief said `develop` — see Evidence Conflict #1) |
| HEAD at analysis start | `6162db448a66c11577ee08b4deba01b76916cb2d` |
| HEAD at document finalization | `29ad61ed406718924b3687715f5fa48ca48dff63` (see Reconciliation note) |
| VERSION file | `1.1.0` (`VERSION`) |
| Tag `v1.1.0` | Present; **HEAD is 68 commits ahead of it** |
| Audit date | 2026-10-01 |
| Language source extension | `.kark` |
| Working tree at audit start | **Clean** (`git status --porcelain` empty) |

### 1.1 What was inspected

Repository source and configuration only:

- `pkg/` (24 packages; ~59 kLOC non-test Go), `src/compiler/` (16 `.kark`, 14 139 lines), `compiler/` (5 `.kark`, 2 584 lines), `stdlib/` (15 modules), `runtime/` (3 C layers), `examples/` (220 `.kark`), `conformance/` (12 files / 64 `test_` functions), `.github/workflows/ci.yml`, `SPEC.md`, `docs/` (115 `.rst`, 140 audit `.md`).
- Behavioural verification by executing the prebuilt `./karkain.exe` on **both engines** (`--engine go`, `--engine kcc`) on small scratch programs in `%TEMP%`. Scratch files were outside the repository; the working tree was re-checked clean afterwards.

### 1.2 What was NOT inspected

- **No test suite was run.** The host has ~300–700 MB free RAM of 4 005 MB; two earlier `go test` batches were killed mid-run. Per the brief, validation was limited to document syntax and referenced-path existence.
- No cross-compilation, release-matrix, or container execution.
- No network calls (the default registry endpoint was not probed).
- External comparison columns (C, C++, Rust, Go, Zig, Swift) come from authoritative public documentation and are marked `Not verified` where no authoritative statement was available.

### 1.3 Evidence conflicts declared up front

**Conflict #1 — branch and baseline mismatch.** The brief states branch `develop` and "do not assume unreleased v1.2.0 work is part of the baseline." Reality: HEAD is on `main` and contains **68 commits of unreleased v1.2.0 work** (increments 150A–150D, 151/151A/151B/151C/151C2/151C3/151P0, plus LH-1/LH-2). `VERSION` still reads `1.1.0`, so the *version identity* is 1.1.0 while the *tree* is well past it.

This audit therefore reports two baselines throughout:

- **[1.1.0]** = what the `v1.1.0` tag actually shipped.
- **[HEAD]** = what is in the tree today.

Any row differing between them is marked. This is not cosmetic: `v1.1.0` shipped **silently mis-compiled bitwise operators** (commit `fc82806 fix(151P0): implement bitwise operators, which were silently mis-compiled`) — a soundness defect in the released tag, fixed only in [HEAD].

**Conflict #2 — dead duplicate compiler tree.** `compiler/` (root, 5 files, 2 584 lines) and `src/compiler/` (16 files, 14 139 lines) both exist. Only `src/compiler/` is referenced by the production pipeline. `compiler/` survives solely because two tests assert its files *exist* (`pkg/cli/bootstrap_test.go:18-31`, `pkg/cli/phase119_stable_test.go:88-99`) without compiling them.

**Conflict #4 — concurrent repository activity during the audit.** Between the start and the finalization of this audit, HEAD moved from `6162db4` to `29ad61e` and the working tree acquired uncommitted changes that this audit did not make: `.github/workflows/ci.yml`, `cmd/karkain/main.go`, `pkg/cli/native_encode.go`, `pkg/cli/phase122_pipeline_ownership_test.go`, `src/compiler/main.kark`, `src/compiler/native_value.kark`, plus a new untracked `pkg/cli/phase151a3_loop_test.go` — consistent with in-flight 151A Step 3 (loop/control-flow) work on the native backend.

Every citation in this document was re-validated against the tree at finalization: all 79 `path:line` citations resolve to existing files and in-range lines, and the files the concurrent actor edited (`src/compiler/main.kark`, `src/compiler/native_value.kark`) were spot-checked to confirm the cited constructs still say what is claimed. The headline finding was also re-executed end to end and still reproduces. Readers should nonetheless re-verify any row before acting on it, because the tree is moving.

**Conflict #3 — docs overstate the type system.** `SPEC.md:63` lists `bigint`/`bigfloat` as language literal kinds and `SPEC.md:93` lists `bigint` as a type, with no mention that the default engine cannot compile them (see A5).
---

## 2. Current Architecture

The real compile path, from `pkg/cli/commands.go:37-108` and `pkg/codegen/codegen.go:352-371`:

```
.kark source
   |- resolveSourcesCheck      pkg/cli/commands.go:37   multi-file assembly + deps
   |- parseSourceWithErrors    pkg/cli/commands.go:46   pkg/lexer -> pkg/parser -> AST
   |- runBorrowCheck           pkg/cli/commands.go:61   pkg/sema/borrow_checker.go
   |- runSemanticPreflight     pkg/cli/commands.go:72   pkg/sema/resolve.go
   |- GenerateAndCompile       pkg/codegen/codegen.go
   |    |- per function: SSA lower -> FoldConstants -> SimplifyCFG -> DCE
   |    |                -> Verify -> emit C23        pkg/codegen/emit_ir.go:315-343
   |    `- legacy direct AST->C fallback on failure    pkg/codegen/codegen.go:370
   |- C23 text (+ embedded runtime preamble)
   `- gcc / clang / cl -> executable
```

**There is no single canonical IR on the primary path.** Three IR-ish subsystems exist with very different wiring:

| Subsystem | Location | On the compile path? |
|---|---|---|
| SSA (per-function emission) | `pkg/ir/ssa` + `pkg/codegen/lower.go` | **Yes**, for non-closure functions (`codegen.go:364`) |
| SSA full optimizer pipeline | `pkg/ir/ssa/pipeline.go:20-31` (6 passes) | **No** - `NewPipeline()` is called only from `pkg/ir/ssa/passes_test.go:367` |
| HIR | `pkg/ir/hir` (`BuildHIR`, `build.go:9`) | **No** - imported by nothing outside its own package |

The optimizer that actually runs is three passes only (`emit_ir.go:337-339`): `FoldConstants`, `SimplifyCFGPass`, `EliminateDeadCode`. `Mem2Reg`, `CSE`, `LICM` and `SROA` are implemented and unit-tested but **unwired**, matching the Phase 141 record (`codegen.go:356-361`).

A **second, independent frontend** exists: the self-hosted compiler `src/compiler/*.kark` (14 139 lines of Karkain) which lexes, parses, type-checks and emits C for the same language, and which is the **default engine** for `check`/`build`/`run`/`test` (`pkg/cli/kcc_engine.go`, `EngineFromEnv`). It has its own 3-stage bootstrap (`pkg/bootstrap/bootstrap.go:155-157`). Its own IR is KIR - a deterministic *text rendering* of the AST (`src/compiler/kir.kark`), not an analysis IR.

A **third, C-free backend** exists: `pkg/native` (7 793 lines) emits ELF/PE/Mach-O machine code directly from the Go AST with no C compiler. At [HEAD] it covers the full value model (ints, floats, arrays, `for-in`, arena/strings, `push`, records, maps, register allocation); at **[1.1.0]** it was a partial int/string/control-flow subset (increment 150 is unreleased).

---

## 3. Maturity Matrix

Status key: **Mature** (implemented + tested + no known defect found) · **Implemented** · **Partial** (works for a subset) · **Experimental** · **Structural Only** (data structures/validation exist, no working execution) · **Missing** · **Not Verified**.

### 3.1 Language and type foundation

| Area | Capability | Status | Evidence | Tests/Evidence | Missing/Gap | Dependencies | Priority |
|---|---|---|---|---|---|---|---|
| A1 | 64-bit signed `int` | Implemented | Width decided in the C preamble: `long long intVal` `pkg/codegen/codegen.go:1004`; kcc mirrors `src/compiler/codegen.kark:714` | Both-engine corpus | No `int` type object in the checker - width is a C-union property, not a type-system fact | - | - |
| A2 | `float64` | Implemented | `TYPE_FLOAT64`, `double floatVal` `codegen.go:1003-1005` | Corpus both engines | - | - | - |
| A3 | `bool` | Implemented | `TYPE_BOOL` `codegen.go:1002` | Corpus | - | - | - |
| A4 | `string` | Implemented | `TYPE_STRING` `codegen.go:1003` | Corpus | - | - | - |
| A5 | `bigint` / `bigfloat` | **Partial - Go engine only** | Lexed `pkg/lexer/lexer.go:447-460`; GMP `mpz_*` `codegen.go:1908-1930`; `-lgmp` `cross_target.go:111`. kcc enum omits `TYPE_BIGINT` (`src/compiler/codegen.kark:708`); its `Value` has no `mpz_t` | **Verified divergent:** Go prints `100000000000000000000`; kcc gives `error[K102] undefined identifier 'n'`, exit 3 | No arbitrary-precision arithmetic on the default engine; `SPEC.md:63,93` does not note this | A13 | High |
| A6 | Unsigned / 128-bit integers | **Missing** | No `uint`/`u64`/`i128` token in `pkg/lexer`; zero matches for `__int128` in `pkg/` | Verified: `let a: u32 = 5` compiles, exit 0 | No unsigned arithmetic, no fixed-width integers | A1 | High |
| A7 | Type annotations | Partial | Stored as bare strings in `VarDeclStmt.Type`, never consulted by width-sensitive code | Verified: `u32`/`u64`/`i32`/`i64` accepted and inert | Annotations are documentation, not semantics | A6 | High |
| A8 | Arrays | Implemented | `make_array`/`array_push` `codegen.go:3641`; elements are `Value**` `codegen.go:1010` | Corpus both engines | Heterogeneous and growable; no static element type | A1 | - |
| A9 | Records / structs | **Partial - runtime maps** | `genStructLiteral` emits `make_map()` + `map_set` `codegen.go:4333-4346`; `genStructDecl` emits **only a C comment**, no `typedef` `codegen.go:4241-4252` | Corpus | No static layout, no field-type checking, no nominal typing; `Point{ zzz: 1 }` is not rejected | Type system | High |
| A10 | Maps | Implemented (Go) / **Divergent (kcc)** | Parallel key/value arrays `codegen.go:1009-1017`; `map_get` `codegen.go:1497`. kcc key comparison is string-only via `strcmp` in **three** helpers: `map_get` `:745`, `map_set` `:746`, `karkain_hasKey` `:855` | Corpus uses **string keys only** | **Verified silent wrong answer:** `let m = {1:"one",2:"two"}; print(m[1]); print(m[2]); print(len(m))` gives Go `one/two/2`, kcc `two/two/1`. Both exit 0 | A1 | **Critical** |
| A11 | First-class `fn` values | Partial | `TYPE_FUNC` `codegen.go:999`; `funcVal{code,env}` `codegen.go:1027`; `karkain_call_fn` `codegen.go:2070`; `IndirectCallExpr` `pkg/parser/ast.go:163` | `examples/closures/` | Bare (non-lambda) function references broken on both engines | A9 | Medium |
| A12 | Enums + `match` | Partial | Enum decls and `match` parse and lower; `MatchArm` `pkg/ir/hir/build.go:192` | Corpus `14_adt_match` | Tag-only patterns; **no payload destructuring**; no exhaustiveness checking | A9 | Medium |
| A13 | Type checking / inference | **Partial** | `pkg/sema/resolve.go` (882 lines) is primarily name resolution plus annotation matching; kcc has a conservative checker `src/compiler/checker.kark` (1 650 lines) | `pkg/sema` suites | No type lattice, no unification, no inference for unannotated `let`; struct field types unchecked | - | High |
| A14 | Name resolution / scope | Implemented | `pkg/sema/resolve.go`, kcc `collectDecls`/`collectLocals` `src/compiler/checker.kark` | Corpus both engines | - | - | - |
| A15 | Module visibility | Implemented | `public` modifier parsed by both engines; kcc `TK_PUB` | Phase 103 gate | Private cross-module calls rejected on both | D1 | - |
| A16 | Compile-time diagnostics | Implemented | Span-carrying diagnostics, `karkain-diagnostics-v1` JSON, `pkg/diagnostics`; kcc `error[K1xx]` with line | Phase 83/105 gates | - | - | - |

### 3.2 Memory model

Karkain specifies no Rust-style ownership or lifetimes. What exists is documented and recorded here without inference.

| Area | Capability | Status | Evidence | Tests/Evidence | Missing/Gap | Dependencies | Priority |
|---|---|---|---|---|---|---|---|
| B1 | Ownership tracking | Partial | `VarOwnership` enum (`Owned`/`Borrowed`/...) `pkg/sema/borrow_checker.go:5-20`; `linearTypes` map `:79` | `pkg/sema` suites | **Lexical, not semantic**: scope-stack identity with shadowing; no borrow of heap data, no lifetimes | B2 | Medium |
| B2 | Borrow checker | Partial | `BorrowChecker.Check` `:167`; three passes: collect linear types, check ownership/borrow, verify linear use `:172-188` | Phase 51 gate | No aliasing rules, no use-after-free detection at runtime, no `&mut` | B1 | Medium |
| B3 | Escape analysis | **Structural Only - dead** | Written `pkg/parser/escape.go:28` (`vd.Escapes = true`); read by `borrow_checker.go:404` into a field never used for any decision; **`pkg/codegen` never reads `Escapes`** (only unrelated comments) | `pkg/parser/escape_test.go` tests only | Computed and discarded. No stack-vs-heap decision is influenced | - | Medium |
| B4 | Heap allocation | Implemented | C runtime allocator `runtime/core/` `karkain_mem`; arena `runtime/freestanding/karkain_memory.c` | Phase 101/102 runtime gates | - | - | - |
| B5 | Deallocation | **Missing** (language level) | No GC, no refcounting, no language-level `free`. `karkain_mem` is an arena/coalescing free list for the runtime's own use | - | User allocations are not reclaimed by any language-visible mechanism | B4 | High |
| B6 | Move semantics | **Missing** | No move/copy distinction in the AST or checker | - | All values are copied by `Value`; no move | B1 | Medium |
| B7 | References / `&` | Partial | `&` is the **Phase 41 borrow operator in prefix position**; infix `&` is bitwise-and (added [HEAD], commit `fc82806`) | Phase 51 gate | No reference types, no reference lifetime | B1 | High |
| B8 | Aliasing rules | **Missing** | No alias analysis; maps and arrays are heap objects shared by reference | - | Two names for one map alias it silently | B1 | Medium |
| B9 | Safety guarantees | Partial | Runtime checks exist (see M1); no static guarantee | Phase 100 gate | Safety is runtime-only, not proven | B2 | High |

### 3.3 Generics and metaprogramming

| Area | Capability | Status | Evidence | Tests/Evidence | Missing/Gap | Dependencies | Priority |
|---|---|---|---|---|---|---|---|
| C1 | Generic functions | Partial | `pkg/sema/generics.go` (437 lines); `substituteType` `:262-289` | Phase 146 gates | **Textual substitution** only: whole-string, `*T`, `[]T`. Cannot substitute `Map[K,V]` | A13 | High |
| C2 | Generic types | Partial | `type Name[T] struct` parsed; monomorphize `pkg/sema/monomorph.go` (340 lines) | Phase 146b stdlib generics gate | Same textual limit; struct bodies are maps (A9) so generic structs gain little | C1, A9 | High |
| C3 | Type parameters | Implemented (v1) | `func name[T,U](...)`; explicit type args at call sites `f[int](args)` because `<`/`>` collide with comparisons | Phase 146 gates | **No inference** in v1 | C1 | Medium |
| C4 | Constraints / traits | **Structural Only** | Constraints parse and store but nothing declares against them | Phase 146d gate | No trait/impl syntax exists to check against; constraints unchecked | C1 | High |
| C5 | Compile-time evaluation | **Missing** | No `const`, no const-fn | - | - | - | Low |
| C6 | Macros | Partial | `pkg/sema/macro.go` (955 lines); `pkg/parser/macro.go` | Phase 82 gates | Expansion-based, not hygienic; operates on the AST | C1 | Medium |
| C7 | Reflection | **Missing** | No runtime type query surface; `ValueType` enum exists in C (`codegen.go:999`) but is not exposed to `.kark` | - | - | A13 | Low |

### 3.4 Modules and libraries

| Area | Capability | Status | Evidence | Tests/Evidence | Missing/Gap | Dependencies | Priority |
|---|---|---|---|---|---|---|---|
| D1 | Modules / imports | Implemented | kcc owns flat assembly itself (`assembleProject` `src/compiler/main.kark`); Go mirrors via `kccStageInput` `pkg/cli/kcc_engine.go` | Phase 122/144 gates | - | - | - |
| D2 | Namespaces | Implemented | Qualified calls resolve against export sets (`checkQualifiedCall` `pkg/sema/resolve.go`); kcc lowers dotted calls to flat `karkain_user_*` | Phase 103 gate | kcc namespaces are flattened, not hierarchical | D1 | - |
| D3 | Multi-file programs | Implemented | Sibling join + `stripModuleImports`; kcc does it itself | Phase 122 gate | - | D1 | - |
| D4 | Library consumption | Implemented | `import std.x`; 15 stdlib modules | Phase 109/131/132 gates | - | D1 | - |
| D5 | Dependency handling | Implemented | `pkg/pm/depsrc.go`, resolver, lockfile `pkg/pm/lock.go` | Phase 97/135/144 gates | - | J1 | - |
| D6 | API stability / versioning | **Missing** | `docs/source/development/semver-policy.rst` describes a policy; **no enforcement mechanism** found in `pkg/` | - | Policy is documentation only | D5 | Medium |

### 3.5 Error handling

| Area | Capability | Status | Evidence | Tests/Evidence | Missing/Gap | Dependencies | Priority |
|---|---|---|---|---|---|---|---|
| E1 | `?` propagation operator | Partial | Parsed; on the native target it is **refused** with `error[K145]` (`src/compiler/native_value.kark`, refusal corpus index 4-5). kcc parity fixed [HEAD] (commit `5dcdd80`) | LH-1 gate | On the C path it lowers; on the C-free native target there is no Result/Option to propagate to | A13 | High |
| E2 | Result / Option values | **Structural Only** | `TYPE_OPTION`, `TYPE_RESULT` exist in the C `ValueType` enum `codegen.go:999` | - | No `.kark`-level construction, inspection, or propagation API found | A13 | High |
| E3 | Exceptions / panic | **Missing** | No `try`/`catch`/`throw`/`panic` anywhere in lexer, parser, or codegen | - | - | - | - |
| E4 | Runtime error reporting | Implemented | Source-located diagnostics: division by zero, bad index; stack frames `runtime error: <kind> at <file>:<line>` | Phase 100/101 gates, both engines | - | - | - |
| E5 | Compile-time error recovery | Implemented | Project-wide multi-error preflight `pkg/cli/multierror.go`; kcc parses each unit independently | Phase 105 gate | - | - | - |
| E6 | Compile-fail corpus | Implemented | `examples/type_errors/` fixtures; `examples/runtime_errors/` | Phase 123 gate | - | - | - |

### 3.6 Concurrency

Language-level surface and runtime facility are recorded separately, as they are genuinely different things.

| Area | Capability | Status | Evidence | Tests/Evidence | Missing/Gap | Dependencies | Priority |
|---|---|---|---|---|---|---|---|
| F1 | `spawn` / `join` / `wait_all` | Implemented | Keyword tokens `pkg/parser/parser.go:737,1644`; builtin lowering table `pkg/codegen/conc_runtime.go:12-16` | Phase 107 gate, both engines | - | - | - |
| F2 | Channels | Implemented | `channel`, `chanSend`, `chanClose`; bounded/unbounded `runtime/concurrency/c/karkain_channel.c` (131 lines) | Phase 107 gate | - | - | - |
| F3 | Actors | Implemented | `actor`, `actorSend`, `actorState`, `setActorState`, `actorStop`; `runtime/concurrency/c/karkain_actor.c` | Phase 107 gate | - | - | - |
| F4 | Scheduler | Implemented (runtime) | Work-stealing scheduler `runtime/concurrency/c/karkain_scheduler.c` (393 lines); generated into programs via `conc_runtime.kark` (928 lines, freshness-gated) | Phase 107 gate, >1M msg/s claim | - | - | - |
| F5 | Atomics exposed to the language | **Missing** | `__atomic` helpers exist **only inside the C runtime** (`runtime/concurrency/c/`); no atomic builtin in `pkg/sema/resolve.go` | - | Language has no atomic or lock primitive | F1 | High |
| F6 | Data-race guarantees | **Missing** | No borrow/ownership interaction with concurrency; no `Send`/`Sync`-style typing | - | Shared mutable state is unconstrained | B2, F5 | High |
| F7 | Parallel execution | Partial (runtime) | Worker threads in the C scheduler | Phase 107 gate | Parallelism is a runtime implementation detail, not a language-level contract | F4 | - |

### 3.7 Runtime, system and FFI

| Area | Capability | Status | Evidence | Tests/Evidence | Missing/Gap | Dependencies | Priority |
|---|---|---|---|---|---|---|---|
| G1 | Layered runtime | Implemented | `runtime/freestanding/` (libc-free), `runtime/core/` (heap, tagged `Value`, `NativeString`, `NativeArray`), `runtime/concurrency/` | Phase 101/102/107 gates | - | - | - |
| G2 | Startup model | Implemented | Generated `main` with frame enter/leave; native backend emits its own `_start` for ELF (`pkg/native/elf.go`) and PEB bootstrap for PE (`pkg/native/pe.go`) | Phase 149 gates | - | - | - |
| G3 | Value representation | Implemented | Tagged `Value` union `codegen.go:1001-1032`; kcc mirrors `src/compiler/codegen.kark:708-726` | Corpus | Boxed everywhere on the C path; the C-free backend is unboxed with static kinds | - | - |
| G4 | C interoperability | Partial | `import "C" { ... }` blocks preserved through module assembly; extern declarations emitted | Phase 89-103 gates | No safety boundary, no type checking of foreign declarations, no link-time verification | - | Medium |
| G5 | System calls | Partial (runtime) | Native backend makes raw syscalls (Linux 1/60, macOS `0x2000004`), PE uses kernel32 via PEB walk | Phase 149 gate | Not exposed as a language surface | - | - |
| G6 | File I/O | Implemented | `stdlib/io/io.kark` (77 lines) + runtime primitives; `std.io` gated native-only | Phase 109 gate | - | - | - |
| G7 | Networking | Implemented (Windows-linked) | `stdlib/net/net.kark` (77 lines) thin wrappers over runtime `net_connect`/`net_listen`/`net_accept`; **every generated-C link on Windows needs `-lws2_32`** (`winsockLibFlag` `pkg/cli/kcc_engine.go`) | Phase 125A gate | Requires a Winsock import on all platforms because the preamble is unconditional | - | Medium |
| G8 | HTTP | Implemented | `stdlib/http/http.kark` (332 lines), codec + loopback | Phase 125A gate | Client-oriented; no TLS | G7 | Medium |
| G9 | Database | Implemented (non-embedded) | `stdlib/db/db.kark` (829 lines) - SQL-text engine with `#karkain-db-v1` pipe-delimited persistence | Phase 125A gate | Not a real database; no transactions beyond begin/commit as string ops | - | Low |
| G10 | Processes / env vars / dynamic libs | **Missing** | No process spawn, environment, or `dlopen` surface found in `pkg/sema/resolve.go` or stdlib | - | - | - | Medium |
| G11 | Static linking | Partial | `-static` not systematically used; Windows needs `libgmp.a`/`libgmp.dll.a` at link time (observed as a CI failure class) | - | Link-time external deps are unmanaged by the toolchain | J7 | Medium |

### 3.8 Compiler architecture

| Area | Capability | Status | Evidence | Tests/Evidence | Missing/Gap | Dependencies | Priority |
|---|---|---|---|---|---|---|---|
| H1 | Lexer | Mature | `pkg/lexer/lexer.go` (610 lines), zero-copy tokens | `pkg/lexer` | - | - | - |
| H2 | Parser | Mature | `pkg/parser/parser.go` (4 670 lines), recursive descent with recovery | `pkg/parser`, Phase 105/123 | - | - | - |
| H3 | AST | Mature | `pkg/parser/ast.go`, arena-allocated | - | - | - | - |
| H4 | Semantic analysis | Partial | `pkg/sema/resolve.go`; kcc `src/compiler/checker.kark` | Phase 99/123 | Primarily resolution + annotation matching (A13) | - | High |
| H5 | SSA IR | Implemented (partial use) | `pkg/ir/ssa` (1 678 lines); per-function lowering on the real path `emit_ir.go:315` | `pkg/ir/ssa`, Phase 141 gate | Closure-bearing functions always take the legacy path (`codegen.go:364`) | - | Medium |
| H6 | HIR | **Structural Only - unwired** | `pkg/ir/hir` (typed nodes, `BuildHIR` `build.go:9`) | Own unit tests only | **Imported by nothing outside its own package** | H5 | Medium |
| H7 | Optimizer | Partial | On-path: fold + simplify-CFG + DCE (`emit_ir.go:337-339`). Unwired: Mem2Reg, CSE, LICM, SROA (`pipeline.go:22-28`) | Phase 141 gate, `phase141_opt_test.go` | 4 of 6 passes are test-only | H5 | Medium |
| H8 | Code generation (C) | Mature | `pkg/codegen/codegen.go` (13 761 lines) + embedded runtime preamble | Phase 114 corpus, both engines | - | - | - |
| H9 | Code generation (self-hosted) | Implemented | `src/compiler/codegen.kark` (3 289 lines) | 61-file corpus byte-parity | Divergences exist (A5, A10) | H4 | High |
| H10 | Code generation (C-free machine code) | Implemented [HEAD] | `pkg/native` (7 793 lines); x86-64 encoder, ELF/PE/Mach-O writers | Phase 145-150, 151B-151C3 gates | Partial at [1.1.0] | H1-H3 | - |
| H11 | Linker integration | Partial | Delegates to gcc/clang (`cross_target.go`); own PE/ELF/Mach-O **writers** but no general linker | Phase 111 gate | No static linking, no LTO, no linker plugin | G11 | Medium |
| H12 | Register allocation | Implemented (native backend only) | 150C, callee-saved R12-R15 with spill-to-frame | Phase 150C gate | C path relies on the C compiler | H10 | - |
| H13 | Incremental compilation | Implemented | `pkg/compiler/incremental.go` (422 lines); content-addressed cache; `--incremental` | Phase 105/134 gates | Whole-assembly cache; native-split cache refused with a measured reason (150D) | H8 | - |
| H14 | Bootstrap / self-hosting | **Partial** | 3-stage bootstrap `pkg/bootstrap/bootstrap.go:155-157`: Go -> kcc -> kcc -> kcc | `TestBootstrap_BitwiseIdentity` (stage2==stage3) | Requires the Go toolchain as the oracle; gcc required to link; native backend not yet produced by kcc (`kccOwnsNativeTargets == false`) | H9, H10 | High |
| H15 | Diagnostics | Mature | Spans, excerpts, JSON contract, kcc `error[K1xx]` | Phase 83/105/123 | - | - | - |

### 3.9 Targets and platforms

| Area | Capability | Status | Evidence | Tests/Evidence | Missing/Gap | Dependencies | Priority |
|---|---|---|---|---|---|---|---|
| I1 | x86_64 Linux (C path) | Implemented + tested | `pkg/target/features.go`, cross triple resolution | Phase 114 corpus; Linux CI leg | - | - | - |
| I2 | x86_64 Windows (C path) | Implemented + tested | Same, plus `-lws2_32` link contract | Phase 88-90 legacy gates on Windows | - | - | - |
| I3 | x86_64 macOS | Implemented, untested here | Triple accepted `pkg/target/triple.go` | No macOS runner | Cross-linker never present on dev host | I1 | Medium |
| I4 | aarch64 (Linux/Windows/macOS) | **Partial** | Triples defined `pkg/target/features.go`; CI **builds** `linux/arm64`, `windows/arm64`, `darwin/arm64` | CI build matrix | Compile-only; no execution evidence | I1 | Medium |
| I5 | riscv64 Linux | **Structural Only** | Triple + ABI rules (`riscv64` = Linux-only) | No runner | Never executed | I1 | Low |
| I6 | WebAssembly / WASI | Implemented | `pkg/wasm` (3 093 lines), handwritten WASM emitter + embedded WASI runtime | Phase 108 gate, wasmtime-executed | Go engine only; no kcc parity | H8 | Medium |
| I7 | Native ELF (C-free) | Implemented [HEAD] | `pkg/native/elf.go`; `--target native-x86_64-linux` | Phase 148 gate; Linux CI executes | kcc cannot emit it yet | H10, H14 | High |
| I8 | Native PE (C-free) | Implemented [HEAD] | `pkg/native/pe.go` + PEB bootstrap + Win64 boundary; `--target native-x86_64-windows` | 10 PE-execution tests on windows/amd64 | kcc cannot emit it yet | H10, H14 | High |
| I9 | Native Mach-O PIE (C-free) | **Structural Only** | `pkg/native/macho.go` PIE + rebase opcodes; `--target native-x86_64-macos` | Structural validation only | **No Intel-mac runner exists; nothing executes** | H10 | Medium |
| I10 | GPU / NPU / SIMD / quantum targets | Experimental | `pkg/target/features.go` catalog; `pkg/backend/gpu` (WGSL emit), `pkg/npu` (5 vendor adapters), SIMD `pkg/codegen/simd_emit.go`, quantum `pkg/sema/quantum*.go` | Phase 124 catalog gate; Phase 78/98/106 gates | Not production targets | - | Low |

### 3.10 Build, package and distribution

| Area | Capability | Status | Evidence | Tests/Evidence | Missing/Gap | Dependencies | Priority |
|---|---|---|---|---|---|---|---|
| J1 | Dependency resolution | Implemented | `pkg/pm/resolver.go`, `lock.go`, `flow.go` | Phase 97/135/144 gates | - | - | - |
| J2 | Lock file | Implemented | `pkg/pm/lock.go`; `karkain.lock` written | Phase 105/135 | - | J1 | - |
| J3 | Package manifests | Implemented | `karkain.toml`; `[dependencies]` parsed by both engines | Phase 144 gate | - | J1 | - |
| J4 | Workspace support | Implemented | `pkg/pm/workspace.go`; both engines | Phase 144 gate | - | J3 | - |
| J5 | Local registry | Implemented | `pkg/pm/local_registry.go`; directory `registry.json` + `packages/<n>/<v>/` + SHA-256 digest verification | Phase 135 gate | - | J1 | - |
| J6 | Remote registry | **Partial** | HTTP client `pkg/pm/registry.go:20` `DefaultRegistryURL = "https://registry.karkain.dev"`; `FetchModule` remote branch `pkg/pm/manager.go:447-456` | Not verified (no network probe) | Client is real; **default endpoint reachability unknown**. Git fetching is an explicit "not available" error | J5 | Medium |
| J7 | Integrity | Implemented | `pkg/pm/integrity.go` SHA-256 per file and per directory; atomic temp->rename cache promotion | Phase 135 gate | Toolchain does not manage *link-time* native deps (the GMP CI failure) | - | Medium |
| J8 | Cross compilation | Implemented | Explicit `--target <triple>`; triple-prefixed GNU cross-gcc, clang `--target`; deterministic `ToolchainError` when absent | Phase 111 gate | Depends on externally installed cross-linkers | I1 | - |
| J9 | Installation / distribution | Implemented | `scripts/install.ps1`, `install.sh`, `verify-install.ps1`; 13-archive release matrix | Phase 142/143 gates | Requires the release job to publish archives | J1 | - |
| J10 | Build profiles | **Missing** | No `--release`/`-O` level selection found | - | No user-visible optimization control | H7 | Low |
| J11 | Reproducible builds | Partial | Incremental cache is content-addressed; bootstrap byte-identity gated | `phase131_repro_test.go` | Ordinary `karkain build` determinism not gated | H13 | Medium |

### 3.11 Standard library

Classification reflects **actual implementation and tests**, not the module's name. Note the pattern: several modules are thin `.kark` wrappers over C builtins, which is a legitimate architecture but not the same as logic written in Karkain.

| Module | Size | Status | Implementation reality | Evidence |
|---|---|---|---|---|
| `stdlib/string` | 321 | Implemented | Canonical `.kark`; both engines | Phase 109/132 freeze gate |
| `stdlib/collections` | 289 | Implemented | Canonical `.kark` | Phase 109 gate |
| `stdlib/io` | 77 | Implemented | `.kark` over runtime primitives; native-only builtins | Phase 109 gate |
| `stdlib/encoding` | 42 | Implemented | **Thin wrapper**: hex/base64/UTF-8 are C builtins registered at `pkg/sema/resolve.go:72-73` | Phase 109 gate, NIST/RFC vectors |
| `stdlib/crypto` | 16 | Implemented | **Thin wrapper**: `sha256_hex`/`sha512_hex` are C builtins; real SHA-256 at `pkg/codegen/codegen.go:2485` | Phase 109 gate, FIPS 180 vectors |
| `stdlib/numerics` | 478 | Implemented | Canonical `.kark`; `numerics_*` functions | Phase 126 gate |
| `stdlib/net` | 77 | Implemented | Thin wrapper over runtime sockets; forces `-lws2_32` on Windows | Phase 125A gate |
| `stdlib/http` | 332 | Implemented | Canonical `.kark` codec + loopback client | Phase 125A gate |
| `stdlib/db` | 829 | Implemented (non-embedded) | Canonical `.kark` SQL-text engine, pipe-delimited persistence | Phase 125A gate |
| `stdlib/testing` | 62 | Implemented | `assert`/`assert_eq`/`assert_ne` used by `karkain test` | KTF-001 gate |
| `stdlib/math` | 623 | Implemented | Canonical `.kark` | Phase 87 probes |
| `stdlib/generics` | 86 | Implemented | Persistent `Stack[T]`/`Queue[T]` - exercises monomorphization | Phase 146b gate |
| `stdlib/core` | 207 | Structural Only | Not part of the 10 frozen modules; canonical-syntax claim unproven | - |
| `stdlib/system` | 79 | **Structural Only** | Thin; no OS API surface behind it (see G10) | - |
| `stdlib/async` | 446 | **Structural Only** | Non-canonical syntax / no builtin backing per Phase 109 boundary | - |
| `stdlib/gpu` | 338 | Experimental | Mirrors `pkg/backend/gpu` | Phase 124 catalog |

**Gaps in K:** no OS/system API module with real backing (G10), no process/env/dynamic-library surface, no TLS.

### 3.12 Developer tooling

| Area | Capability | Status | Evidence | Tests/Evidence | Missing/Gap |
|---|---|---|---|---|---|
| L1 | Formatter | Implemented | `pkg/cli/formatter.go`; `Canonicalize` `:130` token-level, idempotent | Phase 82 gate | Formatting only; no `go fmt`-style reflow |
| L2 | LSP | Implemented | `pkg/lsp` 6 files / 2 937 lines: server, handler, diagnostics, scope, semantic tokens, definition | Phase 136 gate, 30 tests + stdio E2E | No rename/refactor; no incremental semantic-model cache |
| L3 | Debugger | **Partial** | `pkg/cli/dbg.go` drives **gdb in batch mode** and renders a backtrace; Go engine only | Phase 140 gate | Not a DAP server; no breakpoints/stepping/setters; kcc deferred |
| L4 | Test runner | Implemented | `karkain test` with kcc owning discovery + driver synthesis (`collectTestFiles` `src/compiler/main.kark`) | Phase 96 gate, 61-file corpus | - |
| L5 | Doc generation | Implemented | Sphinx (`docs/source`, 115 `.rst`), `linkcheck` job | CI docs job | Language reference is thin vs 140 audit reports |
| L6 | IDE integration | Implemented | `editors/` VS Code extension: commands + `language-server` bootstrap | `TestVSCodeExtension` | - |
| L7 | REPL | **Missing** | No `repl` command, no interactive shell | - | - |
| L8 | Linter | Implemented | `karkain lint`, exit code 7 | Phase 86 gate | - |
| L9 | Profiler | Implemented | `pkg/cli/prof.go`, opt-in instrumentation, JSON/folded/text | Phase 110 gate | Counts only `malloc`/`free` text in generated code |

### 3.13 Security and reliability

No security property is claimed without repository evidence.

| Area | Capability | Status | Evidence | Missing/Gap |
|---|---|---|---|---|
| M1 | Bounds checking | Implemented (runtime) | Checked division/modulo/index: `karkain_checked_div/mod/get/set` | Not a static guarantee |
| M2 | Division-by-zero / bad index | Implemented | Runtime error with file:line `runtime error: <kind> at <file>:<line>` | - |
| M3 | Integer overflow | **Missing** | `<<` documented as wrapping (`SPEC.md:304`); no overflow detection for `+ - *` | No checked arithmetic |
| M4 | Memory safety | **Missing** | Generated C uses raw pointers; user allocations are never reclaimed (B5) | No static memory-safety guarantee |
| M5 | Null / bounds on native backend | Partial | C-free backend traps via `Int3` on array-index and slice bounds | Not a language-level guarantee; C path differs |
| M6 | FFI isolation | **Missing** | `import "C"` declarations are unchecked and unverified at link time | No boundary enforcement |
| M7 | Input validation | **Partial** | Checked stdin reads (`readLineEOF`); stdlib validators exist for some decoders | Not systematic |
| M8 | Determinism | Implemented | Codegen deterministic; incremental cache content-addressed; KIR byte-stable | - |
| M9 | Reproducible builds | Partial | Bootstrap byte-identity gated; ordinary builds not gated | - |
| M10 | Dependency security | **Missing** | SHA-256 checksums for registry packages (`integrity.go`); **no signature verification, no advisory mechanism** | Integrity without authenticity |
| M11 | Compiler crash handling | Partial | SSA `Verify()` gates emission and falls back to legacy path (`codegen.go:364-370`, `emit_ir.go:340`) | Fallback covers malformed IR, **not wrong-code survival** (per comment `codegen.go:356-361`) |
| M12 | Fuzzing | **Missing** | No fuzz targets found | - |

### 3.14 Advanced capabilities (factual inventory only)

| Area | Capability | Status | Evidence | Reality |
|---|---|---|---|---|
| N1 | GPU | Experimental | `pkg/backend/gpu/wgsl_backend.go` emits real WGSL compute shaders from Tensor IR | Compile-only; CPU reference is the oracle; no driver/SDK execution |
| N2 | NPU | Experimental | `pkg/npu` 5 vendor adapters + `NPUDispatcher` | Always falls back to the CPU reference; no hardware |
| N3 | SIMD | Implemented | `pkg/codegen/simd_emit.go`; `__m128`/`vector_size` lane types, portable `karkain_simd_*` helpers | Real and gated |
| N4 | Quantum | **Structural Only** | `pkg/sema/quantum*.go` (~2 700 lines), `pkg/target` `quantum-experimental` | Modeling/simulation only; no quantum runtime |
| N5 | Autodiff | Implemented | `pkg/sema/autodiff.go` (557 lines) | Reverse-mode over the existing IR |
| N6 | Distributed computing | **Missing** | - | - |
| N7 | Embedded / bare-metal targets | **Missing** | - | - |
| N8 | Interpreter / JIT | **Structural Only** | `pkg/jit` (1 497 lines) | Not wired into the compile path (no import from `pkg/cli`/`pkg/codegen` found) |

### 3.15 Documentation and specification

| Area | Capability | Status | Evidence | Reality |
|---|---|---|---|---|
| O1 | Language specification | **Partial** | `SPEC.md` (593 lines, 14 sections) | Present and sectioned, but **behaves as a feature ledger, not a specification**: no formal grammar (Section 3 "Grammar (concrete syntax)" contains a keyword table at `:60`, not productions), and it overstates the type system (Conflict #3) |
| O2 | Architecture documentation | Partial | 140 audit reports under `docs/audit/` | Extensive and evidence-dense; documents intent and history rather than a current architecture reference |
| O3 | Standard-library docs | Implemented | `docs/source/stdlib/*.rst`, one page per module | Per AGENTS.md, the `networking`/`database`/`web` pages were rewritten after they documented invented APIs - docs had drifted ahead of code |
| O4 | Examples | Implemented | 220 `.kark` across 42 categories | Broad; the pinned parity corpus is a 61-file subset |
| O5 | Tutorials | **Missing** | No tutorial path in `docs/source` | - |
| O6 | API reference | Partial | `docs/source/reference/stable-api.rst` | Hand-maintained; not generated from source |
| O7 | Release documentation | Implemented | `docs/release/`, release notes, `SECURITY.md`, LTS branch policy | - |
| O8 | Conformance documentation | Implemented | `conformance/` 12 files / 64 `test_` functions, run through the real front end | Real and enforced on both engines |
| O9 | Spec-vs-code conformance tooling | **Missing** | No check that `SPEC.md` matches the implementation | Root cause of Conflict #3 |

---

## 4. Language Capability Matrix

| Feature | Status | Notes |
|---|---|---|
| 64-bit signed integers | Implemented | Width is a C-preamble property, not a type-system fact |
| Arbitrary-precision integers | **Partial** | Go engine only; default engine rejects them (A5) |
| Unsigned / fixed-width integers | **Missing** | Annotations accepted but inert (A6, A7) |
| Floating point | Implemented | `float64` |
| Booleans | Implemented | |
| Strings | Implemented | |
| Arrays (dynamic, heterogeneous) | Implemented | |
| Records / structs | **Partial** | Runtime maps; no field checking (A9) |
| Maps | Implemented (Go) / **Divergent (kcc)** | Critical defect A10 |
| Enums + `match` | Partial | Tag-only, no payloads, no exhaustiveness |
| Functions | Implemented | |
| First-class `fn` values | Partial | Lambda-backed; bare references broken |
| Closures | Partial | Capture-by-reference; env indirection |
| Generics (functions, types) | Partial | Textual substitution; explicit type args only (C1-C3) |
| Traits / constraints | **Structural Only** | Parsed, unchecked, no declaration syntax |
| Type inference | **Partial** | Annotations and literals; no unification |
| Ownership / borrowing | **Partial** | Lexical scope tracking, not a borrow system (B1-B2) |
| Lifetimes | **Missing** | |
| Macros | Partial | AST expansion, non-hygienic |
| Reflection | **Missing** | |
| `?` propagation | Partial | C path only; refused natively (E1) |
| try/catch/throw/panic | **Missing** | |
| Modules / visibility | Implemented | |
| Concurrency keywords | Implemented | Runtime-backed; no language-level safety (F5-F6) |

## 5. Compiler Capability Matrix

| Stage | Status | Notes |
|---|---|---|
| Lexing | Mature | Zero-copy tokens |
| Parsing | Mature | Recursive descent, error recovery |
| AST | Mature | Arena-allocated |
| Semantic/type analysis | **Partial** | Resolution + annotation matching; no lattice |
| SSA IR | Implemented (partial use) | On path for non-closure functions |
| HIR | **Structural Only** | Imported by nothing |
| Optimizer | Partial | 3 of 6 passes on path |
| C codegen | Mature | 13 761 lines, dual-engine parity |
| Machine-code codegen (C-free) | Implemented [HEAD] | x86-64; full value model |
| Register allocation | Implemented (native only) | |
| Linking | Partial | Delegates to system linker |
| Incremental compilation | Implemented | Content-addressed whole-assembly cache |
| Diagnostics | Mature | Spans, JSON contract, dual-engine |
| Bootstrap / self-hosting | **Partial** | 3 stages converge, but the Go toolchain and gcc remain required |

## 6. Runtime / System Capability Matrix

| Area | Status | Notes |
|---|---|---|
| Runtime layering | Implemented | freestanding / core / concurrency |
| libc-free runtime | Implemented | |
| Tagged value model | Implemented | Boxed on the C path |
| Startup | Implemented | Generated `main`; native `_start` / PEB bootstrap |
| C FFI | Partial | Unchecked declarations |
| File I/O | Implemented | |
| Networking / HTTP | Implemented | Winsock link required on Windows |
| Database | Implemented (non-embedded) | |
| Processes / env / dynamic libs | **Missing** | |
| Language-level atomics / locks | **Missing** | Runtime-internal only |
| Data-race guarantees | **Missing** | |
| GC / refcounting | **Missing** | |

## 7. Build / Package Capability Matrix

| Area | Status | Notes |
|---|---|---|
| Dependency resolution + lock | Implemented | |
| Manifests / workspaces | Implemented | |
| Local registry | Implemented | SHA-256 verified |
| Remote registry | Partial | Client real; default endpoint unverified |
| Git dependency fetching | **Missing** | Explicit "not available" |
| Cross compilation | Implemented | Needs external cross-linkers |
| Installation / archives | Implemented | 13-archive matrix |
| Build profiles | **Missing** | |
| Reproducible builds | Partial | |

---

## 8. Standard Library Matrix

| Module | Canonical `.kark`? | Both engines? | Tested | Classification |
|---|---|---|---|---|
| `std.string` | Yes | Yes | Phase 109/132 | **Implemented** |
| `std.collections` | Yes | Yes | Phase 109/132 | **Implemented** |
| `std.io` | Yes | Yes | Phase 109/132 | **Implemented** |
| `std.encoding` | 42-line wrapper over C builtins | Yes | NIST/RFC vectors, Phase 109 | **Implemented** |
| `std.crypto` | 16-line wrapper over C builtins | Yes | FIPS 180 vectors, Phase 109 | **Implemented** |
| `std.numerics` | Yes | Yes | Phase 126 | **Implemented** |
| `std.math` | Yes | Yes | Phase 87 probes | **Implemented** |
| `std.testing` | Yes | Yes | KTF-001 | **Implemented** |
| `std.net` | 77-line wrapper over runtime sockets | Yes (needs `-lws2_32` on Windows) | Phase 125A | **Implemented** |
| `std.http` | Yes | Yes | Phase 125A | **Implemented** |
| `std.db` | Yes (829 lines) | Yes | Phase 125A | **Implemented**, non-embedded engine |
| `std.generics` | Yes | Go only (kcc boundary pinned) | Phase 146b | **Implemented**, narrow |
| `std.core` | Yes | Not in the frozen 10 | - | **Structural Only** |
| `std.system` | Yes (79 lines) | - | - | **Structural Only** - no real OS surface |
| `std.async` | Yes (446 lines) | - | - | **Structural Only** - no builtin backing |
| `std.gpu` | Yes (338 lines) | Experimental | Phase 124 catalog | **Experimental** |

**No module provides:** process control, environment variables, dynamic libraries, TLS, filesystem metadata beyond basic I/O, or OS scheduling primitives.

---

## 9. Platform Matrix

Codegen existence is not production readiness. The `Status` column reflects executed evidence.

| Platform/Target | Status | Evidence | Limitations |
|---|---|---|---|
| x86_64 Linux (C path) | **Implemented + tested** | Phase 114 corpus; Linux CI legs; conformance 64 `test_` fns | Requires system gcc |
| x86_64 Windows (C path) | **Implemented + tested** | Legacy Phase 88-90 gates; dev-host runs | Requires MinGW gcc + `-lws2_32` + libgmp |
| x86_64 macOS (C path) | **Implemented, untested here** | Triple + flags `pkg/codegen/cross_target.go` | **No macOS runner in CI**; cross-linker never present on the dev host |
| aarch64 Linux | **Partial** | CI builds `linux/arm64` | Compile-only; no execution evidence |
| aarch64 Windows | **Partial** | CI builds `windows/arm64` | Compile-only |
| aarch64 macOS | **Partial** | CI builds `darwin/arm64` | Compile-only |
| riscv64 Linux | **Structural Only** | Triple + ABI rules `pkg/target/features.go` | Never executed anywhere |
| i386 | **Structural Only** | CI builds `linux/386` | Compile-only |
| s390x, ppc64le | **Structural Only** | CI builds only | Compile-only |
| wasm32-wasi | **Implemented + tested** | `pkg/wasm`; Phase 108 gate runs under wasmtime | Go engine only; no kcc parity; requires external wasmtime |
| native-x86_64-linux (C-free) | **Implemented [HEAD]** | `pkg/native/elf.go`; Phase 148; Linux CI executes | kcc cannot emit it yet |
| native-x86_64-windows (C-free) | **Implemented [HEAD]** | `pkg/native/pe.go`; 10 PE-execution tests on windows/amd64 | kcc cannot emit it yet |
| native-x86_64-macos (C-free) | **Structural Only** | `pkg/native/macho.go` PIE + rebase opcodes, validated structurally | **No Intel-mac runner exists; no image has ever executed** |
| arm64 native (C-free) | **Missing** | - | Out of scope per increment 149/150 records |
| GPU (`@target(gpu)`) | **Experimental** | `pkg/backend/gpu` WGSL emit | Compile-only; CPU reference is the oracle |
| NPU (`@target(npu)`) | **Experimental** | `pkg/npu` adapters | Always falls back to CPU |
| quantum | **Structural Only** | `pkg/sema/quantum*.go` | Simulation only |

---

## 10. Comparison Matrix

Descriptive only. Reference columns reflect widely documented, stable facts about those languages; `Not verified` marks anything this audit could not confirm from an authoritative source.

| Capability | Karkain | C | C++ | Rust | Go | Zig | Swift |
|---|---|---|---|---|---|---|---|
| Static type system | Partial - resolution + annotation matching, no lattice (`pkg/sema/resolve.go`) | No | Yes | Yes, HM-based | Yes, structural | Yes | Yes, nominal |
| Type inference | Partial - literals/annotations, no unification | No | Partial | Yes | Yes | Yes | Yes |
| Unsigned / fixed-width ints | **Missing** | Yes | Yes | Yes | Partial | Yes | Yes |
| Arbitrary-precision ints | Partial - Go engine only | No | No | No (needs crate) | Partial (`math/big`) | No | No |
| Ownership + borrowing | **Not present** - lexical scope tracking only (`pkg/sema/borrow_checker.go`) | No | No | Yes | No | No | No |
| Lifetimes | Missing | No | No | Yes | No | No | No |
| Generics | Partial - textual substitution (`pkg/sema/generics.go`) | No | Yes | Yes | Yes | Yes | Yes |
| Traits / protocols | Structural Only - no declaration syntax | No | Concepts | Yes | Interfaces | Interfaces | Protocols |
| Nullability | Optional at runtime (`TYPE_OPTION`) | Null pointers | Nullable + UB | No nulls | Yes | Optional types | Optionals |
| Exceptions | **Missing** | No | Yes | Partial (`Result`, `panic`) | Yes (`panic`/`recover`) | Yes (`error` unions) | Yes |
| Error propagation | Partial - `?` on C path only | No | No | `Result`/`?` | `error` returns | `error` unions | `throws` |
| Memory reclamation | **Missing** at language level | `free` | RAII / smart pointers | Ownership | GC | Allocator | ARC |
| GC | No | No | No | No | Yes | No | Yes |
| Closures | Partial - env indirection (`codegen.go:1027`) | Non-standard | Yes | Yes | Yes | Yes | Yes |
| Data-race freedom | **Missing** | No | No | Yes (`Send`/`Sync`) | Race detector | No | Strict concurrency |
| Atomics/locks in language | **Missing** | No | Yes | Yes | `sync` | Yes | Yes |
| Module system | Implemented - flat assembly, kcc-owned (`src/compiler/main.kark`) | No | Headers | Cargo | Go modules | Package manager | SwiftPM |
| Package registry | Partial - local registry real; remote client unverified (`pkg/pm/registry.go`) | No | vcpkg/Conan | crates.io | module proxy | - | - |
| Compiler bootstrap | Partial - 3 stages converge, needs Go + gcc (`pkg/bootstrap`) | - | - | rustc bootstrap | - | - | - |
| Self-hosted compiler | **Yes** - 14 139 lines of Karkain (`src/compiler/`) | - | - | Yes | - | Yes | Yes |
| Direct machine-code output | Yes [HEAD] - `pkg/native` x86-64 | - | - | Yes (via LLVM) | - | Yes | - |
| WebAssembly target | Yes - `pkg/wasm`, tested | - | - | Yes (`wasm32-unknown-unknown`) | Yes | Yes | Not verified |
| Embedded / bare-metal | Missing | Yes (freestanding) | Partial | Yes (`no_std`) | Partial | Yes | Not verified |
| Formal grammar in spec | **No** - keyword table only (`SPEC.md:60`) | Yes | Yes | Reference grammar | Yes | - | - |
| Reference implementation tests | Partial - 64 conformance fns, no spec-conformance checker | Yes | Yes | Yes | Yes | Yes | Yes |

**Not verified for this audit:** Swift's WebAssembly support, embedded support, and formal-grammar status; Zig's registry and formal grammar; C++ concepts maturity details. These were not confirmed from authoritative sources during this audit.

---

## 11. Dependency Graph

Derived from the actual gaps found above. Arrows read "is required before".

### 11.1 The critical chain - a silent-wrong-answer defect

```
Key representation in the value model
      |
      v
Map key equality (A10)  -->  kcc parity of map_get
      |                              |
      |                              v
      |                     Self-hosted engine is the DEFAULT engine
      |                              |
      v                              v
  Correctness of every map-using program     Silent divergence, exit code 0
```

The defect: kcc compares map keys with `strcmp` against `k.strVal` in **three** places - `map_get` (`src/compiler/codegen.kark:745`), `map_set` (`:746`) and `karkain_hasKey` (`:855`) - so non-string keys silently miss and insertion can overwrite the wrong entry. An earlier draft of this audit understated the scope as a single function; re-verification against the current tree found all three. It is invisible to the parity gate because **every map literal in the pinned corpus has string keys** (`examples/01-fundamentals/09_maps.kark:15`; every other map literal in `examples/` is empty `{}`). Verified by execution: Go `one/two/2`, kcc `two/two/1`, both exit 0.

### 11.2 The type-system chain

```
Type representation (what a type IS)
      |
      +--> Type checking / inference (A13)
      |         |
      |         +--> Struct field checking (A9)      [blocked: structs are maps]
      |         +--> Generic substitution beyond text (C1, C2)
      |         +--> Constraint checking (C4)        [blocked: no trait syntax]
      |         +--> Exhaustiveness checking (A12)
      |
      +--> Result / Option as real values (E2)
      |         |
      |         +--> `?` on the C-free target (E1)
      |         +--> any exception alternative (E3)
      |
      +--> Nullability / error unification
```

Everything in this chain is blocked on one missing thing: **a first-class type object in the language's type system.** Annotations are strings (`A7`), `int` width is decided in the C preamble (`A1`), and structs are maps (`A9`). A type lattice cannot be built on strings.

### 11.3 The memory-model chain

```
Reference / value semantics
      |
      +--> Ownership + move semantics (B6)
      |         |
      |         +--> Lifetimes (B2)
      |         +--> Aliasing rules (B8)
      |         +--> Reclamation strategy (B5)   [needs ownership to know when dead]
      |
      +--> Send / Sync typing (F6)
                |
                +--> Data-race freedom (F6)
                      |
                      +--> requires language-level atomics/locks (F5)
```

`B5` cannot be solved before ownership exists: without ownership there is no way to know when an allocation is dead. `F6` cannot be solved before ownership either.

### 11.4 The self-hosting chain

```
Unified value model (A1-A10 across both engines)
      |
      v
kcc parity of the value model (A5 bigint, A10 maps)
      |
      v
kcc produces machine code (flip kccOwnsNativeTargets)
      |
      v
Self-hosted compile of the compiler without the Go toolchain
      |
      v
C-free toolchain (today kcc still needs gcc to link)
```

### 11.5 The specification chain

```
Formal type / grammar definition
      |
      v
SPEC.md states enforceable facts
      |
      v
Automated spec-vs-implementation conformance check (O9)
```

Conflict #3 (`bigint` documented, unavailable on the default engine) is the direct consequence of `O9` being absent.

---

## 12. Priority Candidates

Dependency ordering only. This is **not** a ranking of desirability and no scores are assigned. Each candidate is a node that other gaps depend on.

| Candidate | Why it is foundational | Prerequisites | Affected components | Estimated implementation scope |
|---|---|---|---|---|
| **P1. Map key equality on the self-hosted engine** | A silently wrong answer on the **default** engine, with exit code 0, in a core collection type. Every other correctness gap is cheaper to fix than this class, because this one is invisible to the user and to the gate | None. Contained in three functions (`src/compiler/codegen.kark:745,746,855`) plus the corpus blind spot that hides it | `src/compiler/codegen.kark`, corpus fixtures, parity gate | Small. Kind-directed key equality, an int-keyed corpus fixture, keep the gate differential |
| **P2. A first-class type object in the checker** | Blocks the largest cluster of gaps at once: real struct field checking, generics beyond textual substitution, constraint checking, exhaustiveness, and a meaningful `?`/Result design. Each is blocked because a "type" is currently a bare string | None, but must not be built on the runtime-map struct representation (A9) or on annotation strings (A7) | `pkg/sema/`, `src/compiler/checker.kark`, `atypes.kark` | Large. Foundational redesign of the checker's representation |
| **P3. Value-model parity on the self-hosted engine** | Makes kcc a real peer of the Go engine rather than a divergent subset; prerequisite for P6 and for trusting the default engine | P1 | `src/compiler/codegen.kark`, `native_value.kark` | Medium-Large. `bigint` is the hard part (needs `mpz_t` in kcc's `Value`) |
| **P4. Reference/value semantics and ownership** | Unblocks lifetimes, aliasing rules, reclamation (B5) and `Send`/`Sync` (F6). The language currently has no way to know when an allocation is dead | P2 (ownership checking needs type information) | `pkg/sema/borrow_checker.go`, `src/compiler/checker.kark` | Very large. The single biggest structural decision in this audit |
| **P5. A real Result/Option type** | `?` cannot work on the C-free target and there is no error-value surface at all; also the honest alternative to missing exceptions (E3) | P2 | `pkg/codegen` `Value`, `src/compiler/codegen.kark`, checker | Medium, but the surface design is a language decision |
| **P6. kcc-owned machine-code emission** | Converts self-hosting from "kcc refuses loudly" to "kcc produces the image"; this is what `kccOwnsNativeTargets` guards | P3 | `src/compiler/native_*.kark`, `pkg/cli/kcc_native.go` | Medium-Large; encoder and all three containers are already byte-identical |
| **P7. Corpus coverage of the value model** | The parity gate is blind to whole defect classes (int-keyed maps). Whatever else changes, the corpus must be able to fail | None | `examples/`, Phase 114 gate | Small. Currently *causing* P1 to be invisible |
| **P8. Formal grammar and spec-conformance checking** | Prevents Conflict #3 recurring: docs state language features the default engine cannot compile | P2 | `SPEC.md`, new conformance tooling | Medium |

**Ordering note:** P7 is listed after P1 only because P1 was discovered first. Logically P7 should precede further value-model work, since it is what makes such work verifiable.

**Status (2026-10-01, recorded after this audit).** P1 and P7 are **closed**. kcc
now compares map keys with the engine's own `values_equal` (typed key storage, no
`strcmp` coercion), the pinned corpus carries an int-keyed map fixture
(`examples/01-fundamentals/09_maps.kark`), and a dedicated differential gate
(`pkg/cli/p1_map_key_test.go`, CI step *Run P1 map-key equality gate*) pins both
halves. P3's stated prerequisite is therefore met. Evidence and mutation
verification: `docs/audit/KCC-MAP-KEY-EQUALITY-FIX.md`. The remaining candidates
(P2–P6, P8) are untouched by that change and keep their ordering above.

---

## 13. Recommended Next Slice

**One slice: make map key equality correct and observable on the self-hosted engine, and close the corpus blind spot that hid it.**

### 13.1 Why it can be isolated

- The defect is in **three functions**: `map_get` (`src/compiler/codegen.kark:745`), `map_set` (`:746`) and `karkain_hasKey` (`:855`), all of which compare keys with `strcmp` against `k.strVal`.
- It needs no type-system work, no ownership model, no IR change and no new syntax.
- It is confined to the self-hosted engine; the Go engine's `map_get` (`pkg/codegen/codegen.go:1497`) already compares `Value`s and is not touched.
- It can be validated with the differential method the existing parity gates already use.

### 13.2 What it touches

- `src/compiler/codegen.kark` - key equality directed by kind (int, string), and a loud refusal for kinds with no defined equality.
- The pinned parity corpus - add an int-keyed map fixture. `examples/01-fundamentals/09_maps.kark:15` uses string keys only, and every other map literal in `examples/` is empty `{}`.
- The parity gate - a differential case comparing Go and kcc output for int-keyed maps, plus a negative case for a key kind with no defined equality.

### 13.3 Prerequisites that already exist

- Both engines run and are differentially testable on this host (`--engine go`, `--engine kcc`) - this audit relied on exactly that.
- The corpus gate already asserts byte-identical output across engines (Phase 114 `KCCParity`).
- The self-hosted refusal mechanism is established (`error[K145]`), so "no defined equality" can be a loud refusal rather than a silent miss.
- KIR continuity is gated (`TestPhase122_PipelineOwnership/KIRContinuity`), so a `codegen.kark` change has a re-measurement path.

### 13.4 What must NOT be included

- **No type-system redesign** (P2). Deciding what a type *is* belongs in its own increment.
- **No `bigint` parity** (P3/A5). A separate and much larger gap; bundling it would make the slice unverifiable.
- **No ownership, lifetimes or `Send`/`Sync`** (P4).
- **No `kccOwnsNativeTargets` flip.** Machine-code emission is P6 and requires P3 first.
- **No new collection types, no stdlib additions.**
- **No spec edits claiming more than is implemented.**

### 13.5 Evidence that would constitute completion

1. `let m = {1:"one",2:"two"}; print(m[1]); print(m[2]); print(len(m))` produces **identical output on both engines** - currently Go `one/two/2` vs kcc `two/two/1`.
2. An int-keyed map fixture is **in the pinned parity corpus**, so this defect class cannot return silently.
3. A key kind with no defined equality (record or array key) produces a **loud, deterministic refusal** on both engines, not a silent miss.
4. The gate is **mutation-verified**: reintroducing `strcmp` comparison fails the new case specifically.
5. The **KIR pin is re-measured** (never hand-edited) and `TestPhase122_PipelineOwnership/KIRContinuity` passes on the new count.
6. The Phase 114 both-engine corpus, the conformance corpus, and the existing `TestPhase151*` gates remain green.

---

## 14. Explicit Out-of-Scope Items

Each of these is a real gap. None belongs in the next slice.

| Item | Why it is out of scope |
|---|---|
| Type-system redesign (P2) | A foundational language decision requiring its own baseline and increment; would swamp a defect fix |
| `bigint`/`bigfloat` parity (A5, P3) | Requires `mpz_t` in the self-hosted `Value` and a GMP dependency in kcc; large and independent of map keys |
| Ownership, move semantics, lifetimes (B6, B2) | The largest structural change in the audit; must follow the type representation |
| Reclamation / GC (B5) | Cannot be designed before ownership defines what "dead" means |
| `Send`/`Sync` and data-race freedom (F6) | Depends on ownership; also needs language-level atomics (F5) which do not exist |
| Exceptions / try-catch (E3) | A language-surface design decision, not a defect |
| Unsigned and fixed-width integers (A6) | Requires a type representation first; annotations are currently inert strings (A7) |
| Traits / constraint checking (C4) | No trait declaration syntax exists to check against |
| kcc machine-code emission flip (P6) | Requires value-model parity (P3) first |
| Mach-O native execution | No Intel-mac runner exists; every Mach-O claim in the repo is structural since increment 149 |
| ARM64 native backend | Out of scope per increments 149/150 |
| Dead `compiler/` tree removal (Conflict #2) | Real housekeeping, but it is a deletion of tracked files and belongs to its own change, not a correctness slice |
| SPEC.md rewrite | Belongs with P8; changing the spec to match a partial fix would misstate capability |
| GPU / NPU / quantum expansion (N1-N4) | Already experimental or structural; no production readiness, and not prerequisites for anything above |
| Remote registry activation (J6) | Endpoint reachability is unverified; unrelated to the compiler |

---

## 15. Audit Conclusion

### 15.1 What Karkain 1.1.0 demonstrably has

- A complete, working toolchain: lexer, parser, AST, semantic resolution, C code generation, system linking, installation, and a release pipeline with 13 archives.
- A genuinely self-hosted compiler: 14 139 lines of Karkain in `src/compiler/`, a 3-stage bootstrap whose stage-2 and stage-3 outputs are byte-identical, and byte-parity across a 61-file corpus on both engines.
- A real, tested standard library of 15 modules, of which 11 are genuinely implemented on both engines - including SHA-256/SHA-512, hex, Base64, UTF-8, sockets, HTTP and a SQL-text engine - though several are thin `.kark` wrappers over C builtins rather than Karkain logic.
- A C-free machine-code backend (x86-64; ELF, PE with PEB bootstrap, Mach-O PIE), unreleased at 1.1.0 but present at HEAD, with PE images genuinely executed on Windows.
- Real concurrency: a work-stealing C scheduler with channels and actors, gated by a conformance gate.
- Substantial developer tooling: formatter, LSP with semantic tokens/hover/definition/completion, a kcc-owned test runner, a gdb-batch debugger, VS Code integration, and 220 examples.
- Strong verification discipline for its own history: 140 audit reports, mutation-verified gates, and a CI matrix with cross-arch builds.

### 15.2 What remains partial

- **The type system.** Resolution and annotation matching work; a type lattice, inference, field checking and constraint checking do not. A "type" is currently a string, and struct fields are unchecked because structs are runtime maps.
- **The optimizer.** Three of six implemented passes are wired; Mem2Reg, CSE, LICM and SROA are test-only.
- **The IR story.** SSA is used per-function; the full pipeline and all of HIR are unwired.
- **The default engine.** kcc is the default engine for check/build/run/test, but it is a *divergent* subset of the Go engine rather than a peer.
- **Error values.** `?` exists on the C path only; Result/Option are enum tags with no language surface.
- **Concurrency safety.** The runtime is capable; the language has no atomics, locks, or race-freedom typing.

### 15.3 What is missing

- Ownership, borrowing, lifetimes, move semantics, reclamation (no GC, no refcounting, no `free`).
- Unsigned and fixed-width integers; exceptions; reflection; traits; a REPL; build profiles; process/environment/dynamic-library APIs; distributed computing; embedded targets; fuzzing; signature verification for dependencies.
- Real struct layout, field typing and exhaustiveness checking.
- A formal grammar and any spec-vs-implementation conformance check.

### 15.4 What the dependency order suggests

The dependency graph points at one thing before anything else: **the default engine is currently capable of producing a silently wrong answer with exit code 0.** Int-keyed maps return the wrong value on kcc (`two/two/1` instead of `one/two/2`) and the parity corpus cannot see it because no fixture uses a non-string key.

That class of defect - wrong answers that no gate and no exit code reveals - is more expensive than its size suggests, because it undermines every other parity claim in the repository. It is also the cheapest thing in this audit to fix, and its prerequisites all exist today.

Behind it, the ordering is: corpus coverage (P7) -> value-model parity (P3) -> a first-class type representation (P2) -> ownership (P4) and Result/Option (P5) -> kcc machine-code emission (P6). Each arrow is a real blocker found in the source, not a preference.

**The recommended next slice is therefore narrow and specific**: make map key equality correct on the self-hosted engine, and put an int-keyed map into the pinned corpus so the blind spot is closed. Nothing else.

### 15.5 Two findings that should be corrected regardless of sequencing

1. **Conflict #2** - the root `compiler/` tree (2 584 lines) is dead code kept alive by two file-existence assertions. It should be deleted in its own change.
2. **Conflict #3** - `SPEC.md` presents `bigint` as a language type without noting the default engine cannot compile it. Until O9 exists, spec claims should be marked with engine scope.

---

*Audit produced 2026-10-01 against HEAD `6162db4`, finalized against `29ad61e`, on branch `main`. No source file was modified. No test suite was executed; validation was limited to document syntax and referenced-path existence.*
