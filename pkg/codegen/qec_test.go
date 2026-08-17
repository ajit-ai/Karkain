package codegen

import (
	"karkain/pkg/sema"
	"strings"
	"testing"
)

func TestQEC_SyndromeCircuitShor(t *testing.T) {
	code := sema.ShorCode913()
	gen := NewQECGenerator()
	qasm, err := gen.GenerateSyndromeCircuit(code, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(qasm, "OPENQASM 3.0;") {
		t.Error("expected OPENQASM header")
	}
	if !strings.Contains(qasm, "Shor") {
		t.Error("expected Shor code name in comments")
	}
	if !strings.Contains(qasm, "qubit[9] data;") {
		t.Errorf("expected 9 data qubits, got:\n%s", qasm)
	}
	if !strings.Contains(qasm, "qubit[8] syndrome;") {
		t.Errorf("expected 8 syndrome qubits, got:\n%s", qasm)
	}
	if !strings.Contains(qasm, "bit[8] syn_out;") {
		t.Error("expected 8-bit syn_out")
	}
	// Should have syndrome extraction rounds
	if !strings.Contains(qasm, "Syndrome Round 1") {
		t.Error("expected syndrome round 1")
	}
}

func TestQEC_SyndromeCircuitSteane(t *testing.T) {
	code := sema.SteaneCode713()
	gen := NewQECGenerator()
	qasm, err := gen.GenerateSyndromeCircuit(code, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(qasm, "Syndrome Round 1") {
		t.Error("expected syndrome round 1")
	}
	if !strings.Contains(qasm, "Syndrome Round 2") {
		t.Error("expected syndrome round 2")
	}
	// Should have CX gates for syndrome extraction
	if !strings.Contains(qasm, "cx ") {
		t.Error("expected cx gates for syndrome extraction")
	}
	// Should have measure calls
	if !strings.Contains(qasm, "measure syndrome") {
		t.Error("expected measure calls")
	}
}

func TestQEC_XTypeStabilizerPrep(t *testing.T) {
	code := sema.SteaneCode713()
	gen := NewQECGenerator()
	qasm, err := gen.GenerateSyndromeCircuit(code, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// X-type stabilizers should have H gates on ancilla
	if !strings.Contains(qasm, "h syndrome") {
		t.Error("expected H gates for X-type stabilizers")
	}
}

func TestQEC_DecodeSection(t *testing.T) {
	code := sema.ShorCode913()
	gen := NewQECGenerator()
	qasm, err := gen.GenerateSyndromeCircuit(code, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(qasm, "Decode and Correct") {
		t.Error("expected decode section")
	}
}

func TestSurfaceCode_SyndromeCircuitD3(t *testing.T) {
	sc := sema.NewSurfaceCode(3)
	gen := NewSurfaceCodeGenerator()
	qasm := gen.GenerateSurfaceCircuit(sc)

	if !strings.Contains(qasm, "OPENQASM 3.0;") {
		t.Error("expected OPENQASM header")
	}
	if !strings.Contains(qasm, "distance d=3") {
		t.Error("expected d=3 in comments")
	}
	if !strings.Contains(qasm, "X (Plaquette) Stabilizers") {
		t.Error("expected X stabilizer section")
	}
	if !strings.Contains(qasm, "Z (Vertex) Stabilizers") {
		t.Error("expected Z stabilizer section")
	}
	if !strings.Contains(qasm, "x_syndrome") {
		t.Error("expected x_syndrome qubits")
	}
	if !strings.Contains(qasm, "z_syndrome") {
		t.Error("expected z_syndrome qubits")
	}
	if !strings.Contains(qasm, "Minimum-weight perfect matching") {
		t.Error("expected decoder comment")
	}
}

func TestSurfaceCode_SyndromeCircuitD5(t *testing.T) {
	sc := sema.NewSurfaceCode(5)
	gen := NewSurfaceCodeGenerator()
	qasm := gen.GenerateSurfaceCircuit(sc)

	if !strings.Contains(qasm, "distance d=5") {
		t.Error("expected d=5 in comments")
	}
	// d=5 should have 25 data qubits
	if !strings.Contains(qasm, "qubit[25] data;") {
		t.Errorf("expected 25 data qubits, got:\n%s", qasm)
	}
}

func TestVerifyCodeProperties_Steane(t *testing.T) {
	code := sema.SteaneCode713()
	issues := VerifyCodeProperties(code)
	if len(issues) != 0 {
		t.Errorf("Steane code should have no issues, got: %v", issues)
	}
}

func TestVerifyCodeProperties_Shor(t *testing.T) {
	code := sema.ShorCode913()
	issues := VerifyCodeProperties(code)
	if len(issues) != 0 {
		t.Errorf("Shor code should have no issues, got: %v", issues)
	}
}

func TestVerifyCodeProperties_FiveQubit(t *testing.T) {
	code := sema.FiveQubitCode513()
	issues := VerifyCodeProperties(code)
	if len(issues) != 0 {
		t.Errorf("Five-qubit code should have no issues, got: %v", issues)
	}
}

func TestQEC_ErrorCorrectionSection(t *testing.T) {
	code := sema.FiveQubitCode513()
	gen := NewQECGenerator()
	qasm, err := gen.GenerateSyndromeCircuit(code, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(qasm, "Decode and Correct") {
		t.Error("expected decode and correct section")
	}
	// Five-qubit code has K=1, d=3 → should use single-error correction path
	if !strings.Contains(qasm, "Single-error correction") {
		t.Error("expected single-error correction path for [[5,1,3]]")
	}
}

func TestSurfaceCode_StabilizerCount(t *testing.T) {
	sc := sema.NewSurfaceCode(3)
	// d=3 rotated surface code: (d-1)^2/2 = 2 per type
	if len(sc.XStabilizers) == 0 {
		t.Error("expected X stabilizers for d=3")
	}
	if len(sc.ZStabilizers) == 0 {
		t.Error("expected Z stabilizers for d=3")
	}
	totalSyndromes := len(sc.XStabilizers) + len(sc.ZStabilizers)
	if sc.SyndromeQubits != totalSyndromes {
		t.Errorf("syndrome qubit count mismatch: %d vs %d", sc.SyndromeQubits, totalSyndromes)
	}
}
