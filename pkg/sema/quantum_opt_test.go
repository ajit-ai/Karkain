package sema

import (
	"math"
	"testing"
)

func TestOpt_CircuitBasics(t *testing.T) {
	c := NewQCircuit(2)
	c.AddGate("h", 0)
	c.AddControlledGate("x", []int{0}, []int{1})

	if c.GateCount() != 2 {
		t.Errorf("expected 2 gates, got %d", c.GateCount())
	}
	if c.TwoQubitCount() != 1 {
		t.Errorf("expected 1 two-qubit gate, got %d", c.TwoQubitCount())
	}
	if c.Depth() < 1 {
		t.Errorf("expected depth >= 1, got %d", c.Depth())
	}
}

func TestOpt_CircuitClone(t *testing.T) {
	c := NewQCircuit(2)
	c.AddGate("h", 0)
	c.AddGate("cx", 0, 1)

	clone := c.Clone()
	clone.AddGate("x", 0)

	if c.GateCount() != 2 {
		t.Error("original should not be modified by clone mutation")
	}
	if clone.GateCount() != 3 {
		t.Errorf("clone should have 3 gates, got %d", clone.GateCount())
	}
}

func TestOpt_CancelAdjacentInverses(t *testing.T) {
	c := NewQCircuit(2)
	c.AddGate("h", 0)
	c.AddGate("h", 0)
	c.AddGate("x", 0)
	c.AddGate("x", 0)
	c.AddGate("cx", 0, 1)

	opt, cancelled := CancelAdjacentInverses(c)
	if cancelled != 2 {
		t.Errorf("expected 2 cancellations, got %d", cancelled)
	}
	if opt.GateCount() != 1 {
		t.Errorf("expected 1 gate after cancellation, got %d", opt.GateCount())
	}
}

func TestOpt_CancelRotationInverses(t *testing.T) {
	c := NewQCircuit(1)
	c.AddParamGate("rz", math.Pi/4, 0)
	c.AddParamGate("rz", -math.Pi/4, 0)

	opt, cancelled := CancelAdjacentInverses(c)
	if cancelled != 1 {
		t.Errorf("expected 1 rotation cancellation, got %d", cancelled)
	}
	if opt.GateCount() != 0 {
		t.Errorf("expected 0 gates after rotation cancellation, got %d", opt.GateCount())
	}
}

func TestOpt_NoCancelDifferentQubits(t *testing.T) {
	c := NewQCircuit(2)
	c.AddGate("h", 0)
	c.AddGate("h", 1) // different qubit — should NOT cancel

	_, cancelled := CancelAdjacentInverses(c)
	if cancelled != 0 {
		t.Errorf("expected 0 cancellations (different qubits), got %d", cancelled)
	}
}

func TestOpt_Commutation(t *testing.T) {
	h0 := QGate{Name: "h", Qubits: []int{0}}
	h1 := QGate{Name: "h", Qubits: []int{1}}
	if !CanCommute(h0, h1) {
		t.Error("H on q0 and H on q1 should commute (no overlap)")
	}

	x0 := QGate{Name: "x", Qubits: []int{0}}
	z0 := QGate{Name: "z", Qubits: []int{0}}
	if !CanCommute(x0, z0) {
		t.Error("X and Z should commute (both Pauli)")
	}

	cx01 := QGate{Name: "cx", Qubits: []int{0, 1}}
	x0g := QGate{Name: "x", Qubits: []int{0}}
	if CanCommute(cx01, x0g) {
		t.Error("CNOT(0,1) and X(0) should not commute in general")
	}
}

func TestOpt_DiagonalCommutation(t *testing.T) {
	z0 := QGate{Name: "z", Qubits: []int{0}}
	t0 := QGate{Name: "t", Qubits: []int{0}}
	if !CanCommute(z0, t0) {
		t.Error("Z and T should commute (both diagonal)")
	}

	s0 := QGate{Name: "s", Qubits: []int{0}}
	rz0 := QGate{Name: "rz", Qubits: []int{0}, Angle: 0.5}
	if !CanCommute(s0, rz0) {
		t.Error("S and Rz should commute (both diagonal)")
	}
}

func TestOpt_MergeRotations(t *testing.T) {
	c := NewQCircuit(1)
	c.AddParamGate("rz", math.Pi/4, 0)
	c.AddParamGate("rz", math.Pi/4, 0)
	c.AddParamGate("rz", math.Pi/2, 0)

	opt, merged := SynthesizeTDepth(c)
	if merged != 2 {
		t.Errorf("expected 2 merges, got %d", merged)
	}
	if opt.GateCount() != 1 {
		t.Errorf("expected 1 gate after merge, got %d", opt.GateCount())
	}
}

func TestOpt_MergeToZero(t *testing.T) {
	c := NewQCircuit(1)
	c.AddParamGate("rx", math.Pi, 0)
	c.AddParamGate("rx", math.Pi, 0) // sum = 2π ≡ 0

	opt, merged := SynthesizeTDepth(c)
	if merged != 1 {
		t.Errorf("expected 1 merge, got %d", merged)
	}
	if opt.GateCount() != 0 {
		t.Errorf("expected 0 gates (merged to zero), got %d", opt.GateCount())
	}
}

func TestOpt_CompileToNativeIBM(t *testing.T) {
	c := NewQCircuit(1)
	c.AddGate("h", 0)
	c.AddGate("t", 0)
	c.AddGate("s", 0)

	native := IBMNativeGates()
	opt, added := CompileToNative(c, native)
	if added < 1 {
		t.Errorf("expected gates added, got %d", added)
	}

	// All output gates should be native
	for _, g := range opt.Gates {
		isNative := false
		for _, ng := range native.Gates {
			if g.Name == ng {
				isNative = true
				break
			}
		}
		if !isNative {
			t.Errorf("gate %s is not in native set %v", g.Name, native.Gates)
		}
	}
}

