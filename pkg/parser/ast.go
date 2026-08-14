package parser

type Node interface{}

type Program struct {
	Statements []Node
	CImports   []*CImportBlock // C import blocks for codegen
}

type FuncDecl struct {
	Name   string
	Params []string
	Body   []Node
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

type ComptimeStmt struct {
	Body []Node // Code executed at compile time
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
