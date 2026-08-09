package parser

type Node interface{}

type Program struct {
	Statements []Node
}

type FuncDecl struct {
	Name   string
	Params []string
	Body   []Node
}

type VarDeclStmt struct {
	Name  string
	Value Node
}

type ReturnStmt struct {
	Value Node
}

type ExprStmt struct {
	Expression Node
}

type AssignStmt struct {
	Name  string
	Value Node
}

type DeleteStmt struct {
	Map Node
	Key Node
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
}

type StructDecl struct {
	Name   string
	Fields []*StructField
}

type StructField struct {
	Name string
	Type string
}

type StructLiteral struct {
	TypeName string
	Fields   map[string]Node
}

type FieldAccess struct {
	Left  Node
	Field string
}

// FieldAssignStmt represents `expr.field = value` (struct field mutation).
type FieldAssignStmt struct {
	Object Node
	Field  string
	Value  Node
}
