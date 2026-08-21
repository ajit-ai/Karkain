package parser

import "fmt"

type Node interface{}

type Program struct {
	Statements []Node
	CImports   []*CImportBlock // C import blocks for codegen
}

type FuncDecl struct {
	Name          string
	Params        []string
	Body          []Node
	GenericParams []GenericTypeParam // Phase 26: generic type parameters
}

type VarDeclStmt struct {
	Name     string
	Value    Node
	Type     string // Optional type information (e.g., "*int")
	IsMatrix bool   // True if this is a matrix declaration
}

type ReturnStmt struct {
	Value Node
}

type ExprStmt struct {
	Expression Node
}

type IfStmt struct {
	Condition   Node
	Consequence []Node
	Alternative []Node
}

type WhileStmt struct {
	Condition Node
	Body      []Node
}

type PrintStmt struct {
	Value Node
}

type StringLiteral struct {
	Value string
}

type IntLiteral struct {
	Value string
}

type Float64Literal struct {
	Value string
}

type BigIntLiteral struct {
	Value string
}

type BigFloatLiteral struct {
	Value string
}

type Identifier struct {
	Name string
}

type ArrayLiteral struct {
	Elements []Node
}

type MapLiteral struct {
	Keys   []Node
	Values []Node
}

type IndexExpr struct {
	Left  Node
	Index Node
}

type BinaryExpr struct {
	Left     Node
	Operator string
	Right    Node
}

type CallExpr struct {
	Function string
	Args     []Node
	IsCFunc  bool // True if this is a C function call (e.g., C.sqrt)
}

// Phase 41: Hybrid Memory Model — safe references + @raw for hardware

// RefType represents a reference type: &T (immutable) or &mut T (mutable)
type RefType struct {
	BaseType string // The referenced type, e.g., "int", "[]float64"
	Mutable  bool   // true for &mut T, false for &T
}

// RawAccessExpr represents hardware memory access: @raw(addr) or @raw(addr, val)
// @raw(addr) reads from memory address, @raw(addr, val) writes to it
type RawAccessExpr struct {
	Address Node // Memory address expression
	Value   Node // Value to write (nil for read-only)
}

// BorrowExpr represents creating a borrow: &x or &mut x
type BorrowExpr struct {
	Operand Node
	Mutable bool // true for &mut x, false for &x
}

// MoveExpr represents an explicit move: move(x)
// Transfers ownership from x to the target
type MoveExpr struct {
	Operand Node
}

// Phase 44: Error propagation, exhaustive match, linear type enforcement

// PropagateExpr represents the ? operator: expr?
// Desugars to: match expr { Ok(v) => v, Err(e) => return Err(e) }
type PropagateExpr struct {
	Operand Node
}

// Phase 42: Option<T>, Result<T,E>, match, SIMD, packed structs, linear types

// OptionSomeExpr represents Some(value) — an option with a value
type OptionSomeExpr struct {
	Value Node
}

// OptionNoneExpr represents None — an empty option
type OptionNoneExpr struct{}

// ResultOkExpr represents Ok(value) — a successful result
type ResultOkExpr struct {
	Value Node
}

// ResultErrExpr represents Err(error) — a failed result
type ResultErrExpr struct {
	Error Node
}

// MatchExpr represents pattern matching:
// match value { pattern => expr, ... }
type MatchExpr struct {
	Value   Node
	Arms    []MatchArm
}

// MatchArm represents one arm of a match expression: pattern => expr
type MatchArm struct {
	Pattern MatchPattern
	Body    Node
}

// MatchPattern represents a pattern in match arms
type MatchPattern struct {
	Type       string // "Some", "None", "Ok", "Err", "literal", "wildcard"
	Value      Node   // For literal patterns (42, "hello", true)
	Binding    string // For variable bindings (e.g., x in Some(x))
}

// SIMDBuiltinExpr represents SIMD intrinsics: @simd_add(a, b), @simd_mul(a, b)
type SIMDBuiltinExpr struct {
	Op   string // "add", "mul", "sub", "div", "min", "max", "sqrt"
	Args []Node
}

// LinearTypeDecl marks a type as linear (must be used exactly once):
// linear type FileHandle { fd: int }
type LinearTypeDecl struct {
	Name   string
	Fields []StructField
}

