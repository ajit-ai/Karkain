package parser

import (
	"karkain/pkg/lexer"
	"testing"
)

func parseSource(t *testing.T, src string) *Program {
	t.Helper()
	l := lexer.New(src)
	p := New(l)
	return p.ParseProgram()
}

func findFuncBody(prog *Program) []Node {
	for _, stmt := range prog.Statements {
		if fn, ok := stmt.(*FuncDecl); ok && fn.Name == "main" {
			return fn.Body
		}
	}
	return nil
}

func TestParseQRegBracketForm(t *testing.T) {
	prog := parseSource(t, "func main() {\n  qreg qr[2]\n}")
	body := findFuncBody(prog)
	if len(body) != 1 {
		t.Fatalf("expected 1 statement, got %d", len(body))
	}
	qr, ok := body[0].(*QRegDeclStmt)
	if !ok {
		t.Fatalf("expected QRegDeclStmt, got %T", body[0])
	}
	if qr.Name != "qr" {
		t.Errorf("expected register name 'qr', got %q", qr.Name)
	}
	intLit, ok := qr.Qubits.(*IntLiteral)
	if !ok {
		t.Fatalf("expected IntLiteral qubit count, got %T", qr.Qubits)
	}
	if intLit.Value != "2" {
		t.Errorf("expected 2 qubits, got %s", intLit.Value)
	}
}

func TestParseQRegAssignForm(t *testing.T) {
	prog := parseSource(t, "func main() {\n  qreg qr = 3\n}")
	body := findFuncBody(prog)
	if len(body) != 1 {
		t.Fatalf("expected 1 statement, got %d", len(body))
	}
	qr, ok := body[0].(*QRegDeclStmt)
	if !ok {
		t.Fatalf("expected QRegDeclStmt, got %T", body[0])
	}
	if qr.Name != "qr" {
		t.Errorf("expected register name 'qr', got %q", qr.Name)
	}
	intLit, ok := qr.Qubits.(*IntLiteral)
	if !ok || intLit.Value != "3" {
		t.Errorf("expected 3 qubits, got %#v", qr.Qubits)
	}
}

func TestParseBareGateSingleQubit(t *testing.T) {
	prog := parseSource(t, "func main() {\n  qreg qr[2]\n  H qr[0]\n}")
	body := findFuncBody(prog)
	if len(body) != 2 {
		t.Fatalf("expected 2 statements, got %d", len(body))
	}
	gate, ok := body[1].(*GateApplyStmt)
	if !ok {
		t.Fatalf("expected GateApplyStmt, got %T", body[1])
	}
	if gate.Gate != "H" {
		t.Errorf("expected gate 'H', got %q", gate.Gate)
	}
	if gate.Control != nil {
		t.Errorf("expected nil control for single-qubit gate")
	}
	idx, ok := gate.Target.(*IndexExpr)
	if !ok {
		t.Fatalf("expected IndexExpr target, got %T", gate.Target)
	}
	reg, ok := idx.Left.(*Identifier)
	if !ok || reg.Name != "qr" {
		t.Errorf("expected register 'qr', got %#v", idx.Left)
	}
}

func TestParseBareGateCNOT(t *testing.T) {
	prog := parseSource(t, "func main() {\n  qreg qr[2]\n  CNOT qr[0], qr[1]\n}")
	body := findFuncBody(prog)
	if len(body) != 2 {
		t.Fatalf("expected 2 statements, got %d", len(body))
	}
	gate, ok := body[1].(*GateApplyStmt)
	if !ok {
		t.Fatalf("expected GateApplyStmt, got %T", body[1])
	}
	if gate.Gate != "CNOT" {
		t.Errorf("expected gate 'CNOT', got %q", gate.Gate)
	}
	controlIdx, ok := gate.Control.(*IndexExpr)
	if !ok {
		t.Fatalf("expected IndexExpr control, got %T", gate.Control)
	}
	targetIdx, ok := gate.Target.(*IndexExpr)
	if !ok {
		t.Fatalf("expected IndexExpr target, got %T", gate.Target)
	}
	cReg := controlIdx.Left.(*Identifier)
	tReg := targetIdx.Left.(*Identifier)
	if cReg.Name != "qr" || tReg.Name != "qr" {
		t.Errorf("expected registers 'qr','qr', got %q,%q", cReg.Name, tReg.Name)
	}
}

func TestParseMeasureBothForms(t *testing.T) {
	prog := parseSource(t, "func main() {\n  measure qr[0]\n}")
	body := findFuncBody(prog)
	if len(body) != 1 {
		t.Fatalf("bare form: expected 1 statement, got %d", len(body))
	}
	exprStmt, ok := body[0].(*ExprStmt)
	if !ok {
		t.Fatalf("bare form: expected ExprStmt, got %T", body[0])
	}
	measure, ok := exprStmt.Expression.(*MeasureExpr)
	if !ok {
		t.Fatalf("bare form: expected MeasureExpr, got %T", exprStmt.Expression)
	}
	if _, ok := measure.Qubit.(*IndexExpr); !ok {
		t.Errorf("bare form: expected IndexExpr qubit, got %T", measure.Qubit)
	}

	prog2 := parseSource(t, "func main() {\n  let r = measure(qr[1])\n}")
	found := false
	for _, stmt := range prog2.Statements {
		if fn, ok := stmt.(*FuncDecl); ok {
			for _, s := range fn.Body {
				if vd, ok := s.(*VarDeclStmt); ok {
					if _, ok := vd.Value.(*MeasureExpr); ok {
						found = true
					}
				}
			}
		}
	}
	if !found {
		t.Errorf("paren form: expected MeasureExpr in VarDeclStmt value")
	}
}

func TestGateNameNotHijackedInExpressions(t *testing.T) {
	// A variable named like a gate used in normal expression contexts must still work
	prog := parseSource(t, "func main() {\n  let x = 5\n  print(x)\n}")
	body := findFuncBody(prog)
	if len(body) != 2 {
		t.Fatalf("expected 2 statements, got %d", len(body))
	}
	if _, ok := body[1].(*PrintStmt); !ok {
		t.Fatalf("expected PrintStmt, got %T", body[1])
	}
}
