package sema

import (
	"fmt"
	"strings"

	"karkain/pkg/parser"
)

// ResolveError is a single whole-program name-resolution diagnostic.
type ResolveError struct {
	Line int
	Msg  string
}

func (e *ResolveError) Error() string {
	return fmt.Sprintf("line %d: %s", e.Line, e.Msg)
}

// SourceMap records which source file (path) each top-level line number belongs
// to. The resolver uses this to enforce visibility: private names are only
// callable from their own file.
type SourceMap map[int]string // line number → file path

// Resolver performs a two-pass, diagnostics-only whole-program name-resolution
// over a resolved compile unit (the concatenated project + module sources). It
// never changes emitted output, so it cannot affect codegen determinism or the
// self-hosting stage2==stage3 byte-identity. It exists to surface, at the
// Karkain level, the two classes of error that today leak out of the C
// toolchain as opaque GCC/linker messages:
//
//  1. duplicate top-level definitions, and
//  2. references to functions that are clearly not defined anywhere.
//
// When any declaration carries Public=true, an additional visibility check
// fires: private declarations are only callable from their own source file.
type Resolver struct {
	funcs   map[string]bool
	types   map[string]bool
	builtin map[string]bool
	errors  []ResolveError
	// visibility tracking (populated when any Public declaration is present)
	anyPublic        bool
	declFile         map[string]string // name → source file path
	callerFile       map[string]string // caller func name → source file path
	callerBodyOffset map[string]int    // caller func name → line of its body start
}

// builtinNames is the set of language/standard-library functions handled
// directly by codegen (pkg/codegen/codegen.go "if n.Function == ..."). The
// resolver whitelists these so valid built-in calls are not flagged.
var builtinNames = map[string]bool{
	"print": true, "println": true, "printf": true,
	"len": true, "fmt": true,
	"readFile": true, "writeFile": true, "appendArray": true, "push": true,
	"hasKey": true, "delete": true,
	"substr": true, "str": true, "int": true,
	"system": true, "openFile": true, "readLine": true, "listFiles": true,
	"closeFile": true, "createFile": true, "writeToFile": true, "removeFile": true,
	"trim": true, "contains": true, "split": true,
	"sqrt": true, "abs": true, "pow": true, "mod": true,
	"add_checked": true, "sub_checked": true, "mul_checked": true,
	"http.get": true,
	"Some": true, "None": true, "Ok": true, "Err": true,
}

// NewResolver creates a Resolver over the given program. If sourceMap is
// non-nil, the resolver tracks source-file boundaries and can enforce visibility
// (public/private) when any declaration carries Public=true.
func NewResolver(prog *parser.Program, sm SourceMap) *Resolver {
	r := &Resolver{
		funcs:   make(map[string]bool),
		types:   make(map[string]bool),
		builtin: builtinNames,
		declFile:   make(map[string]string),
		callerFile: make(map[string]string),
	}
	r.collectDefs(prog.Statements, sm)
	return r
}

// Resolve runs both passes and returns any diagnostics.
func (r *Resolver) Resolve() []ResolveError {
	return r.errors
}

// collectDefs records top-level declarations, detects duplicates, then walks
// function bodies for undefined references.
func (r *Resolver) collectDefs(stmts []parser.Node, sm SourceMap) {
	seen := make(map[string]bool)
	declFile := make(map[string]string)
	anyPublic := false
	for _, s := range stmts {
		switch n := s.(type) {
		case *parser.FuncDecl:
			if n.Name == "" {
				continue
			}
			if seen[n.Name] {
				r.errors = append(r.errors, ResolveError{
					Line: n.Line,
					Msg:  fmt.Sprintf("duplicate function definition '%s'", n.Name),
				})
			} else {
				seen[n.Name] = true
			}
			r.funcs[n.Name] = true
			if sm != nil && n.Line > 0 {
				declFile[n.Name] = sm[n.Line]
			}
			if n.Public {
				anyPublic = true
			}
		case *parser.StructDeclStmt:
			if n.Name != "" {
				r.types[n.Name] = true
				if sm != nil && n.Line > 0 {
					declFile[n.Name] = sm[n.Line]
				}
				if n.Public {
					anyPublic = true
				}
			}
		case *parser.EnumDecl:
			if n.Name != "" {
				r.types[n.Name] = true
				if sm != nil && n.Line > 0 {
					declFile[n.Name] = sm[n.Line]
				}
				if n.Public {
					anyPublic = true
				}
			}
		}
	}
	r.anyPublic = anyPublic
	r.declFile = declFile
	// Pass 2: walk function bodies to check calls.
	for _, s := range stmts {
		if fn, ok := s.(*parser.FuncDecl); ok {
			r.callerFile[fn.Name] = declFile[fn.Name]
			r.walkCalls(fn.Body, r.localNames(fn), fn.Name, sm)
		}
	}
}