// PackedStructDecl marks a struct as packed (no padding):
// packed struct Point { x: float32, y: float32 }
type PackedStructDecl struct {
	Name   string
	Fields []StructField
}

// Phase 45: Custom enum types with algebraic data variants

// EnumDecl represents an enum declaration:
// enum Color { Red, Green, Blue }
// enum Result { Ok(value), Err(error) }
type EnumDecl struct {
	Name     string
	Variants []EnumVariant
}

// EnumVariant represents one variant of an enum
type EnumVariant struct {
	Name    string // e.g., "Red", "Ok", "Err"
	Payload string // type of payload, empty for unit variants
}

// EnumVariantExpr represents constructing an enum variant:
// Color.Red, Ok(42), Err("fail")
type EnumVariantExpr struct {
	EnumName string // type name (empty for inferred)
	Variant  string // variant name
	Value    Node   // payload expression (nil for unit variants)
}

// Phase 11: Native C Interop, Raw Pointers, and Continuous Matrix Memory Layouts

type CImportBlock struct {
	Content string // Raw C code between { }
}

type PointerType struct {
	BaseType string // e.g., "int", "float64"
}

type AddressOf struct {
	Operand Node
}

type Dereference struct {
	Operand Node
}

type AllocExpr struct {
	Type  string // e.g., "int", "float64"
	Count Node   // Number of elements
}

type FreeExpr struct {
	Ptr Node
}

type MatrixDecl struct {
	Rows     Node
	Cols     Node
	DataType string // e.g., "float64", "int"
}

type MatrixIndexExpr struct {
	Matrix Node
	Row    Node
	Col    Node
}

type DotExpr struct {
	Left  Node
	Right string // The field/method name
}

// Phase 14: Quantum Computing AST Nodes

type QRegDeclStmt struct {
	Name   string
	Qubits Node // Number of qubits
}

type GateApplyStmt struct {
	Gate    string // "H", "X", "CNOT", etc.
	Target  Node   // Target qubit(s)
	Control Node   // Control qubit (for CNOT), nil for single-qubit gates
	Params  []Node // Additional parameters (rotation angles, etc.)
}

type MeasureExpr struct {
	Qubit Node // Qubit to measure
}

// Phase 16: Actor-based distributed concurrency AST nodes
type ActorDeclStmt struct {
	Name     string
	Params   []string
	Body     []Node
	State    []ActorStateField  // Phase 37: actor state fields
	Handlers []ActorHandler     // Phase 37: message handlers
}

// ActorStateField represents a field in actor state
type ActorStateField struct {
	Name    string
	Type    string
	Default Node
}

// ActorHandler represents a receive handler in an actor
type ActorHandler struct {
	MessageType string   // message type name
	ParamName   string   // parameter binding name
	ParamType   string   // parameter type
	Body        []Node
	IsReply     bool     // whether this handler replies
}

type SpawnExpr struct {
	ActorName string
	Args      []Node
	NodeAddr  string // Phase 37: optional remote node address
}

type ReceiveStmt struct {
	Channel Node
	VarName string
}

type SendExpr struct {
	Channel    Node
	Message    Node
	IsSync     bool // true = !? (request-reply), false = ! (async)
	Timeout    Node // optional timeout for sync send
}

// Phase 17: Metaprogramming AST nodes

type MacroDeclStmt struct {
	Name       string
	Params     []string
	Body       []Node
	IsHygienic bool // For hygiene tracking
}

type MacroExpandExpr struct {
	MacroName string
	Args      []Node
}

type QuoteExpr struct {
	Expr Node // Quoted AST node
}

type UnquoteExpr struct {
	Expr Node // Unquoted expression to be evaluated
}


type ComptimeExpr struct {
	Expr Node // Expression evaluated at compile time
}

type ReflectTypeExpr struct {
	TypeExpr Node // Type to reflect on
}

type DeriveExpr struct {
	Trait  string // e.g., "JsonSerializable"
	Target Node   // Target struct/type
	Args   []Node // Additional arguments
}

type TagExpr struct {
	Target   Node   // Target field/struct
	TagName  string // Tag name
	TagValue string // Tag value
}


// ComptimeStmt represents a compile-time evaluated statement: `comptime var x = ...`
type ComptimeStmt struct {
	Body []Node
}



