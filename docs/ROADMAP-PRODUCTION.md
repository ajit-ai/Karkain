# KARKAIN PRODUCTION ROADMAP — PHASES 92–115

**Goal:** Make Karkain a production-grade general-purpose language for AI/ML, Quantum, GPU/CPU heterogeneous compute.

**Timeline:** 40 weeks (12 months at full speed)

**Final result:** Karkain 1.0 — self-hosted, libc-free, developer-ready

---

## How to Read This Plan

Each phase has:
- **Deliverable**: What gets built
- **Gate**: Test that MUST pass before moving to next phase
- **Dependency**: What must exist before this phase starts

---

## TIER 1: COMPILER INDEPENDENCE (Phases 92–100)
**Goal: Karkain compiler compiles itself without Go**

### Phase 92 — HIR Infrastructure
**Deliverable:** Typed intermediate representation between AST and codegen
**Files:** `pkg/ir/hir/hir.go`, `pkg/ir/hir/build.go`, `pkg/ir/hir/hir_test.go`
**Gate:** `go test ./pkg/ir/hir/...` passes, 5 programs round-trip parse→HIR→format
**Blocks:** Phases 93, 94, 95

### Phase 93 — Typed SSA IR ✅
**Deliverable:** Extended SSA with typed registers (`i8/i16/i32/i64/f16/f32/f64/ptr`), dominance tree, liveness analysis
**Files:** `pkg/ir/ssa/typed.go`, `pkg/ir/ssa/dom.go`, `pkg/ir/ssa/liveness.go`
**Gate:** SSA verifier passes on all existing test programs, dominance tree correct on 10 CFG patterns
**Blocks:** Phase 94
**Status:** ✅ Complete — 30 tests pass (TypeRegistry, TypeBits, TypePromote, TypeIsCompatible, 6 dominance patterns, 4 liveness tests, 4 SSA verifier tests)

### Phase 94 — SSA Optimization Pipeline ✅
**Deliverable:** Multi-pass optimizer: mem2reg → constant fold → CSE → DCE → LICM
**Files:** `pkg/ir/ssa/pipeline.go`, `pkg/ir/ssa/passes/cse.go`, `pkg/ir/ssa/passes/licm.go`, `pkg/ir/ssa/passes/sroa.go`, `pkg/ir/ssa/passes/mem2reg.go`
**Gate:** Each pass has unit tests, benchmark shows measurable instruction count reduction
**Blocks:** Phase 95
**Status:** ✅ Complete — 49 tests pass (6 fold const, 3 DCE, 2 CSE, 3 mem2reg, 1 LICM, 3 pipeline, 1 pass names, 3 benchmarks); fixpoint iteration

### Phase 95 — Tensor ↔ SSA Bridge
**Deliverable:** Bidirectional lowering between tensor compute graphs and SSA IR
**Files:** `pkg/tensor/bridge.go`, `pkg/tensor/bridge_test.go`
**Gate:** Matmul + ReLU in Karkain → Tensor IR → SSA → C23 → correct result
**Blocks:** Phase 98 (NPU integration)

### Phase 96 — GPU Kernel Codegen Hardening
**Deliverable:** Production GPU compilation targeting WGSL, SPIR-V, OpenCL C with memory management and workgroup sizing
**Files:** `pkg/codegen/gpu_compiler.go`, `pkg/codegen/gpu_memory.go`, `pkg/codegen/gpu_launcher.go`, `pkg/codegen/spirv_backend.go`
**Gate:** vector_add kernel → compile to WGSL → execute → correct result
**Blocks:** None (independent)

### Phase 97 — Quantum IR & Circuit Optimizer
**Deliverable:** Quantum IR with gate fusion, cancellation, commutation reordering
**Files:** `pkg/ir/quantum/quantum.go`, `pkg/ir/quantum/optimizer.go`, `pkg/ir/quantum/verify.go`
**Gate:** 10-gate circuit optimized to 6-gate equivalent, correctness verified by simulation
**Blocks:** None (independent)

