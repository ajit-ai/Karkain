package sema

// Phase 146A: unit gate for MonomorphizeProgram — collection, arity
// validation, instantiation, rewrite/demotion, and idempotency.

import (
	"strings"
	"testing"

	"karkain/pkg/lexer"
	"karkain/pkg/parser"
)

func parse146(t *testing.T, src string) *parser.Program {
	t.Helper()
	p := parser.New(lexer.New(src))
	prog := p.ParseProgram()
	if len(p.Errors) > 0 {
		t.Fatalf("parse errors: %v", p.Errors)
	}
	return parser.ApplyMacroExpansion(prog)
}

func TestMonomorph_GenericFuncInstantiates(t *testing.T) {
	prog := parse146(t, "func id[T](x) {\nreturn x\n}\nfunc main() {\nprint(id[int](1))\n}\n")
	if errs := MonomorphizeProgram(prog); len(errs) > 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	found := false
	for _, s := range prog.Statements {
		if fn, ok := s.(*parser.FuncDecl); ok && fn.Name == "id_int" {
			found = true
			if len(fn.GenericParams) != 0 {
				t.Errorf("specialization must carry no GenericParams")
			}
		}
		if fn, ok := s.(*parser.FuncDecl); ok && fn.Name == "id" && len(fn.GenericParams) == 0 {
			t.Errorf("template lost its GenericParams")
		}
	}
	if !found {
		t.Fatalf("specialization id_int not appended")
	}
}

func TestMonomorph_ArityMismatch(t *testing.T) {
	prog := parse146(t, "func id[T](x) {\nreturn x\n}\nfunc main() {\nprint(id[int, string](1))\n}\n")
	errs := MonomorphizeProgram(prog)
	if len(errs) == 0 {
		t.Fatalf("expected arity error")
	}
	if !strings.Contains(errs[0].Msg, "expects 1 type argument(s), got 2") {
		t.Errorf("unexpected message: %q", errs[0].Msg)
	}
}

func TestMonomorph_BareGenericCallRejected(t *testing.T) {
	prog := parse146(t, "func id[T](x) {\nreturn x\n}\nfunc main() {\nprint(id(1))\n}\n")
	errs := MonomorphizeProgram(prog)
	if len(errs) == 0 {
		t.Fatalf("expected bare-generic error")
	}
	if !strings.Contains(errs[0].Msg, "requires explicit type arguments") {
		t.Errorf("unexpected message: %q", errs[0].Msg)
	}
}

func TestMonomorph_NonGenericDemotesToIndirect(t *testing.T) {
	prog := parse146(t, "func main() {\nlet ops = [1]\nprint(ops[idx](5))\n}\n")
	if errs := MonomorphizeProgram(prog); len(errs) > 0 {
		t.Fatalf("demotion must not error: %v", errs)
	}
	found := false
	var walk func(n parser.Node)
	walk = func(n parser.Node) {
		switch x := n.(type) {
		case *parser.IndirectCallExpr:
			found = true
		case *parser.CallExpr:
			for _, a := range x.Args {
				walk(a)
			}
		case *parser.FuncDecl:
			for _, s := range x.Body {
				walk(s)
			}
		case *parser.ExprStmt:
			walk(x.Expression)
		case *parser.PrintStmt:
			walk(x.Value)
		case *parser.VarDeclStmt:
			if x.Value != nil {
				walk(x.Value)
			}
		}
	}
	for _, s := range prog.Statements {
		walk(s)
	}
	if !found {
		t.Fatalf("expected demoted IndirectCallExpr for non-generic head")
	}
}

func TestMonomorph_Idempotent(t *testing.T) {
	prog := parse146(t, "func id[T](x) {\nreturn x\n}\nfunc main() {\nprint(id[int](1))\nprint(id[int](2))\n}\n")
	if errs := MonomorphizeProgram(prog); len(errs) > 0 {
		t.Fatalf("first run: %v", errs)
	}
	n1 := len(prog.Statements)
	if errs := MonomorphizeProgram(prog); len(errs) > 0 {
		t.Fatalf("second run: %v", errs)
	}
	if len(prog.Statements) != n1 {
		t.Fatalf("not idempotent: %d then %d statements", n1, len(prog.Statements))
	}
	count := 0
	for _, s := range prog.Statements {
		if fn, ok := s.(*parser.FuncDecl); ok && fn.Name == "id_int" {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("want exactly one id_int specialization, got %d", count)
	}
}

func TestMonomorph_GenericStructInstantiates(t *testing.T) {
	prog := parse146(t, "type Point[T] struct { x T, y T }\nfunc main() {\nlet p = Point[int]{x: 1, y: 2}\nprint(p.x)\n}\n")
	if errs := MonomorphizeProgram(prog); len(errs) > 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	found := false
	for _, s := range prog.Statements {
		if st, ok := s.(*parser.StructDeclStmt); ok && st.Name == "Point_int" {
			found = true
			if len(st.Fields) != 2 || st.Fields[0].Type != "int" {
				t.Errorf("field types not substituted: %+v", st.Fields)
			}
		}
	}
	if !found {
		t.Fatalf("specialization Point_int not appended")
	}
}
