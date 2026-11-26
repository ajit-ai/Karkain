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