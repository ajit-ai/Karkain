# KARKAIN GAP ANALYSIS

Divided into CRITICAL (must fix before expansion), ARCHITECTURAL (missing foundational
decisions), IMPLEMENTATION (designed but incomplete), TESTING, DOCUMENTATION, and FUTURE.

## CRITICAL GAPS (block correctness; fix first)

| # | Gap | Evidence | Impact |
|---|-----|----------|--------|
| C1 | **Parser OOM / runaway-memory in function-call arg loops** | parser.go:1148-1153,1190-1195,601-606,619-624 — `parsePrimaryExpr` returns nil without advancing token (parser.go:973); loops append with no no-progress guard → ~3GB growslice | **Compiler crashes on large/malformed input**. Blocks self-hosting (Stage1) and `TestCodeGenProfile`. Top priority. |
| C2 | **No whole-program type checker** | no TypeChecker in pkg/sema | Codegen can emit wrong/invalid C for type errors; language safety unenforceable. |
| C3 | **No symbol table / scope resolution / undefined-var&func detection** | borrow_checker.go:365-369 returns silently; no semantic name pass | Undefined identifiers silently pass sema. |
| C4 | **Borrow checker incomplete: moves never expire (BUG-5) + BorrowError.Line always 0** | borrow_checker.go:74-85 (moves leak), all BorrowError Line=0 | Memory-safety guarantee unsound; diagnostics unusable. |
| C5 | **Error handling non-functional: `?` is a no-op (BUG-4), no error boundary (G2)** | ROADMAP BUG-4; error-handling bug | Option/Result can't be used for real error control flow. |
| C6 | **BUG-2 / BUG-7 / BUG-8 / BUG-1 / BUG-3** codegen correctness | ROADMAP BUG list (all OPEN) | Struct/match/enum/index/typed-decl codegen is broken. |

## ARCHITECTURAL GAPS (missing foundational decisions)

| # | Gap | Notes |
|---|-----|-------|
| A1 | **Memory model decision** (GC vs ownership vs hybrid) not formally adopted | See MEMORY_MODEL_DECISION.md. Ownership/borrow is the evident direction; needs formal commitment + completion. |
| A2 | **Generics strategy** — reified string-substitution partial vs full semantic generics | Currently neither connected nor complete. Must decide: complete reified monomorphization (fits Go-like architecture). |
| A3 | **error-handling philosophy** — Result/Option (no exceptions) is the clear direction; must be made functional end-to-end | Fix BUG-4 first. |
| A4 | **Module/visibility model** (pub/private) — G11 | Needed before package ecosystem (Phase 65). |
| A5 | **Unified execution model feasibility** (fn/task/parallel/remote/gpu) — undecided | See UNIFIED_EXECUTION_MODEL.md. Don't build yet. |
| A6 | **Capability-based security** and **effect system** — undecided | See CAPABILITY_SECURITY_MODEL.md. Don't build yet. |

## IMPLEMENTATION GAPS (designed/started, incomplete)

| # | Feature | Status | Phase |
|---|---------|--------|-------|
| I1 | Generics `<T>` syntax parsing | AST/allocator only; no parser | 56/64 |
| I2 | Generic func instantiation (body substitution missing) | generics.go:216-220 incomplete | 64 |
| I3 | Monomorphizer wired to CLI pipeline | GPU-kernel only | 64 |
| I4 | Sema checkers (FFI/Actor/Coroutine/Quantum) not in run/build | LSP/tests only | 63 |
| I5 | Self-hosting compiler | OOM-blocked Stage1 | 56/68 |
| I6 | std/io.kark + std/string.kark empty | stubs | 60 |
| I7 | Go stdlib stubs (http/reflect/actor) | placeholders | 60/55b |
| I8 | Concurrency runtime not wired into default path | actor/coroutine exist standalone | 57 |
| I9 | GPU/quantum backends not conformance-tested end-to-end | emitters tested, hardware untested | 58/59 |
| I10 | `rawc` escape hatch bypasses SSA | documented seam | 64 |

## TESTING GAPS

| # | Gap | Impact |
|---|-----|--------|
| T1 | No parser unit test (parser_test.go absent) | Core parser bug shipped undetected |
| T2 | examples/ not directly invoked by Go tests | E2E coverage thin |
| T3 | No snapshot/golden tests | Can't catch codegen regressions |
| T4 | No perf/regression gates | Perf deaths silent |
| T5 | No fuzzing (parser/codegen) | Crash surface unprobed (ROADMAP L251 planned) |

## DOCUMENTATION GAPS

| # | Gap |
|---|-----|
| D1 | Many implementations exist WITHOUT SPEC coverage (GPU, quantum, actor, coroutine, match, macros) — spec/impl not converged |
| D2 | No formal memory model spec |
| D3 | No ABI / stability spec |
| D4 | No conformance test matrix linking SPEC → tests |

## FUTURE FEATURES (valuable, NOT before V1 core)

| # | Feature |
|---|---------|
| F1 | Unified execution model (task/parallel/remote/gpu qualifiers) |
| F2 | Capability-based security |
| F3 | Effect system |
| F4 | Explicit SIMD vector types + atomics (Phase 70) |
| F5 | AI-native extension (via packages, not core) |
| F6 | Distributed execution (quantum/lattice) |