// Parameter represents a typed parameter by value (safe, no pointers)
type Parameter struct {
	Name string
	Type string
}

// KernelDeclStmt represents a GPU compute kernel function declaration:
// kernel vector_add(a: []float, b: []float, c: []float) { ... }
type KernelDeclStmt struct {
	Name          string
	Params        []Parameter
	Body          []Node
	WorkGroupX    int
	WorkGroupY    int
	WorkGroupZ    int
	GenericParams []GenericTypeParam // Phase 26: generic type parameters
}

// GlobalIdExpr represents GPU thread indexing: global_id(0)
type GlobalIdExpr struct {
	Dimension int // 0 = X, 1 = Y, 2 = Z
}

// BarrierStmt represents thread block synchronization: barrier()
type BarrierStmt struct{}

// Phase 19: Struct type declarations
type StructField struct {
	Name string
	Type string
}

type StructDeclStmt struct {
	Name          string
	Fields        []StructField
	GenericParams []GenericTypeParam // Phase 26: generic type parameters
}

type StructLiteral struct {
	TypeName string
	Fields   []Node   // BinaryExpr nodes: field = value
}

// Phase 19: Boolean literals
type BoolLiteral struct {
	Value bool
}

// Phase 19: For loops
type ForStmt struct {
	Init      Node
	Condition Node
	Post      Node
	Body      []Node
}

// Phase 19: Unary expressions (-x, !x)
type UnaryExpr struct {
	Operator string
	Operand  Node
}

// Phase 26: Monomorphized Generics and Trait Constraints

// GenericTypeParam represents a type parameter in a generic declaration, e.g., T: Numeric
type GenericTypeParam struct {
	Name       string   // Type parameter name, e.g., "T"
	Constraints []string // Trait constraints, e.g., ["Numeric"]
}

// GenericInst represents a concrete type argument in a generic instantiation, e.g., Vector<int>
type GenericInst struct {
	TypeArgs []string // Concrete type arguments, e.g., ["int", "float"]
}

// TraitDeclStmt represents a trait declaration:
// trait Numeric { fn add(self, other: T) -> T; fn zero() -> T; }
type TraitDeclStmt struct {
	Name   string
	Methods []TraitMethod
}

// TraitMethod represents a method signature inside a trait
type TraitMethod struct {
	Name       string
	Params     []Parameter
	ReturnType string
}

// ImplDeclStmt represents an implementation of a trait for a concrete type:
// impl Numeric for int { ... }
type ImplDeclStmt struct {
	TraitName string   // Name of the trait being implemented
	ForType   string   // Concrete type implementing the trait
	Methods   []FuncDecl // Implemented methods
}

// Phase 27: Hardware-Native Tensor Types & Autograd Engine

// TensorType represents a parametric tensor type: Tensor<f32, [32, 3, 224, 224]>
type TensorType struct {
	ElementType string   // Element scalar type: "f32", "f16", "i32"
	Shape       []int    // Dimension sizes: [32, 3, 224, 224]
	ShapeParams []string // Named shape params: ["B", "C", "H", "W"] for dynamic dims
}

func (tt *TensorType) String() string {
	dims := ""
	for i, s := range tt.Shape {
		if i > 0 {
			dims += ", "
		}
		dims += fmt.Sprintf("%d", s)
	}
	for _, p := range tt.ShapeParams {
		if len(dims) > 0 {
			dims += ", "
		}
		dims += p
	}
	return fmt.Sprintf("Tensor<%s, [%s]>", tt.ElementType, dims)
}

// TensorStmt represents a tensor computation block:
// tensor ForwardPass(x: Tensor<f32, [32, 128]>) -> Tensor<f32, [32, 64]> { ... }
type TensorStmt struct {
	Name       string
	Params     []TensorParam
	ReturnType *TensorType
	Body       []Node
}

// TensorParam represents a parameter in a tensor function signature
type TensorParam struct {
	Name string
	Type *TensorType
}

// TensorOpExpr represents a built-in tensor operation call:
// ops.matmul(x, w), ops.relu(mat), ops.softmax(x), ops.conv2d(x, k), ops.transpose(x)
type TensorOpExpr struct {
	Op     string // "matmul", "relu", "softmax", "conv2d", "transpose"
	Args   []Node // Operands (Identifiers or other expressions)
	Attrs  map[string]Node // Named attributes (e.g., dim: 1 for transpose, stride: 2 for conv2d)
}

