package sema

import (
	"fmt"
	"path/filepath"
	"strings"

	"karkain/pkg/parser"
)

// ResolveError is a single whole-program name-resolution diagnostic.
type ResolveError struct {
	Line   int // 1-based line
	Col    int // 0-based byte column of the offending token (best-effort)
	EndCol int // 0-based byte column just past the offending token (best-effort)
	Msg    string
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
	publicFuncs      map[string]bool   // name → true if declared public
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
	"Some":     true, "None": true, "Ok": true, "Err": true,
	"assert": true, "assert_eq": true, "assert_ne": true,
}

// NewResolver creates a Resolver over the given program. If sourceMap is
// non-nil, the resolver tracks source-file boundaries and can enforce visibility
// (public/private) when any declaration carries Public=true. It also validates
// that each module import references a source file present in the compile unit.
func NewResolver(prog *parser.Program, sm SourceMap) *Resolver {
	r := &Resolver{
		funcs:      make(map[string]bool),
		types:      make(map[string]bool),
		builtin:    builtinNames,
		declFile:   make(map[string]string),
		publicFuncs: make(map[string]bool),
		callerFile: make(map[string]string),
	}
	r.collectDefs(prog.Statements, sm)
	if sm != nil && len(prog.Imports) > 0 {
		r.validateImports(prog.Imports, sm)
	}
	return r
}

// validateImports checks that each import <name> matches a source file in the
// compile unit (a file whose basename, minus the .kark extension, equals name).
func (r *Resolver) validateImports(imports []*parser.ModuleImport, sm SourceMap) {
	// Build a set of known module names from file paths in the SourceMap.
	knownModules := make(map[string]bool)
	for _, fpath := range sm {
		base := filepath.Base(fpath)
		name := strings.TrimSuffix(base, ".kark")
		if name != "" && name != base {
			knownModules[name] = true
		}
	}
	for _, imp := range imports {
		if !knownModules[imp.Name] {
			r.errors = append(r.errors, ResolveError{
				Line: imp.Line,
				Msg:  fmt.Sprintf("module '%s' not found in compile unit", imp.Name),
			})
		}
	}
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
	publicFuncs := make(map[string]bool)
	anyPublic := false
	for _, s := range stmts {
		switch n := s.(type) {
		case *parser.FuncDecl:
			if n.Name == "" {
				continue
			}
			if seen[n.Name] {
				r.errors = append(r.errors, ResolveError{
					Line:   n.Line,
					Col:    n.Col,
					EndCol: n.EndCol,
					Msg:    fmt.Sprintf("duplicate function definition '%s'", n.Name),
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
				publicFuncs[n.Name] = true
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
	r.publicFuncs = publicFuncs
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
		// A bare assignment `x = v` implicitly declares x (Phase 51 semantics):
		// register the target before checking the value so reads of x resolve and
		// the assignment target itself is not misreported as an undefined read.
		if be, ok := n.Expression.(*parser.BinaryExpr); ok && be.Operator == "=" {
			if id, ok := be.Left.(*parser.Identifier); ok && locals != nil && id.Name != "" {
				locals[id.Name] = true
			}
		}
		r.checkExpr(n.Expression, locals, callerName, sm)
	case *parser.VarDeclStmt:
		if locals != nil && n.Name != "" {
			locals[n.Name] = true
		}
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
		r.checkExpr(n.Iter, locals, callerName, sm)
		bodyLocals := locals
		if locals != nil && (n.VarName != "" || n.KeyName != "") {
			bodyLocals = make(map[string]bool, len(locals)+2)
			for k := range locals {
				bodyLocals[k] = true
			}
			if n.VarName != "" {
				bodyLocals[n.VarName] = true
			}
			if n.KeyName != "" {
				bodyLocals[n.KeyName] = true
			}
		}
		for _, x := range n.Body {
			r.checkStmt(x, bodyLocals, callerName, sm)
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
	case *parser.Identifier:
		r.checkUndefinedName(n, locals, sm)
	case *parser.CallExpr:
		r.checkCallExpr(n, locals, callerName, sm)
	case *parser.BinaryExpr:
		// Struct-literal fields arrive as BinaryExpr "=" nodes (field key Left,
		// value Right); those are handled by the StructLiteral case, never here.
		r.checkExpr(n.Left, locals, callerName, sm)
		r.checkExpr(n.Right, locals, callerName, sm)
	case *parser.DotExpr:
		// obj.field — the right side is a field name, not a value read.
		r.checkExpr(n.Left, locals, callerName, sm)
	case *parser.ArrayLiteral:
		for _, x := range n.Elements {
			r.checkExpr(x, locals, callerName, sm)
		}
	case *parser.MapLiteral:
		// Keys are conservatively treated as labels (map keys are frequently
		// identifier literals); only values are value-position reads.
		for _, v := range n.Values {
			r.checkExpr(v, locals, callerName, sm)
		}
	case *parser.StructLiteral:
		// Fields arrive as BinaryExpr "="; the left side is a field key.
		for _, f := range n.Fields {
			if be, ok := f.(*parser.BinaryExpr); ok {
				r.checkExpr(be.Right, locals, callerName, sm)
			} else {
				r.checkExpr(f, locals, callerName, sm)
			}
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
		// @raw(addr) addresses are raw C-level addresses, not Karkain values:
		// skip the address, check only the optional write value.
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

// checkUndefinedName flags a bare identifier used in value position that is not
// a local (var/param/loop var/implicit assignment target), not a defined
// function or type, and not a language builtin. It only fires inside function
// bodies (locals != nil); top-level statements never reach this walk, which
// keeps module-global patterns free of false positives.
func (r *Resolver) checkUndefinedName(n *parser.Identifier, locals map[string]bool, sm SourceMap) {
	if n == nil || n.Name == "" || locals == nil {
		return
	}
	name := n.Name
	if locals[name] || r.funcs[name] || r.types[name] || r.builtin[name] {
		return
	}
	// Guard the few keyword-shaped literals that the lexer may surface as
	// identifiers in some contexts.
	switch name {
	case "true", "false", "nil", "THIS", "self":
		return
	}
	r.errors = append(r.errors, ResolveError{
		Line:   n.Line,
		Col:    n.Col,
		EndCol: n.EndCol,
		Msg:    fmt.Sprintf("undefined identifier '%s'", name),
	})
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
		// Public names are callable from any file.
		if r.anyPublic && sm != nil && r.funcs[name] && !r.publicFuncs[name] {
			callerFile := r.callerFile[callerName]
			declFile := r.declFile[name]
			if callerFile != "" && declFile != "" && callerFile != declFile {
				r.errors = append(r.errors, ResolveError{
					Line:   n.Line,
					Col:    n.Col,
					EndCol: n.EndCol,
					Msg:    fmt.Sprintf("private function '%s' is not accessible from file '%s' (defined in '%s')", name, callerFile, declFile),
				})
			}
		}
		return
	}
	// Unknown bare function reference.
	r.errors = append(r.errors, ResolveError{
		Line:   n.Line,
		Col:    n.Col,
		EndCol: n.EndCol,
		Msg:    fmt.Sprintf("undefined function '%s'", name),
	})
	// Still recurse into args to find nested undefined calls.
	for _, a := range n.Args {
		r.checkExpr(a, locals, callerName, sm)
	}
}
