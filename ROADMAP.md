# Karkain Roadmap Ã¢â‚¬â€ Complete Development Plan

## Vision

Karkain is a unified heterogeneous systems language for:
- **CPU/GPU computing** Ã¢â‚¬â€ write host + kernel in one language
- **Quantum computing** Ã¢â‚¬â€ first-class quantum primitives, circuits, error correction
- **Multi-dimensional lattice simulation** Ã¢â‚¬â€ tensor types, autograd, distributed computation
- **Systems programming** Ã¢â‚¬â€ Rust-like safety, C-like performance, Go-like simplicity

## Guiding Principles

1. **Maturity over features** Ã¢â‚¬â€ depth, correctness, performance, verification before expansion
2. **Semantic foundations first** Ã¢â‚¬â€ value representation, ownership, lifetimes, IR before features
3. **Working > ambitious** Ã¢â‚¬â€ fix broken fundamentals before adding new capabilities
4. **Incremental verification** Ã¢â‚¬â€ every phase must compile, pass E2E, pass all tests

---

## Project Statistics (as of Phase 49)

| Metric | Value |
|--------|-------|
| Phases completed | 49 |
| Go code (compiler) | ~34,600 lines across 100 files, 13 packages |
| C runtime | ~777 lines (actor, quantum, reflect, rpc) |
| Self-hosting sources | 6 .kark files (src/compiler/) |
| Backend codegen files | 15 (C, WGSL, OpenCL, QIR, OpenASM, OpenPulse, QML, QEC, etc.) |
| Standard library | 6 .kark files (io, string, math, async/actor, gpu) |

---

## Completed Phases Summary

### Tier 1: Language Core (Phases 1Ã¢â‚¬â€œ17)
Lexer, parser, code generator, basic types (int, float, string, bool), arrays, maps,
control flow (if/else, while, for), functions, closures, structs, enums, quantum
primitives (qreg, gates, measurement), kernel declarations, matrix operations, address-of/dereference.

### Tier 2: Multi-Backend Emission (Phases 18Ã¢â‚¬â€œ26)
GPU host-side codegen, OpenCL kernel emission, WGSL compute shader generation,
generic kernel monomorphization, multi-backend dispatch, AVX2 SIMD matrix kernels,
monomorphized generics with trait constraints, hardware-native tensor types, autograd engine.

### Tier 3: Quantum Computing Stack (Phases 28Ã¢â‚¬â€œ36)
Quantum primitives, OpenQASM 3.0 + QIR backends, hybrid VQE/QML gradient engine,
GPU quantum state-vector simulator, distributed multi-GPU quantum, quantum noise models,
OpenPulse codegen, quantum error correction, quantum circuit optimization,
binary IR bytecode (.kbc), unified multi-target JIT engine + C ABI FFI.

### Tier 4: Developer Experience & Concurrency (Phases 37Ã¢â‚¬â€œ40)
Distributed actor system, coroutine/async runtime & green thread scheduler,
LSP engine with diagnostics/completion/hover/go-to-def, package manager.

### Tier 5: Safety & Correctness (Phases 41Ã¢â‚¬â€œ49) Ã¢â€ Â CURRENT
- **41**: Hybrid memory model Ã¢â‚¬â€ `&T`/`&mut T` references, `move(x)`, `@raw(addr)`
- **42**: `Option<T>`, `Result<T,E>`, match expressions, SIMD intrinsics
- **43**: Compile-time borrow checker
- **44**: Error propagation (`?`), exhaustive match checking, linear type enforcement
- **45**: Custom enum types with match pattern support
- **46**: Typed function parameters, enum match pattern fix
- **47**: For-in loops, method call dispatch, `push()` builtin
- **48**: Break/continue, lambda expressions, map iteration
- **49**: Value representation & allocation model (C23 migration, small integer pool,
  stack value macros, value classification, escape analysis foundation)

---

## Known Critical Bugs (Must Fix Before New Features)