// TensorReturnStmt represents a return statement inside a tensor block
type TensorReturnStmt struct {
	Value Node
}

// TensorIndexExpr represents element access: t[i, j, k]
type TensorIndexExpr struct {
	Source Node
	Indices []Node
}

// TensorShapeOfExpr represents querying tensor shape: shape_of(x)
type TensorShapeOfExpr struct {
	Operand Node
}

// Phase 28: Quantum Circuit Primitives, QIR & OpenQASM Backend

// QubitType represents a native qubit type: Qubit or Qubit[N] for register
type QubitType struct {
	Size int // 0 for single qubit, N for Qubit[N] register
}

// BitType represents a native classical bit type: Bit or Bit[N]
type BitType struct {
	Size int // 0 for single bit, N for Bit[N] register
}

// CircuitDecl represents a quantum circuit declaration block:
// circuit BellPair(q: Qubit[2]) -> Bit[2] { ... }
type CircuitDecl struct {
	Name       string
	Params     []CircuitParam
	ReturnType *BitType
	Body       []Node
}

// CircuitParam represents a parameter in a circuit signature
type CircuitParam struct {
	Name string
	Type *QubitType
}

// QPUOpExpr represents a built-in quantum gate operation:
// qpu.h(q[0]), qpu.cx(q[0], q[1]), qpu.rx(theta, q[0])
type QPUOpExpr struct {
	Op    string // "h", "x", "y", "z", "rx", "ry", "rz", "cx", "cz", "swap", "measure", "reset"
	Args  []Node // Target qubits, control qubits, rotation angles
	Angle Node   // Rotation angle for parameterized gates (rx, ry, rz)
}

// CircuitReturnStmt represents a return statement inside a circuit block
type CircuitReturnStmt struct {
	Value Node
}

// QubitIndexExpr represents indexed qubit access: q[0], q[1]
type QubitIndexExpr struct {
	Qubit Node
	Index Node
}

// QubitAssignStmt represents qubit assignment: q = alloc qubit[2]
type QubitAssignStmt struct {
	Name string
	Size int  // Number of qubits in the register
	Init bool // true for alloc, false for alias
}

// StmtList is a group of statements, used by macro expansion
type StmtList struct {
	Statements []Node
}

// ============================================================
// Phase 38: Coroutine/Async Runtime & Green Thread Scheduler
// ============================================================

// CoroutineDecl represents a coroutine declaration:
// co name(params) { body }
type CoroutineDecl struct {
	Name          string
	Params        []Parameter
	Body          []Node
	IsAsync       bool   // async coroutine
	RetType       string // return type
	GenericParams []GenericTypeParam
}

// AsyncExpr represents an async expression block: async { ... }
type AsyncExpr struct {
	Body []Node
}

// AwaitExpr represents an await expression: await(expr)
type AwaitExpr struct {
	Operand Node
	Timeout Node // optional timeout
}

// YieldExpr represents a yield expression: yield(value)
type YieldExpr struct {
	Value Node
}

// ChSendExpr represents a channel send: ch <- value
type ChSendExpr struct {
	Channel Node
	Value   Node
}

// ChRecvExpr represents a channel receive: <-ch
type ChRecvExpr struct {
	Channel Node
}

// ChDeclExpr represents a channel declaration: chan<T>(buffer_size)
type ChDeclExpr struct {
	ElementType string
	BufferSize  Node // nil for unbuffered
}

// SelectStmt represents a select multiplexer:
// select { case v <- ch1: ... case v = <-ch2: ... default: ... }
type SelectStmt struct {
	Cases   []SelectCase
	Default []Node
}

// SelectCase represents one branch of a select statement
type SelectCase struct {
	Channel Node    // the channel expression
	Dir     string  // "send" or "recv"
	VarName string  // variable binding for recv
	Value   Node    // value for send
	Body    []Node  // case body
}

// GreenSpawnExpr spawns a green thread: gospawn(fn(args...))
type GreenSpawnExpr struct {
	Function string
	Args     []Node
}

// AwaitAllExpr awaits multiple futures: await_all(f1, f2, ...)
type AwaitAllExpr struct {
	Futures []Node
}