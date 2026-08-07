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