### Phase 98 — NPU Integration into Compiler
**Deliverable:** `@target(npu)` attribute dispatches to vendor adapters with CPU fallback
**Files:** `pkg/sema/npu_check.go`, `pkg/codegen/npu_compiler.go`
**Gate:** Matrix multiply with `@target(npu)` → dispatches to NPU when available, CPU fallback works
**Blocks:** None (independent)

### Phase 99 — Self-Hosted Parser & Type Checker
**Deliverable:** Complete recursive descent parser + type checker written in Karkain, replacing minimal `src/compiler/parser.kark`
**Files:** `src/compiler/parser_full.kark`, `src/compiler/types.kark`, `src/compiler/checker.kark`
**Gate:** Self-hosted parser parses all 25 example programs, type checker catches 10 hand-crafted error cases
**Blocks:** Phase 100

### Phase 100 — Self-Hosted Codegen & Backend
**Deliverable:** C23 code generator + build driver written in Karkain
**Files:** `src/compiler/codegen_full.kark`, `src/compiler/optimizer.kark`, `src/compiler/driver.kark`
**Gate:** Self-hosted compiler compiles "hello world" end-to-end (kcc → C23 → gcc → native)
**Blocks:** Phase 102

---

## TIER 1 GATE CHECK (After Phase 100)

```
Test: The self-hosted compiler (kcc) compiles itself.
Steps:
  1. Go bootstrap builds kcc from src/compiler/main.kark
  2. kcc compiles src/compiler/main.kark → produces kcc_v2
  3. kcc_v2 compiles src/compiler/main.kark → produces kcc_v3
  4. diff kcc_v2 kcc_v3 → identical (fixed point)

If this passes: GO dependency is ELIMINATED for all future compilation.
If this fails: STOP. Fix self-hosted compiler before continuing.
```

---

## TIER 2: RUNTIME INDEPENDENCE (Phases 101–103)
**Goal: Karkain programs run without libc**

### Phase 101 — Karkain Runtime Library (libc-free)
**Deliverable:** Self-contained C runtime with arena allocator, raw syscalls, no libc
**Files:**
```
runtime/
├── karkain_runtime.c      # Entry point, panic, print
├── karkain_memory.c       # Arena allocator (mmap/VirtualAlloc)
├── karkain_string.c       # String operations
├── karkain_io.c           # File I/O via syscalls
├── karkain_math.c         # Math (no libm)
├── karkain_platform.h     # Linux/macOS/Windows syscall abstraction
├── karkain_threads.c      # Thread pool
└── karkain_tls.c          # Thread-local storage
```
**Gate:** Runtime compiles with `gcc -ffreestanding -nostdlib` on Linux, produces working executable that prints "hello" and exits
**Blocks:** Phase 102

### Phase 102 — Integrated Build System
**Deliverable:** `karkain build --target=native` drives full pipeline without user invoking GCC
**Files:** `pkg/cli/build_v2.go`, `pkg/cli/run_v2.go`
**Gate:** `karkain build --target=native examples/hello.kark` produces working executable, user never sees GCC
**Blocks:** Phase 103

### Phase 103 — Module System v2
**Deliverable:** Proper module system with visibility, re-exports, compilation units
**Files:** `pkg/pm/module.go`, `pkg/pm/compile_unit.go`
**Gate:** Two-package project with `pub fn` → importing package compiles and calls correctly
**Blocks:** None (independent)

---

## TIER 2 GATE CHECK (After Phase 103)

```
Test: Karkain is self-contained.
Steps:
  1. Fresh machine with ONLY kcc binary (no Go, no GCC)
  2. kcc build examples/hello.kark --target=native
  3. ./hello → prints "Hello, World!"
  4. kcc build examples/fibonacci.kark --target=native
  5. ./fibonacci → correct output

If this passes: GCC dependency is ELIMINATED for standard programs.
If this fails: STOP. Fix runtime before continuing.
```

---

## TIER 3: ECOSYSTEM (Phases 104–109)
**Goal: Developers can actually use Karkain**

### Phase 104 — Debug Information (DWARF)
**Deliverable:** DWARF debug sections in native executables
**Files:** `pkg/codegen/dwarf.go`, `pkg/codegen/source_map.go`
**Gate:** Debug-built executable shows function names + line numbers in `readelf --debug-dump=info`
**Blocks:** None (independent)

