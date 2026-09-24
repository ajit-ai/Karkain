package sema

import (
	"fmt"

	"karkain/pkg/parser"
)

// Phase 146A: MonomorphizeProgram implements the Go front end's explicit-
// type-argument generics (Generics v1):
//
//   - Decl: `func name[T, U](params)`, `type Name[T] struct {...}`.
//     Constraints parse and store but are UNCHECKED in v1.
//   - Call: `f[int](args)`, `Point[int]{...}`. No inference in v1.
//
// The pass collects generic templates, validates call-site arity
// (mismatch is a resolve-class diagnostic, exit 3 — kcc reports the same
// shape as error[K115]), instantiates specializations via Monomorphizer
// (`Name_arg` mangling), appends them as plain units, and rewrites call
// sites to the specialized names. It runs to a fixpoint over appended
// specializations so recursive/nested generic calls instantiate too.
//
// A `CallExpr` carrying TypeArgs whose head is NOT a generic template is
// demoted back to `IndirectCallExpr(IndexExpr)` — the exact Phase-133
// index-then-call lowering — so `ops[idx](5)` programs keep working
// byte-identically. The parser can only produce the TypeArgs shape for
// bare-identifier heads with bare-identifier indices, so demotion only
// ever rebuilds that shape.
//
// The pass is idempotent: re-running finds no TypeArgs left and every
// specialization name already present, so nothing is appended twice.
// Generic templates themselves are never rewritten (their bodies keep
// the type parameters for future instantiation) and are skipped by
// codegen emission (plain-unit specializations emit instead).
func MonomorphizeProgram(prog *parser.Program) []ResolveError {
	if prog == nil {
		return nil
	}
	m := &monomorphCtx{
		prog:       prog,
		funcTmpl:   map[string]*parser.FuncDecl{},
		structTmpl: map[string]*parser.StructDeclStmt{},
		existing:   map[string]bool{},
		mono:       NewMonomorphizer(),
	}
	for _, stmt := range prog.Statements {
		switch n := stmt.(type) {
		case *parser.FuncDecl:
			m.existing[n.Name] = true
			if len(n.GenericParams) > 0 {
				if _, dup := m.funcTmpl[n.Name]; !dup {
					m.funcTmpl[n.Name] = n
				}
			}
		case *parser.StructDeclStmt:
			m.existing[n.Name] = true
			if len(n.GenericParams) > 0 {
				if _, dup := m.structTmpl[n.Name]; !dup {
					m.structTmpl[n.Name] = n
				}
			}
		}
	}
	// Fixpoint worklist: appended specializations extend prog.Statements
	// and are walked in turn (they carry no GenericParams, so their
	// nested generic calls instantiate). Templates are skipped — their
	// bodies keep type parameters for future instantiation.
	for i := 0; i < len(prog.Statements); i++ {
		if fn, ok := prog.Statements[i].(*parser.FuncDecl); ok && len(fn.GenericParams) > 0 {
			continue
		}
		if st, ok := prog.Statements[i].(*parser.StructDeclStmt); ok && len(st.GenericParams) > 0 {
			continue
		}
		prog.Statements[i] = m.rewriteTop(prog.Statements[i])
	}
	return m.errors
}

type monomorphCtx struct {
	prog       *parser.Program
	funcTmpl   map[string]*parser.FuncDecl
	structTmpl map[string]*parser.StructDeclStmt
	existing   map[string]bool
	mono       *Monomorphizer
	errors     []ResolveError
}

func (m *monomorphCtx) errf(line, col, endCol int, format string, args ...any) {
	m.errors = append(m.errors, ResolveError{
		Line:   line,
		Col:    col,
		EndCol: endCol,
		Msg:    fmt.Sprintf(format, args...),
	})
}

// rewriteTop rewrites a single top-level statement's contained
// expressions (function bodies, top-level values). Template declarations
// never reach here (skipped by the worklist driver).
func (m *monomorphCtx) rewriteTop(stmt parser.Node) parser.Node {
	if fn, ok := stmt.(*parser.FuncDecl); ok {
		fn.Body = m.rewriteStmts(fn.Body)
		return fn
	}
	return m.rewriteNode(stmt)
}

func (m *monomorphCtx) rewriteStmts(stmts []parser.Node) []parser.Node {
	for i, s := range stmts {
		stmts[i] = m.rewriteNode(s)
	}
	return stmts
}

