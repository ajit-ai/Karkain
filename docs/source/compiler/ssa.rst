Typed SSA IR
============

**Package:** ``pkg/ir/ssa`` (Phases 93–94)

The SSA IR is a register-based, single-assignment intermediate
representation used by the Go front end. It underpins the optimizer
pipeline and is verified for well-formedness before code generation.

Core register types
-------------------

The base SSA ``Type`` is a ``uint8`` with seven values:

.. list-table::
   :header-rows: 1
   :widths: 12 40

   * - Type
     - Meaning
   * - ``Void``
     - No value
   * - ``I``
     - Integer (i64 semantics)
   * - ``F``
     - Float (float64)
   * - ``Bool``
     - Boolean
   * - ``Str``
     - String
   * - ``Value``
     - Dynamic boxed Karkain value
   * - ``Ptr``
     - Raw pointer (``alloc``/``free``, references ``&T``)

Extended typed types
--------------------

``typed.go`` adds precise bit-width types on top of the legacy enum:
``IType`` (8/16/32/64 bits), ``FType`` (16/32/64 bits), ``PtrType``
(typed element), ``ArrayType`` (fixed length), ``TensorType`` (hardware
tensor for NPU/GPU), and ``FuncType`` (indirect calls / closures).

TypeRegistry
~~~~~~~~~~~~

``TypeRegistry`` maps between the legacy enum and the typed system,
resolving 14 primitive type names:

- ``void`` → ``Void``
- ``i8`` / ``i16`` / ``i32`` / ``i64`` → ``I``
- ``f16`` / ``f32`` / ``f64`` → ``F``
- ``bool`` → ``Bool``
- ``str`` / ``string`` → ``Str``
- ``ptr`` → ``Ptr``
- everything else → ``Value``

Type helpers
~~~~~~~~~~~~

- ``TypeBits(t)`` — bit width of a register type (I=64, F=64, Bool=1,
  Ptr=64, Str=0, compound=0).
- ``TypeWidth(t)`` — ``(bits + 7) / 8`` bytes.
- ``TypeAlign(t)`` — min(width, 8) bytes, default 1.
- ``TypeIsInteger`` / ``TypeIsFloat`` / ``TypeIsNumeric`` /
  ``TypeIsPointer`` — type predicates.
- ``TypeIsCompatible(a, b)`` — same type, integer↔integer, float↔float,
  or any interaction with ``Value`` (dynamic boxing).
- ``TypePromote(a, b)`` — numeric promotion following standard rules:
  same → same; either ``Value`` → ``Value``; either float → ``F``;
  both integer → ``I``; otherwise ``Value``.

Instructions
------------

An ``Instr`` has an opcode, optional destination register, result type,
operands, and (for branches) targets and block args:

- ``OpConst`` — materialize a constant (i64, f64, bool, str).
- ``OpBinOp`` — ``%d = binop %a OP %b`` over the validated operator
  set (``+ - * / % == != < > <= >= && ||``).
- ``OpUnOp`` — ``%d = unop OP %a`` (``-``, ``!``).
- ``OpCall`` / ``OpCallVoid`` — direct calls, typed results or void.
- ``OpBr`` — conditional branch with block arguments.
- ``OpJmp`` — unconditional jump with block arguments.
- ``OpRet`` — return (value or void).
- ``OpIndexGet`` / ``OpIndexSet`` — array indexing.
- ``OpPrint`` — I/O print.
- ``OpLoad`` / ``OpStore`` — variable cell read/write.
- ``OpRawC`` — escape hatch: opaque pre-rendered C expression or
  statement.

Blocks use named ``BlockParam`` entries (replacing phi nodes). A valid
block must end in exactly one terminator (``br``/``jmp``/``ret``).

Dominance tree
--------------

``dom.go`` implements the iterative immediate-dominator algorithm of
Cooper, Harvey, and Kennedy ("A Simple, Fast Dominance Algorithm").

- ``BuildDominatorTree(fn) *DomTree`` — ``IDom`` maps each block to its
  immediate dominator; the entry block dominates itself.
- ``DomChildren`` — the inverse mapping for tree traversal.
- ``Dominates(a, b)`` / ``StrictlyDominates(a, b)`` — ancestry checks
  over the IDom chain.
- ``DominanceFrontier(fn, block)`` — blocks where the block's dominance
  ends (used by the optimizer).
- ``DomTreeDepth(name)`` — tree depth of a block.

Liveness analysis
-----------------

``liveness.go`` computes per-block live in/out sets via backward
iterative dataflow.

- ``LiveIn`` / ``LiveOut`` / ``LiveGen`` / ``LiveKill`` maps per block.
- ``Ranges`` — per-register ``LiveRange`` (def site, last use site,
  defining block).
- ``TotalRegs`` — distinct register count in the function.
- ``NumLiveAt(block, index)`` — register pressure at a program point.
- ``Interferences(reg)`` — registers whose live ranges overlap.
- ``IsDead(reg)`` — defined but never used.

SSA verifier
------------

``verify.go`` checks module well-formedness (originally Phase 53):

1. Every function's entry block is first in ``Blocks``.
2. Every block ends in exactly one terminator.
3. No instructions after the terminator.
4. Branch targets exist and receive the correct number of block args.
5. Register single-assignment — each ``Dest`` is written at most once
   per function (SSA discipline).
6. ``BinOp`` operators are known; ``rawc`` has a non-empty payload;
   calls have a target.

Optimization pipeline
---------------------

``pipeline.go`` chains passes in a fixed order:

.. code-block:: text

   mem2reg → fold-const → cse → dce → licm → sroa

The orchestrator iterates to a fixpoint with a maximum of 10
iterations; each pass reports whether it changed the function.
``RunModule`` applies the pipeline to every function. ``RunWithStats``
records per-pass ``PassStats``.

Passes
~~~~~~

.. list-table::
   :header-rows: 1
   :widths: 20 70

   * - Pass
     - Details
   * - ``Mem2RegPass``
     - Promotes ``OpLoad``/``OpStore`` pairs to SSA registers when a
       cell has exactly one store that dominates all its loads.
       Rewrites subsequent uses and removes the load/store.
   * - ``FoldConstPass``
     - Compile-time evaluation of ``binop const OP const → const`` and
       ``unop OP const → const``, plus cross-instruction constant
       propagation. Algebraic simplification: ``x+0``, ``x*1``,
       ``x*0 → 0``, ``x-0``, ``x/1``, ``x&&true``, ``x&&false → false``,
       ``x||false``, ``x||true → true``.
   * - ``CSEPass``
     - Block-local common subexpression elimination over pure ops
       (const, binop, unop); later uses rewritten to the earlier
       result.
   * - ``DCESimplePass``
     - Dead code elimination. Unused-result instructions are removed;
       side-effecting ops (call, callvoid, index_set, print, rawc,
       terminators) are preserved. Iterates to fixpoint for dead-use
       chains.
   * - ``LICMPass``
     - Loop-invariant code motion over natural loops found via
       back-edges (branch target dominates branch source). Pure,
       loop-invariant computations are hoisted into the loop header.
   * - ``SAROPass``
     - Scalar replacement of aggregates (simplified). Recognizes
       constant-index ``index_get``; full array-construction tracking
       is a placeholder.

Phase history
-------------

- **Phase 93** — Typed SSA IR: TypeRegistry, TypeBits, TypePromote,
  TypeIsCompatible, dominance tree, liveness, SSA verifier (30 tests).
- **Phase 94** — Optimization pipeline: Mem2Reg, FoldConst, CSE, DCE,
  LICM, fixpoint orchestration, statistics (49 tests including
  benchmarks).