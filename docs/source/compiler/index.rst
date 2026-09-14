Compiler Internals
==================

The Karkain compiler is implemented as two cooperating engines that share
the same front end but differ in how they produce code.

.. list-table::
   :header-rows: 1
   :widths: 20 40 40

   * - Engine
     - Role
     - Key packages
   * - **Go front end**
     - Primary engine for lexing, parsing, semantic analysis, and C code
       generation. Drives the SSA optimizer, DWARF emission, and every
       advanced backend (WASM, native, SIMD, concurrency).
     - ``pkg/lexer``, ``pkg/parser``, ``pkg/sema``, ``pkg/codegen``,
       ``pkg/ir/ssa``, ``pkg/ir/hir``
   * - **kcc (self-hosted)**
     - A Karkain-to-C23 transpiler written in Karkain itself. The default
       engine for ``karkain check``/``build``/``run``/``test``. Produces
       byte-identical C output on both engines. Also owns the Karkain IR
       (KIR) emitter — the first compiler component written entirely in
       Karkain (Phase 120).
     - ``src/compiler/*.kark``

Both engines transpile to **C23**, which is then compiled to a native binary
by GCC or Clang. The generated C includes an embedded runtime (arena
allocator, tagged Value type, I/O, concurrency scheduler, profiler) — no
external ``.c`` files are needed for standard builds.

SSA optimization
----------------

The Go front end lowers typed HIR into a **typed SSA IR** and runs a
multi-pass optimizer (Mem2Reg, constant folding with algebraic
simplification, CSE, DCE, LICM, SROA) to fixpoint before emitting C.
The SSA pipeline lives in ``pkg/ir/ssa`` and is detailed in
:doc:`ssa`.

Self-hosting bootstrap
----------------------

The self-hosted compiler proves its own correctness through a three-stage
bootstrap:

1. **Stage 1** — The Go front end compiles ``src/compiler/*.kark`` into
   C23, which GCC links into ``karkain-compiler1``.
2. **Stage 2** — ``karkain-compiler1`` re-compiles the same sources to
   produce ``karkain-compiler2``.
3. **Stage 3** — ``karkain-compiler2`` re-compiles again to produce
   ``karkain-compiler3``.

Bitwise identity (``Stage-2 SHA256 == Stage-3 SHA256``) proves the
compiler is a deterministic fixed point of itself. The bootstrap pipeline
is implemented in ``pkg/bootstrap`` and exercised by
``TestBootstrap_BitwiseIdentity``.

.. note::

   On hosts with approximately 4 GB of RAM, Stage 2 may exhaust memory
   during compilation of the full compiler source tree. This is a known
   environmental class (not a defect) documented in the bootstrap gate
   tests.

Contents
--------

.. toctree::
   :maxdepth: 2

   architecture
   pipeline
   kcc
   kir
   hir
   ssa
   runtime
   bootstrap
