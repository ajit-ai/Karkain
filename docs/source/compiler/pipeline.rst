Pipeline Stages
===============

This page walks through each stage of the Karkain compilation pipeline
in detail.

Source → Lexer
--------------

**Package:** ``pkg/lexer``

The lexer converts raw ``.kark`` source text into a flat sequence of
tokens. It handles:

- Keywords (``func``, ``let``, ``mut``, ``if``, ``else``, ``while``,
  ``for``, ``return``, ``struct``, ``enum``, ``spawn``, ``send``,
  ``channel``, ``actor``, and many more).
- Integer and floating-point literals, string literals with escape
  sequences, boolean literals.
- Operators (arithmetic, comparison, logical, bitwise, ``::``).
- Identifiers.
- Line and column tracking for diagnostics.

Block comment nesting (``/* ... /* ... */ ... */``) is supported in the
Go lexer only. The self-hosted lexer handles a flat ``/* ... */`` subset.

Lexer → Parser
--------------

**Package:** ``pkg/parser``

The parser is a recursive-descent parser that produces an AST with over
20 node types:

- Declarations: ``FuncDecl``, ``StructDecl``, ``EnumDecl``,
  ``TraitDecl``, ``ImplDecl``, ``VarDecl``
- Statements: ``IfStmt``, ``WhileStmt``, ``ForStmt``, ``ForInStmt``,
  ``ReturnStmt``, ``PrintStmt``, ``BreakStmt``, ``ContinueStmt``,
  ``BlockStmt``, ``ExprStmt``, ``MatchExpr``, ``ReceiveStmt``,
  ``QubitAssignStmt``
- Expressions: ``IntLiteral``, ``Float64Literal``, ``StringLiteral``,
  ``BoolLiteral``, ``Identifier``, ``BinaryExpr``, ``UnaryExpr``,
  ``CallExpr``, ``IndexExpr``, ``SliceExpr``, ``ArrayLiteral``,
  ``MapLiteral``, ``StructLiteral``, ``DotExpr``, ``LambdaExpr``,
  ``AddressOf``, ``Dereference``, ``BorrowExpr``, ``MoveExpr``,
  ``OptionSomeExpr``, ``OptionNoneExpr``, ``ResultOkExpr``,
  ``ResultErrExpr``, ``PropagateExpr``, ``EnumVariantExpr``,
  ``SIMDBuiltinExpr``, ``AtomicOp``, ``GateApplyStmt``,
  ``MeasureExpr``, ``SpawnExpr``, ``SendExpr``, ``ChDeclExpr``,
  ``ChSendExpr``, ``ChRecvExpr``, ``AsyncExpr``, ``AwaitExpr``,
  ``AwaitAllExpr``, ``GreenSpawnExpr``, ``TensorOpExpr``,
  ``TensorShapeOfExpr``, ``TensorIndexExpr``, ``QPUOpExpr``,
  ``ReflectTypeExpr``, ``DeriveExpr``, ``TagExpr``,
  ``ComptimeExpr``, ``FuncRefExpr``, ``GlobalIdExpr``

The parser performs macro expansion during AST construction. Generic
type parameters and constraints are captured on function, struct, and
impl declarations.

Parser → Borrow Checker
-----------------------

**Package:** ``pkg/sema`` (Phase 51)

The borrow checker runs on the parsed AST before full type resolution.
It implements lexical scoping with a scope stack and enforces:

- Use-after-scope-end detection.
- Borrow reversion via declaring scope.
- Escape analysis flag wiring.

Borrow checking is a prerequisite for the soundness of ownership
semantics but does not affect code generation for the current language
subset.

Borrow Checker → Type Checker
-----------------------------

**Package:** ``pkg/sema``

The resolver performs semantic analysis:

- **Scope tracking** — two-pass analysis (collect declarations, then
  resolve references).
- **Type inference** — conservative static type inference for primitives
  and structural types.
- **Module resolution** — ``import std.x`` resolves against the module
  graph; ``public`` exports are enforced across module boundaries.
