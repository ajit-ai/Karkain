Architecture
============

Karkain is a **statically typed systems programming language** with a
self-hosted compiler and a two-engine pipeline: a Go front end (the
reference compiler) and the self-hosted ``kcc`` engine. This page describes
the repository layout, the key packages and how they relate.

.. contents:: Sections
   :local:
   :depth: 1

Repository top-level
====================

.. code-block:: text

   cmd/karkain/main.go        # CLI entry point and command dispatch
   pkg/*                       # 23 Go packages (the compiler and toolchain)
   src/compiler/*.kark         # self-hosted kcc compiler (8 source files)
   runtime/freestanding/       # libc-free arena allocator + platform abstraction
   runtime/core/               # libc-free heap, tagged Value, native strings/arrays
   runtime/concurrency/        # work-stealing scheduler, channels, actors (C runtime)
   stdlib/                     # 11 stdlib modules (6 importable, 5 boundary)
   examples/                   # language_foundation, algorithms, concurrency, wasm, ...
   conformance/                # 11 test files, 59 test functions
   docs/                       # official documentation (Sphinx RST)
   scripts/                    # build and utility scripts

The CLI
=======

``cmd/karkain/main.go`` is the single binary: ``karkain build``, ``run``,
``test``, ``check``, ``fmt``, ``lint``, ``debug``, ``prof``, ``target``,
``pkg``, ``lsp`` and more are all dispatched from one ``main`` function.
The command surface is documented in :doc:`/tools/cli`.

The 23 Go packages
==================

.. list-table:: Key packages
   :widths: 30 70
   :header-rows: 1

   * - Package
     - Role
   * - ``pkg/codegen``
     - C emitter — produces C23 source from Karkain AST/IR, with the
       profiling/trace/SIMD/target-header emitters embedded. The dominant
       codegen package (emits most of the generated C).
   * - ``pkg/cli``
     - All CLI commands, drivers and test harnesses.
   * - ``pkg/ir/ssa``
     - Typed SSA IR with dominance tree, liveness, optimization pipeline
       (Mem2Reg, FoldConst, CSE, DCE, LICM) and verifier.
   * - ``pkg/ir/hir``
     - High-level IR with typed expression/statement/type nodes.
   * - ``pkg/parser``
     - AST model (``ast.go``) and recursive-descent parser.
   * - ``pkg/lexer``
     - Tokenizer with the ``const`` keyword (Phase 112) and nested block
       comments.
   * - ``pkg/sema``
     - Semantic analysis — ``resolve.go`` (exports, const tracking, borrow
       model), ``npu_check.go``.
   * - ``pkg/backend/cpu``
     - Reference CPU backend (correctness oracle).
   * - ``pkg/backend/gpu``
     - GPU/WGSL backend (no language surface yet).
   * - ``pkg/npu/*``
     - NPU vendor adapters (intel/qualcomm/apple/amd/arm) and quantum
       backend abstraction.
   * - ``pkg/wasm``
     - Karkain-owned ``wasm32-wasi`` backend (WASM binary v1 emitter,
       runtime glue).
   * - ``pkg/compiler``
     - Incremental compilation cache (content-addressed, interface-hashed).
   * - ``pkg/target``
     - Target-triple model, feature flags, supported-target matrix
       (Phase 111).
   * - ``pkg/pm``
     - Package manager — resolver, lockfile, local fetch.
   * - ``pkg/source``
     - Source file line-index model (used by CLI and LSP).
   * - ``pkg/lsp``
     - Language Server Protocol implementation.
   * - ``pkg/math``
     - Math IR (internal, no language surface).
   * - ``pkg/tensor``
     - Tensor IR (internal, no language surface).
   * - ``pkg/testing``
     - Test-driver utilities.
   * - ``pkg/jit``
     - JIT engine boundary (no user-facing surface).
   * - ``pkg/bootstrap``
     - Bootstrap pipeline (``KARKAIN_ENGINE=go`` pinned).
   * - ``pkg/diagnostics``
     - Structured diagnostic model.
   * - ``pkg/module``
     - Module graph and ``std.*`` resolution.

The self-hosted compiler
========================

``src/compiler/`` contains 8 ``.kark`` source files:

.. code-block:: text

   main.kark     ast.kark      parser.kark    sema.kark
   checker.kark  codegen.kark  lexer.kark     atypes.kark

The self-hosted compiler is the primary engine for ``check`` / ``build`` /
``run`` / ``test`` (Phase 95–99). It is compiled via bootstrap (Go
codegen → C23 → native executable) and produces byte-identical C output to
the Go front end on the same source files.

``src/compiler/codegen.kark`` mirrors the C emission of
``pkg/codegen/codegen.go``; ``src/compiler/checker.kark`` mirrors
``pkg/sema/resolve.go``. Language surface changes need changes in both
trees.

The transitional architecture
=============================

Karkain is in a transitional state:

* **Go front end** — handles all exotic backends (SIMD, profiling, tracing,
  WASM, native linking), is the reference for runtime behavior, and is the
  only engine that supports concurrency, ``karkain debug`` and ``karkain prof``.
* **Self-hosted kcc** — the default engine for ``check``/``build``/``run``/
  ``test`` on ordinary single-threaded programs; kcc parity for concurrency
  and some advanced surfaces is deferred.

``pkg/codegen`` is the largest package because it handles most of the C
emission. ``pkg/cli`` is the second-largest because it wires every CLI
command and hosts the test harnesses. The self-hosted compiler
(``src/compiler/``) mirrors the Go packages but does not yet cover SIMD,
profiling, tracing, WASM or native-linking codegen.

The runtime boundary
====================

.. code-block:: text

   runtime/freestanding/   # arena allocator, platform abstraction (no libc)
   runtime/core/           # heap, Value, NativeString, NativeArray (no libc)
   runtime/concurrency/    # C scheduler, channels, actors (no libc)

The freestanding and core layers compile with ``-ffreestanding -nostdlib``.
The concurrency runtime is embedded into generated C via
``pkg/codegen/conc_runtime.go``.

.. seealso::

   :doc:`/compiler/architecture` — the compiler internals in depth.
   :doc:`/compiler/kcc` — the self-hosted engine specifically.
   :doc:`/tools/cli` — every CLI command and exit code.