### BUG-1: Struct codegen is broken
- `genStructDecl` emits C typedef with raw field types (`char* name; int64_t age;`)
- `genStructLiteral` assigns boxed `Value*` to raw fields (`_s.name = make_string("Alice")`)
- Top-level struct declarations are **never emitted** (`GenerateAndCompile` doesn't iterate them)
- Field access passes raw `char*` to `print_value(Value*)` Ã¢â‚¬â€ type mismatch
- **Impact**: Structs are unparseable/unusable end-to-end

### BUG-2: Option/Result match arms are broken
- `Some(v)` and `None` patterns both compile to condition `1` (always true)
- First match arm always wins; later arms are dead code
- Only enum/literal/wildcard patterns produce real comparisons
- **Impact**: Core error-handling semantics are non-functional

### BUG-3: Index assignment generates invalid C
- `m["k"] = v` becomes `array_get(m, make_string("k")) = v;` Ã¢â‚¬â€ assignment to rvalue
- **Impact**: Map/string mutation is impossible

### BUG-4: `?` error propagation is a no-op
- `PropagateExpr` passes operand through without any Result checking
- **Impact**: Error propagation doesn't work

### BUG-5: Borrow checker lacks lexical scoping
- Flat variable map shared across function body Ã¢â‚¬â€ two blocks using same name share state
- Borrows never expire Ã¢â‚¬â€ `&x` poisons variable for rest of function
- `BorrowError.Line` is always 0
- **Impact**: Safety guarantees are unreliable

### BUG-6: `make_int` pool is mutable shared state
- Small integer pool values can be mutated, corrupting the pool for all users
- **Impact**: Subtle data corruption possible

### BUG-7: Enum variants with payloads unimplemented
- `EnumVariantExpr` with payload emits `EnumName_Variant_make(val)` Ã¢â‚¬â€ nothing defines this
- **Impact**: Tagged unions with data don't compile

### BUG-8: Typed declarations mix representations
- `var s string = "hi"` Ã¢â€ â€™ `char* s = make_string("hi")` (Value* into char* slot)
- **Impact**: Typed variable declarations are broken

---

## Remaining Roadmap

### PHASE 50: Bug Fixes & Language Correctness
**Goal**: Make all existing features work correctly before building more.

| Task | Details | Priority |
|------|---------|----------|
| Fix struct codegen | Emit struct typedefs at top level, use `Value` fields, fix field access | CRITICAL |
| Fix Option/Result match | Proper runtime tag checking in match codegen | CRITICAL |
| Fix index assignment | Generate `array_set()`/`map_set()` calls for lvalue index | CRITICAL |
| Fix `?` propagation | Desugar `x?` to `match x { Ok(v) => v, Err(e) => return Err(e) }` | HIGH |
| Fix enum payload variants | Generate `EnumName_Variant_make` constructor macros | HIGH |
| Fix typed declarations | Correct type mapping in `mapLiteralToC` and `genVarDecl` | HIGH |
| Fix `make_int` pool safety | Use `const` or copy-on-read for pooled values | MEDIUM |

### PHASE 51: Borrow Checker Lexical Scoping
**Goal**: Make the safety system actually safe.

| Task | Details | Priority |
|------|---------|----------|
| Lexical scope stack | Push/pop scopes on `{` `}` boundaries | CRITICAL |
| Borrow lifetime tracking | Borrows expire when scope exits | CRITICAL |
| Shadow variable support | Inner `let x` shadows outer `x` correctly | HIGH |
| Reference passing validation | Ensure `&x` only used while `x` is alive | HIGH |
| Integration with escape analysis | Connect Phase 49 escape analysis to borrow checker | MEDIUM |

### PHASE 52: Value-by-Value Runtime API
**Goal**: Eliminate unnecessary heap allocation for primitives.

| Task | Details | Priority |
|------|---------|----------|
| `binary_op(Value, op, Value)` Ã¢â€ â€™ `Value` | Change signature to accept/return by value | HIGH |
| `is_truthy(Value)` by value | Change signature | HIGH |
| `print_value(Value)` by value | Change signature | HIGH |
| `values_equal(Value, Value)` by value | Change signature | HIGH |
| Array stores `Value` not `Value*` | Eliminate double indirection | HIGH |
| `array_get` returns `Value` | By-value element access | HIGH |
| Update all codegen call sites | Adapt generated C to new API | HIGH |

### PHASE 53: IR Infrastructure
**Goal**: Make IR the center of the compiler (per user's architectural correction).

| Task | Details | Priority |
|------|---------|----------|
| Define IR instruction set | SSA-form intermediate representation | CRITICAL |
| AST Ã¢â€ â€™ IR lowering | New pass replacing direct AST Ã¢â€ â€™ C | CRITICAL |
| IR Ã¢â€ â€™ C23 emission | IR drives the C backend | CRITICAL |
| IR type system | Typed registers, not just Value* everywhere | HIGH |
| IR optimization passes | Constant folding, dead code elimination | MEDIUM |
| IR verification | Validate IR well-formedness | MEDIUM |

### PHASE 54: Closure & Capture Semantics
**Goal**: Lambdas properly capture outer variables.

| Task | Details | Priority |
|------|---------|----------|
| Closure data structures | Heap-allocated capture blocks | HIGH |
| Escape analysis Ã¢â€ â€™ closure conversion | Escaping locals become capture fields | HIGH |
| Lambda codegen with captures | Pass capture block to lambda function | HIGH |
| Mutable capture semantics | `&mut` captures update outer variable | MEDIUM |

### PHASE 55: String & Slice Types
**Goal**: Proper string and dynamic array types.

| Task | Details | Priority |
|------|---------|----------|
| String slicing | `str[0:5]` returns substring view | HIGH |
| `[]T` slice type | View into contiguous memory (pointer + len + cap) | HIGH |
| String comparison operators | `==`, `!=`, `<`, `>` | HIGH |
| String formatting | `fmt()` or interpolation syntax | MEDIUM |
| Slice bounds checking | Runtime bounds validation | MEDIUM |

### PHASE 56: Self-Hosting Compiler Completion
**Goal**: `src/compiler/*.kark` can compile itself.

| Task | Details | Priority |
|------|---------|----------|
| Port lexer to Karkain | `src/compiler/lexer.kark` functional | HIGH |
| Port parser to Karkain | `src/compiler/parser.kark` functional | HIGH |
| Port codegen to Karkain | `src/compiler/codegen.kark` functional | HIGH |
| 3-stage bootstrap pipeline | Karkain Ã¢â€ â€™ C Ã¢â€ â€™ GCC Ã¢â€ â€™ karkain.exe | HIGH |
| SHA-256 byte parity verification | Bootstrap binary matches | MEDIUM |

### PHASE 57: Actor & Concurrency Runtime
**Goal**: Working actor model (currently placeholder only).

| Task | Details | Priority |
|------|---------|----------|
| Actor state management | Per-actor mutable state with message passing | HIGH |
| `spawn` codegen | Generate C actor creation code | HIGH |
| `send`/`receive` codegen | Generate message queue operations | HIGH |
| Channel operations | `!` (send), `!?` (sync send), channel declaration | HIGH |
| `select` multiplexing | Pattern-match on multiple channels | MEDIUM |
| Actor supervision trees | Restart strategies | LOW |

### PHASE 58: GPU Kernel Integration
**Goal**: Wire existing backends into the main compilation pipeline.

| Task | Details | Priority |
|------|---------|----------|
| Kernel Ã¢â€ â€™ WGSL pipeline | `kernel` Ã¢â€ â€™ WGSL Ã¢â€ â€™ WebGPU compute | HIGH |
| Kernel Ã¢â€ â€™ OpenCL pipeline | `kernel` Ã¢â€ â€™ OpenCL C Ã¢â€ â€™ clBuildProgram | HIGH |
| Host-side GPU launch | Memory transfer, dispatch, synchronization | HIGH |
| GPU memory model | Device/Host/Shared memory domains | HIGH |
| CPU fallback kernels | Auto-generate CPU version of each kernel | MEDIUM |
| GPU safety verification | Compile-time checks for kernel restrictions | MEDIUM |

### PHASE 59: Quantum Compilation Pipeline
**Goal**: Wire quantum backends into main pipeline.

| Task | Details | Priority |
|------|---------|----------|
| Circuit Ã¢â€ â€™ OpenQASM 3.0 export | `circuit` Ã¢â€ â€™ `.qasm` file | HIGH |
| Circuit Ã¢â€ â€™ QIR LLVM IR | `circuit` Ã¢â€ â€™ QIR-compatible LLVM IR | HIGH |
| Circuit Ã¢â€ â€™ OpenPulse | Pulse-level control output | MEDIUM |
| Quantum simulator bridge | Host-side state vector simulation | HIGH |
| Distributed quantum execution | Multi-GPU state vector (already coded, needs wiring) | MEDIUM |
| QEC integration | Error correction codes Ã¢â€ â€™ syndrome extraction circuits | LOW |

### PHASE 60: Standard Library
**Goal**: Practical standard library for real programs.

| Task | Details | Priority |
|------|---------|----------|
| `io` module | File read/write, stdin/stdout | HIGH |
| `string` module | Trim, split, join, contains, replace | HIGH |
| `math` module | Basic math functions (sqrt, pow, abs already exist) | MEDIUM |
| `collections` module | Set, Queue, Stack | MEDIUM |
| `json` module | JSON parse/serialize | MEDIUM |
| `net` module | TCP/UDP sockets | LOW |

### PHASE 61: Testing & Verification Infrastructure
**Goal**: Systematic correctness guarantees.

| Task | Details | Priority |
|------|---------|----------|
| `karkain test` runner | Discover and run `_test.kark` files | HIGH |
| Property-based testing | Randomized input testing framework | MEDIUM |
| Compiler snapshot tests | C output golden files with diff checking | HIGH |
| Fuzz testing | Parser/codegen fuzzing for crashes | MEDIUM |
| Bootstrap verification | SHA-256 parity check in CI | HIGH |

### PHASE 62: Native Codegen (Stage 2)
**Goal**: Direct native compilation without GCC.

| Task | Details | Priority |
|------|---------|----------|
| x86-64 codegen | Register allocation, instruction selection | HIGH |
| Memory instruction selection | Load/store/arithmetic for x86-64 | HIGH |
| ELF/PE/COFF emission | Platform-specific binary output | HIGH |
| Linking | System linker invocation or built-in | MEDIUM |
| Optimization levels | `-O0`, `-O1`, `-O2` flags | LOW |

---

## Execution Priority Order

```
PHASE 50: Bug Fixes & Language Correctness        Ã¢â€ Â NEXT
PHASE 51: Borrow Checker Lexical Scoping
PHASE 52: Value-by-Value Runtime API
PHASE 53: IR Infrastructure
PHASE 54: Closure & Capture Semantics
PHASE 55: String & Slice Types
PHASE 56: Self-Hosting Compiler
PHASE 57: Actor & Concurrency Runtime
PHASE 58: GPU Kernel Integration
PHASE 59: Quantum Compilation Pipeline
PHASE 60: Standard Library
PHASE 61: Testing & Verification
PHASE 62: Native Codegen
```

**Rationale**: Phases 50Ã¢â‚¬â€œ55 are foundational correctness and language semantics.
Phases 56Ã¢â‚¬â€œ59 wire up the existing (but disconnected) backend infrastructure.
Phases 60Ã¢â‚¬â€œ62 complete the ecosystem.

---

## Explicitly Out of Scope (Decision Record)

**DSP and FPGA backend targets Ã¢â‚¬â€ REMOVED from roadmap (decision: 2026-08, revisited 2026-09 for NPU).**

Rationale for DSP/FPGA removal: these targets fragment portability and cannot be CI-tested
without hardware. Karkain's differentiator remains: **compile-time ownership safety + performance
on CPU/GPU/NPU/quantum**, not breadth of all hardware targets.

**NPU support Ã¢â‚¬â€ RE-ADDED (decision: 2026-09).**

NPU is a first-class target because:
- Every major CPU vendor ships NPU silicon (Intel, Qualcomm, Apple, AMD, Arm)
- ONNX provides a universal interchange format with vendor runtime support
- MLIR provides a standards-based lowering path for kernel-level NPU control
- NPU inference is the dominant deployment path for edge AI/ML workloads
- Karkain's tensor pipeline + autodiff engine + WGSL codegen provide the foundation

Design principle: ONNX export for portability (all NPUs), MLIR for performance (custom fusion).
Both paths are implemented; ONNX first (faster to market), MLIR second (deeper optimization).


---

## Phase 63+: Independence Roadmap (Decision Record, 2026-08)

**Goal**: `.kark` files and the Karkain toolchain become fully independent for all
system-level activity. Every known capability gap is tracked as a phase below.
The milestone definition of INDEPENDENCE: `karkain` compiles Karkain source,
including its own compiler, with zero dependency on any other language's
toolchain for day-to-day development.

### Gap Register (source of the phases below)

| # | Gap | Status today |
|---|-----|--------------|
| G1 | Borrow checker not enforcing full ownership | lexical scoping only (Ph 51) |
| G2 | SSA optimizer too shallow vs LLVM-class pipelines | fold + DCE only |
| G3 | No package manager / module versioning | import blocks only |
| G4 | No LSP, formatter, REPL, structured diagnostics | ad-hoc errors |
| G5 | Self-hosting partial | seed tests only |
| G6 | Concurrency ergonomics below goroutine class | spawn/receive primitives |
| G7 | No explicit SIMD vector types; no atomics/memory ordering | GCC auto-vectorization only; lock-free structures impossible |

### Phases

```
PHASE 63: Full Ownership & Borrow Checker        [closes G1]
PHASE 64: IR Optimizer Depth I                   [closes G2]
          GVN/CSE, constant propagation, inlining on SSA
PHASE 65: Package Manager & Module System (kpm)  [closes G3]
          semantic versioning, lockfiles, registry layout
PHASE 66: Toolchain Polish                       [closes G4]
          LSP server, kfmt formatter, rich diagnostics, REPL
PHASE 67: IR Optimizer Depth II                  [completes G2]
          LICM, loop unrolling, bounds-check hoisting, pass manager
PHASE 68: Self-Hosting Completion                [closes G5 -> INDEPENDENCE]
          lexer+parser+sema+codegen rewritten in .kark;
          karkain.exe bootstraps itself; drop Go toolchain from release path
PHASE 69: Ecosystem Hardening & v1.0 Freeze      [sustainment]
          stdlib audit, fuzzing, conformance suite, language spec v1.0
PHASE 70: SIMD Vector Types & Atomics            [closes G7]
          explicit lane types ([4]f32), portable intrinsics surface,
          atomics + memory orderings (seq_cst/acq_rel/relaxed),
          cache-line alignment attributes; freestanding mode candidate
PHASE 71: Math IR Foundation                      [Karkain-owned mathematical IR]
          Math IR data model, node taxonomy, type metadata, builder/factory,
          validation, printer, basic lowering from AST, extensible function registry
PHASE 72: Tensor IR Foundation                    [Karkain-owned tensor IR]
          Tensor IR in SSA pipeline, shape types (static/dynamic/symbolic),
          core ops (create, reshape, transpose, matmul, add, mul, slice, reduce),
          compile-time shape validation, broadcasting rules
PHASE 73: CPU Reference Backend                   [correctness oracle]
          CPU execution of Tensor IR, elementwise ops, matmul, reshape,
          transpose, broadcasting, reductions — the semantic reference
PHASE 74: Autodiff Integration                    [gradient computation]
          Wire existing autodiff engine to Tensor IR, gradient propagation,
          reverse-mode AD through tensor ops, CPU gradient execution
PHASE 75: Backend Abstraction                     [execution planner]
          Backend interface (CPU/GPU/NPU/Future), capability model,
          cost-based backend selection, fallback dispatch
PHASE 76: GPU/WGSL Integration                    [existing codegen wiring]
          Wire existing tensor_wgsl.go to execution planner, Tensor IR →
          GPU lowering → WGSL → WebGPU, optional backend
PHASE 77: NPU Abstraction + Backends              [vendor-neutral NPU]
          NPU capability model, vendor-neutral interface, Intel (OpenVINO),
          Qualcomm (QNN), Apple (CoreML) adapters, graceful fallback
PHASE 78: NPU Optimization                        [fusion + memory planning]
          Operator fusion (Conv+Relu+BN), DDR<->TCM tiling, INT8/INT4
          quantization, memory planning, MLIR codegen for kernel control
```

### Ordering Rules

1. Phase 63 before 64: optimizer may assume verified ownership semantics
2. Phase 68 requires 60 (stdlib) and 65 (kpm): self-hosted build must fetch deps
3. Phase 70 atomics depend on the threading model from Phase 57 — land the
   atomics portion with or after 57, not before
4. INDEPENDENCE is declared only when CI builds karkain-from-karkain green
   for three consecutive releases
5. Phase 71 (Math IR) before 72: Tensor IR builds on Math IR foundation
6. Phase 72 (Tensor IR) before 73-74: CPU backend and autodiff need Tensor IR
7. Phase 73 (CPU backend) before 74: autodiff gradient execution needs reference backend
8. Phase 75 (Backend abstraction) before 76-78: GPU/NPU backends need abstraction layer
9. Phase 76 (GPU/WGSL) independent of 77-78: GPU and NPU are separate backends

### SSA Pipeline Positioning (context for G2)

Karkain's strategy mirrors Go's, not LLVM's: a compact in-house SSA mid-level IR
(block-param CFG, typed registers) performs language-aware optimizations, then
emits C23 and delegates register allocation + machine codegen to GCC/Clang.
This is NOT an LLVM replacement and must not become one Ã¯Â¿Â½ the C backend IS our
portable backend. G2 work targets passes LLVM cannot see (value semantics,
ownership-driven DCE) rather than duplicating machine-level optimization.

---

## Unified Audit & Phase Readjustment (2026-08)

**Source**: ROADMAP bug register + codebase sweep + GCC `-Wall` + gap register (L1â€“L10, G1â€“G7).
Total tracked items: **38** (8 bugs, 20 placeholders, 15 gaps, 6 dead code issues).

### Bugs (must fix)

| ID | Issue | Status |
|---|---|---|
| BUG-1 | Struct codegen broken â€” raw field types, boxed assignments | FIXED |
| BUG-2 | Option/Result match arms always compile to `1` | FIXED |
| BUG-3 | Index assignment `m[k] = v` generates assignment to rvalue | FIXED |
| BUG-4 | `?` error propagation is a no-op | FIXED |
| BUG-5 | Borrow checker lacks lexical scoping | FIXED |
| BUG-6 | `make_int` pool mutable shared state | FIXED |
| BUG-7 | Enum variants with payloads emit undefined `_make` | FIXED |
| BUG-8 | Typed declarations mix `char*` vs `Value*` | FIXED |

Audited in PHASE 79: all BUG-1..8 fixed with regression coverage.
Evidence: `pkg/cli/bugfix_e2e_test.go` (BUG-4/7/8), `pkg/sema/borrow_checker_test.go` Phase 51 Groups A-I + `TestBorrowCheck_*` (BUG-5), `pkg/codegen/phase19_test.go` (BUG-1/2/3), `pkg/codegen/codegen.go:572` value-by-value pool (BUG-6).

### Placeholders (emit only comments or dummy values)

| ID | Feature | Phase |
|---|---|---|
| P1-P4 | Actor spawn/receive/send/runtime | 57 |
| P5-P8 | HTTP, reflect, comptime | 60 |
| P9 | GPU kernel host launch | 58 |
| P10 | LSP server | 66 |
| P11-P12 | Package manager fetch | 65 |
| P13-P20 | Quantum distributed/QEC/QML/CNOT/CZ/RY | 59 |

### Gaps (safety, quality, tooling)

| ID | Gap | Phase |
|---|---|---|
| G1 | No ownership-enforced deallocation | 63 |
| G2 | No panic/error boundary | 63 |
| G3 | `?` propagation incomplete | 63 |
| G4 | UTF-8 blindness | 55b |
| G5 | Slices copy, don't view | 55b |
| G6 | Deep equality missing | 55b |
| G7 | No checked arithmetic | 55b |
| G8 | No defer/RAII | 63 |
| G9 | rawc bypasses SSA | 64 |
| G10 | Concurrency below goroutine-class | 57 |
| G11 | No pub/private visibility | 65 |
| G12 | SSA optimizer too shallow | 64/67 |
| G13 | No package manager | 65 |
| G14 | No LSP/formatter/REPL | 66 |
| G15 | No SIMD/atomics | 70 |

### Dead Code / Build Quality

| ID | Issue | Phase |
|---|---|---|
| D1 | HTTP stub in every binary | 55b |
| D2 | Unused mat_rows/mat_cols | 55b |
| D3 | Unused SHA256 results | 55b |
| D4 | MSVC #pragma on GCC | 55b |
| D5 | No verbose flag | 55b |
| D6 | No #line directives | 55b |

### Unified Execution Order

``
55b: Tooling & Dead Code Cleanup + deep equality + checked arithmetic
56:  Self-Hosting Completion
57:  Actor & Concurrency Runtime (P1-P4)
58:  GPU Kernel Integration (P9)
59:  Quantum Pipeline (P13-P20)
60:  Standard Library (P5-P8)
63:  Full Borrow Checker + Ownership (BUG-5, G1-G3, G8)
64:  IR Optimizer Depth I (G9, M2)
65:  Package Manager + Modules (P11-P12, G11)
66:  Toolchain - LSP/fmt/REPL (P10)
67:  IR Optimizer Depth II
68:  Self-Hosting Completion (INDEPENDENCE)
69:  Ecosystem Hardening + v1.0
70:  SIMD Vector Types + Atomics
71:  Math IR Foundation (Karkain-owned mathematical IR)
72:  Tensor IR Foundation (Karkain-owned tensor IR, shape system)
73:  CPU Reference Backend (correctness oracle for tensor ops)
74:  Autodiff Integration (wire existing autodiff to Tensor IR)
75:  Backend Abstraction (execution planner, CPU/GPU/NPU dispatch)
76:  GPU/WGSL Integration (wire existing WGSL codegen)
77:  NPU Abstraction + Backends (Intel/Qualcomm/Apple adapters)
78:  NPU Optimization (fusion, memory planning, INT8/INT4 quantization)
``

### Math/Tensor/NPU Phase Dependency Graph

``
71 (Math IR)
  |
  v
72 (Tensor IR)
  |
  v
73 (CPU Backend) <-- 74 (Autodiff) depends on this
  |
  v
75 (Backend Abstraction)
  |          |
  v          v
76 (GPU)    77 (NPU)
             |
             v
           78 (NPU Optimization)
``

## Phase Completion Record (post-78)

| Phase | Scope | Status |
|-------|-------|--------|
| 79 | Compiler Integrity, IR Architecture & Self-Hosting Readiness Audit (evidence-based audit; Phase 80 gate = READY WITH PREREQUISITES) | COMPLETE (`docs/audit/PHASE-79-*`) |
| 80 | SIMD / Vector Execution Architecture (cross-backend parity baseline `pkg/backend/parity`; GNU-vector SIMD runtime; oracle completeness) | COMPLETE (`docs/audit/PHASE-80-*`) |
| 81 | Compiler Correctness (immutable/mutability semantics, escape analysis wiring, executable probes, diagnostics hardening) | COMPLETE |
| 82 | Language Conformance, Examples & Developer Tooling Foundation: conformance corpus (48 native tests), probes corpus (11 golden programs), algorithm corpus expansion (21 programs), `check --format=json` + `fmt` toolchain contract, real LSP wiring, `ide info` contract, VS Code extension, CI steps | COMPLETE (`conformance/`, `examples/probes/`, `docs/audit/PHASE-82-*`) |
| 83 | Compiler Symbol Namespacing, True Diagnostic Spans & LSP↔CLI Pipeline Sharing: deterministic `karkain_user_*` C namespace (Go + self-host parity), user-function builtin shadowing, `E-K-RES` true columns (`endColumn` + `excerpt` in the v1 JSON contract), `pkg/source` line-index model, single `cli.AnalyzeSource` driver shared by `check` and LSP with real-time `didChange` sync, undefined-identifier resolution, array-return semantics, conformance corpus 48→59 tests + `namespace` probe golden (12) | COMPLETE (`pkg/source/`, `pkg/cli/checker.go`, `conformance/010..011`, `docs/audit/PHASE-83-FINAL-REPORT.md`) |
| 84 | Native Codegen, Linker & Debug Information: Karkain-owned object model (sections/symbols/relocations/debug-info), scope-aware SymbolTable with `karkain_user_*` namespacing via `CollectSymbolsFromAST`, RelocationManager (Addr32/64, PCRel32/64, PLT32, GOT32), Linker (symbol resolution, section layout, entry-point handling, Executable), DebugInfoBuilder + SourceAddressMap bidirectional source↔address, CodegenDiagnostics wrapping Phase-83 `E-K-CG`, `NativeGenerator.GenerateObject` AST→Object, 48 focused tests | COMPLETE (`pkg/codegen/object.go`, `pkg/codegen/linker.go`, `pkg/codegen/symbols.go`, `pkg/codegen/relocation.go`, `pkg/codegen/debug.go`, `pkg/codegen/diagnostics.go`, `docs/native-codegen.md`) |
| 85 | Native Build & Linking Integration: NativeBuilder orchestrating Object→Linker→Executable pipeline (`NativeBuilder.Build`, `BuildMultiObject`), CLI integration (`--target=native-link` dispatches to `nativeBuildCommand`/`nativeRunCommand`), KOBJ binary artifact format (sections/symbols), debug address relocation from 0-based counters to linked .text base (0x1000+) via `relocateDebugAddresses`, SourceAddressMap built from relocated DebugInfo, 8 NativeBuilder tests + 6 CLI integration tests | COMPLETE (`pkg/codegen/native_builder.go`, `pkg/cli/commands.go`, `pkg/cli/native_build_test.go`, `docs/native-codegen.md`) |
| 86 | Developer Toolchain: test timeout safety (30s goroutine+select), multi-file formatter (`karkain fmt .` recursive), exported `Canonicalize` for LSP reuse, `textDocument/formatting` LSP support, `ExitLint(7)` exit code, 8 focused tests | COMPLETE (`pkg/cli/testing.go`, `pkg/cli/formatter.go`, `pkg/cli/exitcodes.go`, `pkg/lsp/handler.go`, `pkg/lsp/protocol.go`, `docs/audit/PHASE-86-FINAL-REPORT.md`) |
| 87 | Standard Library Foundation: stdlib structure (core/string/collections/math/io/system modules), `std.*` import resolution wired into module graph via `findStdlibDir`, 6 focused module-resolution tests | COMPLETE (`stdlib/core/core.kark`, `stdlib/string/string.kark`, `stdlib/collections/collections.kark`, `stdlib/system/system.kark`, `pkg/module/module.go`, `docs/stdlib.md`) |
| 88 | Self-Hosted Compiler Foundation: self-hosted Karkain compiler compiles via bootstrap (src/compiler/*.kark → C23 → native executable), lexer/parser/AST/sema/codegen in Karkain, 7 representative test programs, 8 focused Go tests, build script | COMPLETE (`src/compiler/main.kark`, `src/compiler/ast.kark`, `src/compiler/lexer.kark`, `src/compiler/parser.kark`, `src/compiler/sema.kark`, `src/compiler/codegen.kark`, `scripts/build-kcc.sh`, `kcc-tests/`, `pkg/cli/phase88_test.go`, `docs/self-hosted-compiler.md`) |
| 89 | Self-Hosted Runtime & Toolchain: Karkain-owned runtime boundary (Value type system, container ops, I/O primitives, platform abstraction, initialization contract), runtime/ directory with 5 boundary docs, acceptance test proving self-hosted compiler → C23 → gcc → native executable pipeline, 12 focused Go tests | COMPLETE (`runtime/types.kark`, `runtime/io.kark`, `runtime/platform.kark`, `runtime/init.kark`, `runtime/boundary.kark`, `kcc-tests/acceptance.kark`, `pkg/cli/phase89_test.go`, `docs/runtime.md`) |
| 90 | Production Release Gate / Karkain 1.0: production build workflow verified, CLI commands validated (check/build/fmt/lint), stdlib imports work, runtime type discrepancy resolved as intentional bootstrap limitation, release acceptance test, 15 focused Go tests, version 1.0.0 consistent | COMPLETE (`kcc-tests/release.kark`, `kcc-tests/stdlib_import_test.kark`, `pkg/cli/phase90_test.go`, `docs/audit/PHASE-90-FINAL-REPORT.md`) |
| 91 | Comprehensive Validation & Release Decision: documentation audit (SPEC.md 0.14.0→1.0.0, Phase 56→88 ref, stdlib.md async/gpu), regression suite GREEN across all packages, release decision KARKAIN 1.0 — RELEASE READY | COMPLETE (`docs/audit/PHASE-91-FINAL-REPORT.md`) |
| 92 | HIR Infrastructure: High-Level IR with typed nodes (20 expression kinds, 16 statement kinds, 19 type kinds), AST→HIR lowering via `BuildHIR()`, `Format()` debug output, 7 passing tests including 5-program round-trip | COMPLETE (`pkg/ir/hir/`) |
| 93 | Typed SSA IR: TypeRegistry, TypeBits/TypePromote/TypeIsCompatible type-lattice, Cooper et al. dominance tree, iterative liveness, SSA verifier, 30 tests | COMPLETE (`pkg/ir/ssa/typed.go`, `dom.go`, `liveness.go`) |
| 94 | SSA Optimization Pipeline: Mem2Reg, FoldConst with algebraic simplification, CSE, DCE, LICM, Pipeline orchestrator with fixpoint iteration and stats, 49 passing tests including benchmarks | COMPLETE (`pkg/ir/ssa/`) |
| 95 | Self-Hosted kcc Owns the Core Pipeline: `src/compiler` self-hosted compiler is the primary engine for lex+parse+sema+C codegen (`--engine=kcc`/`KARKAIN_ENGINE=kcc`), staleness-checked rebuild, Go fallback for LSP/exotic backends, parity gate reproducing all 12 probe goldens + 11 conformance files (59 assertions), bootstrap bitwise identity | COMPLETE (`pkg/cli/kcc_engine.go`, `cmd/karkain/main.go`, `pkg/cli/phase95_parity_test.go`, `docs/audit/PHASE-95-FINAL-REPORT.md`) |
| 96 | Self-Hosted kcc Owns the Test Runner: `karkain test --engine=kcc` runs through the self-hosted engine — `*_test.kark` discovery mirroring Go `findTestFiles`, per-test synthesized drivers with PASS/FAIL, `--filter`, Go-parity summary parsed by `KCCTestCommand` (`failed>0` → `ExitTest(4)`), temp-sandboxed; root-cause fixes for `INT==BOOL` comparison, Windows `system("./")`, and empty-directory classification; phase-96 parity gate (conformance 59 assertions, failing test, single-file+filter, empty dir); bootstrap bitwise identity preserved | COMPLETE (`src/compiler/main.kark`, `pkg/cli/kcc_engine.go`, `cmd/karkain/main.go`, `pkg/cli/phase96_parity_test.go`, `docs/audit/PHASE-96-FINAL-REPORT.md`) |
| 97 | Default kcc Engine + Manifest Dependency Resolution: kcc is the DEFAULT engine (`EngineFromEnv`), `--engine go|kcc` unchanged, manifest dependency resolution wired into source assembly (`kccAssembleSource`, `projectModuleSources`), bootstrap pipeline pinned Go, parity gate 6 tests | COMPLETE (`pkg/cli/phase97_parity_test.go`, `docs/audit/PHASE-97-FINAL-REPORT.md`) |
| 98 | NPU Integration into Compiler: `@target(...)` attribute end-to-end (parser → sema validation `targets: cpu, npu` → C comment → `NPUDispatcher` with CPU oracle fallback; self-hosted parser/codegen parity; `kccTargetPreflight` on both engines; 26 parity tests) | COMPLETE (`pkg/sema/npu_check.go`, `pkg/codegen/npu_compiler.go`, `docs/npu-targeting.md`, `docs/audit/PHASE-98-FINAL-REPORT.md`) |
| 99 | Self-Hosted Parser & Type Checker: `src/compiler/atypes.kark` (type inference) + `src/compiler/checker.kark` (two-pass mirror of `pkg/sema/resolve.go`), `checkFile` gate `error[K1XX]`→exit 3, 14 error fixtures, compiler sources type-check clean (assembled 214,127 bytes), bootstrap identity stage2==stage3, harness stabilizers (kcc subprocess 5min timeout, Go engine pins) | COMPLETE (`pkg/cli/phase99_selfhosted_test.go`, `examples/type_errors/`, `docs/audit/PHASE-99-FINAL-REPORT.md`) |
| 100 | Runtime Error Model: `karkain_runtime_error()` + checked div/mod/indexing on both engines, `runtime error: <kind> at <file>:<line>` + exit(1), source-file tracking, string concat int/float/bool, 7 runtime-error fixtures + positive control, parity gates | COMPLETE (`pkg/codegen/codegen.go`, `src/compiler/codegen.kark`, `examples/runtime_errors/`, `pkg/cli/phase100_runtime_test.go`, `docs/audit/PHASE-100-FINAL-REPORT.md`) |
| 101 | Libc-Free Runtime Foundation: `runtime/freestanding/` arena allocator + raw OS/I/O/string/math primitives (compile+link WITHOUT libc), runtime-error stack traces end-to-end (`KARKAIN_MAX_FRAMES` 128, `stack:` dump parity `inner:2/outer:5/main:9`), self-hosted checker guards, GMP boundary deferred to 109 | COMPLETE (`pkg/cli/phase101_stacktrace_test.go`, `pkg/runtime/phase101_freestanding_test.go`, `docs/audit/PHASE-101-FINAL-REPORT.md`) |
| 102 | Native Runtime Core: `runtime/core/` libc-free heap (alloc/calloc/realloc/free, coalescing free list), tagged `Value` (int/float/bool/string/array), length-prefixed `NativeString`, `NativeArray`, value-level I/O; freestanding arena rewrite (`add>=used+need`), compiled `-ffreestanding -nostdlib` without libc, GMP/map/option stay behind Phase 109 | COMPLETE (`runtime/core/`, `pkg/runtime/phase102_core_test.go`, `docs/audit/PHASE-102-FINAL-REPORT.md`) |
| 102-F | Language Foundation Completion: 13 golden targets `examples/language_foundation/` byte-identical on Go+kcc engines; parity fixes (kcc `parseStructDecl` `;` separator, Go BorrowChecker `fnRoot`, kcc `for` init forms, Go `genForStmt` double-`;;`); foundation/compiler-self-check gates | COMPLETE (`pkg/cli/phase102_foundation_test.go`, `docs/audit/PHASE-102-LANGUAGE-FOUNDATION-FINAL-REPORT.md`) |
| 103 | Module System v2 (Core): export sets (`public` func/type/enum) on BOTH engines, qualified-name resolution (`math.twice(21)` → module export set), cross-module private/missing-import/undefined/wrong-module/duplicate diagnostics on the Go resolver, self-hosted kcc `public` parse + qualified-call lowering, stdlib private-export exemption, module acceptance program + 4 rejection fixtures | COMPLETE (`pkg/sema/resolve.go`, `src/compiler/parser.kark`, `src/compiler/ast.kark`, `examples/module_system/`, `examples/module_system_errors/`, `pkg/cli/phase103_module_test.go`, `docs/audit/PHASE-103-MODULE-SYSTEM-FINAL-REPORT.md`) |

### Current phase

**PHASE 107 COMPLETE** — Concurrency Runtime (work-stealing scheduler,
tasks, channels, actors — compiler-integrated end-to-end).
See `docs/audit/PHASE-107-CONCURRENCY-RUNTIME-FINAL-REPORT.md` (and the
long-form entries below for Phase 106, 103–105).
`.kark` programs can now `spawn(fn, args...)` + `join(t)` tasks with
deterministic failure status, stream over bounded/unbounded blocking
channels (`channel(cap)` / `chanSend` / `chanClose` / `receive(ch)`, close
deterministic + drains-then-`0`), and run serialized actors
(`actor("handler", state)` / `actorSend` / `actorState` /
`setActorState` / `actorStop`) on a work-stealing scheduler — through the
Go engine front end, the Phase 107 codegen wrappers, the embedded C runtime
and the host C compiler. Gates: the standalone C-runtime test (> 1M msg/s),
7 codegen gates (incl. the 999-task `332833500` stress) and 2 E2E CLI gates
(`examples/concurrency/pipeline/main.kark` → `144/10/20/30/0/6`,
byte-identical repeat runs). kcc parses the keywords already; codegen
parity is the documented post-107 boundary.

**PHASE 106 COMPLETE** — SIMD & Vector Types.
See `docs/audit/PHASE-106-SIMD-VECTOR-TYPES-FINAL-REPORT.md` (and the
long-form entries below for Phase 103–105).
The Go resolver (`checkQualifiedCall` in `pkg/sema/resolve.go`) enforces the
module contract with precise diagnostics: private cross-module calls, missing
`import`, undefined module functions, wrong-module targets, and cross-module
name collisions all exit 3. The self-hosted parser gained a `TK_PUB` branch
(`public func/type/enum` mirrors Go's parse-error contract — `public let` is
rejected on both engines) and lowers dotted calls onto the flat
`karkain_user_*` namespace (`math.twice(21)` and `acc.deposit(10)` identically
emit the bare callee), keeping C-interop (`C.*`) dotted. Stdlib files are
exempt from the private-export rule (framework API surface; physical `public`
markers land with the stdlib-v2 Phase 109 boundary). Module acceptance target
`examples/module_system/` runs byte-identical on both engines; 4 rejection
fixtures under `examples/module_system_errors/` pin the diagnostics. Full
regression sweep green: parser/sema/codegen/vet, Phase 97/99/100/101/102
gates, conformance 59/59, foundation Go+kcc goldens, compiler-sources
self-check under kcc.

**PHASE 104 COMPLETE** - Debug Information (DWARF). Tier 3 (Phases
104-109) begins. Native executables now carry real DWARF 4 debug sections
(`.debug_info`/`.debug_abbrev`/`.debug_str`/`.debug_line`) emitted by the
Karkain-owned `DwarfEmitter` (`pkg/codegen/dwarf.go`) from the Phase 84
`DebugInfo` model: a compile unit with one subprogram per function (nested
local variables, base-type DIEs, `DW_AT_high_pc` size form on relocated
addresses), and a correct v4 line-number state machine with end-sequence
per-function resets. The linker lays out `SectionTypeDebug` and
`NativeBuilder.Build`/`BuildMultiObject` attach the sections after
relocation. A self-hosted DWARF-4 reader (`pkg/codegen/dwarf_parse.go`)
round-trips the sections and drives an `eval`-free
`DwarfTextDump` (readelf-style). Gates: `pkg/codegen/dwarf_test.go`
(8 tests) + `pkg/cli/phase104_dwarf_test.go` (3 E2E through the real
pipeline); full regression sweep green (conformance 59/59, Phase 102
goldens, probes, all pkg suites, `go vet`). Known limitation (documented,
NOT a defect): no ELF/PE container writer exists, so debuggers/readelf
remain on the gcc C-transpile path; the DWARF sections are ready to embed
once a container writer lands.

**PHASE 105 COMPLETE** - Error Recovery & Incremental Compilation.
Multi-error reporting: an engine-agnostic project-wide syntax preflight
(`pkg/cli/multierror.go`) parses every unit file independently and renders
ALL recoverable parse errors in one invocation (exit 3) through BOTH
`check` engine paths (Go `CheckCommandFormatted` and default-kcc
`KCCCheckCommand`, the latter previously `[ok]`-ing multi-error inputs);
the semantic/type-check stage aggregates resolve diagnostics the same way
(`examples/phase105_errors/` — 6 recoverable parse errors + 3 resolve
errors, both gated). Incremental compilation: new `pkg/compiler` cache
engine — content-addressed whole-assembly cache (generated C + linked exe)
keyed by an ordered project content-hash, per-module sha256 + interface
hashes (public signature/header/import only — bodies never recompile
dependents, zero reparse on no-op), dependency-aware invalidation
(compiled/reused/invalidated), compiler-identity key invalidation, atomic
writes, and failed builds every-never poison the cache; `karkain build
--incremental` / `--incremental-cache` and `karkain clean` purge
(`.karkain-cache`). Real 3-module example `examples/phase105/`
(main+math+strings). Gates: `pkg/compiler/incremental_test.go` (10) +
`pkg/cli/phase105_incremental_test.go` (3 E2E: equivalence, cache
correctness, failed-build-safe) + `pkg/cli/phase105_multierror_test.go`
(5). Measured: clean 2790 ms → no-op 101 ms (~27×); post-clean rebuild
byte-identical C and stdout. Full regression sweep green including the
entire `pkg/cli` suite (conformance 59/59, all Phase 97–104 gates), all
other pkg suites, `go vet`, `go build`. Design decision (documented,
started by design): whole-assembly cache now; per-module `.o` TU splitting
deferred as post-105 work on top of the same interface machinery.

**PHASE 106 COMPLETE** - SIMD & Vector Types. Lane-vector types:
`[N]f32`/`[N]f64`/`[N]i32`/`[N]i64` variable annotations select Karkain-owned
C lane types `karkain_<elem><width>x<lanes>` (float → `__m128`/`__m256` with
auto `-mavx` for 256-bit widths on x86; int → GNU `vector_size` types), and
`@simd_splat/add/sub/mul/div/sum` lower to portable `karkain_simd_*` runtime
helpers using GNU vector operators (one body serves x86 intrinsics and ARM
`vector_size` types; integer mul/div/sum reduce lane-wise). Type flow:
SIMD declaration records the name→type in `Generator.simdVars`; operand
dispatch takes lane width from the vector operand; splat seed infers f32 vs
i32; `@simd_sum` reduces a lane vector to a printable scalar `Value`;
scalar-only operands keep the Phase 70 SSE fallback. Emitted via
`pkg/codegen/simd_emit.go` + `genSIMDDecl`/typed `genSIMDExpr` in
`pkg/codegen/codegen.go`, SSA `lowerStmt` → raw-c SIMD decls in
`pkg/codegen/lower.go`, and the C header now appends the SIMD runtime.
Bonus hardening: the AVX2 matrix kernel is now portable `mul+add` (no
`_mm256_fmadd_pd`), so `-mavx2` no longer demands `-mfma`. Gates:
`pkg/codegen/simd_emit_test.go` (7) + updated `phase70_test.go` +
`pkg/cli/phase106_simd_test.go` (3 E2E: golden executable output 40/-8/48/
12/5/56/40, SIMD-vs-scalar differential, and the AVX assembly probe proving
`vaddps`-family instructions under the pipeline's own `-O0 -mavx` flags).
Full regression sweep green: full `pkg/cli` 940.5s (conformance 59/59 and
all Phase 97–105 gates), whole `pkg/codegen`, sema/parser/ir,
backend/npu/runtime/diagnostics, `go vet`, `go build`; bootstrap stage-1
build succeeds (codegen change safe for the compiler's own sources); the
stage-2 build SEGFAULT on the ~4GB-RAM host remains the documented
kcc-build-mode OOM limitation (no `src/compiler` changes; `@simd` unused
by compiler sources). kcc already parses `@simd_*` (`NODE_SIMD_BUILTIN` in
`src/compiler/ast.kark`); kcc semantic/codegen parity is a documented
post-106 boundary. Report:
`docs/audit/PHASE-106-SIMD-VECTOR-TYPES-FINAL-REPORT.md`.

**PHASE 107 COMPLETE** - Concurrency Runtime (work-stealing scheduler,
tasks, channels, actors; compiler integration + gates; §38 IMPLEMENTATION
RESULT). A compiler-neutral C runtime under `runtime/concurrency/c/`
(pthreads/Win32: `karkain_sched_t` workers, grab queue, shared pending
drain, work stealing, `karkain_task_t` with atomic `done` + cv + `status`
long, `karkain_channel_t` bounded/unbounded blocking send/recv with
deterministic close, `karkain_actor_t` mailbox + serialized dispatcher with
state-cell ownership; runtime frees task `ctx`; no busy-spin shutdown).
Embedded via `runtime/concurrency/embed.go` (`//go:embed c/...` — the
runtime sources are assembly-time strings). Compiler integration: parser
(`spawn(fn, args...)` `SpawnExpr`, `receive(ch)` expression, `channel(...)`
/`actor(...)` keyword calls, legacy `receive(ch) -> var` preserved), sema
builtins, codegen `pkg/codegen/conc_runtime.go` (recursive scans → stable
spawn-site wrapper indices + actor handler ids; `concWrapperC` per-site
`karkain_run_<idx>` wrappers + actor dispatcher adopting returned state;
`concRuntimeAPIC` Value glue; `Generator` `usesConcurrency`/`concFns`/
`concSpawned`/`concHandlers` state; header appended after SIMD, wrappers
after forward decls). Language surface:
`spawn/join/wait_all/channel/chanSend/chanClose/receive` and
`actor/actorSend/actorState/setActorState/actorStop`; `send` is a lexed
keyword so the builtin is `chanSend`. Gates: `pkg/runtime/
phase107_concurrency_test.go` (7 scenarios + >1M msg/s; `spawn:0,-1`,
`sum:210`, `actor:100`, `actorstop:1`, `steal:1000`, `bounded:210`,
`close:1,1,0,0,7,1`), `pkg/codegen/phase107_concurrency_test.go` (7:
spawn/join 42/10/-1/done, spawn statements, channels 10/20/0, actors `6`,
embedded-runtime markers, 999-task stress `332833500`, generated-source
presence) and `pkg/cli/phase107_concurrency_test.go` (2 E2E:
`examples/concurrency/pipeline/main.kark` → `144/10/20/30/0/6` +
byte-identical repeat runs). Full regression sweep green: full `pkg/cli`
1723.8s (conformance 59/59 + all Phase 97–107 gates), `pkg/codegen`,
sema/parser/lexer, ir/hir/ssa, pm, source, diagnostics, compiler, module,
backend/npu/runtime, `go vet`, `go build`; bootstrap stage-1 build ok
(Go codegen change safe for the compiler's own sources);
stage-2 SEGFAULT reproducibly = documented ~3.9GB-RAM host OOM class
(no `src/compiler` changes; concurrency codegen is gated by
`usesConcurrency`; Go-engine `check` of `src/compiler/main.kark` clean).
kcc already lexes/parses the keywords (`TK_SPAWN/TK_SEND/TK_RECEIVE/
TK_ACTOR/TK_CHANNEL`); kcc codegen parity is a documented post-107
boundary. Report:
`docs/audit/PHASE-107-CONCURRENCY-RUNTIME-FINAL-REPORT.md`.
Report: `docs/audit/PHASE-105-ERROR-RECOVERY-INCREMENTAL-FINAL-REPORT.md`.
