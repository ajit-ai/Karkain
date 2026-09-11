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
	consts  map[string]bool // Phase 112: module-level `const` declarations
	fnConsts map[string]map[string]bool // Phase 112: per-function local const names
	errors  []ResolveError
	// visibility tracking (populated when any Public declaration is present)
	anyPublic        bool
	declFile         map[string]string // name → source file path
	publicFuncs      map[string]bool   // name → true if declared public
	callerFile       map[string]string // caller func name → source file path
	callerBodyOffset map[string]int    // caller func name → line of its body start
	// module tracking (Phase 103)
	modules  map[string]string // module name (file basename) → source file path
	imported map[string]bool   // declared `import <name>` names
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
	"system": true, "openFile": true, "readLine": true, "readLineEOF": true, "listFiles": true,
	"closeFile": true, "createFile": true, "writeToFile": true, "removeFile": true,
	"trim": true, "contains": true, "split": true,
	// Phase 109: stdlib v2 runtime byte helpers (encoding / crypto / map iteration).
	"hex_encode_bytes": true, "hex_decode_bytes": true,
	"base64_encode_bytes": true, "base64_decode_bytes": true,
	"utf8_valid_bytes": true, "sha256_hex": true, "sha512_hex": true,
	"map_keys_of": true,
	"sqrt": true, "abs": true, "pow": true, "mod": true,
	"float": true, // Phase 112: float() conversion builtin
	"add_checked": true, "sub_checked": true, "mul_checked": true,
	"http.get": true,
	"Some":     true, "None": true, "Ok": true, "Err": true,
	"assert": true, "assert_eq": true, "assert_ne": true,
	// Phase 107: concurrency builtins (channels, tasks, actors).
	"channel": true, "send": true, "chanSend": true, "chanClose": true,
	"join": true, "wait_all": true,
	"actor": true, "actorSend": true, "actorState": true,
	"setActorState": true, "actorStop": true,
}

// NewResolver creates a Resolver over the given program. If sourceMap is
// non-nil, the resolver tracks source-file boundaries and can enforce visibility
// (public/private) when any declaration carries Public=true. It also validates
// that each module import references a source file present in the compile unit.
func NewResolver(prog *parser.Program, sm SourceMap) *Resolver {
	r := &Resolver{
		funcs:       make(map[string]bool),
		types:       make(map[string]bool),
		builtin:     builtinNames,
		consts:      make(map[string]bool),
		fnConsts:    make(map[string]map[string]bool),
		declFile:    make(map[string]string),
		publicFuncs: make(map[string]bool),
		callerFile:  make(map[string]string),
		modules:    make(map[string]string),
		imported:   make(map[string]bool),
	}
	for _, imp := range prog.Imports {
		r.imported[imp.Name] = true
		r.imported[moduleBasename(imp.Name)] = true
	}
	if sm != nil {
		r.indexModules(sm)
	}
	r.collectDefs(prog.Statements, sm)
	if sm != nil && len(prog.Imports) > 0 {
		r.validateImports(prog.Imports, sm)
	}
	return r
}

// moduleBasename returns the module name a dotted import resolves to: the
// final path segment (import std.string → "string"). Plain names map to
// themselves.
func moduleBasename(name string) string {
	if i := strings.LastIndex(name, "."); i >= 0 {
		return name[i+1:]
	}
	return name
}

// moduleFile resolves a qualified module name to its source file path.
// Dotted std.* imports map onto the file whose basename is the last segment.
func (r *Resolver) moduleFile(module string) (string, bool) {
	if f, ok := r.modules[module]; ok {
		return f, true
	}
	if strings.HasPrefix(module, "std.") {
		if f, ok := r.modules[module[len("std."):]]; ok {
			return f, true
		}
	}
	return "", false
}

// validateImports checks that each import <name> matches a source file in the
// compile unit (a file whose basename, minus the .kark extension, equals name).
// std.* imports resolve to the file whose basename is the last path segment.
func (r *Resolver) validateImports(imports []*parser.ModuleImport, sm SourceMap) {
	for _, imp := range imports {
		if _, ok := r.moduleFile(imp.Name); !ok {
			r.errors = append(r.errors, ResolveError{
				Line: imp.Line,
				Msg:  fmt.Sprintf("module '%s' not found in compile unit", imp.Name),
			})
		}
	}
}

