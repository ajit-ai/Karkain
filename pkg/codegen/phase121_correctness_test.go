package codegen

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"karkain/pkg/parser"
)

// Phase 121 gate — correctness sweep for `let f = fn(...)` closures and
// `alloc(T, n)` / `free(x)` on the Go engine. Unit-level: capture analysis
// (incl. nested let-bound lambdas), generated C markers (env typedefs, holders,
// capture macros, binding-site inits, call routing) and the managed alloc/free
// emission (no raw pointer Value type, no malloc surface). The pkg/cli gate
// proves both engines end-to-end on every fixture.

func compileCOnly(t *testing.T, src string) (string, string) {
	t.Helper()
	prog := parseProg(t, src)
	tmp := t.TempDir()
	exePath := filepath.Join(tmp, "probe.exe")
	g := New(Config{OutputPath: exePath, CompileOnly: true})
	if err := g.GenerateAndCompile(prog, filepath.Join(tmp, "main.kark")); err != nil {
		t.Fatalf("GenerateAndCompile: %v", err)
	}
	c, err := os.ReadFile(filepath.Join(tmp, "main.c"))
	if err != nil {
		t.Fatalf("generated main.c missing: %v", err)
	}
	return string(c), tmp
}

// TestPhase121_CaptureAnalysisNested pins the parser's capture propagation for a
// let-bound lambda whose body contains another let-bound lambda. The outer
// closure must capture only the identifiers that are free in its own scope once
// locals and params are removed — including names referenced only inside the
// nested lambda (the binding site for the nested closure is inside the outer
// body, so the outer env must carry them).
func TestPhase121_CaptureAnalysisNested(t *testing.T) {
	src := `
func main() {
	let base = 10
	let outer = fn(a) {
		let m = 3
		let inner = fn(b) { return base + m + b }
		return inner(a)
	}
	println(outer(5))
}
`
	prog := parseProg(t, src)
	mainFn, ok := programFunc(prog, "main")
	if !ok {
		t.Fatal("main not found")
	}
	var outer *parser.FuncDecl
	for _, st := range mainFn.Body {
		if vd, ok := st.(*parser.VarDeclStmt); ok {
			if vd.Name == "outer" {
				outer, _ = vd.Value.(*parser.FuncDecl)
			}
		}
	}
	if outer == nil {
		t.Fatal("outer closure was not desugared to a FuncDecl")
	}
	// inner's captures: base, m (b is a param, inner is its own binding).
	wantInner := []string{"base", "m"}
	if got := outerCaptures(t, prog, "main", "inner"); strings.Join(got, ",") != "base,m" {
		t.Errorf("inner captures = %v, want %v", got, wantInner)
	}
	// outer's captures: base only (m is an outer local, a is a param, inner is
	// a local binding).
	if got := outer.Captures; len(got) != 1 || got[0] != "base" {
		t.Errorf("outer captures = %v, want [base]", got)
	}
}

func outerCaptures(t *testing.T, prog *parser.Program, fnName, closureName string) []string {
	t.Helper()
	fn, ok := programFunc(prog, fnName)
	if !ok {
		t.Fatalf("%s not found", fnName)
	}
	return walkCaptureNames(t, fn.Body, closureName)
}

func walkCaptureNames(t *testing.T, body []parser.Node, name string) []string {
	t.Helper()
	for _, st := range body {
		if vd, ok := st.(*parser.VarDeclStmt); ok {
			if fd, ok2 := vd.Value.(*parser.FuncDecl); ok2 {
				if vd.Name == name {
					return fd.Captures
				}
				if got := walkCaptureNames(t, fd.Body, name); got != nil {
					return got
				}
			}
		}
	}
	return nil
}

// TestPhase121_CaptureClosureCodegenMarkers pins the emitted C for a capturing
// closure: prefixed env fields (macro-safe), the typedef+holder, the capture
// macro, the env-taking def, the binding-site init and the routed call.
func TestPhase121_CaptureClosureCodegenMarkers(t *testing.T) {
	src := `
func main() {
	let base = 10
	let f = fn(x) { return base + x }
	println(f(5))
}
`
	generated, _ := compileCOnly(t, src)
	for _, marker := range []string{
		"typedef struct {\n\tValue* karkain_cap_base;\n} ClosureEnv_f;",
		"static ClosureEnv_f* _genv_f;",
		"Value karkain_user_f(ClosureEnv_f* _env, Value x);",
		"#define base (*_env->karkain_cap_base)",
		"#undef base",
		"Value karkain_user_f(ClosureEnv_f* _env, Value x) {",
		"{ static ClosureEnv_f _e; _e.karkain_cap_base = &base; _genv_f = &_e; }",
		"karkain_user_f(_genv_f, make_int(5))",
	} {
		if !strings.Contains(generated, marker) {
			t.Errorf("generated C missing %q", marker)
		}
	}
}

