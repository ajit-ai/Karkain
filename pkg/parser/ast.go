package parser

import "fmt"

type Node interface{}

type Program struct {
	Statements []Node
	CImports   []*CImportBlock // C import blocks for codegen
	Imports    []*ModuleImport // Karkain module imports
	Line       int
}

// ModuleImport represents a top-level `import <name>` declaration that
// imports another Karkain module's public symbols into the current compile
// unit. The concatenated source model means all symbols are already flat; this
// declaration is used by the resolver to validate that the referenced module
// exists in the project and to document explicit dependency edges.
type ModuleImport struct {
	Name string
	Line int
}

type FuncDecl struct {
	Name          string
	Params        []string
	ParamTypes    []string // Phase 46: typed parameters (e.g., "int", "string")
	Body          []Node
	GenericParams []GenericTypeParam // Phase 26: generic type parameters
	Captures      []string           // Phase 54: free variables captured from enclosing scope (lambdas only)
	Public        bool               // Phase 80: visibility modifier (public decl usable across files/modules)
	Target        string             // Phase 98: execution target from @target(...); "" = default (cpu)
	Line          int
	Col           int // Phase 83: 0-based byte column of the function name token
	EndCol        int // Phase 83: 0-based byte column just past the function name
}

type VarDeclStmt struct {
	Name     string
	Value    Node
	Type     string // Optional type information (e.g., "*int", "[4]f32")
	Const    bool   // Phase 112: true for `const` declarations (immutable binding)
	IsMatrix bool   // True if this is a matrix declaration
	IsSIMD   bool   // True if this is a fixed-lane SIMD vector ([4]f32)
	Align    int    // Cache-line alignment attribute (0 = none)
	Escapes  bool   // Phase 49: true if variable escapes current scope (passed to func, returned, captured by lambda)
	Line     int
	Col      int // Phase 83: 0-based byte column of the variable name token
	EndCol   int // Phase 83: 0-based byte column just past the variable name
}

type ReturnStmt struct {
	Value Node
	Line  int
}

type ExprStmt struct {
	Expression Node
	Line       int
}

type IfStmt struct {
	Condition   Node
	Consequence []Node
	Alternative []Node
	Line        int
}

type WhileStmt struct {
	Condition Node
	Body      []Node
	Line      int
}

type PrintStmt struct {
	Value Node
	Line  int
}

type BlockStmt struct {
	Statements []Node
}

type StringLiteral struct {
	Value string
	Line  int
}

type IntLiteral struct {
	Value string
	Line  int
}

type Float64Literal struct {
	Value string
	Line  int
}

type BigIntLiteral struct {
	Value string
	Line  int
}

type BigFloatLiteral struct {
	Value string
	Line  int
}

type Identifier struct {
	Name   string
	Line   int
	Col    int // Phase 83: 0-based byte column of the identifier token (0 when not tracked)
	EndCol int // Phase 83: 0-based byte column just past the identifier
}

type ArrayLiteral struct {
	Elements []Node
	Line     int
}

type MapLiteral struct {
	Keys   []Node
	Values []Node
	Line   int
}

type IndexExpr struct {
	Left  Node
	Index Node
	Line  int
}

// Phase 55: slice expression target[start:end]; End nil = open-ended (to length)
type SliceExpr struct {
	Target Node
	Start  Node
	End    Node
	Line   int
}

type BinaryExpr struct {
	Left     Node
	Operator string
	Right    Node
	Line     int
}

type CallExpr struct {
	Function string
	Module   string // Phase 103: module qualifier for `mod.fn(...)`; "" = bare call
	Args     []Node
	IsCFunc  bool // True if this is a C function call (e.g., C.sqrt)
	Line     int
	Col      int // Phase 83: 0-based byte column of the callee start (function-name token)
	EndCol   int // Phase 83: 0-based byte column just past the callee
}

// Phase 133: IndirectCallExpr represents a call through a computed callee —
// `ops[0](5)`, `get_fn()(1)`, `(expr)(args)`. Identifier callees keep using
// CallExpr (zero regression to static dispatch); only non-identifier targets
// lower here. The target evaluates to a TYPE_FUNC Value cell at runtime.
type IndirectCallExpr struct {
	Target Node
	Args   []Node
	Line   int
	Col    int
	EndCol int
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
	Line    int
}