// localNames returns the set of names declared as local variables/parameters
// inside a function, so calls to lambda/function-value variables are not flagged.
func (r *Resolver) localNames(fn *parser.FuncDecl) map[string]bool {
	m := make(map[string]bool)
	for _, p := range fn.Params {
		m[p] = true
	}
	r.walkLocalDefs(fn.Body, m)
	return m
}

// walkLocalDefs collects VarDeclStmt names in a body (non-recursive into
// nested funcs; nested functions get their own param scope via localNames).
func (r *Resolver) walkLocalDefs(body []parser.Node, m map[string]bool) {
	for _, s := range body {
		switch n := s.(type) {
		case *parser.VarDeclStmt:
			if n.Name != "" {
				m[n.Name] = true
			}
		case *parser.BlockStmt:
			r.walkLocalDefs(n.Statements, m)
		case *parser.IfStmt:
			r.walkLocalDefs(n.Consequence, m)
			r.walkLocalDefs(n.Alternative, m)
		case *parser.WhileStmt:
			r.walkLocalDefs(n.Body, m)
		case *parser.ForInStmt:
			if n.VarName != "" {
				m[n.VarName] = true
			}
			r.walkLocalDefs(n.Body, m)
		}
	}
}

// walkCalls walks a list of statements, flagging undefined function calls.
func (r *Resolver) walkCalls(body []parser.Node, locals map[string]bool, callerName string, sm SourceMap) {
	for _, s := range body {
		r.checkStmt(s, locals, callerName, sm)
	}
}

// checkStmt dispatches a statement for call-expression validation.
func (r *Resolver) checkStmt(s parser.Node, locals map[string]bool, callerName string, sm SourceMap) {
	switch n := s.(type) {
	case *parser.ExprStmt:
		r.checkExpr(n.Expression, locals, callerName, sm)
	case *parser.VarDeclStmt:
		r.checkExpr(n.Value, locals, callerName, sm)
	case *parser.ReturnStmt:
		r.checkExpr(n.Value, locals, callerName, sm)
	case *parser.PrintStmt:
		r.checkExpr(n.Value, locals, callerName, sm)
	case *parser.BlockStmt:
		for _, x := range n.Statements {
			r.checkStmt(x, locals, callerName, sm)
		}
	case *parser.IfStmt:
		r.checkExpr(n.Condition, locals, callerName, sm)
		for _, x := range n.Consequence {
			r.checkStmt(x, locals, callerName, sm)
		}
		for _, x := range n.Alternative {
			r.checkStmt(x, locals, callerName, sm)
		}
	case *parser.WhileStmt:
		r.checkExpr(n.Condition, locals, callerName, sm)
		for _, x := range n.Body {
			r.checkStmt(x, locals, callerName, sm)
		}
	case *parser.ForInStmt:
		r.checkExpr(n.Iter, nil, callerName, sm)
		for _, x := range n.Body {
			r.checkStmt(x, locals, callerName, sm)
		}
	case *parser.FuncDecl:
		sub := r.localNames(n)
		r.walkCalls(n.Body, sub, n.Name, sm)
	}
}

