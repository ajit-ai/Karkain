package sema

import (
	"path/filepath"
	"strings"
	"testing"

	"karkain/pkg/lexer"
	"karkain/pkg/parser"
)

func parseTestProg(t *testing.T, src string) *parser.Program {
	t.Helper()
	l := lexer.New(src)
	p := parser.New(l)
	prog := p.ParseProgram()
	if len(p.Errors) > 0 {
		t.Fatalf("parse errors: %v", p.Errors)
	}
	return prog
}

func errLines(r []ResolveError) string {
	s := ""
	for i, e := range r {
		if i > 0 {
			s += "\n"
		}
		s += e.Error()
	}
	return s
}

func hasErr(errs []ResolveError, msg string) bool {
	for _, e := range errs {
		if strings.Contains(e.Msg, msg) {
			return true
		}
	}
	return false
}

func TestResolver_DuplicateFuncDetected(t *testing.T) {
	prog := parseTestProg(t, "func foo() {\n  print(1)\n}\nfunc foo() {\n  print(2)\n}\nfunc main() {\n  foo()\n}\n")
	r := NewResolver(prog, nil)
	errs := r.Resolve()
	if len(errs) == 0 {
		t.Fatal("expected duplicate func error, got none")
	}
	if !hasErr(errs, "duplicate function definition 'foo'") {
		t.Errorf("expected duplicate error for 'foo': %s", errLines(errs))
	}
}

func TestResolver_NoDuplicateForDistinctFuncs(t *testing.T) {
	prog := parseTestProg(t, "func foo() {\n  print(1)\n}\nfunc bar() {\n  print(2)\n}\nfunc main() {\n  foo()\n  bar()\n}\n")
	r := NewResolver(prog, nil)
	errs := r.Resolve()
	for _, e := range errs {
		t.Errorf("unexpected error: %s", e.Msg)
	}
}

func TestResolver_UndefinedFuncDetected(t *testing.T) {
	prog := parseTestProg(t, "func main() {\n  bar()\n}\n")
	r := NewResolver(prog, nil)
	errs := r.Resolve()
	if len(errs) == 0 {
		t.Fatal("expected undefined func error, got none")
	}
	if !hasErr(errs, "undefined function 'bar'") {
		t.Errorf("expected undefined error for 'bar': %s", errLines(errs))
	}
}

func TestResolver_DefinedFuncNotFlagged(t *testing.T) {
	prog := parseTestProg(t, "func helper() {\n  print(1)\n}\nfunc main() {\n  helper()\n}\n")
	r := NewResolver(prog, nil)
	errs := r.Resolve()
	for _, e := range errs {
		if e.Msg == "undefined function 'helper'" {
			t.Errorf("helper is defined but was flagged as undefined: %s", e.Msg)
		}
	}
}

func TestResolver_BuiltinNotFlagged(t *testing.T) {
	prog := parseTestProg(t, "func main() {\n  let x = 5\n  print(x)\n}\n")
	r := NewResolver(prog, nil)
	errs := r.Resolve()
	for _, e := range errs {
		if e.Msg == "undefined function 'print'" {
			t.Errorf("builtin should not be flagged: %s", e.Msg)
		}
	}
}

func TestResolver_MethodCallNotFlagged(t *testing.T) {
	prog := parseTestProg(t, "func main() {\n  let s = \"hi\"\n  s.len()\n}\n")
	r := NewResolver(prog, nil)
	errs := r.Resolve()
	for _, e := range errs {
		t.Errorf("unexpected error: %s", e.Msg)
	}
}

func TestResolver_ForwardDeclValid(t *testing.T) {
	prog := parseTestProg(t, "func main() {\n  foo()\n}\nfunc foo() {\n  print(1)\n}\n")
	r := NewResolver(prog, nil)
	errs := r.Resolve()
	for _, e := range errs {
		if e.Msg == "undefined function 'foo'" {
			t.Errorf("forward declaration should not be flagged: %s", e.Msg)
		}
	}
}

func TestResolver_LocalVarNotFlaggedAsFunction(t *testing.T) {
	prog := parseTestProg(t, "func main() {\n  let myFunc = fn() {\n    print(1)\n  }\n  myFunc()\n}\n")
	r := NewResolver(prog, nil)
	errs := r.Resolve()
	for _, e := range errs {
		if e.Msg == "undefined function 'fn'" {
			t.Errorf("local lambda variable should not be flagged: %s", e.Msg)
		}
	}
}