// BorrowExpr represents creating a borrow: &x or &mut x
type BorrowExpr struct {
	Operand Node
	Mutable bool // true for &mut x, false for &x
	Line    int
}

// MoveExpr represents an explicit move: move(x)
// Transfers ownership from x to the target
type MoveExpr struct {
	Operand Node
	Line    int
}

// Phase 44: Error propagation, exhaustive match, linear type enforcement

// PropagateExpr represents the ? operator: expr?
// Desugars to: match expr { Ok(v) => v, Err(e) => return Err(e) }
type PropagateExpr struct {
	Operand Node
	Line    int
}

// Phase 42: Option<T>, Result<T,E>, match, SIMD, packed structs, linear types

// OptionSomeExpr represents Some(value) — an option with a value
type OptionSomeExpr struct {
	Value Node
	Line  int
}

// OptionNoneExpr represents None — an empty option
type OptionNoneExpr struct{ Line int }

// ResultOkExpr represents Ok(value) — a successful result
type ResultOkExpr struct {
	Value Node
	Line  int
}

// ResultErrExpr represents Err(error) — a failed result
type ResultErrExpr struct {
	Error Node
	Line  int
}

// MatchExpr represents pattern matching:
// match value { pattern => expr, ... }
type MatchExpr struct {
	Value Node
	Arms  []MatchArm
	Line  int
}

// MatchArm represents one arm of a match expression: pattern => expr
type MatchArm struct {
	Pattern MatchPattern
	Body    Node
}

// MatchPattern represents a pattern in match arms
type MatchPattern struct {
	Type    string // "Some", "None", "Ok", "Err", "literal", "wildcard"
	Value   Node   // For literal patterns (42, "hello", true)
	Binding string // For variable bindings (e.g., x in Some(x))
}

// SIMDBuiltinExpr represents SIMD intrinsics: @simd_add(a, b), @simd_mul(a, b)
type SIMDBuiltinExpr struct {
	Op   string // "add", "mul", "sub", "div", "min", "max", "sqrt", "splat", "load"
	Args []Node
	Line int
}

// SIMDVectorType represents an explicit lane vector type: [4]f32, [8]f32, etc.
// It is stored in VarDeclStmt.Type as a structured, machine-checkable form.
type SIMDVectorType struct {
	Elem  string // "f32", "f64", "i32", "i64"
	Lanes int    // 4, 8, 16, ...
	Line  int
}

// LaneCount returns the number of SIMD lanes for a lane-vector type string
// like "[4]f32". Returns 0 if the string is not a valid lane-vector type.
func LaneCount(typeStr string) int {
	l, _, ok := ParseSIMDVectorType(typeStr)
	if !ok {
		return 0
	}
	return l
}

// ParseSIMDVectorType parses a "[N]T" type string into (lanes, elem). ok is
// false when the string is not a lane-vector type.
func ParseSIMDVectorType(typeStr string) (lanes int, elem string, ok bool) {
	if len(typeStr) < 5 || typeStr[0] != '[' {
		return 0, "", false
	}
	closeIdx := -1
	for i := 1; i < len(typeStr); i++ {
		if typeStr[i] == ']' {
			closeIdx = i
			break
		}
	}
	if closeIdx <= 1 {
		return 0, "", false
	}
	n := 0
	for i := 1; i < closeIdx; i++ {
		c := typeStr[i]
		if c < '0' || c > '9' {
			return 0, "", false
		}
		n = n*10 + int(c-'0')
	}
	elem = typeStr[closeIdx+1:]
	switch elem {
	case "f32", "float32", "f64", "float64", "i32", "int32", "i64", "int64":
		return n, elem, true
	}
	return 0, "", false
}

// AtomicOp represents an atomic builtin: @atomic_load/store/fetch_add/cas.
type AtomicOp struct {
	Op    string // "load", "store", "fetch_add", "fetch_sub", "cas", "exchange"
	Args  []Node
	Order string // "relaxed", "acquire", "release", "acq_rel", "seq_cst"
	Line  int
}

