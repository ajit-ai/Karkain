package codegen

import (
	"karkain/pkg/parser"
	"strings"
	"testing"
)

func TestQuantum_OpenQASMGeneration(t *testing.T) {
	circuit := &parser.CircuitDecl{
		Name: "BellPair",
		Params: []parser.CircuitParam{
			{Name: "q", Type: &parser.QubitType{Size: 2}},
		},
		ReturnType: &parser.BitType{Size: 2},
		Body: []parser.Node{
			// H on q[0]
			&parser.QPUOpExpr{
				Op:   "h",
				Args: []parser.Node{&parser.QubitIndexExpr{Qubit: &parser.Identifier{Name: "q"}, Index: &parser.IntLiteral{Value: "0"}}},
			},
			// CX on q[0], q[1]
			&parser.QPUOpExpr{
				Op: "cx",
				Args: []parser.Node{
					&parser.QubitIndexExpr{Qubit: &parser.Identifier{Name: "q"}, Index: &parser.IntLiteral{Value: "0"}},
					&parser.QubitIndexExpr{Qubit: &parser.Identifier{Name: "q"}, Index: &parser.IntLiteral{Value: "1"}},
				},
			},
			// Measure both
			&parser.QPUOpExpr{
				Op: "measure",
				Args: []parser.Node{
					&parser.QubitIndexExpr{Qubit: &parser.Identifier{Name: "q"}, Index: &parser.IntLiteral{Value: "0"}},
					&parser.QubitIndexExpr{Qubit: &parser.Identifier{Name: "q"}, Index: &parser.IntLiteral{Value: "1"}},
				},
			},
		},
	}

	gen := NewOpenQASMGenerator()
	qasm, err := gen.GenerateCircuit(circuit)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify header
	if !strings.Contains(qasm, "OPENQASM 3.0;") {
		t.Errorf("expected OPENQASM 3.0 header, got:\n%s", qasm)
	}
	if !strings.Contains(qasm, "include \"stdgates.inc\";") {
		t.Errorf("expected stdgates include, got:\n%s", qasm)
	}

	// Verify qubit register declaration
	if !strings.Contains(qasm, "qubit[2] q;") {
		t.Errorf("expected qubit[2] q declaration, got:\n%s", qasm)
	}

	// Verify classical register
	if !strings.Contains(qasm, "bit[2] c;") {
		t.Errorf("expected bit[2] c declaration, got:\n%s", qasm)
	}

	// Verify gate operations
	if !strings.Contains(qasm, "h q[0];") {
		t.Errorf("expected 'h q[0];', got:\n%s", qasm)
	}
	if !strings.Contains(qasm, "cx q[0], q[1];") {
		t.Errorf("expected 'cx q[0], q[1];', got:\n%s", qasm)
	}

	// Verify measurement
	if !strings.Contains(qasm, "measure") {
		t.Errorf("expected measure operation, got:\n%s", qasm)
	}
}

func TestQuantum_OpenQASMRotations(t *testing.T) {
	circuit := &parser.CircuitDecl{
		Name: "RzCircuit",
		Params: []parser.CircuitParam{
			{Name: "q", Type: &parser.QubitType{Size: 1}},
		},
		Body: []parser.Node{
			// Rx(pi/4) on q[0]
			&parser.QPUOpExpr{
				Op:    "rx",
				Args:  []parser.Node{&parser.Identifier{Name: "q"}},
				Angle: &parser.Float64Literal{Value: "3.14159/4"},
			},
			// Ry(pi/2) on q[0]
			&parser.QPUOpExpr{
				Op:    "ry",
				Args:  []parser.Node{&parser.Identifier{Name: "q"}},
				Angle: &parser.Float64Literal{Value: "3.14159/2"},
			},
			// Rz(pi) on q[0]
			&parser.QPUOpExpr{
				Op:    "rz",
				Args:  []parser.Node{&parser.Identifier{Name: "q"}},
				Angle: &parser.Float64Literal{Value: "3.14159"},
			},
		},
	}

	gen := NewOpenQASMGenerator()
	qasm, err := gen.GenerateCircuit(circuit)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(qasm, "rx(") {
		t.Errorf("expected rx( rotation, got:\n%s", qasm)
	}
	if !strings.Contains(qasm, "ry(") {
		t.Errorf("expected ry( rotation, got:\n%s", qasm)
	}
	if !strings.Contains(qasm, "rz(") {
		t.Errorf("expected rz( rotation, got:\n%s", qasm)
	}
}

func TestQuantum_OpenQASMReset(t *testing.T) {
	circuit := &parser.CircuitDecl{
		Name: "ResetCircuit",
		Params: []parser.CircuitParam{
			{Name: "q", Type: &parser.QubitType{Size: 1}},
		},
		Body: []parser.Node{
			&parser.QPUOpExpr{
				Op:   "h",
				Args: []parser.Node{&parser.Identifier{Name: "q"}},
			},
			&parser.QPUOpExpr{
				Op:   "reset",
				Args: []parser.Node{&parser.Identifier{Name: "q"}},
			},
			&parser.QPUOpExpr{
				Op:   "x",
				Args: []parser.Node{&parser.Identifier{Name: "q"}},
			},
		},
	}

	gen := NewOpenQASMGenerator()
	qasm, err := gen.GenerateCircuit(circuit)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(qasm, "reset q;") {
		t.Errorf("expected 'reset q;', got:\n%s", qasm)
	}
}

