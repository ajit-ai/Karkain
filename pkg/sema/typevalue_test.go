package sema

import (
	"testing"

	"karkain/pkg/parser"
)

func TestTypeValue_ParseRoundTrip(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"int", "int"},
		{"float64", "float64"},
		{"bool", "bool"},
		{"string", "string"},
		{"int[]", "int[]"},
		{"Option", "Option"},
	}
	for _, c := range cases {
		got := ParseTypeString(c.in)
		if got == nil || got.String() != c.want {
			t.Errorf("ParseTypeString(%q) = %v, want %q", c.in, got, c.want)
		}
	}
}

func TestTypeValue_Equal(t *testing.T) {
	a := ParseTypeString("int[]")
	b := ParseTypeString("int[]")
	if !a.Equal(b) {
		t.Fatal("equal types reported unequal")
	}
	c := ParseTypeString("string[]")
	if a.Equal(c) {
		t.Fatal("distinct types reported equal")
	}
}

func TestTypeValue_Infer(t *testing.T) {
	src := "func main() { let x = 42; let y = \"hi\"; let z = true; }"
	prog := parseTestProg(t, src)
	r := NewResolver(prog, nil)
	r.Resolve()
	// find VarDeclStmts
	var decls []*parser.VarDeclStmt
	for _, s := range prog.Statements {
		if fn, ok := s.(*parser.FuncDecl); ok {
			for _, st := range fn.Body {
				if vd, ok := st.(*parser.VarDeclStmt); ok {
					decls = append(decls, vd)
				}
			}
		}
	}
	if len(decls) != 3 {
		t.Fatalf("expected 3 decls, got %d", len(decls))
	}
	wants := []string{"int", "string", "bool"}
	for i, d := range decls {
		tv, ok := d.TypeValue.(*Type)
		if !ok || tv == nil {
			t.Fatalf("decl %d TypeValue not set", i)
		}
		if tv.String() != wants[i] {
			t.Errorf("decl %d: got %q, want %q", i, tv.String(), wants[i])
		}
	}
}

func TestTypeValue_ReturnInfer(t *testing.T) {
	src := "func f() { return 42 }"
	prog := parseTestProg(t, src)
	r := NewResolver(prog, nil)
	r.Resolve()
	fn := prog.Statements[0].(*parser.FuncDecl)
	rtv, ok := fn.ReturnTypeValue.(*Type)
	if !ok || rtv == nil {
		t.Fatal("ReturnTypeValue not set")
	}
	if rtv.String() != "int" {
		t.Errorf("got %q, want int", rtv.String())
	}
}

// v1.2.26 S7: an expected type refines ambiguous initializer shapes. These
// are metadata assertions only — no strict type-checking behavior may change.
func TestTypeValue_ContextualInfer(t *testing.T) {
	optInt := &Type{Kind: TypeOption, Name: "Option", Elem: ParseTypeString("int")}
	resInt := &Type{Kind: TypeResult, Name: "Result", Elem: ParseTypeString("int")}
	cases := []struct {
		name     string
		node     parser.Node
		expected *Type
		want     string
	}{
		{"empty array from annotation", &parser.ArrayLiteral{}, ParseTypeString("string[]"), "string[]"},
		{"some element from annotation", &parser.OptionSomeExpr{Value: &parser.IntLiteral{Value: "1"}}, optInt, "Option<int>"},
		{"ok element from annotation", &parser.ResultOkExpr{Value: &parser.IntLiteral{Value: "1"}}, resInt, "Result<int>"},
		{"numeric widening", &parser.IntLiteral{Value: "1"}, ParseTypeString("float64"), "float64"},
		{"no expected falls back", &parser.IntLiteral{Value: "1"}, nil, "int"},
		{"concrete literal wins over unrelated expected", &parser.StringLiteral{Value: "x"}, ParseTypeString("int"), "string"},
	}
	for _, c := range cases {
		got := InferTypeValueWithContext(c.node, c.expected)
		if got == nil || got.String() != c.want {
			t.Errorf("%s: got %v, want %q", c.name, got, c.want)
		}
	}
	arr := InferTypeValueWithContext(&parser.ArrayLiteral{}, ParseTypeString("string[]"))
	if arr.Elem == nil || arr.Elem.String() != "string" {
		t.Errorf("empty array did not inherit element type: %v", arr.Elem)
	}
}

func TestTypeValue_ContextualInferInResolver(t *testing.T) {
	src := "func main() { string[] xs = [\"a\"] }"
	prog := parseTestProg(t, src)
	r := NewResolver(prog, nil)
	r.Resolve()
	fn := prog.Statements[0].(*parser.FuncDecl)
	vd, ok := fn.Body[0].(*parser.VarDeclStmt)
	if !ok {
		t.Fatalf("first statement is not a VarDeclStmt: %T", fn.Body[0])
	}
	tv, ok := vd.TypeValue.(*Type)
	if !ok || tv == nil || tv.String() != "string[]" {
		t.Fatalf("TypeValue not contextually inferred: %v", vd.TypeValue)
	}
}

// v1.2.26 S4a corrective: inferred return types are metadata on FuncDecl only.
// Writing them into Resolver.funcReturnTypes (which feeds typecheck.go's
// checkDecl) made previously accepted declarations such as `int x = f()`
// fail when f has no explicit annotation and returns a string/array/Value —
// 124 name-resolution errors in the kcc whole-tree self-check of the
// compiler's own sources. Inference must stay additive.
func TestTypeValue_ReturnInferDoesNotChangeStrictChecking(t *testing.T) {
	src := "func f() { return \"hi\" } func main() { int x = f(); print(x) }"
	prog := parseTestProg(t, src)
	r := NewResolver(prog, nil)
	if errs := r.Resolve(); len(errs) != 0 {
		t.Fatalf("inferred return type changed strict declaration checking: %v", errs)
	}
	fn := prog.Statements[0].(*parser.FuncDecl)
	rtv, ok := fn.ReturnTypeValue.(*Type)
	if !ok || rtv == nil || rtv.String() != "string" {
		t.Fatalf("ReturnTypeValue not inferred as string: %v", fn.ReturnTypeValue)
	}
}