// AlignmentAttr represents a cache-line/alignment attribute: @aligned(64).
type AlignmentAttr struct {
	Alignment int
	Line      int
}

// LinearTypeDecl marks a type as linear (must be used exactly once):
// linear type FileHandle { fd: int }
type LinearTypeDecl struct {
	Name   string
	Fields []StructField
	Line   int
}

// PackedStructDecl marks a struct as packed (no padding):
// packed struct Point { x: float32, y: float32 }
type PackedStructDecl struct {
	Name   string
	Fields []StructField
	Line   int
}

// Phase 45: Custom enum types with algebraic data variants

// EnumDecl represents an enum declaration:
// enum Color { Red, Green, Blue }
// enum Result { Ok(value), Err(error) }
type EnumDecl struct {
	Name     string
	Variants []EnumVariant
	Public   bool // Phase 80: visibility modifier
	Line     int
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
	Line     int
}

// Phase 11: Native C Interop, Raw Pointers, and Continuous Matrix Memory Layouts

type CImportBlock struct {
	Content string // Raw C code between { }
	Line    int
}

type PointerType struct {
	BaseType string // e.g., "int", "float64"
}

type AddressOf struct {
	Operand Node
	Line    int
}

type Dereference struct {
	Operand Node
	Line    int
}

type AllocExpr struct {
	Type  string // e.g., "int", "float64"
	Count Node   // Number of elements
	Line  int
}

type FreeExpr struct {
	Ptr  Node
	Line int
}

type MatrixDecl struct {
	Rows     Node
	Cols     Node
	DataType string // e.g., "float64", "int"
	Line     int
}

type MatrixIndexExpr struct {
	Matrix Node
	Row    Node
	Col    Node
	Line   int
}

type DotExpr struct {
	Left  Node
	Right string // The field/method name
	Line  int
}

// Phase 14: Quantum Computing AST Nodes

type QRegDeclStmt struct {
	Name   string
	Qubits Node // Number of qubits
	Line   int
}

type GateApplyStmt struct {
	Gate    string // "H", "X", "CNOT", etc.
	Target  Node   // Target qubit(s)
	Control Node   // Control qubit (for CNOT), nil for single-qubit gates
	Params  []Node // Additional parameters (rotation angles, etc.)
	Line    int
}

type MeasureExpr struct {
	Qubit Node // Qubit to measure
	Line  int
}

// Phase 16: Actor-based distributed concurrency AST nodes
type ActorDeclStmt struct {
	Name     string
	Params   []string
	Body     []Node
	State    []ActorStateField // Phase 37: actor state fields
	Handlers []ActorHandler    // Phase 37: message handlers
	Line     int
}

// ActorStateField represents a field in actor state
type ActorStateField struct {
	Name    string
	Type    string
	Default Node
}

// ActorHandler represents a receive handler in an actor
type ActorHandler struct {
	MessageType string // message type name
	ParamName   string // parameter binding name
	ParamType   string // parameter type
	Body        []Node
	IsReply     bool // whether this handler replies
}

type SpawnExpr struct {
	ActorName string
	Args      []Node
	NodeAddr  string // Phase 37: optional remote node address
	Line      int
}

type ReceiveStmt struct {
	Channel Node
	VarName string
	Line    int
}

type SendExpr struct {
	Channel Node
	Message Node
	IsSync  bool // true = !? (request-reply), false = ! (async)
	Timeout Node // optional timeout for sync send
	Line    int
}

// Phase 17: Metaprogramming AST nodes

type MacroDeclStmt struct {
	Name       string
	Params     []string
	Body       []Node
	IsHygienic bool // For hygiene tracking
	Line       int
}

type MacroExpandExpr struct {
	MacroName string
	Args      []Node
	Line      int
}

type QuoteExpr struct {
	Expr Node // Quoted AST node
	Line int
}

type UnquoteExpr struct {
	Expr Node // Unquoted expression to be evaluated
	Line int
}

type ComptimeExpr struct {
	Expr Node // Expression evaluated at compile time
	Line int
}