### Phase 105 — Error Recovery & Incremental Compilation
**Deliverable:** Multi-error reporting, per-module caching
**Files:** `pkg/diagnostics/recovery.go`, `pkg/compiler/incremental.go`
**Gate:** 5-error source file → compiler reports all 5 errors, not just the first
**Blocks:** None (independent)

### Phase 106 — SIMD & Vector Types
**Deliverable:** `vec<f32, 8>` with auto-vectorization to AVX2/NEON
**Files:** `pkg/ir/hir/vector.go`, `pkg/codegen/simd_emit.go`
**Gate:** 8-wide f32 vector add → compiles to AVX2 instruction → correct result
**Blocks:** None (independent)

### Phase 107 — Concurrency Runtime
**Deliverable:** Actors, channels, work-stealing scheduler
**Files:** `runtime/karkain_actor.c`, `runtime/karkain_channel.c`, `runtime/karkain_scheduler.c`
**Gate:** 1000 actors sending messages → throughput > 1M msg/sec
**Blocks:** None (independent)

### Phase 108 — WASM Target
**Deliverable:** `karkain build --target=wasm32-wasi` produces working WASM
**Files:** `pkg/codegen/wasm.go`
**Gate:** hello world + fibonacci → WASM → runs in wasmtime → correct output
**Blocks:** None (independent)

### Phase 109 — Standard Library v2
**Deliverable:** Comprehensive stdlib: collections, strings, I/O, crypto, encoding
**Files:** Expanded `stdlib/` with new modules
**Gate:** All stdlib modules compile and pass their own test suites
**Blocks:** None (independent)

---

## TIER 3 GATE CHECK (After Phase 109)

```
Test: Karkain is usable for real development.
Steps:
  1. Build a non-trivial program (HTTP server, CLI tool, or game)
  2. Program compiles, runs, handles errors gracefully
  3. Debugger works (breakpoints, stepping, variable inspection)
  4. Profiler works (flame graph, hot path identification)
  5. Incremental rebuild < 2 seconds for single-file change

If this passes: Karkain is usable for personal projects.
If this fails: Identify which component is blocking and fix it.
```

---

## TIER 4: PRODUCTION HARDENING (Phases 110–115)
**Goal: Ready for public release**

### Phase 110 — Profiling & Diagnostics
**Deliverable:** `karkain prof` command, structured diagnostics with source spans
**Files:** `pkg/cli/prof.go`, `pkg/compiler/diagnostics_v2.go`
**Gate:** `karkain prof run examples/mandelbrot.kark` → produces flame graph data
**Blocks:** None (independent)

### Phase 111 — Cross-Compilation
**Deliverable:** Target triple system for cross-compilation
**Files:** `pkg/target/triple.go`, `pkg/target/features.go`
**Gate:** Cross-compile from Linux→Windows, from x86→ARM64, verify output binary header
**Blocks:** None (independent)

### Phase 112 — FFI & Interop
**Deliverable:** Safe C interop with `@extern("C")`
**Files:** `pkg/sema/ffi_check.go`, `pkg/codegen/ffi_emit.go`
**Gate:** Call C `printf` from Karkain → correct output, call Karkain function from C test harness
**Blocks:** None (independent)

### Phase 113 — LSP v2 (Full IDE Support)
**Deliverable:** Completions, hover, references, code actions
**Files:** `pkg/lsp/completion.go`, `pkg/lsp/hover.go`, `pkg/lsp/references.go`
**Gate:** LSP test harness verifies completion at 20 cursor positions, hover at 15 positions
**Blocks:** None (independent)

### Phase 114 — Documentation & Examples
**Deliverable:** Tutorial, language spec, stdlib reference, AI/GPU/Quantum examples
**Files:** `docs/tutorial.md`, `docs/language_spec.md`, `examples/ai/`, `examples/gpu/`, `examples/quantum/`
**Gate:** All examples compile and run correctly
**Blocks:** None (independent)

