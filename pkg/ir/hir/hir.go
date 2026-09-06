package hir

import "fmt"

// HIR is the High-Level IR — a typed, desugared representation of Karkain programs.
// It sits between the parser AST and the SSA IR / backend codegen.
// HIR resolves all types, monomorphizes generics, and separates host vs device code.

// ─── Top-level Module ────────────────────────────────────────────

type Module struct {
	Functions []*Function
	Structs   []*StructDecl
	Enums     []*EnumDecl
	Traits    []*TraitDecl
	Impls     []*ImplDecl
	Globals   []*GlobalVar
	Imports   []string
}

// ─── Types ───────────────────────────────────────────────────────

type TypeKind uint8

const (
	TypeInt TypeKind = iota
	TypeFloat
	TypeBool
	TypeString
	TypeArray
	TypeMap
	TypePtr
	TypeVoid
	TypeTensor
	TypeKernel
	TypeCircuit
	TypeQubit
	TypeBit
	TypeStruct
	TypeEnum
	TypeLambda
	TypeOption
	TypeResult
	TypeUnknown
)

type Type struct {
	Kind     TypeKind
	Elem     *Type    // for Array, Ptr, Option, Result
	KeyType  *Type    // for Map
	ValType  *Type    // for Map
	Fields   []*Field // for Struct
	Variants []*Variant // for Enum
	Name     string   // for named types (struct/enum)
	Bits     int      // for Int/Float (0 = default: i64/f64)
	Shape    []int    // for Tensor
	Qubits   int      // for Qubit, Circuit
}

func (t *Type) String() string {
	if t == nil {
		return "void"
	}
	switch t.Kind {
	case TypeInt:
		if t.Bits > 0 {
			return fmt.Sprintf("i%d", t.Bits)
		}
		return "int"
	case TypeFloat:
		if t.Bits > 0 {
			return fmt.Sprintf("f%d", t.Bits)
		}
		return "float64"
	case TypeBool:
		return "bool"
	case TypeString:
		return "string"
	case TypeArray:
		return fmt.Sprintf("%s[]", t.Elem)
	case TypeMap:
		return fmt.Sprintf("map[%s]%s", t.KeyType, t.ValType)
	case TypePtr:
		return fmt.Sprintf("*%s", t.Elem)
	case TypeVoid:
		return "void"
	case TypeTensor:
		return fmt.Sprintf("tensor<%s, %v>", t.Elem, t.Shape)
	case TypeKernel:
		return "kernel"
	case TypeCircuit:
		return fmt.Sprintf("circuit<%d>", t.Qubits)
	case TypeQubit:
		return fmt.Sprintf("qubit<%d>", t.Qubits)
	case TypeBit:
		return fmt.Sprintf("bit<%d>", t.Qubits)
	case TypeStruct:
		return t.Name
	case TypeEnum:
		return t.Name
	case TypeLambda:
		return "fn"
	case TypeOption:
		return fmt.Sprintf("Option<%s>", t.Elem)
	case TypeResult:
		return fmt.Sprintf("Result<%s>", t.Elem)
	case TypeUnknown:
		return "?"
	default:
		return "unknown"
	}
}

type Field struct {
	Name string
	Type *Type
}

type Variant struct {
	Name    string
	Payload *Type // nil = no payload
}

// ─── Function ────────────────────────────────────────────────────

type Function struct {
	Name       string
	Params     []*Param
	ReturnType *Type
	Body       []Stmt
	Public     bool
	Generic    []*GenericParam
	Captures   []string
	Line       int
}

type Param struct {
	Name string
	Type *Type
}

type GenericParam struct {
	Name        string
	Constraints []string
}

// ─── Statements ──────────────────────────────────────────────────

type StmtKind uint8

const (
	StmtReturn StmtKind = iota
	StmtAssign
	StmtExpr
	StmtIf
	StmtWhile
	StmtFor
	StmtForIn
	StmtBreak
	StmtContinue
	StmtPrint
	StmtVarDecl
	StmtBlock
	StmtMatch
	StmtDefer
	StmtReceive
	StmtQubitAssign
)

type Stmt struct {
	Kind StmtKind
	Line int

	// Return
	ReturnVal *Expr

	// Assign
	AssignTarget *Expr
	AssignValue  *Expr

	// Expr statement
	ExprStmt *Expr

	// If
	IfCond *Expr
	IfThen []Stmt
	IfElse []Stmt

	// While
	WhileCond *Expr
	WhileBody []Stmt

	// For
	ForInit *Stmt
	ForCond *Expr
	ForPost *Stmt
	ForBody []Stmt

	// ForIn
	ForInVar     string
	ForInKeyName string
	ForInIter    *Expr
	ForInBody    []Stmt

	// Print
	PrintVal *Expr

	// VarDecl
	VarName     string
	VarType     *Type
	VarValue    *Expr
	VarIsMutable bool

	// Block
	BlockStmts []Stmt

	// Match
	MatchValue *Expr
	MatchArms  []*MatchArm

	// Receive
	RecvChannel *Expr
	RecvVarName string

	// QubitAssign
	QubitName string
	QubitSize int
	QubitInit bool
}

type MatchArm struct {
	Pattern  *MatchPattern
	Body     []Stmt
}