type ReflectTypeExpr struct {
	TypeExpr Node // Type to reflect on
	Line     int
}

type DeriveExpr struct {
	Trait  string // e.g., "JsonSerializable"
	Target Node   // Target struct/type
	Args   []Node // Additional arguments
	Line   int
}

type TagExpr struct {
	Target   Node   // Target field/struct
	TagName  string // Tag name
	TagValue string // Tag value
	Line     int
}

// ComptimeStmt represents a compile-time evaluated statement: `comptime var x = ...`
type ComptimeStmt struct {
	Body []Node
	Line int
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
	Line          int
}

// GlobalIdExpr represents GPU thread indexing: global_id(0)
type GlobalIdExpr struct {
	Dimension int // 0 = X, 1 = Y, 2 = Z
	Line      int
}

// BarrierStmt represents thread block synchronization: barrier()
type BarrierStmt struct{ Line int }

// Phase 19: Struct type declarations
type StructField struct {
	Name string
	Type string
}

type StructDeclStmt struct {
	Name          string
	Fields        []StructField
	GenericParams []GenericTypeParam // Phase 26: generic type parameters
	Public        bool               // Phase 80: visibility modifier
	Line          int
}

type StructLiteral struct {
	TypeName string
	Fields   []Node // BinaryExpr nodes: field = value
	Line     int
}

// Phase 19: Boolean literals
type BoolLiteral struct {
	Value bool
	Line  int
}

// Phase 19: For loops
type ForStmt struct {
	Init      Node
	Condition Node
	Post      Node
	Body      []Node
	Line      int
}

// Phase 47: for-in loops (for x in arr { ... })
type ForInStmt struct {
	VarName string // iterator variable name (value for maps, element for arrays)
	KeyName string // Phase 48: key variable for map iteration (empty for arrays)
	Iter    Node   // expression to iterate over
	Body    []Node
	Line    int
}

// Phase 48: break/continue
type BreakStmt struct{ Line int }
type ContinueStmt struct{ Line int }

// Phase 48: Lambda / function pointer expressions: fn(a, b) { return a + b }
type LambdaExpr struct {
	Params     []string
	ParamTypes []string
	Body       []Node
	Captures   []string // Phase 54: free variables captured from enclosing scope
	Line       int
}

// Phase 127: Closure expression with explicit capture semantics.
// A ClosureExpr represents a fn expression that captures variables from
// its enclosing scope. Captures can be by-reference (for `var`) or by-value
// (for `let`). The CapturesByRef field tracks which captures are by-reference.
// The EnvName is the generated environment struct name for this closure.
type ClosureExpr struct {
	Params        []string
	ParamTypes    []string
	Body          []Node
	Captures      []string // All captured variable names
	CapturesByRef []bool   // Parallel to Captures: true = by-ref (var), false = by-value (let)
	EnvName       string   // Generated environment struct name (e.g., "_closure_env_1")
	ReturnsValue  bool     // True if closure body returns a value (not just statements)
	Line          int
}

// Phase 48: Function reference expression (used when let x = fn(...) is desugared to named function)
type FuncRefExpr struct {
	Name string
	Line int
}

// Phase 19: Unary expressions (-x, !x)
type UnaryExpr struct {
	Operator string
	Operand  Node
	Line     int
}

// Phase 26: Monomorphized Generics and Trait Constraints

// GenericTypeParam represents a type parameter in a generic declaration, e.g., T: Numeric
type GenericTypeParam struct {
	Name        string   // Type parameter name, e.g., "T"
	Constraints []string // Trait constraints, e.g., ["Numeric"]
}

// GenericInst represents a concrete type argument in a generic instantiation, e.g., Vector<int>
type GenericInst struct {
	TypeArgs []string // Concrete type arguments, e.g., ["int", "float"]
}

// TraitDeclStmt represents a trait declaration:
// trait Numeric { fn add(self, other: T) -> T; fn zero() -> T; }
type TraitDeclStmt struct {
	Name    string
	Methods []TraitMethod
	Line    int
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
	TraitName string     // Name of the trait being implemented
	ForType   string     // Concrete type implementing the trait
	Methods   []FuncDecl // Implemented methods
	Line      int
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
	Line       int
}

