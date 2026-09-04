package sema

import (
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