type MatchPattern struct {
	Kind    string // "literal", "binding", "wildcard", "variant"
	Value   *Expr  // for literal patterns
	Binding string // for binding patterns
	Variant string // for enum variant patterns
}

// ─── Expressions ─────────────────────────────────────────────────

type ExprKind uint8

const (
	ExprInt ExprKind = iota
	ExprFloat
	ExprString
	ExprBool
	ExprIdent
	ExprBinary
	ExprUnary
	ExprCall
	ExprIndex
	ExprSlice
	ExprArrayLit
	ExprMapLit
	ExprStructLit
	ExprDot
	ExprLambda
	ExprFuncRef
	ExprAddressOf
	ExprDereference
	ExprAlloc
	ExprFree
	ExprBorrow
	ExprMove
	ExprRawAccess
	ExprOptionSome
	ExprOptionNone
	ExprResultOk
	ExprResultErr
	ExprMatch
	ExprPropagate
	ExprSIMDBuiltin
	ExprAtomicOp
	ExprGateApply
	ExprMeasure
	ExprSpawn
	ExprSend
	ExprChannelDecl
	ExprGlobalId
	ExprTensorOp
	ExprTensorShapeOf
	ExprTensorIndex
	ExprQPUOp
	ExprCircuitReturn
	ExprQubitIndex
	ExprAwait
	ExprYield
	ExprChSend
	ExprChRecv
	ExprAsync
	ExprAwaitAll
	ExprGreenSpawn
	ExprReflectType
	ExprDerive
	ExprTag
	ExprComptime
	ExprEnumVariant
)

type Expr struct {
	Kind ExprKind
	Type *Type // resolved type (nil until type-checking)
	Line int

	// Literals
	IntVal    string
	FloatVal  string
	StringVal string
	BoolVal   bool

	// Identifier
	IdentName string

	// Binary
	BinOp    string
	BinLeft  *Expr
	BinRight *Expr

	// Unary
	UnOp      string
	UnOperand *Expr

	// Call
	CallFunc string
	CallArgs []*Expr
	CallIsC  bool

	// Index
	IndexTarget *Expr
	IndexKey    *Expr

	// Slice
	SliceTarget *Expr
	SliceStart  *Expr
	SliceEnd    *Expr

	// Array literal
	ArrayElems []*Expr

	// Map literal
	MapKeys   []*Expr
	MapValues []*Expr

	// Struct literal
	StructTypeName string
	StructFields   []*Expr

	// Dot
	DotLeft  *Expr
	DotRight string

	// Lambda
	LambdaParams   []*Param
	LambdaBody     []Stmt
	LambdaCaptures []string

	// FuncRef
	FuncRefName string

	// Memory
	MemOperand *Expr
	MemType    *Type
	MemCount   *Expr
	MemMutable bool

	// Option/Result
	OptVal *Expr
	ResVal *Expr
	ResErr *Expr

	// Match
	MatchVal  *Expr
	MatchArms []*MatchArm

	// Propagate
	PropagateOperand *Expr

	// SIMD
	SIMDOp   string
	SIMDArgs []*Expr

	// Atomic
	AtomicOp    string
	AtomicArgs  []*Expr
	AtomicOrder string

	// Quantum
	QGate    string
	QTarget  *Expr
	QControl *Expr
	QParams  []*Expr
	QMeasure *Expr
	QAngle   *Expr

	// GPU
	GPUDim int

	// Actors
	SpawnActor string
	SpawnArgs  []*Expr
	SendChan   *Expr
	SendMsg    *Expr
	SendSync   bool
	ChElemType *Type
	ChBufSize  *Expr

	// Tensor
	TensorOp    string
	TensorArgs  []*Expr
	TensorAttrs map[string]*Expr
	TensorSrc   *Expr
	TensorIdx   []*Expr

	// QPU
	QPUOp   string
	QPUArgs []*Expr

	// Enum variant
	EnumName   string
	Variant    string
	VariantVal *Expr

	// Reflect/Derive/Tag
	ReflectType  *Expr
	DeriveTrait  string
	DeriveTarget *Expr
	DeriveArgs   []*Expr
	TagTarget    *Expr
	TagName      string
	TagValue     string
	ComptimeExpr *Expr

	// Async/Await
	AsyncBody    []Stmt
	AwaitOperand *Expr
	AwaitTimeout *Expr
	YieldVal     *Expr
	AwaitFutures []*Expr
	GreenFunc    string
	GreenArgs    []*Expr
}

// ─── Declarations ────────────────────────────────────────────────

type StructDecl struct {
	Name    string
	Fields  []*Field
	Public  bool
	Generic []*GenericParam
	Line    int
}

type EnumDecl struct {
	Name     string
	Variants []*Variant
	Public   bool
	Line     int
}

type TraitDecl struct {
	Name    string
	Methods []*FuncSignature
	Line    int
}

type FuncSignature struct {
	Name       string
	Params     []*Param
	ReturnType *Type
}

type ImplDecl struct {
	TraitName string
	ForType   string
	Methods   []*Function
	Line      int
}

type GlobalVar struct {
	Name   string
	Type   *Type
	Value  *Expr
	Public bool
	Line   int
}