// TensorParam represents a parameter in a tensor function signature
type TensorParam struct {
	Name string
	Type *TensorType
}

// TensorOpExpr represents a built-in tensor operation call:
// ops.matmul(x, w), ops.relu(mat), ops.softmax(x), ops.conv2d(x, k), ops.transpose(x)
type TensorOpExpr struct {
	Op    string          // "matmul", "relu", "softmax", "conv2d", "transpose"
	Args  []Node          // Operands (Identifiers or other expressions)
	Attrs map[string]Node // Named attributes (e.g., dim: 1 for transpose, stride: 2 for conv2d)
	Line  int
}

// TensorReturnStmt represents a return statement inside a tensor block
type TensorReturnStmt struct {
	Value Node
	Line  int
}

// TensorIndexExpr represents element access: t[i, j, k]
type TensorIndexExpr struct {
	Source  Node
	Indices []Node
	Line    int
}

// TensorShapeOfExpr represents querying tensor shape: shape_of(x)
type TensorShapeOfExpr struct {
	Operand Node
	Line    int
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
	Line       int
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
	Line  int
}

// CircuitReturnStmt represents a return statement inside a circuit block
type CircuitReturnStmt struct {
	Value Node
	Line  int
}

// QubitIndexExpr represents indexed qubit access: q[0], q[1]
type QubitIndexExpr struct {
	Qubit Node
	Index Node
	Line  int
}

// QubitAssignStmt represents qubit assignment: q = alloc qubit[2]
type QubitAssignStmt struct {
	Name string
	Size int  // Number of qubits in the register
	Init bool // true for alloc, false for alias
	Line int
}

// StmtList is a group of statements, used by macro expansion
type StmtList struct {
	Statements []Node
	Line       int
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
	Line          int
}

// AsyncExpr represents an async expression block: async { ... }
type AsyncExpr struct {
	Body []Node
	Line int
}

// AwaitExpr represents an await expression: await(expr)
type AwaitExpr struct {
	Operand Node
	Timeout Node // optional timeout
	Line    int
}

// YieldExpr represents a yield expression: yield(value)
type YieldExpr struct {
	Value Node
	Line  int
}

// ChSendExpr represents a channel send: ch <- value
type ChSendExpr struct {
	Channel Node
	Value   Node
	Line    int
}

// ChRecvExpr represents a channel receive: <-ch
type ChRecvExpr struct {
	Channel Node
	Line    int
}

// ChDeclExpr represents a channel declaration: chan<T>(buffer_size)
type ChDeclExpr struct {
	ElementType string
	BufferSize  Node // nil for unbuffered
	Line        int
}

// SelectStmt represents a select multiplexer:
// select { case v <- ch1: ... case v = <-ch2: ... default: ... }
type SelectStmt struct {
	Cases   []SelectCase
	Default []Node
	Line    int
}

// SelectCase represents one branch of a select statement
type SelectCase struct {
	Channel Node   // the channel expression
	Dir     string // "send" or "recv"
	VarName string // variable binding for recv
	Value   Node   // value for send
	Body    []Node // case body
}

// GreenSpawnExpr spawns a green thread: gospawn(fn(args...))
type GreenSpawnExpr struct {
	Function string
	Args     []Node
	Line     int
}

// AwaitAllExpr awaits multiple futures: await_all(f1, f2, ...)
type AwaitAllExpr struct {
	Futures []Node
	Line    int
}