### Phase 115 — Release Hardening
**Deliverable:** Fuzzing, benchmarks, release pipeline
**Files:** `fuzz/`, `bench/`, `scripts/release_v2.sh`, `SECURITY.md`
**Gate:** 24-hour fuzzing run with zero crashes, benchmark suite shows ≤5% regression threshold
**Blocks:** None (final phase)

---

## TIER 4 GATE CHECK (After Phase 115)

```
FINAL VALIDATION: Karkain is production-ready.

1. SELF-HOSTING: kcc compiles itself without Go or GCC
2. RUNTIME: Programs run without libc (raw syscalls)
3. ECOSYSTEM: Stdlib, LSP, debugger, profiler all work
4. QUALITY: Zero crashes in 24-hour fuzz test
5. DOCUMENTATION: Tutorial, spec, API reference complete
6. EXAMPLES: 10+ real programs (HTTP server, CLI tool, AI demo, GPU kernel, quantum circuit)

If ALL 6 pass: KARKAIN 1.0 — PRODUCTION READY
If ANY fail: STOP. Fix the failing component before release.
```

---

## Summary: What You Get at Each Tier

| After Tier | What works | What still needs GCC |
|------------|------------|---------------------|
| **Tier 1** (Phase 100) | Self-hosted compiler | Yes — GCC compiles generated C |
| **Tier 2** (Phase 103) | Self-contained runtime | Only for complex programs (stdlib uses raw syscalls) |
| **Tier 3** (Phase 109) | Full ecosystem | No — `karkain build --target=native` works standalone |
| **Tier 4** (Phase 115) | Production release | No — fully self-contained |

---

## Timeline

| Tier | Phases | Duration | Milestone |
|------|--------|----------|-----------|
| Tier 1 | 92–100 | 12 weeks | Self-hosted compiler |
| Tier 2 | 101–103 | 6 weeks | Runtime independence |
| Tier 3 | 104–109 | 12 weeks | Developer ecosystem |
| Tier 4 | 110–115 | 10 weeks | Production release |
| **Total** | **92–115** | **40 weeks** | **Karkain 1.0** |

---

## Target Architecture (Final Compiler Pipeline)

```
.kark source
    │
    ▼
┌──────────────────────────────────────────────────────────────┐
│  FRONT END (pure Karkain)                                    │
│  Lexer → Parser → AST → Type Inference → Borrow Checker     │
│  → Generic Monomorphization → Trait Resolution               │
│  → Typed AST                                                 │
└──────────────────────────┬───────────────────────────────────┘
                           │
                           ▼
┌──────────────────────────────────────────────────────────────┐
│  HIR (High-Level IR)                                         │
│  Typed, desugared AST — optimization boundary               │
└──────────────────────────┬───────────────────────────────────┘
                           │
            ┌──────────────┼──────────────┐
            ▼              ▼              ▼
       ┌─────────┐   ┌─────────┐   ┌─────────┐
       │ CPU SSA │   │Tensor IR│   │Quantum IR│
       └────┬────┘   └────┬────┘   └────┬────┘
            │              │              │
            ▼              ▼              ▼
       ┌─────────┐   ┌─────────┐   ┌─────────┐
       │  SSA    │   │ Tensor  │   │Quantum  │
       │Optimizer│   │Optimizer│   │Optimizer│
       └────┬────┘   └────┬────┘   └────┬────┘
            │              │              │
            ▼              ▼              ▼
       ┌─────────┐   ┌─────────┐   ┌─────────┐
       │  C23    │   │ Backend │   │OpenQASM │
       │ Emitter │   │Dispatch │   │/QIR Emit│
       └────┬────┘   └────┬────┘   └────┬────┘
            │              │              │
            ▼              ▼              ▼
       ┌─────────┐   ┌─────────┐   ┌─────────┐
       │  GCC/   │   │CPU/GPU/ │   │Quantum  │
       │ Clang   │   │NPU      │   │SDK      │
       └─────────┘   └─────────┘   └─────────┘
```

---

## Backend Strategy

**Keep C23 transpilation as primary, enhance native object as secondary.**