func TestResolver_CrossFileValid(t *testing.T) {
	src := "func lib_add(a, b) {\n  return a + b\n}\nfunc helper_double(x) {\n  return x * 2\n}\nfunc main() {\n  print(helper_double(lib_add(10, 11)))\n}\n"
	prog := parseTestProg(t, src)
	r := NewResolver(prog, nil)
	errs := r.Resolve()
	for _, e := range errs {
		t.Errorf("cross-file call should not be flagged: %s", e.Msg)
	}
}

func TestResolver_DuplicateAcrossFilesDetected(t *testing.T) {
	src := "func lib_add(a, b) {\n  return a + b\n}\nfunc lib_add(a, b) {\n  return a + b\n}\nfunc main() {\n  print(lib_add(1, 2))\n}\n"
	prog := parseTestProg(t, src)
	r := NewResolver(prog, nil)
	errs := r.Resolve()
	if len(errs) == 0 {
		t.Fatal("expected duplicate error, got none")
	}
	if !hasErr(errs, "duplicate function definition 'lib_add'") {
		t.Errorf("expected duplicate error for 'lib_add': %s", errLines(errs))
	}
}

func TestResolver_VisibilityEnforcement_PrivateCrossFile(t *testing.T) {
	src := "public func exported() { print(0) }\nfunc private_helper() { print(1) }\nfunc main() { private_helper() }\n"
	prog := parseTestProg(t, src)
	// exported (line 1) and private_helper (line 2) in file_a; main (line 3) in file_b.
	sm := SourceMap{1: "file_a.kark", 2: "file_a.kark", 3: "file_b.kark"}
	r := NewResolver(prog, sm)
	errs := r.Resolve()
	if len(errs) == 0 {
		t.Fatal("expected visibility error for cross-file private call, got none")
	}
	if !hasErr(errs, "private function") {
		t.Errorf("expected 'private' in error, got: %s", errLines(errs))
	}
}

func TestResolver_VisibilityEnforcement_PublicCrossFile(t *testing.T) {
	src := "public func exported() { print(1) }\nfunc main() { exported() }\n"
	prog := parseTestProg(t, src)
	sm := SourceMap{1: "lib.kark", 2: "lib.kark", 3: "main.kark", 4: "main.kark"}
	r := NewResolver(prog, sm)
	errs := r.Resolve()
	for _, e := range errs {
		if hasErr([]ResolveError{e}, "private function") {
			t.Errorf("public function should not be flagged: %s", e.Msg)
		}
	}
}

func TestResolver_VisibilityNoEnforcement_NoPublic(t *testing.T) {
	src := "func helper() { print(1) }\nfunc main() { helper() }\n"
	prog := parseTestProg(t, src)
	sm := SourceMap{1: "lib.kark", 2: "lib.kark", 3: "main.kark", 4: "main.kark"}
	r := NewResolver(prog, sm)
	errs := r.Resolve()
	for _, e := range errs {
		if hasErr([]ResolveError{e}, "private function") {
			t.Errorf("no enforcement expected without any public declarations: %s", e.Msg)
		}
	}
}

func TestResolver_VisibilityEnforcement_SameFile(t *testing.T) {
	src := "func helper() { print(1) }\nfunc main() { helper() }\n"
	prog := parseTestProg(t, src)
	sm := SourceMap{1: "same.kark", 2: "same.kark", 3: "same.kark", 4: "same.kark"}
	r := NewResolver(prog, sm)
	errs := r.Resolve()
	for _, e := range errs {
		if hasErr([]ResolveError{e}, "private function") {
			t.Errorf("same-file call should not be flagged: %s", e.Msg)
		}
	}
}

func TestResolver_ImportValidation_Found(t *testing.T) {
	src := "import math\nfunc main() { print(1) }\n"
	prog := parseTestProg(t, src)
	sm := SourceMap{1: "math.kark", 2: "main.kark", 3: "main.kark"}
	r := NewResolver(prog, sm)
	errs := r.Resolve()
	for _, e := range errs {
		if hasErr([]ResolveError{e}, "module 'math' not found") {
			t.Errorf("import math should resolve against math.kark: %s", e.Msg)
		}
	}
}

func TestResolver_ImportValidation_NotFound(t *testing.T) {
	src := "import nonexistent\nfunc main() { print(1) }\n"
	prog := parseTestProg(t, src)
	sm := SourceMap{1: "main.kark", 2: "main.kark", 3: "main.kark"}
	r := NewResolver(prog, sm)
	errs := r.Resolve()
	if !hasErr(errs, "module 'nonexistent' not found") {
		t.Errorf("expected 'module not found' error, got: %s", errLines(errs))
	}
}