// GetLine returns the source line number for any AST node, or 0 if unknown.
func GetLine(node Node) int {
	if node == nil {
		return 0
	}
	// Statement-level nodes (most common for #line directives)
	switch n := node.(type) {
	case *Program:
		return n.Line
	case *FuncDecl:
		return n.Line
	case *VarDeclStmt:
		return n.Line
	case *ReturnStmt:
		return n.Line
	case *ExprStmt:
		return n.Line
	case *IfStmt:
		return n.Line
	case *WhileStmt:
		return n.Line
	case *PrintStmt:
		return n.Line
	case *ForStmt:
		return n.Line
	case *ForInStmt:
		return n.Line
	case *StructDeclStmt:
		return n.Line
	case *EnumDecl:
		return n.Line
	case *QRegDeclStmt:
		return n.Line
	case *GateApplyStmt:
		return n.Line
	case *ActorDeclStmt:
		return n.Line
	case *MacroDeclStmt:
		return n.Line
	case *ComptimeStmt:
		return n.Line
	case *KernelDeclStmt:
		return n.Line
	case *CoroutineDecl:
		return n.Line
	case *TensorStmt:
		return n.Line
	case *CircuitDecl:
		return n.Line
	case *SelectStmt:
		return n.Line
	case *QubitAssignStmt:
		return n.Line
	case *TraitDeclStmt:
		return n.Line
	case *ImplDeclStmt:
		return n.Line
	case *LinearTypeDecl:
		return n.Line
	case *PackedStructDecl:
		return n.Line
	case *StmtList:
		return n.Line
	case *BarrierStmt:
		return n.Line
	case *BreakStmt:
		return n.Line
	case *ContinueStmt:
		return n.Line
	// Expression nodes
	case *StringLiteral:
		return n.Line
	case *IntLiteral:
		return n.Line
	case *Float64Literal:
		return n.Line
	case *BigIntLiteral:
		return n.Line
	case *BigFloatLiteral:
		return n.Line
	case *Identifier:
		return n.Line
	case *BoolLiteral:
		return n.Line
	case *ArrayLiteral:
		return n.Line
	case *MapLiteral:
		return n.Line
	case *IndexExpr:
		return n.Line
	case *SliceExpr:
		return n.Line
	case *BinaryExpr:
		return n.Line
	case *CallExpr:
		return n.Line
	case *IndirectCallExpr:
		return n.Line
	case *DotExpr:
		return n.Line
	case *MatrixIndexExpr:
		return n.Line
	case *UnaryExpr:
		return n.Line
	case *LambdaExpr:
		return n.Line
	case *FuncRefExpr:
		return n.Line
	case *OptionSomeExpr:
		return n.Line
	case *OptionNoneExpr:
		return n.Line
	case *ResultOkExpr:
		return n.Line
	case *ResultErrExpr:
		return n.Line
	case *MatchExpr:
		return n.Line
	case *EnumVariantExpr:
		return n.Line
	case *RawAccessExpr:
		return n.Line
	case *BorrowExpr:
		return n.Line
	case *MoveExpr:
		return n.Line
	case *PropagateExpr:
		return n.Line
	case *AddressOf:
		return n.Line
	case *Dereference:
		return n.Line
	case *AllocExpr:
		return n.Line
	case *FreeExpr:
		return n.Line
	case *MatrixDecl:
		return n.Line
	case *MeasureExpr:
		return n.Line
	case *SpawnExpr:
		return n.Line
	case *ReceiveStmt:
		return n.Line
	case *SendExpr:
		return n.Line
	case *CImportBlock:
		return n.Line
	case *SIMDBuiltinExpr:
		return n.Line
	case *MacroExpandExpr:
		return n.Line
	case *QuoteExpr:
		return n.Line
	case *UnquoteExpr:
		return n.Line
	case *ComptimeExpr:
		return n.Line
	case *ReflectTypeExpr:
		return n.Line
	case *DeriveExpr:
		return n.Line
	case *TagExpr:
		return n.Line
	case *TensorOpExpr:
		return n.Line
	case *TensorReturnStmt:
		return n.Line
	case *TensorIndexExpr:
		return n.Line
	case *TensorShapeOfExpr:
		return n.Line
	case *QPUOpExpr:
		return n.Line
	case *CircuitReturnStmt:
		return n.Line
	case *QubitIndexExpr:
		return n.Line
	case *GlobalIdExpr:
		return n.Line
	case *AsyncExpr:
		return n.Line
	case *AwaitExpr:
		return n.Line
	case *YieldExpr:
		return n.Line
	case *ChSendExpr:
		return n.Line
	case *ChRecvExpr:
		return n.Line
	case *ChDeclExpr:
		return n.Line
	case *GreenSpawnExpr:
		return n.Line
	case *AwaitAllExpr:
		return n.Line
	}
	return 0
}