func (m *monomorphCtx) rewriteNode(n parser.Node) parser.Node {
	switch t := n.(type) {
	case *parser.ExprStmt:
		t.Expression = m.rewriteNode(t.Expression)
		return t
	case *parser.VarDeclStmt:
		if t.Value != nil {
			t.Value = m.rewriteNode(t.Value)
		}
		return t
	case *parser.ReturnStmt:
		if t.Value != nil {
			t.Value = m.rewriteNode(t.Value)
		}
		return t
	case *parser.IfStmt:
		t.Condition = m.rewriteNode(t.Condition)
		t.Consequence = m.rewriteStmts(t.Consequence)
		t.Alternative = m.rewriteStmts(t.Alternative)
		return t
	case *parser.WhileStmt:
		t.Condition = m.rewriteNode(t.Condition)
		t.Body = m.rewriteStmts(t.Body)
		return t
	case *parser.ForStmt:
		if t.Init != nil {
			t.Init = m.rewriteNode(t.Init)
		}
		if t.Condition != nil {
			t.Condition = m.rewriteNode(t.Condition)
		}
		if t.Post != nil {
			t.Post = m.rewriteNode(t.Post)
		}
		t.Body = m.rewriteStmts(t.Body)
		return t
	case *parser.ForInStmt:
		t.Iter = m.rewriteNode(t.Iter)
		t.Body = m.rewriteStmts(t.Body)
		return t
	case *parser.PrintStmt:
		t.Value = m.rewriteNode(t.Value)
		return t
	case *parser.BlockStmt:
		t.Statements = m.rewriteStmts(t.Statements)
		return t
	case *parser.BinaryExpr:
		t.Left = m.rewriteNode(t.Left)
		t.Right = m.rewriteNode(t.Right)
		return t
	case *parser.UnaryExpr:
		t.Operand = m.rewriteNode(t.Operand)
		return t
	case *parser.IndexExpr:
		t.Left = m.rewriteNode(t.Left)
		t.Index = m.rewriteNode(t.Index)
		return t
	case *parser.MatrixIndexExpr:
		t.Matrix = m.rewriteNode(t.Matrix)
		t.Row = m.rewriteNode(t.Row)
		t.Col = m.rewriteNode(t.Col)
		return t
	case *parser.ArrayLiteral:
		for i, e := range t.Elements {
			t.Elements[i] = m.rewriteNode(e)
		}
		return t
	case *parser.MapLiteral:
		for i, k := range t.Keys {
			t.Keys[i] = m.rewriteNode(k)
		}
		for i, v := range t.Values {
			t.Values[i] = m.rewriteNode(v)
		}
		return t
	case *parser.DotExpr:
		t.Left = m.rewriteNode(t.Left)
		return t
	case *parser.CallExpr:
		return m.rewriteCall(t)
	case *parser.IndirectCallExpr:
		t.Target = m.rewriteNode(t.Target)
		for i, a := range t.Args {
			t.Args[i] = m.rewriteNode(a)
		}
		return t
	case *parser.StructLiteral:
		return m.rewriteStructLit(t)
	case *parser.FuncDecl:
		// Nested function values: rewrite their bodies unless generic.
		if len(t.GenericParams) == 0 {
			t.Body = m.rewriteStmts(t.Body)
		}
		return t
	default:
		return n
	}
}

// rewriteCall handles `f[args](callargs)` and bare calls to generic
// templates. Args always recurse first so nested generic calls inside
// argument position instantiate even on error paths.
func (m *monomorphCtx) rewriteCall(n *parser.CallExpr) parser.Node {
	for i, a := range n.Args {
		n.Args[i] = m.rewriteNode(a)
	}
	if len(n.TypeArgs) > 0 {
		return m.rewriteGenericCall(n)
	}
	if _, isGeneric := m.funcTmpl[n.Function]; isGeneric && n.Module == "" && !n.IsCFunc {
		m.errf(n.Line, n.Col, n.EndCol,
			"generic function '%s' requires explicit type arguments (e.g. %s[T](...))", n.Function, n.Function)
	}
	return n
}

