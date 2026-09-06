package sema

import (
	"fmt"
	"sort"

	"karkain/pkg/parser"
)

// Warning is a non-fatal front-end finding. Warnings share the ResolveError
// location model (1-based line, 0-based byte columns) but never terminate
// compilation: the CLI collects them, renders them, and continues.
type Warning struct {
	Line   int // 1-based line of the finding
	Col    int // 0-based byte column of the offending token (best-effort)
	EndCol int // 0-based byte column just past the offending token (best-effort)
	Msg    string
}

// UnusedVars reports `let`/`var` declarations inside function bodies that are
// never read in that function — the warning-infrastructure workhorse. It is
// deliberately conservative:
//
//   - Parameters and loop variables are exempt (only explicit VarDeclStmt
//     names are candidates).
//   - Any construct the walker does not understand (lambdas, actors,
//     quantum/tensor/borrow syntax, match arms with bindings, ...) suppresses
//     the whole function's findings, so the warning can never fire on a
//     program whose semantics the walker cannot see.
//   - Writes to a variable (assignment target) are not reads; a variable that
//     is only assigned but never read is still reported.
func UnusedVars(prog *parser.Program) []Warning {
	var warns []Warning
	for _, s := range prog.Statements {
		fn, ok := s.(*parser.FuncDecl)
		if !ok {
			continue
		}
		warns = append(warns, unusedVarsInFunc(fn)...)
	}
	return warns
}

// declInfo pins a declaration's location for reporting.
type declInfo struct {
	line   int
	col    int
	endCol int
}

// unusedScanner walks one function body, collecting declared variables and the
// names that are actually read. `unsupported` arms the conservative bail-out.
type unusedScanner struct {
	reads       map[string]bool
	decls       map[string]declInfo
	unsupported bool
}

func unusedVarsInFunc(fn *parser.FuncDecl) []Warning {
	u := &unusedScanner{
		reads: map[string]bool{},
		decls: map[string]declInfo{},
	}
	for _, s := range fn.Body {
		u.scanStmt(s)
	}
	if u.unsupported {
		return nil
	}

	var warns []Warning
	for name, d := range u.decls {
		if !u.reads[name] {
			warns = append(warns, Warning{
				Line:   d.line,
				Col:    d.col,
				EndCol: d.endCol,
				Msg:    fmt.Sprintf("unused variable `%s`", name),
			})
		}
	}
	sort.Slice(warns, func(i, j int) bool {
		if warns[i].Line != warns[j].Line {
			return warns[i].Line < warns[j].Line
		}
		return warns[i].Col < warns[j].Col
	})
	return warns
}

// scanStmt collects statements, descending into nested blocks.
func (u *unusedScanner) scanStmt(s parser.Node) {
	switch n := s.(type) {
	case *parser.VarDeclStmt:
		if n.Name != "" {
			u.decls[n.Name] = declInfo{line: n.Line, col: n.Col, endCol: n.EndCol}
		}
		if n.Value != nil {
			u.scanExpr(n.Value)
		}
	case *parser.ExprStmt:
		if n.Expression != nil {
			u.scanExpr(n.Expression)
		}
	case *parser.ReturnStmt:
		if n.Value != nil {
			u.scanExpr(n.Value)
		}
	case *parser.IfStmt:
		if n.Condition != nil {
			u.scanExpr(n.Condition)
		}
		u.scanBody(n.Consequence)
		u.scanBody(n.Alternative)
	case *parser.WhileStmt:
		if n.Condition != nil {
			u.scanExpr(n.Condition)
		}
		u.scanBody(n.Body)
	case *parser.ForStmt:
		if n.Init != nil {
			u.scanStmt(n.Init)
		}
		if n.Condition != nil {
			u.scanExpr(n.Condition)
		}
		if n.Post != nil {
			u.scanExpr(n.Post)
		}
		u.scanBody(n.Body)
	case *parser.ForInStmt:
		if n.Iter != nil {
			u.scanExpr(n.Iter)
		}
		u.scanBody(n.Body)
	case *parser.BlockStmt:
		u.scanBody(n.Statements)
	case *parser.PrintStmt:
		if n.Value != nil {
			u.scanExpr(n.Value)
		}
	case *parser.BreakStmt, *parser.ContinueStmt:
		// no reads
	default:
		u.unsupported = true
	}
}