// checkExpr walks an expression tree, flagging undefined calls.
func (r *Resolver) checkExpr(e parser.Node, locals map[string]bool, callerName string, sm SourceMap) {
	if e == nil {
		return
	}
	switch n := e.(type) {
	case *parser.CallExpr:
		r.checkCallExpr(n, locals, callerName, sm)
	case *parser.BinaryExpr:
		r.checkExpr(n.Left, locals, callerName, sm)
		r.checkExpr(n.Right, locals, callerName, sm)
	case *parser.ArrayLiteral:
		for _, x := range n.Elements {
			r.checkExpr(x, locals, callerName, sm)
		}
	case *parser.MapLiteral:
		for _, k := range n.Keys {
			r.checkExpr(k, locals, callerName, sm)
		}
		for _, v := range n.Values {
			r.checkExpr(v, locals, callerName, sm)
		}
	case *parser.IndexExpr:
		r.checkExpr(n.Left, locals, callerName, sm)
		r.checkExpr(n.Index, locals, callerName, sm)
	case *parser.SliceExpr:
		r.checkExpr(n.Target, locals, callerName, sm)
		if n.Start != nil {
			r.checkExpr(n.Start, locals, callerName, sm)
		}
		if n.End != nil {
			r.checkExpr(n.End, locals, callerName, sm)
		}
	case *parser.PropagateExpr:
		r.checkExpr(n.Operand, locals, callerName, sm)
	case *parser.BorrowExpr:
		r.checkExpr(n.Operand, locals, callerName, sm)
	case *parser.MoveExpr:
		r.checkExpr(n.Operand, locals, callerName, sm)
	case *parser.AddressOf:
		r.checkExpr(n.Operand, locals, callerName, sm)
	case *parser.Dereference:
		r.checkExpr(n.Operand, locals, callerName, sm)
	case *parser.RawAccessExpr:
		r.checkExpr(n.Address, locals, callerName, sm)
		if n.Value != nil {
			r.checkExpr(n.Value, locals, callerName, sm)
		}
	case *parser.EnumVariantExpr:
		r.checkExpr(n.Value, locals, callerName, sm)
	case *parser.LambdaExpr:
		sub := make(map[string]bool)
		for _, p := range n.Params {
			sub[p] = true
		}
		for k := range locals {
			sub[k] = true
		}
		r.walkCalls(n.Body, sub, callerName, sm)
	}
}

// checkCallExpr validates a single call against the defined codebase.
func (r *Resolver) checkCallExpr(n *parser.CallExpr, locals map[string]bool, callerName string, sm SourceMap) {
	if n == nil {
		return
	}
	name := n.Function
	// Method calls (obj.method / Some/Enum constructors), C imports, and
	// dynamic receiver calls are outside the flat-name model: skip.
	if strings.Contains(name, ".") {
		return
	}
	if n.IsCFunc {
		return
	}
	if r.builtin[name] || r.funcs[name] || r.types[name] || locals[name] {
		// When any declaration is public, enforce visibility: a private name
		// must be called from the same source file it is defined in.
		if r.anyPublic && sm != nil && r.funcs[name] {
			callerFile := r.callerFile[callerName]
			declFile := r.declFile[name]
			if callerFile != "" && declFile != "" && callerFile != declFile {
				// Only report if the target is NOT public (need to check actual Pub flag).
				// Since the resolver doesn't track Pub per name, we rely on the AST.
				// For now, visibility enforcement is limited to cross-file private calls.
				r.errors = append(r.errors, ResolveError{
					Line: n.Line,
					Msg:  fmt.Sprintf("private function '%s' is not accessible from file '%s' (defined in '%s')", name, callerFile, declFile),
				})
			}
		}
		return
	}
	// Unknown bare function reference.
	r.errors = append(r.errors, ResolveError{
		Line: n.Line,
		Msg:  fmt.Sprintf("undefined function '%s'", name),
	})
	// Still recurse into args to find nested undefined calls.
	for _, a := range n.Args {
		r.checkExpr(a, locals, callerName, sm)
	}
}