// TestResolver_DiagnosticLineSurvivesMacroExpansion locks the Phase 81
// diagnostics fix: ApplyMacroExpansion rebuilds AST nodes, so it must carry
// each node's Line through. Without that, an undefined-function error inside
// an expanded program was reported at line 0 instead of the real source line.
func TestResolver_DiagnosticLineSurvivesMacroExpansion(t *testing.T) {
	src := "func main() {\n  let a = 1\n  let b = 2\n  unknown_function(5)\n}\n"
	prog := parseTestProg(t, src)
	expanded := parser.ApplyMacroExpansion(prog)
	r := NewResolver(expanded, nil)
	errs := r.Resolve()
	if !hasErr(errs, "undefined function 'unknown_function'") {
		t.Fatalf("expected undefined-function error, got: %s", errLines(errs))
	}
	for _, e := range errs {
		if e.Line == 0 {
			t.Errorf("diagnostic line was lost during macro expansion: %s", e.Msg)
		}
		if e.Line != 4 {
			t.Errorf("expected line 4 for undefined function, got %d: %s", e.Line, e.Msg)
		}
	}
}

// TestResolver_UndefinedIdentifierDetected is Phase 83: a bare identifier used
// in value position inside a function body that is neither a local, parameter,
// defined function, type, nor builtin is a name-resolution error.
func TestResolver_UndefinedIdentifierDetected(t *testing.T) {
	src := "func main() {\n  let result = unknown_name + 5\n  print(result)\n}\n"
	prog := parseTestProg(t, src)
	r := NewResolver(prog, nil)
	errs := r.Resolve()
	if !hasErr(errs, "undefined identifier 'unknown_name'") {
		t.Fatalf("expected undefined-identifier error, got: %s", errLines(errs))
	}
	for _, e := range errs {
		if e.Line != 2 {
			t.Errorf("expected line 2, got %d", e.Line)
		}
		// unknown_name starts at byte 15 (0-based) of
		// "  let result = unknown_name + 5".
		if e.Col != 15 {
			t.Errorf("expected col 15, got %d", e.Col)
		}
		if e.EndCol != 27 {
			t.Errorf("expected endCol 27, got %d", e.EndCol)
		}
	}
}

// TestResolver_UndefinedIdentifierSkipsFalsePositives locks the Phase 83
// conservative guards: locals, assignment targets, struct field keys, method
// receivers, enum names and top-level globals must never be flagged.
func TestResolver_UndefinedIdentifierSkipsFalsePositives(t *testing.T) {
	src := "func helper() int {\n  return 5\n}\n" +
		"func main() {\n" +
		"  let x = 1\n" +
		"  x = helper()\n" +
		"  let d = Point{x: 1}\n" +
		"  let m = {a: x}\n" +
		"  let e = Some(x)\n" +
		"  let f = helper()\n" +
		"  let someArray = [1, 2, 3]\n" +
		"  for v in someArray {\n" +
		"    print(v)\n" +
		"  }\n" +
		"}\n"
	prog := parseTestProg(t, src)
	r := NewResolver(prog, nil)
	errs := r.Resolve()
	for _, e := range errs {
		if strings.HasPrefix(e.Msg, "undefined identifier") {
			t.Errorf("unexpected undefined-identifier error: %s", e.Msg)
		}
	}
}

// TestResolver_UndefinedIdentifierSpanSurvivesMacroExpansion locks Phase 83:
// the macro-expansion clone constructors must carry Col/EndCol through (the
// Phase 81 line fix was span-incomplete for FuncDecl/CallExpr).
func TestResolver_UndefinedIdentifierSpanSurvivesMacroExpansion(t *testing.T) {
	src := "func main() {\n  let result = unknown_name + 5\n}\n"
	prog := parseTestProg(t, src)
	expanded := parser.ApplyMacroExpansion(prog)
	r := NewResolver(expanded, nil)
	errs := r.Resolve()
	found := false
	for _, e := range errs {
		if strings.HasPrefix(e.Msg, "undefined identifier") {
			found = true
			if e.Col != 15 {
				t.Errorf("expected col 15 after macro expansion, got %d", e.Col)
			}
		}
	}
	if !found {
		t.Fatalf("expected undefined-identifier error, got: %s", errLines(errs))
	}
}

// ---- Phase 103: module-qualified call resolution ----

