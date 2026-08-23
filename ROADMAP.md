# Karkain Roadmap — Complete Development Plan

## Vision

Karkain is a unified heterogeneous systems language for:
- **CPU/GPU computing** — write host + kernel in one language
- **Quantum computing** — first-class quantum primitives, circuits, error correction
- **Multi-dimensional lattice simulation** — tensor types, autograd, distributed computation
- **Systems programming** — Rust-like safety, C-like performance, Go-like simplicity

## Guiding Principles

1. **Maturity over features** — depth, correctness, performance, verification before expansion
2. **Semantic foundations first** — value representation, ownership, lifetimes, IR before features
3. **Working > ambitious** — fix broken fundamentals before adding new capabilities
4. **Incremental verification** — every phase must compile, pass E2E, pass all tests

---

## Project Statistics (as of Phase 49)

| Metric | Value |
|--------|-------|
| Phases completed | 49 |
| Go code (compiler) | ~34,600 lines across 100 files, 13 packages |
| C runtime | ~777 lines (actor, quantum, reflect, rpc) |
| Self-hosting sources | 6 .kar files (src/compiler/) |
| Backend codegen files | 15 (C, WGSL, OpenCL, QIR, OpenASM, OpenPulse, QML, QEC, etc.) |
| Standard library | 6 .kar files (io, string, math, async/actor, gpu) |

---

## Completed Phases Summary

### Tier 1: Language Core (Phases 1–17)
Lexer, parser, code generator, basic types (int, float, string, bool), arrays, maps,
control flow (if/else, while, for), functions, closures, structs, enums, quantum
primitives (qreg, gates, measurement), kernel declarations, matrix operations, address-of/dereference.

### Tier 2: Multi-Backend Emission (Phases 18–26)
GPU host-side codegen, OpenCL kernel emission, WGSL compute shader generation,
generic kernel monomorphization, multi-backend dispatch, AVX2 SIMD matrix kernels,
monomorphized generics with trait constraints, hardware-native tensor types, autograd engine.

### Tier 3: Quantum Computing Stack (Phases 28–36)
Quantum primitives, OpenQASM 3.0 + QIR backends, hybrid VQE/QML gradient engine,
GPU quantum state-vector simulator, distributed multi-GPU quantum, quantum noise models,
OpenPulse codegen, quantum error correction, quantum circuit optimization,
binary IR bytecode (.kbc), unified multi-target JIT engine + C ABI FFI.

### Tier 4: Developer Experience & Concurrency (Phases 37–40)
Distributed actor system, coroutine/async runtime & green thread scheduler,
LSP engine with diagnostics/completion/hover/go-to-def, package manager.

### Tier 5: Safety & Correctness (Phases 41–49) ← CURRENT
- **41**: Hybrid memory model — `&T`/`&mut T` references, `move(x)`, `@raw(addr)`
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
- Field access passes raw `char*` to `print_value(Value*)` — type mismatch
- **Impact**: Structs are unparseable/unusable end-to-end

### BUG-2: Option/Result match arms are broken
- `Some(v)` and `None` patterns both compile to condition `1` (always true)
- First match arm always wins; later arms are dead code
- Only enum/literal/wildcard patterns produce real comparisons
- **Impact**: Core error-handling semantics are non-functional

### BUG-3: Index assignment generates invalid C
- `m["k"] = v` becomes `array_get(m, make_string("k")) = v;` — assignment to rvalue
- **Impact**: Map/string mutation is impossible

### BUG-4: `?` error propagation is a no-op
- `PropagateExpr` passes operand through without any Result checking
- **Impact**: Error propagation doesn't work

### BUG-5: Borrow checker lacks lexical scoping
- Flat variable map shared across function body — two blocks using same name share state
- Borrows never expire — `&x` poisons variable for rest of function
- `BorrowError.Line` is always 0
- **Impact**: Safety guarantees are unreliable

### BUG-6: `make_int` pool is mutable shared state
- Small integer pool values can be mutated, corrupting the pool for all users
- **Impact**: Subtle data corruption possible