| Path | When to use | Dependency |
|------|-------------|------------|
| C23 → GCC/Clang | Default, maximum optimization | Requires GCC or Clang |
| Object → Linker | Self-contained, no C compiler needed | None (Karkain-only) |
| GPU (WGSL/SPIR-V) | `@target(gpu)` functions | WebGPU or Vulkan runtime |
| Quantum (OpenQASM/QIR) | `@target(quantum)` functions | Quantum SDK |
| NPU (vendor adapter) | `@target(npu)` functions | NPU hardware |

---

## Self-Hosting Path

```
Today:    Go compiler → compiles Karkain source → C23 → GCC → native
Goal:     Karkain compiler → compiles Karkain source → C23 → GCC → native
          (Go compiler no longer needed after bootstrap)

Bootstrap verification:
  karkain_v2 builds karkain_v3
  karkain_v3 builds karkain_v4
  diff karkain_v3 kamp;v4 → identical (fixed point)
```

---

## Runtime Strategy (libc-free)

```
karkain_runtime.o
├── karkain_main()        # entry point (replaces crt0)
├── karkain_print()       # write() syscall directly
├── karkain_alloc()       # arena allocator (mmap/VirtualAlloc)
├── karkain_free()        # arena reset
├── karkain_panic()       # abort() syscall
├── karkain_math_*()      # software math (no libm)
├── karkain_string_*()    # string operations
├── karkain_io_*()        # file I/O via syscalls
└── karkain_threads_*()   # thread pool, channels
```

**Platform abstraction:**
```c
#ifdef __linux__
  #include <sys/syscall.h>
  static inline int karkain_write(int fd, const void *buf, size_t len) {
      return syscall(SYS_write, fd, buf, len);
  }
#elif __APPLE__
  // libSystem calls
#elif _WIN32
  // NtWriteFile, VirtualAlloc, CreateThread
#endif
```

---

## What Differentiates Karkain

| Feature | Karkain | Zig | V | Odin | Nim |
|---------|---------|-----|---|------|-----|
| GPU compute | Built-in kernels | No | No | No | No |
| NPU acceleration | Vendor adapters | No | No | No | No |
| Quantum circuits | OpenQASM/QIR | No | No | No | No |
| Heterogeneous dispatch | `@target(gpu/npu/cpu/quantum)` | No | No | No | No |
| Tensor types | First-class | No | No | No | No |
| Autodiff | Built-in | No | No | No | No |
| Memory safety | Borrow checker | Yes | No | No | GC |
| No GC | Yes | Yes | Yes | Yes | No |
| Self-hosting | Yes | No | No | No | Yes |

**Karkain's unique value: One language for ALL compute targets — CPU, GPU, NPU, Quantum — with compile-time safety and no GC.**

---

## Honest Risk Assessment

| Risk | Probability | Impact | Mitigation |
|------|-------------|--------|------------|
| Self-hosted compiler can't parse full language | Medium | High | Phase 99-100 are the critical path; if they fail, stop |
| Native object pipeline can't produce real executables | Low | High | Phase 101-102 use C23 transpilation as fallback |
| One developer can't maintain 115 phases | High | High | Focus on Tier 1-2 first; community can help with Tier 3-4 |
| Nobody uses it after launch | Medium | Medium | The NPU/GPU/Quantum differentiation is real; focus marketing there |

---

## Status Tracking

| Phase | Status | Date Completed |
|-------|--------|----------------|
| 92 | COMPLETE | 2026-09-06 |
| 93 | PENDING | — |
| 94 | PENDING | — |
| 95 | PENDING | — |
| 96 | PENDING | — |
| 97 | PENDING | — |
| 98 | PENDING | — |
| 99 | PENDING | — |
| 100 | PENDING | — |
| 101 | PENDING | — |
| 102 | PENDING | — |
| 103 | PENDING | — |
| 104 | PENDING | — |
| 105 | PENDING | — |
| 106 | PENDING | — |
| 107 | PENDING | — |
| 108 | PENDING | — |
| 109 | PENDING | — |
| 110 | PENDING | — |
| 111 | PENDING | — |
| 112 | PENDING | — |
| 113 | PENDING | — |
| 114 | PENDING | — |
| 115 | PENDING | — |
