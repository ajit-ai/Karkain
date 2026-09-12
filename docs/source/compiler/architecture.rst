Overall Architecture
====================

The Karkain compilation pipeline transforms source text into machine code
through a series of well-defined stages.

Pipeline diagram
----------------

::

   Source (.kark)
        │
        ▼
   Lexer (pkg/lexer)
        │  tokens
        ▼
   Parser (pkg/parser)
        │  AST
        ▼
   Macro Expander
        │
        ▼
   Borrow Checker (Phase 51)
        │
        ▼
   Type Checker (pkg/sema)
        │  typed AST
        ▼
   HIR (pkg/ir/hir)
        │
        ▼
   Typed SSA IR (pkg/ir/ssa)
        │
        ▼
   Optimizer (Mem2Reg → FoldConst → CSE → DCE → LICM)
        │
        ▼
   SSA Verifier
        │
        ├──► C Emitter (pkg/codegen) ──► GCC/Clang ──► Native Binary
        │
        ├──► WASM Backend (pkg/wasm) ──► wasmtime
        │
        ├──► Native Builder (Object → Linker → Executable)
        │
        ├──► GPU Backend (planned: WGSL / SPIR-V / OpenCL)
        │
        └──► Quantum Backend (planned: OpenQASM 3.0 / QIR)

Two engines
-----------

The pipeline is realized by two engines that produce byte-identical output
for the supported language subset.

Go front end
~~~~~~~~~~~~

The Go front end is the primary engine. It owns the full feature set:
SSA optimization, DWARF debug information, SIMD vector types, the
concurrency runtime, profiling instrumentation, cross-compilation, and
every advanced backend.

.. list-table::
   :header-rows: 1
   :widths: 25 50

   * - Package
     - Responsibility
   * - ``pkg/lexer``
     - Tokenizer: keywords, literals, block-comment nesting (Go-only)
   * - ``pkg/parser``
     - Recursive-descent parser, AST with 20+ node types, macro expansion
   * - ``pkg/sema``
     - Resolver with scope tracking, borrow checker (Phase 51), const
       tracking (Phase 112)
   * - ``pkg/codegen``
     - C23 emitter with embedded runtime preamble, deterministic
       ``karkain_user_*`` namespace, native builder, DWARF, profiling
   * - ``pkg/ir/hir``
     - High-Level IR: typed intermediate representation between AST and SSA
   * - ``pkg/ir/ssa``
     - Typed SSA IR, dominance tree, liveness analysis, optimization
       pipeline, verifier

Self-hosted kcc
~~~~~~~~~~~~~~~

The self-hosted compiler (``src/compiler/*.kark``) implements the same
pipeline — lex, parse, type check, C code generation — in Karkain
itself. It is assembled into C23 and compiled with GCC. Since Phase 97
kcc is the **default engine** for ``karkain check``/``build``/``run``/
``test``. The Go front end remains the fallback for the LSP, test runner,
and features not yet ported.

Package layout
--------------

.. list-table::
   :header-rows: 1
   :widths: 28 62

   * - Package
     - Description
   * - ``pkg/lexer``
     - Tokenizer
   * - ``pkg/parser``
     - Recursive-descent parser and AST definitions
   * - ``pkg/sema``
     - Semantic analysis: resolver, borrow checker, type checker
   * - ``pkg/codegen``
     - C23 code generation, native builder, linker, DWARF, profiling
   * - ``pkg/ir/hir``
     - High-Level IR (typed nodes, 20 expression kinds, 16 statement
       kinds, 19 type kinds)
   * - ``pkg/ir/ssa``
     - Typed SSA IR, optimization pipeline, dominance tree, liveness
   * - ``pkg/compiler``
     - Incremental compilation cache (content-addressed, whole-assembly)
   * - ``pkg/backend/cpu``
     - CPU reference backend (correctness oracle)
   * - ``pkg/backend/gpu``
     - GPU backend (WGSL emission)
   * - ``pkg/npu/``
     - NPU abstraction + 5 vendor adapters (Intel, Qualcomm, Apple, AMD,
       Arm)
   * - ``pkg/wasm``
     - Dependency-free WASM binary v1 emitter with WASI runtime
   * - ``pkg/runtime/``
     - Runtime compilation gates for freestanding, core, and concurrency
       layers
   * - ``pkg/cli``
     - CLI commands, engine routing, diagnostics rendering
   * - ``pkg/pm``
     - Package manager: resolver, lockfile, fetch cache
   * - ``pkg/source``
     - Line-index / excerpt utilities for diagnostics
   * - ``pkg/module``
     - Module graph and import resolution
   * - ``pkg/diagnostics``
     - Structured diagnostic types
   * - ``pkg/lsp``
     - Language Server Protocol implementation
   * - ``pkg/math``
     - Math IR (Phase 71)
   * - ``pkg/tensor``
     - Tensor IR (Phase 72)
   * - ``pkg/jit``
     - JIT compilation infrastructure
   * - ``pkg/testing``
     - Test runner framework (KTF-001)
   * - ``pkg/target``
     - Cross-compilation target model (arch, OS, canonical triples)
   * - ``pkg/bootstrap``
     - Three-stage self-hosting bootstrap pipeline
   * - ``pkg/codegen/dwarf.go``
     - Karkain-owned DWARF 4 emitter and self-hosted parser

Transitional architecture
-------------------------

Several subsystems exist in a transitional state where only the Go front
end implements the feature and kcc parity is a documented future work
item:

- **SSA optimization** — only the Go front end lowers to SSA and runs the
  optimizer pipeline; kcc generates C directly from AST.
- **DWARF debug information** — emitted by the Go codegen; kcc does not
  produce debug sections.
- **Concurrency runtime** — embedded by the Go codegen when
  ``usesConcurrency`` is true; kcc supports the surface syntax (lexed,
  parsed) but the C helper emission is a documented post-107 boundary.
- **SIMD vector types** — the Go codegen emits typed lane vectors and
  runtime helpers; kcc preserves the syntax as inert code.
- **Cross-compilation** (Phase 111) — the Go codegen handles
  ``--target <triple>`` with cross-gcc toolchain selection; kcc does not
  implement target switching.
- **Profiling / trace hooks** — emitted by the Go codegen; kcc does not
  instrument generated code.
- **Native linker** — the Go codegen's ``NativeBuilder`` produces object
  files and linked executables; kcc always shells out to GCC for linking.

These boundaries are intentional: the Go engine serves as the reference
implementation, and kcc parity is verified incrementally through
conformance and parity test gates.