// moduleUnit returns a program whose compile unit spans lib.kark (greet +
// hidden), other.kark (otherFn) and main.kark (main), with real line numbers,
// plus the SourceMap tying each declaration line to its file.
func moduleUnit(t *testing.T, imports string, mainBody string) (*parser.Program, SourceMap) {
	t.Helper()
	src := imports +
		"public func greet() { print(1) }\n" +
		"func hidden() { print(1) }\n" +
		"public func otherFn() { print(1) }\n" +
		"func main() { " + mainBody + " }\n"
	prog := parseTestProg(t, src)
	importLines := strings.Count(imports, "\n")
	// decl lines: greet/hidden in lib.kark, otherFn in other.kark, main in main.kark
	sm := SourceMap{
		importLines + 1: "lib.kark",
		importLines + 2: "lib.kark",
		importLines + 3: "other.kark",
		importLines + 4: "main.kark",
	}
	return prog, sm
}

func TestResolver_QualifiedCall_PublicExport(t *testing.T) {
	prog, sm := moduleUnit(t, "import lib\nimport other\n", "lib.greet()")
	r := NewResolver(prog, sm)
	errs := r.Resolve()
	for _, e := range errs {
		t.Errorf("public module call should be clean: %s", e.Msg)
	}
}

func TestResolver_QualifiedCall_PrivateRejected(t *testing.T) {
	prog, sm := moduleUnit(t, "import lib\nimport other\n", "lib.hidden()")
	r := NewResolver(prog, sm)
	errs := r.Resolve()
	if !hasErr(errs, "private") {
		t.Errorf("expected private-export error, got: %s", errLines(errs))
	}
}

func TestResolver_QualifiedCall_WrongModule(t *testing.T) {
	prog, sm := moduleUnit(t, "import lib\nimport other\n", "lib.otherFn()")
	r := NewResolver(prog, sm)
	errs := r.Resolve()
	if !hasErr(errs, "not exported by module 'lib'") {
		t.Errorf("expected not-exported error, got: %s", errLines(errs))
	}
}

func TestResolver_QualifiedCall_UndefinedFunc(t *testing.T) {
	prog, sm := moduleUnit(t, "import lib\nimport other\n", "lib.nope()")
	r := NewResolver(prog, sm)
	errs := r.Resolve()
	if !hasErr(errs, "function 'nope' is not defined") {
		t.Errorf("expected undefined-function error, got: %s", errLines(errs))
	}
}

func TestResolver_QualifiedCall_NotImported(t *testing.T) {
	prog, sm := moduleUnit(t, "import other\n", "lib.greet()")
	r := NewResolver(prog, sm)
	errs := r.Resolve()
	if !hasErr(errs, "module 'lib' is not imported; add 'import lib'") {
		t.Errorf("expected not-imported error, got: %s", errLines(errs))
	}
}

func TestResolver_QualifiedCall_ReceiverMethodSkipped(t *testing.T) {
	prog, sm := moduleUnit(t, "import lib\n", "let s = \"hi\"\ns.len()")
	r := NewResolver(prog, sm)
	errs := r.Resolve()
	for _, e := range errs {
		if strings.Contains(e.Msg, "module") && strings.Contains(e.Msg, "not") {
			t.Errorf("receiver method call must not be treated as a module call: %s", e.Msg)
		}
	}
}

func TestResolver_ImportValidation_DottedStd(t *testing.T) {
	src := "import std.string\nfunc main() { print(1) }\n"
	prog := parseTestProg(t, src)
	sm := SourceMap{1: "string.kark", 2: "main.kark", 3: "main.kark"}
	r := NewResolver(prog, sm)
	errs := r.Resolve()
	if hasErr(errs, "module 'std.string' not found") {
		t.Errorf("dotted std import should resolve to the string module: %s", errLines(errs))
	}
}

// TestResolver_QualifiedCall_StdlibExempt locks the Phase 103 stdlib exemption:
// a qualified call into a stdlib/<module>/ file must NOT trip the private-export
// error even when the stdlib function is not marked `public` (physical markers
// land with the stdlib-v2 boundary).
func TestResolver_QualifiedCall_StdlibExempt(t *testing.T) {
	src := "import std.string\n" +
		"func trim(s) { return s }\n" +
		"func main() { print(string.trim(\" x \")) }\n"
	prog := parseTestProg(t, src)
	stdlibFile := filepath.Join("repo", "stdlib", "string", "string.kark")
	sm := SourceMap{2: stdlibFile, 3: "main.kark"}
	r := NewResolver(prog, sm)
	errs := r.Resolve()
	for _, e := range errs {
		if strings.Contains(e.Msg, "private") {
			t.Errorf("stdlib module export must be exempt from the private rule: %s", e.Msg)
		}
	}
}
