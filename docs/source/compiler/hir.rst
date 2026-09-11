High-Level IR (HIR)
===================

**Package:** ``pkg/ir/hir`` (Phase 92)

The High-Level IR is a typed, desugared representation of Karkain
programs. It sits between the parser AST and the SSA IR, and serves the
backend codegen. HIR resolves types, carries generics, and separates
host vs. device code.

Purpose
-------

The HIR exists to bridge the parser AST and the SSA optimizer:

- The AST is production-ready but verbose and untyped.
- The SSA IR is low-level and register-based.
- The HIR provides a typed, high-level statement/expression tree that
  is cheap to build from the AST and cheap to lower into SSA.

Expressions
-----------

The ``Expr`` structure carries 55 expression kinds (``ExprKind``). The
original Phase 92 design documented 20 core kinds; the enum has since
grown along with the language surface:

.. list-table::
   :header-rows: 1
   :widths: 40 50

   * - Kind
     - Notes
   * - ``ExprInt``, ``ExprFloat``, ``ExprString``, ``ExprBool``
     - Literals (``IntVal``, ``FloatVal``, ``StringVal``, ``BoolVal``)
   * - ``ExprIdent``
     - Identifier reference (``IdentName``)
   * - ``ExprBinary``
     - Binary operation (``BinOp``, ``BinLeft``, ``BinRight``)
   * - ``ExprUnary``
     - Unary operation (``UnOp``, ``UnOperand``)
   * - ``ExprCall``
     - Function call (``CallFunc``, ``CallArgs``, ``CallIsC`` for
       C-interop)
   * - ``ExprIndex``
     - ``IndexTarget``, ``IndexKey``
   * - ``ExprSlice``
     - ``SliceTarget``, ``SliceStart``, ``SliceEnd``
   * - ``ExprArrayLit``
     - ``ArrayElems``
   * - ``ExprMapLit``
     - ``MapKeys``, ``MapValues``
   * - ``ExprStructLit``
     - ``StructTypeName``, ``StructFields``
   * - ``ExprDot``
     - Member access (``DotLeft``, ``DotRight``)
   * - ``ExprLambda``
     - ``LambdaParams``, ``LambdaBody``, ``LambdaCaptures``
   * - ``ExprFuncRef``
     - ``FuncRefName``
   * - ``ExprAddressOf``, ``ExprDereference``, ``ExprBorrow``, ``ExprMove``, ``ExprAlloc``, ``ExprFree``, ``ExprRawAccess``
     - Memory operations
   * - ``ExprOptionSome``, ``ExprOptionNone``, ``ExprResultOk``, ``ExprResultErr``, ``ExprPropagate``
     - Option/Result handling
   * - ``ExprMatch``
     - Match expression
   * - ``ExprEnumVariant``
     - ``EnumName``, ``Variant``, ``VariantVal``
   * - ``ExprSIMDBuiltin``
     - ``SIMDOp``, ``SIMDArgs``
   * - ``ExprAtomicOp``
     - ``AtomicOp``, ``AtomicArgs``, ``AtomicOrder``
   * - ``ExprGateApply``, ``ExprMeasure``, ``ExprQPUOp``, ``ExprCircuitReturn``, ``ExprQubitIndex``
     - Quantum operations
   * - ``ExprSpawn``, ``ExprChannelDecl``, ``ExprSend``, ``ExprChSend``, ``ExprChRecv``, ``ExprAwait``, ``ExprAwaitAll``, ``ExprAsync``, ``ExprGreenSpawn``, ``ExprYield``
     - Concurrency / green threading
   * - ``ExprGlobalId``
     - GPU global id query
   * - ``ExprTensorOp``, ``ExprTensorShapeOf``, ``ExprTensorIndex``
     - Tensor operations
   * - ``ExprReflectType``, ``ExprDerive``, ``ExprTag``, ``ExprComptime``
     - Reflection, derive macros, attributes, compile-time evaluation

Statements
----------