- **Const tracking** (Phase 112) — compile-time constant propagation
  where applicable.
- **NPU target validation** — ``@target(...)`` attributes are checked
  against the known target set (``cpu``, ``npu``); unknown targets are
  rejected with ``error[K004]``.

The resolver produces structured diagnostics in the
``karkain-diagnostics-v1`` contract with file, line, column, and
excerpt fields.

Type Checker → HIR
------------------

**Package:** ``pkg/ir/hir``

The HIR (High-Level IR) is a typed, desugared representation that sits
between the parser AST and the SSA IR. ``BuildHIR()`` lowers a parser
``Program`` into an HIR ``Module``.

**19 type kinds:** ``TypeInt``, ``TypeFloat``, ``TypeBool``,
``TypeString``, ``TypeArray``, ``TypeMap``, ``TypePtr``, ``TypeVoid``,
``TypeTensor``, ``TypeKernel``, ``TypeCircuit``, ``TypeQubit``,
``TypeBit``, ``TypeStruct``, ``TypeEnum``, ``TypeLambda``,
``TypeOption``, ``TypeResult``, ``TypeUnknown``.

**20 expression kinds** (representative, full list in ``pkg/ir/hir/
hir.go``): ``ExprBinary``, ``ExprUnary``, ``ExprCall``, ``ExprIndex``,
``ExprDot``, ``ExprArrayLit``, ``ExprMapLit``, ``ExprStructLit``,
``ExprLambda``, ``ExprAddressOf``, ``ExprBorrow``, ``ExprMove``,
``ExprOptionSome``, ``ExprResultOk``, ``ExprMatch``, ``ExprSpawn``,
``ExprSend``, ``ExprChannelDecl``, ``ExprSIMDBuiltin``,
``ExprTensorOp``.

**16 statement kinds:** ``StmtReturn``, ``StmtAssign``, ``StmtExpr``,
``StmtIf``, ``StmtWhile``, ``StmtFor``, ``StmtForIn``, ``StmtBreak``,
``StmtContinue``, ``StmtPrint``, ``StmtVarDecl``, ``StmtBlock``,
``StmtMatch``, ``StmtDefer``, ``StmtReceive``, ``StmtQubitAssign``.

The HIR carries resolved type information on every expression node
(``Type *Type``) and supports ``Format()`` for debug output. It is the
bridge between the high-level AST and the SSA lowering pass.

HIR → Typed SSA IR
------------------

**Package:** ``pkg/ir/ssa`` (Phase 93)

The SSA IR lowers HIR into a register-based, single-assignment form.

**Core types** (7 SSA register types):

- ``Void``, ``I`` (i64), ``F`` (f64), ``Bool``, ``Str``, ``Value``
  (dynamic boxed), ``Ptr`` (raw pointer).

**TypeRegistry** provides 14 primitive type mappings with bit-width
information, type promotion rules (``TypePromote``), and compatibility
checks (``TypeIsCompatible``).

**Module structure:**

- ``Module`` → ``Function`` → ``Block`` → ``Instr``
- Blocks have named ``BlockParam`` entries (replacing phi nodes).
- Instructions: ``OpConst``, ``OpBinOp``, ``OpUnOp``, ``OpCall``,
  ``OpCallVoid``, ``OpBr``, ``OpJmp``, ``OpRet``, ``OpIndexGet``,
  ``OpIndexSet``, ``OpPrint``, ``OpLoad``, ``OpStore``, ``OpRawC``.
- ``OpRawC`` is an escape hatch for opaque pre-rendered C expressions
  or statements.

SSA → Optimizer
---------------

**Package:** ``pkg/ir/ssa`` (Phase 94)

The optimization pipeline runs passes sequentially to fixpoint (maximum
10 iterations):

