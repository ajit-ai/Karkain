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
