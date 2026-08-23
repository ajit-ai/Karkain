package parser

import (
	"testing"

	"karkain/pkg/lexer"
)

func TestComputeCaptures(t *testing.T) {
	// fn(x) { let y = 2; return base + x + y } with `y` declared locally
	body := []Node{
		&VarDeclStmt{Name: "y", Value: &IntLiteral{Value: "2"}},
		&ReturnStmt{Value: &BinaryExpr{
			Left:     &Identifier{Name: "base"},
			Operator: "+",
			Right: &BinaryExpr{
				Left:     &Identifier{Name: "x"},
				Operator: "+",
				Right:    &Identifier{Name: "y"},
			},
		}},
	}
	caps := ComputeCaptures([]string{"x"}, body)
	if len(caps) != 1 || caps[0] != "base" {
		t.Fatalf("expected [base], got %v", caps)
	}
}

func TestComputeCapturesLocalNotCaptured(t *testing.T) {
	body := []Node{
		&VarDeclStmt{Name: "tmp", Value: &IntLiteral{Value: "1"}},
		&ExprStmt{Expression: &Identifier{Name: "tmp"}},
		&ExprStmt{Expression: &Identifier{Name: "outer"}},
	}
	caps := ComputeCaptures(nil, body)
	if len(caps) != 1 || caps[0] != "outer" {
		t.Fatalf("expected [outer], got %v", caps)
	}
}

func TestParseLambdaCaptures(t *testing.T) {
	p := New(lexer.New("func main() { let f = fn() { return counter } }"))
	prog := p.ParseProgram()
	if len(prog.Statements) == 0 {
		t.Fatal("no statements parsed")
	}
	fd, ok := prog.Statements[0].(*FuncDecl)
	if !ok || len(fd.Body) == 0 {
		t.Fatalf("expected FuncDecl with body, got %T", prog.Statements[0])
	}
	vd, ok := fd.Body[0].(*VarDeclStmt)
	if !ok {
		t.Fatalf("expected VarDeclStmt, got %T", fd.Body[0])
	}
	fn, ok := vd.Value.(*FuncDecl)
	if !ok {
		t.Fatalf("expected FuncDecl desugar, got %T", vd.Value)
	}
	if len(fn.Captures) != 1 || fn.Captures[0] != "counter" {
		t.Fatalf("expected captures [counter], got %v", fn.Captures)
	}
}