func TestOpt_FullPipeline(t *testing.T) {
	c := NewQCircuit(2)
	// Add some redundant gates
	c.AddGate("h", 0)
	c.AddGate("h", 0)            // cancel
	c.AddGate("x", 0)
	c.AddGate("x", 0)            // cancel
	c.AddGate("h", 1)
	c.AddParamGate("rz", math.Pi/4, 0)
	c.AddParamGate("rz", math.Pi/4, 0)

	cfg := DefaultOptConfig()
	result := OptimizeCircuit(c, cfg)

	if result.OptimizedGateCount >= c.GateCount() {
		t.Errorf("expected optimization to reduce gates: %d → %d",
			c.GateCount(), result.OptimizedGateCount)
	}
	if result.GatesRemoved == 0 {
		t.Error("expected some gates to be removed")
	}
	if result.OriginalDepth != c.Depth() {
		t.Error("original depth should match")
	}
}

func TestOpt_FullPipelineWithNative(t *testing.T) {
	c := NewQCircuit(1)
	c.AddGate("h", 0)
	c.AddGate("h", 0)
	c.AddGate("t", 0)

	cfg := OptConfig{
		MaxIterations:   3,
		CancelInverses:  true,
		MergeRotations:  true,
		CompileToNative: true,
		NativeGates:     IBMNativeGates(),
		ApplyTemplates:  true,
	}
	result := OptimizeCircuit(c, cfg)

	// After cancellation, we should have 1 gate (t), then compiled to native
	for _, g := range result.Optimized.Gates {
		isNative := false
		for _, ng := range cfg.NativeGates.Gates {
			if g.Name == ng {
				isNative = true
				break
			}
		}
		if !isNative {
			t.Errorf("gate %s should be native after compilation", g.Name)
		}
	}
}

func TestOpt_CircuitStats(t *testing.T) {
	c := NewQCircuit(3)
	c.AddGate("h", 0)
	c.AddGate("cx", 0, 1)
	c.AddGate("rz", 2)
	c.AddGate("t", 0)
	c.AddGate("x", 2)
	c.AddControlledGate("x", []int{0}, []int{2})

	stats := ComputeStats(c)
	if stats.TotalGates != 6 {
		t.Errorf("expected 6 total gates, got %d", stats.TotalGates)
	}
	if stats.TwoQubitGates != 2 {
		t.Errorf("expected 2 two-qubit gates, got %d", stats.TwoQubitGates)
	}
	if stats.TGateCount != 1 {
		t.Errorf("expected 1 T gate, got %d", stats.TGateCount)
	}
	if stats.GateCounts["h"] != 1 {
		t.Errorf("expected 1 H gate, got %d", stats.GateCounts["h"])
	}
}

func TestOpt_IsInversePair(t *testing.T) {
	h0a := QGate{Name: "h", Qubits: []int{0}}
	h0b := QGate{Name: "h", Qubits: []int{0}}
	if !IsInversePair(h0a, h0b) {
		t.Error("H-H should be inverse pair")
	}

	h0 := QGate{Name: "h", Qubits: []int{0}}
	h1 := QGate{Name: "h", Qubits: []int{1}}
	if IsInversePair(h0, h1) {
		t.Error("H(0)-H(1) should not be inverse pair")
	}

	x0a := QGate{Name: "x", Qubits: []int{0}}
	x0b := QGate{Name: "x", Qubits: []int{0}}
	if !IsInversePair(x0a, x0b) {
		t.Error("X-X should be inverse pair")
	}

	cx01a := QGate{Name: "cx", Qubits: []int{0, 1}}
	cx01b := QGate{Name: "cx", Qubits: []int{0, 1}}
	if !IsInversePair(cx01a, cx01b) {
		t.Error("CX-CX should be inverse pair")
	}
}

func TestOpt_NormalizeAngle(t *testing.T) {
	tests := []struct {
		input, expected float64
	}{
		{0, 0},
		{2 * math.Pi, 0},
		{math.Pi, math.Pi},
		{3 * math.Pi, math.Pi},
		{-math.Pi, math.Pi},
	}
	for _, tt := range tests {
		got := normalizeAngle(tt.input)
		if math.Abs(got-tt.expected) > 1e-10 {
			t.Errorf("normalizeAngle(%.4f) = %.4f, want %.4f", tt.input, got, tt.expected)
		}
	}
}

func TestOpt_NativeGateSets(t *testing.T) {
	ibm := IBMNativeGates()
	if ibm.Name != "ibm" {
		t.Error("expected ibm native gate set")
	}
	if len(ibm.Gates) == 0 {
		t.Error("expected non-empty IBM gate set")
	}

	google := GoogleNativeGates()
	if google.Name != "google" {
		t.Error("expected google native gate set")
	}

	ionq := IonQNativeGates()
	if ionq.Name != "ionq" {
		t.Error("expected ionq native gate set")
	}
}

func TestOpt_IdempotentOptimization(t *testing.T) {
	c := NewQCircuit(2)
	c.AddGate("h", 0)
	c.AddGate("h", 0)
	c.AddGate("x", 1)

	cfg := DefaultOptConfig()
	r1 := OptimizeCircuit(c, cfg)
	r2 := OptimizeCircuit(r1.Optimized, cfg)

	if r1.OptimizedGateCount != r2.OptimizedGateCount {
		t.Errorf("optimization should be idempotent: %d vs %d",
			r1.OptimizedGateCount, r2.OptimizedGateCount)
	}
}