16 statement kinds (``StmtKind``):

.. list-table::
   :header-rows: 1
   :widths: 30 60

   * - Kind
     - Notes
   * - ``StmtReturn``
     - ``ReturnVal``
   * - ``StmtAssign``
     - ``AssignTarget``, ``AssignValue``
   * - ``StmtExpr``
     - ``ExprStmt``
   * - ``StmtIf``
     - ``IfCond``, ``IfThen``, ``IfElse``
   * - ``StmtWhile``
     - ``WhileCond``, ``WhileBody``
   * - ``StmtFor``
     - ``ForInit``, ``ForCond``, ``ForPost``, ``ForBody``
   * - ``StmtForIn``
     - ``ForInVar``, ``ForInKeyName``, ``ForInIter``, ``ForInBody``
   * - ``StmtBreak``
     - ``StmtContinue``
   * - ``StmtPrint``
     - ``PrintVal``
   * - ``StmtVarDecl``
     - ``VarName``, ``VarType``, ``VarValue``
   * - ``StmtBlock``
     - ``BlockStmts``
   * - ``StmtMatch``
     - ``MatchValue``, ``MatchArms`` (patterns: literal / binding /
       wildcard / variant)
   * - ``StmtDefer``
     - Deferred execution
   * - ``StmtReceive``
     - ``RecvChannel``, ``RecvVarName``
   * - ``StmtQubitAssign``
     - ``QubitName``, ``QubitSize``, ``QubitInit``

Types
-----

19 type kinds (``TypeKind``):

.. list-table::
   :header-rows: 1
   :widths: 30 60

   * - Kind
     - Notes
   * - ``TypeInt``
     - Optional ``Bits`` (0 = default i64)
   * - ``TypeFloat``
     - Optional ``Bits`` (0 = default f64)
   * - ``TypeBool``
     - ``bool``
   * - ``TypeString``
     - ``string``
   * - ``TypeArray``
     - ``Elem``
   * - ``TypeMap``
     - ``KeyType``, ``ValType``
   * - ``TypePtr``
     - ``Elem``
   * - ``TypeVoid``
     - ``void``
   * - ``TypeTensor``
     - ``Elem`` + ``Shape []int``
   * - ``TypeKernel``
     - ``kernel``
   * - ``TypeCircuit``
     - ``Qubits``
   * - ``TypeQubit``
     - ``Qubits``
   * - ``TypeBit``
     - ``bit``
   * - ``TypeStruct``
     - ``Name`` + ``Fields``
   * - ``TypeEnum``
     - ``Name`` + ``Variants``
   * - ``TypeLambda``
     - ``fn``
   * - ``TypeOption``
     - ``Elem``
   * - ``TypeResult``
     - ``Elem``
   * - ``TypeUnknown``
     - Placeholder until type resolution

Each ``Type`` carries ``Kind``, optional ``Elem``/``KeyType``/
``ValType``, ``Fields`` (structs), ``Variants`` (enums), ``Name``,
``Bits``, ``Shape``, and ``Qubits``.

Module structure
----------------

``BuildHIR()`` (``pkg/ir/hir/build.go``) lowers a parser ``Program``
into a ``Module``:

.. code-block:: go

   type Module struct {
       Functions []*Function
       Structs   []*StructDecl
       Enums     []*EnumDecl
       Traits    []*TraitDecl
       Impls     []*ImplDecl
       Globals   []*GlobalVar
       Imports   []string
   }

A ``Function`` carries params, return type, generics, captures, body
statements, and the source line for diagnostics. Structure and enum
declarations preserve ``public`` visibility and generic parameters.

Debug output
------------

HIR nodes expose ``Type.String()`` and the module supports
``Format()``-style output for debugging. Type rendering matches the
spec language: ``i64``/``f64`` for primitive ints and floats, ``tensor
<T, shape>`` for tensors, ``circuit<N>``/``qubit<N>`` for quantum
types, and ``Option<T>``/``Result<T>`` for option and result types.