func (m *monomorphCtx) rewriteGenericCall(n *parser.CallExpr) parser.Node {
	head := n.Function
	targs := n.TypeArgs
	tmpl, ok := m.funcTmpl[head]
	if !ok || n.Module != "" || n.IsCFunc {
		// Not a generic callee: demote to the Phase-133
		// index-then-call lowering this syntax parsed as before
		// Phase 146 (`ops[idx](5)` with an identifier index).
		return m.demoteCall(n)
	}
	if len(targs) != len(tmpl.GenericParams) {
		m.errf(n.Line, n.Col, n.EndCol,
			"generic function '%s' expects %d type argument(s), got %d", head, len(tmpl.GenericParams), len(targs))
		return n
	}
	subst := make(map[string]string, len(targs))
	for i, gp := range tmpl.GenericParams {
		subst[gp.Name] = targs[i]
	}
	spec, err := m.mono.InstantiateGenericFunc(tmpl, subst)
	if err != nil {
		m.errf(n.Line, n.Col, n.EndCol, "generic function '%s': %v", head, err)
		return n
	}
	m.emitFuncSpec(spec)
	return &parser.CallExpr{
		Function: spec.Name,
		Module:   n.Module,
		Args:     n.Args,
		IsCFunc:  n.IsCFunc,
		Line:     n.Line,
		Col:      n.Col,
		EndCol:   n.EndCol,
	}
}

// demoteCall rebuilds the legacy IndirectCallExpr(IndexExpr) lowering for
// a TypeArgs call whose head is not a generic template.
func (m *monomorphCtx) demoteCall(n *parser.CallExpr) parser.Node {
	head := &parser.Identifier{Name: n.Function, Line: n.Line, Col: n.Col, EndCol: n.EndCol}
	var target parser.Node
	switch len(n.TypeArgs) {
	case 1:
		target = &parser.IndexExpr{
			Left:  head,
			Index: &parser.Identifier{Name: n.TypeArgs[0], Line: n.Line},
			Line:  n.Line,
		}
	case 2:
		target = &parser.MatrixIndexExpr{
			Matrix: head,
			Row:    &parser.Identifier{Name: n.TypeArgs[0], Line: n.Line},
			Col:    &parser.Identifier{Name: n.TypeArgs[1], Line: n.Line},
			Line:   n.Line,
		}
	default:
		m.errf(n.Line, n.Col, n.EndCol,
			"generic function '%s' expects %d type argument(s), got %d", n.Function, len(n.TypeArgs), len(n.TypeArgs))
		return n
	}
	return &parser.IndirectCallExpr{
		Target: target,
		Args:   n.Args,
		Line:   n.Line,
		Col:    n.Col,
		EndCol: n.EndCol,
	}
}

func (m *monomorphCtx) rewriteStructLit(n *parser.StructLiteral) parser.Node {
	for i, f := range n.Fields {
		n.Fields[i] = m.rewriteNode(f)
	}
	if len(n.TypeArgs) == 0 {
		if _, isGeneric := m.structTmpl[n.TypeName]; isGeneric {
			m.errf(n.Line, 0, 0,
				"generic struct '%s' requires explicit type arguments (e.g. %s[T]{...})", n.TypeName, n.TypeName)
		}
		return n
	}
	tmpl, ok := m.structTmpl[n.TypeName]
	if !ok {
		m.errf(n.Line, 0, 0,
			"generic struct '%s' is not defined", n.TypeName)
		return n
	}
	if len(n.TypeArgs) != len(tmpl.GenericParams) {
		m.errf(n.Line, 0, 0,
			"generic struct '%s' expects %d type argument(s), got %d", n.TypeName, len(tmpl.GenericParams), len(n.TypeArgs))
		return n
	}
	subst := make(map[string]string, len(n.TypeArgs))
	for i, gp := range tmpl.GenericParams {
		subst[gp.Name] = n.TypeArgs[i]
	}
	spec, err := m.mono.InstantiateGenericStruct(tmpl, subst)
	if err != nil {
		m.errf(n.Line, 0, 0, "generic struct '%s': %v", n.TypeName, err)
		return n
	}
	m.emitStructSpec(spec)
	return &parser.StructLiteral{
		TypeName: spec.Name,
		Fields:   n.Fields,
		Line:     n.Line,
	}
}

func (m *monomorphCtx) emitFuncSpec(spec *parser.FuncDecl) {
	if m.existing[spec.Name] {
		return
	}
	m.existing[spec.Name] = true
	m.prog.Statements = append(m.prog.Statements, spec)
}

func (m *monomorphCtx) emitStructSpec(spec *parser.StructDeclStmt) {
	if m.existing[spec.Name] {
		return
	}
	m.existing[spec.Name] = true
	m.prog.Statements = append(m.prog.Statements, spec)
}
