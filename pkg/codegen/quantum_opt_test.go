package codegen

import (
	"karkain/pkg/sema"
	"strings"
	"testing"
)

func TestQOpt_GenerateCircuit(t *testing.T) {
	c := sema.NewQCircuit(3)
	c.AddGate("h", 0)
	c.AddControlledGate("x", []int{0}, []int{1})
	c.AddControlledGate("x", []int{1}, []int{2})

	gen := NewQOptGenerator()
	qasm := gen.GenerateCircuit(c)

	if !strings.Contains(qasm, "OPENQASM 3.0;") {
		t.Error("expected OPENQASM header")
	}
	if !strings.Contains(qasm, "qubit[3] q;") {
		t.Errorf("expected 3 qubits, got:\n%s", qasm)
	}
	if !strings.Contains(qasm, "h q[0];") {
		t.Errorf("expected h q[0], got:\n%s", qasm)
	}
	if !strings.Contains(qasm, "cx q[0], q[1];") {
		t.Errorf("expected cx q[0], q[1], got:\n%s", qasm)
	}
}

func TestQOpt_GenerateParameterizedCircuit(t *testing.T) {
	c := sema.NewQCircuit(1)
	c.AddParamGate("rz", 1.5708, 0)

	gen := NewQOptGenerator()
	qasm := gen.GenerateCircuit(c)

	if !strings.Contains(qasm, "rz(1.570800) q[0];") {
		t.Errorf("expected parameterized rz gate, got:\n%s", qasm)
	}
}

func TestQOpt_OptimizationReport(t *testing.T) {
	c := sema.NewQCircuit(2)
	c.AddGate("h", 0)
	c.AddGate("h", 0)

	cfg := sema.DefaultOptConfig()
	result := sema.OptimizeCircuit(c, cfg)

	gen := NewQOptGenerator()
	report := gen.GenerateOptReport(result)

	if !strings.Contains(report, "Gates removed:") {
		t.Error("expected gates removed in report")
	}
	if !strings.Contains(report, "Depth reduction:") {
		t.Error("expected depth reduction in report")
	}
}

func TestQOpt_StatsReport(t *testing.T) {
	c := sema.NewQCircuit(2)
	c.AddGate("h", 0)
	c.AddGate("cx", 0, 1)
	c.AddGate("t", 0)

	gen := NewQOptGenerator()
	report := gen.GenerateCircuitStatsReport(c)

	if !strings.Contains(report, "Circuit Statistics") {
		t.Error("expected circuit statistics header")
	}
	if !strings.Contains(report, "Two-qubit gates:") {
		t.Error("expected two-qubit gate count")
	}
}

func TestQOpt_ControlledRotation(t *testing.T) {
	c := sema.NewQCircuit(2)
	c.AddControlledGate("rz", []int{0}, []int{1})

	gen := NewQOptGenerator()
	qasm := gen.GenerateCircuit(c)

	if !strings.Contains(qasm, "crz") {
		t.Errorf("expected crz gate, got:\n%s", qasm)
	}
}

func TestQOpt_OptimizedToNative(t *testing.T) {
	c := sema.NewQCircuit(1)
	c.AddGate("h", 0)
	c.AddGate("h", 0) // cancel
	c.AddGate("t", 0)

	cfg := sema.OptConfig{
		MaxIterations:   3,
		CancelInverses:  true,
		MergeRotations:  true,
		CompileToNative: true,
		NativeGates:     sema.IBMNativeGates(),
	}
	result := sema.OptimizeCircuit(c, cfg)

	gen := NewQOptGenerator()
	qasm := gen.GenerateCircuit(result.Optimized)

	// Should only contain IBM native gates (id, rz, sx, x, cx)
	lines := strings.Split(qasm, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "//") || strings.HasPrefix(line, "OPENQASM") ||
			strings.HasPrefix(line, "include") || strings.HasPrefix(line, "qubit") ||
			strings.HasPrefix(line, "bit") {
			continue
		}
		// Check if gate is native
		isNative := false
		for _, ng := range cfg.NativeGates.Gates {
			if strings.HasPrefix(line, ng+" ") || strings.HasPrefix(line, ng+"(") {
				isNative = true
				break
			}
		}
		if !isNative {
			t.Errorf("non-native gate in output: %s", line)
		}
	}
}
