package parser

import (
	"testing"

	"karkain/pkg/lexer"
)

// Phase 112 gate: `const` is a first-class declaration keyword on the Go
// engine. parseVarDecl flags the statement Const so the semantic layer can
// reject reassignment for parity with the self-hosted kcc engine.

func TestPhase112_ParseFunctionScopeConst(t *testing.T) {
	p := parseHelper(t, "func main() {\n    const MAX = 100\n}")
	fn, ok := p.Statements[0].(*FuncDecl)
	if !ok || fn.Name != "main" {
		t.Fatalf("expected FuncDecl main, got %v", p.Statements[0])
	}
	if len(fn.Body) != 1 {
		t.Fatalf("expected 1 body statement, got %d", len(fn.Body))
	}
	vd, ok := fn.Body[0].(*VarDeclStmt)
	if !ok {
		t.Fatalf("expected VarDeclStmt, got %T", fn.Body[0])
	}
	if !vd.Const {
		t.Fatal("expected Const=true on const declaration")
	}
	if vd.Name != "MAX" {
		t.Fatalf("expected name MAX, got %q", vd.Name)
	}
}

func TestPhase112_ParseTypedConst(t *testing.T) {
	p := parseHelper(t, "func main() {\n    const RATE float64 = 2.5\n}")
	fn := p.Statements[0].(*FuncDecl)
	vd := fn.Body[0].(*VarDeclStmt)
	if !vd.Const {
		t.Fatal("expected Const=true on typed const")
	}
	if vd.Type != "float64" {
		t.Fatalf("expected type float64, got %q", vd.Type)
	}
}

func TestPhase112_LetsAndVarsNotConst(t *testing.T) {
	p := parseHelper(t, "func main() {\n    let a = 1\n    var b = 2\n}")
	fn := p.Statements[0].(*FuncDecl)
	for i, s := range fn.Body {
		vd := s.(*VarDeclStmt)
		if vd.Const {
			t.Errorf("statement %d should not be const", i)
		}
	}
}

func parseHelper(t *testing.T, src string) *Program {
	t.Helper()
	l := lexer.New(src)
	p := New(l)
	prog := p.ParseProgram()
	if len(p.Errors) > 0 {
		t.Fatalf("parse errors: %v", p.Errors)
	}
	return prog
}