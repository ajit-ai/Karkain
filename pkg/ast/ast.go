package ast

// PointerType represents *T
type PointerType struct {
	BaseType string
}

func (pt *PointerType) String() string { return "*" + pt.BaseType }

// PointerDereferenceExpr represents *p
type PointerDereferenceExpr struct {
	Value Expr
}

func (pd *PointerDereferenceExpr) exprNode() {}

// AddressOfExpr represents addr(x) or &x
type AddressOfExpr struct {
	Value Expr
}

func (ao *AddressOfExpr) exprNode() {}

// AllocExpr represents alloc(T, count)
type AllocExpr struct {
	Type  string
	Count Expr
}

func (ae *AllocExpr) exprNode() {}

// FreeStmt represents free(ptr)
type FreeStmt struct {
	Target Expr
}

func (fs *FreeStmt) stmtNode() {}

// CImportDecl represents import "C" { ... }
type CImportDecl struct {
	RawCHeader string // Contains #include <math.h>, function prototypes, etc.
}

func (ci *CImportDecl) stmtNode() {}

// MatrixDeclStmt represents var mat = matrix[M, N]float64
type MatrixDeclStmt struct {
	Name        string
	Rows        Expr
	Cols        Expr
	ElementType string
}

func (md *MatrixDeclStmt) stmtNode() {}

// MatrixAccessExpr represents mat[row, col]
type MatrixAccessExpr struct {
	MatrixExpr Expr
	Row        Expr
	Col        Expr
}

func (ma *MatrixAccessExpr) exprNode() {}

// Phase 17: Metaprogramming AST nodes

// MacroDeclStmt represents macro declaration
type MacroDeclStmt struct {
	Name       string
	Params     []string
	Body       []interface{}
	IsHygienic bool
}

func (md *MacroDeclStmt) stmtNode() {}

// MacroExpandExpr represents macro invocation/expansion
type MacroExpandExpr struct {
	MacroName string
	Args      []interface{}
}

func (me *MacroExpandExpr) exprNode() {}

// QuoteExpr represents quoted expression (AST as data)
type QuoteExpr struct {
	Expr interface{}
}

func (qe *QuoteExpr) exprNode() {}

// UnquoteExpr represents unquoted expression (code splicing)
type UnquoteExpr struct {
	Expr interface{}
}

func (ue *UnquoteExpr) exprNode() {}

// ComptimeStmt represents compile-time executed code
type ComptimeStmt struct {
	Body []interface{}
}

func (cs *ComptimeStmt) stmtNode() {}

// ComptimeExpr represents compile-time evaluated expression
type ComptimeExpr struct {
	Expr interface{}
}

func (ce *ComptimeExpr) exprNode() {}

// ReflectTypeExpr represents type reflection expression
type ReflectTypeExpr struct {
	TypeExpr interface{}
}

func (rt *ReflectTypeExpr) exprNode() {}

// DeriveExpr represents trait derivation (e.g., @derive(JsonSerializable))
type DeriveExpr struct {
	Trait  string
	Target interface{}
	Args   []interface{}
}

func (de *DeriveExpr) exprNode() {}

// TagExpr represents struct field tagging (e.g., @tag("json:name"))
type TagExpr struct {
	Target   interface{}
	TagName  string
	TagValue string
}

func (te *TagExpr) exprNode() {}