### BUG-7: Enum variants with payloads unimplemented
- `EnumVariantExpr` with payload emits `EnumName_Variant_make(val)` — nothing defines this
- **Impact**: Tagged unions with data don't compile

### BUG-8: Typed declarations mix representations
- `var s string = "hi"` → `char* s = make_string("hi")` (Value* into char* slot)
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
| `binary_op(Value, op, Value)` → `Value` | Change signature to accept/return by value | HIGH |
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
| AST → IR lowering | New pass replacing direct AST → C | CRITICAL |
| IR → C23 emission | IR drives the C backend | CRITICAL |
| IR type system | Typed registers, not just Value* everywhere | HIGH |
| IR optimization passes | Constant folding, dead code elimination | MEDIUM |
| IR verification | Validate IR well-formedness | MEDIUM |

### PHASE 54: Closure & Capture Semantics
**Goal**: Lambdas properly capture outer variables.

| Task | Details | Priority |
|------|---------|----------|
| Closure data structures | Heap-allocated capture blocks | HIGH |
| Escape analysis → closure conversion | Escaping locals become capture fields | HIGH |
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
**Goal**: `src/compiler/*.kar` can compile itself.

| Task | Details | Priority |
|------|---------|----------|
| Port lexer to Karkain | `src/compiler/lexer.kar` functional | HIGH |
| Port parser to Karkain | `src/compiler/parser.kar` functional | HIGH |
| Port codegen to Karkain | `src/compiler/codegen.kar` functional | HIGH |
| 3-stage bootstrap pipeline | Karkain → C → GCC → karkain.exe | HIGH |
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
| Kernel → WGSL pipeline | `kernel` → WGSL → WebGPU compute | HIGH |
| Kernel → OpenCL pipeline | `kernel` → OpenCL C → clBuildProgram | HIGH |
| Host-side GPU launch | Memory transfer, dispatch, synchronization | HIGH |
| GPU memory model | Device/Host/Shared memory domains | HIGH |
| CPU fallback kernels | Auto-generate CPU version of each kernel | MEDIUM |
| GPU safety verification | Compile-time checks for kernel restrictions | MEDIUM |

### PHASE 59: Quantum Compilation Pipeline
**Goal**: Wire quantum backends into main pipeline.

| Task | Details | Priority |
|------|---------|----------|
| Circuit → OpenQASM 3.0 export | `circuit` → `.qasm` file | HIGH |
| Circuit → QIR LLVM IR | `circuit` → QIR-compatible LLVM IR | HIGH |
| Circuit → OpenPulse | Pulse-level control output | MEDIUM |
| Quantum simulator bridge | Host-side state vector simulation | HIGH |
| Distributed quantum execution | Multi-GPU state vector (already coded, needs wiring) | MEDIUM |
| QEC integration | Error correction codes → syndrome extraction circuits | LOW |

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
| `karkain test` runner | Discover and run `_test.kar` files | HIGH |
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
PHASE 50: Bug Fixes & Language Correctness        ← NEXT
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

**Rationale**: Phases 50–55 are foundational correctness and language semantics.
Phases 56–59 wire up the existing (but disconnected) backend infrastructure.
Phases 60–62 complete the ecosystem.

---

## Explicitly Out of Scope (Decision Record)

**NPU, DSP, and FPGA backend targets — REMOVED from roadmap (decision: 2026-08).**

Rationale: pursuing these targets risks degrading the three non-negotiable properties
of Karkain — **speed, multithreading, memory safety**:

- Vendor-specific lowering paths fragment portability and cannot be CI-tested without hardware
- Device execution models (async NPU graphs, FPGA pipelines, real-time DSP) complicate the
  concurrency story and introduce cross-device data races
- Escape-hatch pressure from DMA/ring-buffer/quantization paths erodes borrow-checker guarantees
- Optimization effort splits across incompatible pass pipelines, threatening the "fastest" goal

Karkain's differentiator remains: **compile-time ownership safety + performance on CPU/GPU/quantum**,
not breadth of hardware targets. This decision may be revisited only after Phases 53–62 are
complete AND a portable-fallback + golden-test strategy exists per target.
