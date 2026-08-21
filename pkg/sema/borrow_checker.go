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
	variables     map[string]VarOwnership
	linearTypes   map[string]bool   // types declared as linear
	usageCount    map[string]int    // usage count for linear type variables
	varTypes      map[string]string // variable name -> type name
	errors        []BorrowError
}

func NewBorrowChecker() *BorrowChecker {
	return &BorrowChecker{
		variables:   make(map[string]VarOwnership),
		linearTypes: make(map[string]bool),
		usageCount:  make(map[string]int),
		varTypes:    make(map[string]string),
	}
}

func (bc *BorrowChecker) Check(prog *parser.Program) []BorrowError {
	bc.errors = nil
	bc.variables = make(map[string]VarOwnership)
	bc.linearTypes = make(map[string]bool)
	bc.usageCount = make(map[string]int)
	bc.varTypes = make(map[string]string)

	// First pass: collect linear type declarations
	for _, stmt := range prog.Statements {
		if ltd, ok := stmt.(*parser.LinearTypeDecl); ok {
			bc.linearTypes[ltd.Name] = true
		}
	}

	// Second pass: check ownership and borrow rules
	for _, stmt := range prog.Statements {
		bc.checkNode(stmt)
	}

	// Third pass: verify linear types were used exactly once
	for name, typeName := range bc.varTypes {
		if bc.linearTypes[typeName] {
			count := bc.usageCount[name]
			if count == 0 {
				bc.errors = append(bc.errors, BorrowError{
					Message: "linear type variable '" + name + "' of type '" + typeName + "' was never used",
				})
			} else if count > 1 {
				bc.errors = append(bc.errors, BorrowError{
					Message: "linear type variable '" + name + "' of type '" + typeName + "' was used " + itoa(count) + " times (must be exactly 1)",
				})
			}
		}
	}

	return bc.errors
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	result := ""
	for n > 0 {
		result = string(rune('0'+n%10)) + result
		n /= 10
	}
	return result
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
		if n.Type != "" {
			bc.varTypes[n.Name] = n.Type
		} else {
			// Phase 44: Infer Option/Result types from initializers
			typeName := bc.inferMatchType(n.Value)
			if typeName != "" {
				bc.varTypes[n.Name] = typeName
			}
		}
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
		bc.checkExhaustiveMatch(n)
	case *parser.PropagateExpr:
		bc.checkNode(n.Operand)
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
	// Track usage for linear type enforcement
	bc.usageCount[name]++
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

// checkExhaustiveMatch verifies that match expressions cover all variants
// for Option<T> (Some/None) and Result<T,E> (Ok/Err) types
func (bc *BorrowChecker) checkExhaustiveMatch(node *parser.MatchExpr) {
	// Determine the type of the matched value
	valueType := bc.inferMatchType(node.Value)
	if valueType == "" {
		return
	}

	covered := make(map[string]bool)
	for _, arm := range node.Arms {
		covered[arm.Pattern.Type] = true
	}

	switch valueType {
	case "Option":
		if !covered["Some"] || !covered["None"] {
			missing := ""
			if !covered["Some"] {
				missing += "Some "
			}
			if !covered["None"] {
				missing += "None"
			}
			bc.errors = append(bc.errors, BorrowError{
				Message: "non-exhaustive match on Option: missing pattern(s): " + missing,
			})
		}
	case "Result":
		if !covered["Ok"] || !covered["Err"] {
			missing := ""
			if !covered["Ok"] {
				missing += "Ok "
			}
			if !covered["Err"] {
				missing += "Err"
			}
			bc.errors = append(bc.errors, BorrowError{
				Message: "non-exhaustive match on Result: missing pattern(s): " + missing,
			})
		}
	}
}

// inferMatchType attempts to determine the type of a matched expression
func (bc *BorrowChecker) inferMatchType(node parser.Node) string {
	switch n := node.(type) {
	case *parser.Identifier:
		typeName, exists := bc.varTypes[n.Name]
		if !exists {
			return ""
		}
		if typeName == "Option" {
			return "Option"
		}
		if typeName == "Result" {
			return "Result"
		}
	case *parser.OptionSomeExpr:
		return "Option"
	case *parser.OptionNoneExpr:
		return "Option"
	case *parser.ResultOkExpr:
		return "Result"
	case *parser.ResultErrExpr:
		return "Result"
	case *parser.CallExpr:
		// Could be a function returning Option/Result
		// For now, return empty
	}
	return ""
}