func TestQuantum_QIRLLVMGeneration(t *testing.T) {
	circuit := &parser.CircuitDecl{
		Name: "RyCircuit",
		Params: []parser.CircuitParam{
			{Name: "q", Type: &parser.QubitType{Size: 1}},
		},
		Body: []parser.Node{
			// Ry(theta) on q[0]
			&parser.QPUOpExpr{
				Op:    "ry",
				Args:  []parser.Node{&parser.Identifier{Name: "q"}},
				Angle: &parser.Float64Literal{Value: "1.5707963"},
			},
			// Measure
			&parser.QPUOpExpr{
				Op:   "measure",
				Args: []parser.Node{&parser.Identifier{Name: "q"}},
			},
		},
	}

	gen := NewQIRGenerator()
	qir, err := gen.GenerateCircuit(circuit)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify QIR types
	if !strings.Contains(qir, "%Qubit = type opaque") {
		t.Errorf("expected %%Qubit type declaration, got:\n%s", qir)
	}
	if !strings.Contains(qir, "%Result = type opaque") {
		t.Errorf("expected %%Result type declaration, got:\n%s", qir)
	}

	// Verify QIS intrinsic declarations
	if !strings.Contains(qir, "@__quantum__qis__ry__body") {
		t.Errorf("expected @__quantum__qis__ry__body declaration, got:\n%s", qir)
	}
	if !strings.Contains(qir, "@__quantum__qis__m__body") {
		t.Errorf("expected @__quantum__qis__m__body declaration, got:\n%s", qir)
	}

	// Verify function definition
	if !strings.Contains(qir, "define void @RyCircuit(") {
		t.Errorf("expected function definition, got:\n%s", qir)
	}

	// Verify ry call
	if !strings.Contains(qir, "call void @__quantum__qis__ry__body(double 1.5707963, %Qubit*") {
		t.Errorf("expected ry body call with angle, got:\n%s", qir)
	}

	// Verify measurement call
	if !strings.Contains(qir, "call %Result* @__quantum__qis__m__body(%Qubit*") {
		t.Errorf("expected measurement call, got:\n%s", qir)
	}

	// Verify QIR Alliance reference
	if !strings.Contains(qir, "QIR Alliance") {
		t.Errorf("expected QIR Alliance reference, got:\n%s", qir)
	}
}

func TestQuantum_QIRCNOT(t *testing.T) {
	circuit := &parser.CircuitDecl{
		Name: "CNOTCircuit",
		Params: []parser.CircuitParam{
			{Name: "q", Type: &parser.QubitType{Size: 2}},
		},
		Body: []parser.Node{
			&parser.QPUOpExpr{
				Op:   "h",
				Args: []parser.Node{&parser.Identifier{Name: "q"}},
			},
			&parser.QPUOpExpr{
				Op: "cx",
				Args: []parser.Node{
					&parser.Identifier{Name: "q"},
					&parser.QubitIndexExpr{Qubit: &parser.Identifier{Name: "q"}, Index: &parser.IntLiteral{Value: "1"}},
				},
			},
		},
	}

	gen := NewQIRGenerator()
	qir, err := gen.GenerateCircuit(circuit)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(qir, "@__quantum__qis__cnot__body") {
		t.Errorf("expected CNOT declaration, got:\n%s", qir)
	}
	if !strings.Contains(qir, "@__quantum__qis__h__body") {
		t.Errorf("expected H declaration, got:\n%s", qir)
	}
}

func TestQuantum_QIRAllGates(t *testing.T) {
	// Verify all gate declarations are emitted
	gen := NewQIRGenerator()
	qir, err := gen.GenerateCircuit(&parser.CircuitDecl{
		Name:   "EmptyCircuit",
		Params: []parser.CircuitParam{},
		Body:   []parser.Node{},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expectedDecls := []string{
		"@__quantum__qis__h__body",
		"@__quantum__qis__x__body",
		"@__quantum__qis__y__body",
		"@__quantum__qis__z__body",
		"@__quantum__qis__rx__body",
		"@__quantum__qis__ry__body",
		"@__quantum__qis__rz__body",
		"@__quantum__qis__cnot__body",
		"@__quantum__qis__cz__body",
		"@__quantum__qis__swap__body",
		"@__quantum__qis__m__body",
		"@__quantum__qis__reset__body",
	}

	for _, decl := range expectedDecls {
		if !strings.Contains(qir, decl) {
			t.Errorf("expected declaration %s, not found in QIR output", decl)
		}
	}
}