func (u *unusedScanner) scanBody(stmts []parser.Node) {
	for _, s := range stmts {
		u.scanStmt(s)
	}
}

// scanWriteTarget marks the parts of an assignment target that are read. A
// plain identifier target is a pure write; indexed, matrix-indexed and member
// targets read their base/index/key expressions.
func (u *unusedScanner) scanWriteTarget(e parser.Node) {
	switch n := e.(type) {
	case *parser.Identifier:
		// plain variable write — no read
	case *parser.IndexExpr:
		u.scanExpr(n)
	case *parser.MatrixIndexExpr:
		u.scanExpr(n)
	case *parser.DotExpr:
		u.scanExpr(n)
	default:
		u.scanExpr(e)
	}
}

// scanExpr marks identifier reads in value position. Field keys, assignment
// targets, map keys and call callees are not value reads (a callee may be a
// stored function, though, so it is still marked to avoid a false positive).
func (u *unusedScanner) scanExpr(e parser.Node) {
	if e == nil {
		return
	}
	switch n := e.(type) {
	case *parser.Identifier:
		if n.Name != "" {
			u.reads[n.Name] = true
		}
	case *parser.CallExpr:
		if n.Function != "" {
			u.reads[n.Function] = true // may be a variable holding a function
		}
		for _, a := range n.Args {
			u.scanExpr(a)
		}
	case *parser.BinaryExpr:
		if n.Operator == "=" {
			// The bare variable name as assignment target is a write, not a
			// read — but the index/key/base of a compound target still uses it.
			u.scanWriteTarget(n.Left)
			u.scanExpr(n.Right)
		} else {
			u.scanExpr(n.Left)
			u.scanExpr(n.Right)
		}
	case *parser.DotExpr:
		// obj.field — the right side is a field name, not a value read.
		u.scanExpr(n.Left)
	case *parser.IndexExpr:
		u.scanExpr(n.Left)
		u.scanExpr(n.Index)
	case *parser.SliceExpr:
		u.scanExpr(n.Target)
		u.scanExpr(n.Start)
		u.scanExpr(n.End)
	case *parser.ArrayLiteral:
		for _, x := range n.Elements {
			u.scanExpr(x)
		}
	case *parser.MapLiteral:
		// Keys are conservatively treated as labels; only values are reads.
		for _, v := range n.Values {
			u.scanExpr(v)
		}
	case *parser.StructLiteral:
		for _, f := range n.Fields {
			if be, ok := f.(*parser.BinaryExpr); ok {
				u.scanExpr(be.Right) // field key is Left
			} else {
				u.scanExpr(f)
			}
		}
	case *parser.UnaryExpr:
		u.scanExpr(n.Operand)
	case *parser.AddressOf:
		u.scanExpr(n.Operand)
	case *parser.Dereference:
		u.scanExpr(n.Operand)
	case *parser.AllocExpr:
		u.scanExpr(n.Count)
	case *parser.FreeExpr:
		u.scanExpr(n.Ptr)
	case *parser.RawAccessExpr:
		u.scanExpr(n.Address)
		u.scanExpr(n.Value)
	case *parser.BorrowExpr:
		u.scanExpr(n.Operand)
	case *parser.MoveExpr:
		u.scanExpr(n.Operand)
	case *parser.PropagateExpr:
		u.scanExpr(n.Operand)
	case *parser.MatrixIndexExpr:
		u.scanExpr(n.Matrix)
		u.scanExpr(n.Row)
		u.scanExpr(n.Col)
	case *parser.SIMDBuiltinExpr:
		for _, a := range n.Args {
			u.scanExpr(a)
		}
	case *parser.OptionSomeExpr:
		u.scanExpr(n.Value)
	case *parser.ResultOkExpr:
		u.scanExpr(n.Value)
	case *parser.ResultErrExpr:
		u.scanExpr(n.Error)
	case *parser.MatchExpr:
		u.scanExpr(n.Value)
		for _, arm := range n.Arms {
			u.scanExpr(arm.Body)
		}
	// Literals and no-read leaves.
	case *parser.StringLiteral, *parser.IntLiteral, *parser.Float64Literal,
		*parser.BigIntLiteral, *parser.BigFloatLiteral, *parser.BoolLiteral,
		*parser.OptionNoneExpr:
		// no reads
	default:
		u.unsupported = true
	}
}