// TestPhase121_ZeroCaptureClosureCodegenMarkers pins zero-capture closures:
// no env typedef at all, plain symbol prototype/def, and a bare call site.
func TestPhase121_ZeroCaptureClosureCodegenMarkers(t *testing.T) {
	src := `
func main() {
	let add = fn(a, b) { return a + b }
	println(add(1, 2))
}
`
	generated, _ := compileCOnly(t, src)
	if strings.Contains(generated, "ClosureEnv_add") {
		t.Errorf("zero-capture closure must not emit an env for add")
	}
	for _, marker := range []string{
		"Value karkain_user_add(Value a, Value b);",
		"karkain_user_add(make_int(1), make_int(2))",
	} {
		if !strings.Contains(generated, marker) {
			t.Errorf("generated C missing %q", marker)
		}
	}
}

// TestPhase121_NestedClosureCodegenMarkers pins the nested-closure case: the
// outer closure carries its own env and the inner binding inside the outer body
// reads captured-through names from the outer env (pointer copy) while outer
// locals stay plain addresses.
func TestPhase121_NestedClosureCodegenMarkers(t *testing.T) {
	src := `
func main() {
	let base = 10
	let outer = fn(a) {
		let m = 3
		let inner = fn(b) { return base + m + b }
		return inner(a)
	}
	println(outer(5))
}
`
	generated, _ := compileCOnly(t, src)
	for _, marker := range []string{
		"typedef struct {\n\tValue* karkain_cap_base;\n} ClosureEnv_outer;",
		"static ClosureEnv_outer* _genv_outer;",
		"typedef struct {\n\tValue* karkain_cap_base;\n\tValue* karkain_cap_m;\n} ClosureEnv_inner;",
		"#define base (*_env->karkain_cap_base)",
		// inner binding inside outer: base is an outer capture (copy the
		// pointer), m is an outer local (plain address).
		"{ static ClosureEnv_inner _e; _e.karkain_cap_base = _env->karkain_cap_base; _e.karkain_cap_m = &m; _genv_inner = &_e; }",
		// outer binding in main: base is a plain local.
		"{ static ClosureEnv_outer _e; _e.karkain_cap_base = &base; _genv_outer = &_e; }",
		"karkain_user_inner(_genv_inner, a)",
		"karkain_user_outer(_genv_outer, make_int(5))",
	} {
		if !strings.Contains(generated, marker) {
			t.Errorf("generated C missing %q", marker)
		}
	}
}

// TestPhase121_AllocFreeCodegenMarkers pins the managed alloc/free emission:
// `alloc(T, n)` lowers to the pre-sized make_alloc_array helper and `free(x)`
// to a no-op (void) discard — both in statement and expression forms.
func TestPhase121_AllocFreeCodegenMarkers(t *testing.T) {
	src := `
func main() {
	let n = 3
	let a = alloc(int, n)
	a[0] = 10
	a[1] = 5
	a[2] = a[0] + a[1]
	free(a)
	let b = alloc(int, 2) + make_array()
	println(a[2])
}
`
	generated, _ := compileCOnly(t, src)
	for _, ptr := range []string{"Value* a;", "Value* a ="} {
		if strings.Contains(generated, ptr) {
			t.Errorf("alloc must not produce a raw pointer variable for a (found %q)", ptr)
		}
	}
	for _, marker := range []string{
		"make_alloc_array(n)",
		"make_alloc_array(make_int(2))",
		"\t(void)(a);",
		"karkain_checked_set(&a, make_int(0), make_int(10)",
	} {
		if !strings.Contains(generated, marker) {
			t.Errorf("generated C missing %q", marker)
		}
	}
}

// TestPhase121_AllocPreambleHelper pins make_alloc_array after the array_push
// helper (managed array with n pre-sized zero slots).
func TestPhase121_AllocPreambleHelper(t *testing.T) {
	src := `
func main() {
	let a = alloc(int, 4)
	print(a[3])
}
`
	generated, _ := compileCOnly(t, src)
	for _, marker := range []string{
		"Value make_alloc_array(Value n) {",
		"make_alloc_array(make_int(4))",
	} {
		if !strings.Contains(generated, marker) {
			t.Errorf("generated C missing %q", marker)
		}
	}
}

func programFunc(prog *parser.Program, name string) (*parser.FuncDecl, bool) {
	for _, st := range prog.Statements {
		if fd, ok := st.(*parser.FuncDecl); ok && fd.Name == name {
			return fd, true
		}
	}
	return nil, false
}