// indexModules maps each compile-unit source file's basename to its path so
// qualified calls and imports can resolve module names to files.
func (r *Resolver) indexModules(sm SourceMap) {
	for _, fpath := range sm {
		base := filepath.Base(fpath)
		name := strings.TrimSuffix(base, ".kark")
		if name != "" && name != base {
			r.modules[name] = fpath
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
		case *parser.VarDeclStmt:
			// Phase 112: track module-level const declarations
			if n.Const && n.Name != "" {
				r.consts[n.Name] = true
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
			r.fnConsts[fn.Name] = r.localConsts(fn)
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

// walkLocalConsts collects VarDeclStmt CONST names in a body (Phase 112).
// Mirrors walkLocalDefs' scoping so const reassignment can be rejected.
func (r *Resolver) walkLocalConsts(body []parser.Node, m map[string]bool) {
	for _, s := range body {
		switch n := s.(type) {
		case *parser.VarDeclStmt:
			if n.Const && n.Name != "" {
				m[n.Name] = true
			}
		case *parser.BlockStmt:
			r.walkLocalConsts(n.Statements, m)
		case *parser.IfStmt:
			r.walkLocalConsts(n.Consequence, m)
			r.walkLocalConsts(n.Alternative, m)
		case *parser.WhileStmt:
			r.walkLocalConsts(n.Body, m)
		case *parser.ForInStmt:
			r.walkLocalConsts(n.Body, m)
		}
	}
}

// localConsts returns the set of const names declared inside a function.
func (r *Resolver) localConsts(fn *parser.FuncDecl) map[string]bool {
	m := make(map[string]bool)
	r.walkLocalConsts(fn.Body, m)
	return m
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
				// Phase 112: reject reassignment to const names — module-level
				// (not shadowed by a local) or declared `const` in this function.
				isLocalConst := false
				if fc := r.fnConsts[callerName]; fc != nil && fc[id.Name] {
					isLocalConst = true
				}
				if !locals[id.Name] {
					if _, isConst := r.consts[id.Name]; isConst {
						r.errors = append(r.errors, ResolveError{
							Line: id.Line, Col: id.Col, EndCol: id.EndCol,
							Msg:  fmt.Sprintf("cannot reassign constant '%s'", id.Name),
						})
					}
				} else if isLocalConst {
					r.errors = append(r.errors, ResolveError{
						Line: id.Line, Col: id.Col, EndCol: id.EndCol,
						Msg:  fmt.Sprintf("cannot reassign constant '%s'", id.Name),
					})
				}
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
	// Module-qualified calls (mod.fn) resolve against the module's export set
	// (Phase 103); method/DotExpr-style and C calls are outside this model.
	if n.Module != "" {
		r.checkQualifiedCall(n, locals, callerName, sm)
		return
	}
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

// checkQualifiedCall validates a module-qualified call `mod.fn(...)` against
// the module's export set (Phase 103). The qualifier is a module when it was
// declared via `import`; otherwise it may be a record-idiom method receiver
// (a local/param) and is skipped like the pre-Phase-103 dotted-call model.
// Requirements for a true module call:
//
//   - the module is imported and resolves to a file in the compile unit
//   - the function exists in that module's file
//   - the function is declared public (so other modules may call it)
func (r *Resolver) checkQualifiedCall(n *parser.CallExpr, locals map[string]bool, callerName string, sm SourceMap) {
	name := n.Function
	module := n.Module
	mod := moduleBasename(module)
	imported := r.imported[module] || r.imported[mod]
	if !imported {
		if locals[module] {
			// Record-idiom method call on a local/param receiver: outside the
			// module model (legacy dotted-call skip).
			r.recurseArgs(n, locals, callerName, sm)
			return
		}
		if _, ok := r.moduleFile(module); ok {
			r.errors = append(r.errors, ResolveError{
				Line:   n.Line,
				Col:    n.Col,
				EndCol: n.EndCol,
				Msg:    fmt.Sprintf("module '%s' is not imported; add 'import %s'", module, module),
			})
		}
		r.recurseArgs(n, locals, callerName, sm)
		return
	}
	file, known := r.moduleFile(module)
	if !known {
		r.errors = append(r.errors, ResolveError{
			Line:   n.Line,
			Col:    n.Col,
			EndCol: n.EndCol,
			Msg:    fmt.Sprintf("module '%s' is not part of the compile unit", module),
		})
		r.recurseArgs(n, locals, callerName, sm)
		return
	}
	if !r.funcs[name] {
		r.errors = append(r.errors, ResolveError{
			Line:   n.Line,
			Col:    n.Col,
			EndCol: n.EndCol,
			Msg:    fmt.Sprintf("function '%s' is not defined", name),
		})
		r.recurseArgs(n, locals, callerName, sm)
		return
	}
	decl := r.declFile[name]
	if decl != "" && decl != file {
		r.errors = append(r.errors, ResolveError{
			Line:   n.Line,
			Col:    n.Col,
			EndCol: n.EndCol,
			Msg:    fmt.Sprintf("function '%s' is not exported by module '%s' (defined in '%s')", name, module, decl),
		})
		r.recurseArgs(n, locals, callerName, sm)
		return
	}
	// Stdlib modules are exempt from the private-export rule: their functions
	// are framework-level API surface, and physical `public` markers land with
	// stdlib v2 (Phase 109 boundary). Any other module's non-public function is
	// unit-internal and cannot be called across modules.
	if !r.publicFuncs[name] && !isStdlibFile(file) {
		r.errors = append(r.errors, ResolveError{
			Line:   n.Line,
			Col:    n.Col,
			EndCol: n.EndCol,
			Msg:    fmt.Sprintf("function '%s' in module '%s' is private and cannot be called by another module", name, module),
		})
		r.recurseArgs(n, locals, callerName, sm)
		return
	}
	r.recurseArgs(n, locals, callerName, sm)
}

// isStdlibFile reports whether a source file belongs to the stdlib tree
// (stdlib/<module>/<module>.kark), whose exports are self-owned and not
// subject to the user-module public/private rule.
func isStdlibFile(path string) bool {
	if path == "" {
		return false
	}
	return strings.Contains(path, string(filepath.Separator)+"stdlib"+string(filepath.Separator)) ||
		strings.Contains(path, "/stdlib/")
}

// recurseArgs continues the walk into a call's argument expressions.
func (r *Resolver) recurseArgs(n *parser.CallExpr, locals map[string]bool, callerName string, sm SourceMap) {
	for _, a := range n.Args {
		r.checkExpr(a, locals, callerName, sm)
	}
}
