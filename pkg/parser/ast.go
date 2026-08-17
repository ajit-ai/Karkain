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
	Name   string
	Params []string
	Body   []Node
}

type SpawnExpr struct {
	ActorName string
	Args      []Node
}

type ReceiveStmt struct {
	Channel Node
	VarName string
}

type SendExpr struct {
	Channel Node
	Message Node
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