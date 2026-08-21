package sema

import "karkain/pkg/parser"

type VarOwnership int

const (
	Owned VarOwnership = iota
	Moved
	Borrowed
	BorrowedMut
)

type BorrowError struct {
	Message string
	Line    uint16
}

type BorrowChecker struct {
	variables map[string]VarOwnership
	errors    []BorrowError
}

func NewBorrowChecker() *BorrowChecker {
	return &BorrowChecker{
		variables: make(map[string]VarOwnership),
	}
}

func (bc *BorrowChecker) Check(prog *parser.Program) []BorrowError {
	bc.errors = nil
	bc.variables = make(map[string]VarOwnership)

	for _, stmt := range prog.Statements {
		bc.checkNode(stmt)
	}
	return bc.errors
}

func (bc *BorrowChecker) checkNode(node parser.Node) {
	if node == nil {
		return
	}
	switch n := node.(type) {
	case *parser.FuncDecl:
		bc.checkBlock(n.Body)
	case *parser.VarDeclStmt:
		bc.checkNode(n.Value)
		bc.variables[n.Name] = Owned
	case *parser.ReturnStmt:
		bc.checkNode(n.Value)
	case *parser.ExprStmt:
		bc.checkNode(n.Expression)
	case *parser.IfStmt:
		bc.checkNode(n.Condition)
		bc.checkBlock(n.Consequence)
		bc.checkBlock(n.Alternative)
	case *parser.WhileStmt:
		bc.checkNode(n.Condition)
		bc.checkBlock(n.Body)
	case *parser.ForStmt:
		bc.checkNode(n.Init)
		bc.checkNode(n.Condition)
		bc.checkNode(n.Post)
		bc.checkBlock(n.Body)
	case *parser.PrintStmt:
		bc.checkNode(n.Value)
	case *parser.BinaryExpr:
		bc.checkNode(n.Left)
		bc.checkNode(n.Right)
	case *parser.UnaryExpr:
		bc.checkNode(n.Operand)
	case *parser.CallExpr:
		for _, arg := range n.Args {
			bc.checkNode(arg)
		}
	case *parser.ArrayLiteral:
		for _, elem := range n.Elements {
			bc.checkNode(elem)
		}
	case *parser.MapLiteral:
		for i, key := range n.Keys {
			bc.checkNode(key)
			bc.checkNode(n.Values[i])
		}
	case *parser.IndexExpr:
		bc.checkNode(n.Left)
		bc.checkNode(n.Index)
	case *parser.DotExpr:
		bc.checkNode(n.Left)
	case *parser.Identifier:
		bc.checkIdentifierUse(n.Name)
	case *parser.BorrowExpr:
		bc.checkBorrow(n)
	case *parser.MoveExpr:
		bc.checkMove(n)
	case *parser.RawAccessExpr:
		bc.checkNode(n.Address)
		bc.checkNode(n.Value)
	case *parser.OptionSomeExpr:
		bc.checkNode(n.Value)
	case *parser.OptionNoneExpr:
	case *parser.ResultOkExpr:
		bc.checkNode(n.Value)
	case *parser.ResultErrExpr:
		bc.checkNode(n.Error)
	case *parser.MatchExpr:
		bc.checkNode(n.Value)
		for _, arm := range n.Arms {
			bc.checkNode(arm.Body)
		}
	case *parser.SIMDBuiltinExpr:
		for _, arg := range n.Args {
			bc.checkNode(arg)
		}
	case *parser.StructLiteral:
		for _, field := range n.Fields {
			bc.checkNode(field)
		}
	case *parser.AddressOf:
		bc.checkNode(n.Operand)
	case *parser.Dereference:
		bc.checkNode(n.Operand)
	}
}

func (bc *BorrowChecker) checkBlock(block []parser.Node) {
	for _, stmt := range block {
		bc.checkNode(stmt)
	}
}

func (bc *BorrowChecker) checkIdentifierUse(name string) {
	state, exists := bc.variables[name]
	if !exists {
		return
	}
	switch state {
	case Moved:
		bc.errors = append(bc.errors, BorrowError{
			Message: "use of moved value: '" + name + "'",
		})
	case BorrowedMut:
		bc.errors = append(bc.errors, BorrowError{
			Message: "cannot use '" + name + "' while mutably borrowed",
		})
	}
}

func (bc *BorrowChecker) checkBorrow(n *parser.BorrowExpr) {
	ident, ok := n.Operand.(*parser.Identifier)
	if !ok {
		bc.checkNode(n.Operand)
		return
	}
	name := ident.Name

	state, exists := bc.variables[name]
	if !exists {
		return
	}

	if state == Moved {
		bc.errors = append(bc.errors, BorrowError{
			Message: "cannot borrow moved value: '" + name + "'",
		})
		return
	}

	if n.Mutable {
		if state == Borrowed || state == BorrowedMut {
			bc.errors = append(bc.errors, BorrowError{
				Message: "cannot mutably borrow '" + name + "' — already borrowed",
			})
			return
		}
		bc.variables[name] = BorrowedMut
	} else {
		if state == BorrowedMut {
			bc.errors = append(bc.errors, BorrowError{
				Message: "cannot borrow '" + name + "' — mutably borrowed",
			})
			return
		}
		bc.variables[name] = Borrowed
	}
}

func (bc *BorrowChecker) checkMove(n *parser.MoveExpr) {
	ident, ok := n.Operand.(*parser.Identifier)
	if !ok {
		bc.checkNode(n.Operand)
		return
	}
	name := ident.Name

	state, exists := bc.variables[name]
	if !exists {
		return
	}

	if state == Moved {
		bc.errors = append(bc.errors, BorrowError{
			Message: "cannot move '" + name + "' — already moved",
		})
		return
	}

	if state == Borrowed || state == BorrowedMut {
		bc.errors = append(bc.errors, BorrowError{
			Message: "cannot move '" + name + "' — value is borrowed",
		})
		return
	}

	bc.variables[name] = Moved
}