.. list-table::
   :header-rows: 1
   :widths: 18 72

   * - Pass
     - Description
   * - ``Mem2Reg``
     - Promotes ``OpLoad``/``OpStore`` pairs for single-store variables
       to SSA registers. Requires the store block to dominate all load
       blocks.
   * - ``FoldConst``
     - Evaluates pure operations on constants at compile time
       (``binop const OP const → const``). Also performs algebraic
       identity simplification: ``x + 0``, ``x * 1``, ``x / 1``,
       ``x && true``, ``x || false``.
   * - ``CSE``
     - Common subexpression elimination within a single basic block.
       Detects identical pure operations (``OpConst``, ``OpBinOp``,
       ``OpUnOp``) and replaces later uses with the earlier result.
   * - ``DCE``
     - Dead code elimination. Removes instructions whose results are
       never used. Side-effecting ops (call, print, index_set, rawc,
       terminators) are always kept. Iterates to fixpoint to handle
       dead-use chains.
   * - ``LICM``
     - Loop-invariant code motion. Identifies natural loops via
       back-edges, then hoists loop-invariant computations out of loop
       bodies into the loop header.
   * - ``SROA``
     - Scalar replacement of aggregates (simplified). Placeholder for
       full constant-index array element resolution.

The pipeline is orchestrated by ``Pipeline.RunModule()`` which applies
all passes per-function and reports total change count.

SSA Verifier
------------

**Package:** ``pkg/ir/ssa`` (Phase 53)

The verifier validates module well-formedness:

1. Every function has an entry block that is first in ``Blocks``.
2. Every block ends in exactly one terminator (``br``/``jmp``/``ret``).
3. No instructions exist after the terminator.
4. Branch targets exist and receive the correct number of block args.
5. Register single-assignment: each ``Dest`` is written at most once
   per function.
6. ``BinOp`` operators are known; ``rawc`` must have a non-empty
   payload.

SSA → C Emission
----------------

**Package:** ``pkg/codegen``

The C emitter translates SSA IR (or, when ``DisableSSA`` is set, the
legacy AST path) into a self-contained C23 translation unit.

**Runtime preamble:** The generated C begins with an embedded runtime
— a large C string literal containing type definitions, the arena
allocator, the tagged ``Value`` type, I/O helpers, and (when the
program uses them) the concurrency scheduler, channel, and actor code.
No external ``.c`` files are needed for standard builds.

**Deterministic namespace:** All user-defined Karkain functions are
emitted into the ``karkain_user_*`` C namespace
(``pkg/codegen/codegen.go:49``). This prevents collisions with C
standard library symbols (``abs``, ``min``, etc.). ``main`` and
``getArgs`` keep their canonical names.

**Source mapping:** Every emitted function carries a ``#line``
directive and a ``karkain-frame-enter`` / ``karkain-set-line`` call
for runtime stack traces and profiling hooks.

**Debug information:** When enabled, the emitter produces DWARF 4
sections (``.debug_info``, ``.debug_abbrev``, ``.debug_str``,
``.debug_line``) via the ``DwarfEmitter`` in ``pkg/codegen/dwarf.go``.

WASM path
~~~~~~~~~

**Package:** ``pkg/wasm``

The WASM backend is a dependency-free, handwritten WASM binary v1
emitter. It embeds a WASI-compatible runtime (``fd_write``,
``args_sizes_get``, ``args_get`` and ``proc_exit`` via
``wasi_snapshot_preview1``), boxed value semantics (``i64``-tagged
integers, heap cells with type tag + length + data), 25 runtime bodies
plus a ``_start`` bootstrap, and a pattern that derives the process exit
code from ``main``'s returned value. It is invoked by ``karkain build
--target wasm32-wasi`` and run under ``wasmtime``. Unsupported language
constructs are rejected deterministically with ``error K108 ...``
(stderr on runtime errors; ``getArgs()`` returns the WASI argument
vector). See :doc:`/status/scope` for the
:production-candidate:`Production Candidate` classification.

Native path
~~~~~~~~~~~

The native build path uses the Go codegen's ``NativeBuilder`` which
produces an ``Object`` with sections, symbols, and relocations; a
``Linker`` resolves symbols, lays out sections, handles entry points,
and produces an ``Executable``. DWARF debug sections are attached after
address relocation. Invoked via ``karkain build`` with the default
native target.
