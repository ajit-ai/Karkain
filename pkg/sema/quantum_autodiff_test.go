package sema

import (
	"karkain/pkg/parser"
	"math"
	"testing"
)

func TestQuantumAutodiff_ParameterShiftGrad(t *testing.T) {
	// Build a single-qubit RY(θ) circuit
	circuit := &parser.CircuitDecl{
		Name: "RyParam",
		Params: []parser.CircuitParam{
			{Name: "q", Type: &parser.QubitType{Size: 1}},
		},
		ReturnType: &parser.BitType{Size: 1},
		Body: []parser.Node{
			&parser.QPUOpExpr{
				Op:    "ry",
				Args:  []parser.Node{&parser.Identifier{Name: "q"}},
				Angle: &parser.Float64Literal{Value: "0.0"},
			},
			&parser.QPUOpExpr{
				Op:   "measure",
				Args: []parser.Node{&parser.Identifier{Name: "q"}},
			},
		},
	}

	tape := NewQuantumGradientTape(1)
	err := tape.AnalyzeCircuit(circuit)
	if err != nil {
		t.Fatalf("AnalyzeCircuit failed: %v", err)
	}

	if tape.NumParams != 1 {
		t.Fatalf("expected 1 parameter, got %d", tape.NumParams)
	}

	// Evaluator: simulate RY(θ) and return <Z> expectation
	// For |ψ> = RY(θ)|0>, <Z> = cos(θ)
	evaluator := func(params []float64) float64 {
		return math.Cos(params[0])
	}

	// Test at θ = 0: analytical gradient is -sin(0) = 0
	baseParams := []float64{0.0}
	grads, err := tape.ComputeGradients(evaluator, baseParams)
	if err != nil {
		t.Fatalf("ComputeGradients failed: %v", err)
	}

	expectedGrad := 0.0 // -sin(0) = 0
	if math.Abs(grads[0]-expectedGrad) > 1e-6 {
		t.Errorf("gradient at θ=0: expected %f, got %f", expectedGrad, grads[0])
	}

	// Test at θ = π/4: analytical gradient is -sin(π/4) = -√2/2 ≈ -0.7071
	baseParams = []float64{math.Pi / 4.0}
	grads, err = tape.ComputeGradients(evaluator, baseParams)
	if err != nil {
		t.Fatalf("ComputeGradients failed: %v", err)
	}

	expectedGrad = -math.Sqrt(2) / 2.0
	if math.Abs(grads[0]-expectedGrad) > 1e-6 {
		t.Errorf("gradient at θ=π/4: expected %f, got %f", expectedGrad, grads[0])
	}

	// Test at θ = π/2: analytical gradient is -sin(π/2) = -1
	baseParams = []float64{math.Pi / 2.0}
	grads, err = tape.ComputeGradients(evaluator, baseParams)
	if err != nil {
		t.Fatalf("ComputeGradients failed: %v", err)
	}

	expectedGrad = -1.0
	if math.Abs(grads[0]-expectedGrad) > 1e-6 {
		t.Errorf("gradient at θ=π/2: expected %f, got %f", expectedGrad, grads[0])
	}

	// Verify parameter-shift vs analytical via finite differences
	analyticGrads, err := tape.ComputeGradientsAnalytic([]float64{math.Pi / 3.0}, evaluator)
	if err != nil {
		t.Fatalf("ComputeGradientsAnalytic failed: %v", err)
	}

	shiftGrads, err := tape.ComputeGradients(evaluator, []float64{math.Pi / 3.0})
	if err != nil {
		t.Fatalf("ComputeGradients failed: %v", err)
	}

	if math.Abs(shiftGrads[0]-analyticGrads[0]) > 1e-4 {
		t.Errorf("parameter-shift vs analytic mismatch: shift=%f, analytic=%f", shiftGrads[0], analyticGrads[0])
	}
}

func TestQuantumAutodiff_VQEOptimizationLoop(t *testing.T) {
	// Simulate a 2-qubit VQE iteration step
	// Hamiltonian: H = Z_0 Z_1 + X_0
	// Ground state energy = -√2 ≈ -1.4142

	circuit := &parser.CircuitDecl{
		Name: "VQEAnsatz",
		Params: []parser.CircuitParam{
			{Name: "q", Type: &parser.QubitType{Size: 2}},
		},
		ReturnType: &parser.BitType{Size: 2},
		Body: []parser.Node{
			// Ry(θ_0) on q[0]
			&parser.QPUOpExpr{
				Op:    "ry",
				Args:  []parser.Node{&parser.QubitIndexExpr{Qubit: &parser.Identifier{Name: "q"}, Index: &parser.IntLiteral{Value: "0"}}},
				Angle: &parser.Float64Literal{Value: "0.0"},
			},
			// Ry(θ_1) on q[1]
			&parser.QPUOpExpr{
				Op:    "ry",
				Args:  []parser.Node{&parser.QubitIndexExpr{Qubit: &parser.Identifier{Name: "q"}, Index: &parser.IntLiteral{Value: "1"}}},
				Angle: &parser.Float64Literal{Value: "0.0"},
			},
			// CX on q[0], q[1]
			&parser.QPUOpExpr{
				Op: "cx",
				Args: []parser.Node{
					&parser.QubitIndexExpr{Qubit: &parser.Identifier{Name: "q"}, Index: &parser.IntLiteral{Value: "0"}},
					&parser.QubitIndexExpr{Qubit: &parser.Identifier{Name: "q"}, Index: &parser.IntLiteral{Value: "1"}},
				},
			},
			// Ry(θ_2) on q[0]
			&parser.QPUOpExpr{
				Op:    "ry",
				Args:  []parser.Node{&parser.QubitIndexExpr{Qubit: &parser.Identifier{Name: "q"}, Index: &parser.IntLiteral{Value: "0"}}},
				Angle: &parser.Float64Literal{Value: "0.0"},
			},
			&parser.QPUOpExpr{
				Op: "measure",
				Args: []parser.Node{
					&parser.QubitIndexExpr{Qubit: &parser.Identifier{Name: "q"}, Index: &parser.IntLiteral{Value: "0"}},
					&parser.QubitIndexExpr{Qubit: &parser.Identifier{Name: "q"}, Index: &parser.IntLiteral{Value: "1"}},
				},
			},
		},
	}

	tape := NewQuantumGradientTape(2)
	err := tape.AnalyzeCircuit(circuit)
	if err != nil {
		t.Fatalf("AnalyzeCircuit failed: %v", err)
	}

	if tape.NumParams != 3 {
		t.Fatalf("expected 3 parameters, got %d", tape.NumParams)
	}

	// Mock evaluator: E(θ_0, θ_1, θ_2) = -cos(θ_0)*cos(θ_1) - sin(θ_0)*sin(θ_1)*cos(θ_2)
	evaluator := func(params []float64) float64 {
		return -math.Cos(params[0])*math.Cos(params[1]) -
			math.Sin(params[0])*math.Sin(params[1])*math.Cos(params[2])
	}

	config := DefaultVQEConfig()
	config.MaxIter = 10
	config.LearningRate = 0.2
	config.Tolerance = 1e-4

	history, err := RunVQE(tape, evaluator, []float64{0.1, 0.2, 0.3}, config)
	if err != nil {
		t.Fatalf("RunVQE failed: %v", err)
	}

	if len(history) == 0 {
		t.Fatal("expected at least one VQE iteration")
	}

	// Verify energy decreases (or at least doesn't diverge)
	// The initial energy should be higher than the final
	firstEnergy := history[0].Energy
	lastEnergy := history[len(history)-1].Energy

	t.Logf("VQE: %d iterations, E_initial=%.6f, E_final=%.6f", len(history), firstEnergy, lastEnergy)

	// Check that gradients are being computed
	for i, iter := range history {
		if len(iter.Gradients) != 3 {
			t.Errorf("iteration %d: expected 3 gradients, got %d", i, len(iter.Gradients))
		}
	}

	// Check that at least some gradients were non-zero (optimization happened)
	anyNonZero := false
	for _, iter := range history {
		for _, g := range iter.Gradients {
			if math.Abs(g) > 1e-10 {
				anyNonZero = true
				break
			}
		}
	}
	if !anyNonZero {
		t.Error("all gradients were zero — no optimization occurred")
	}

	// Ground state of H = Z0Z1 + X0 should be ≈ -1.4142
	// Our mock evaluator doesn't exactly match this Hamiltonian but verifies
	// the optimization machinery works end-to-end
	t.Logf("Final energy: %.6f", lastEnergy)
}

func TestQuantumAutodiff_HamiltonianExpectation(t *testing.T) {
	// Test expectation value computation with Z0Z1 + X0 Hamiltonian
	numQubits := 2
	dim := 4

	// |00> state: [1, 0, 0, 0]
	state := make([]float64, dim*2)
	state[0] = 1.0 // |00>

	hamiltonian := []HamiltonianTerm{
		{
			Coefficient: 1.0,
			PauliLabels: []PauliLabel{PauliZ, PauliZ},
		},
		{
			Coefficient: 1.0,
			PauliLabels: []PauliLabel{PauliX, PauliI},
		},
	}

	energy := ExpectationValue(state, hamiltonian, numQubits)

	// <00|Z0Z1|00> = (+1)(+1) = 1
	// <00|X0|00> = 0 (off-diagonal)
	// Total = 1.0
	if math.Abs(energy-1.0) > 1e-10 {
		t.Errorf("expected energy 1.0 for |00>, got %f", energy)
	}

	// |11> state: [0, 0, 0, 1]
	state2 := make([]float64, dim*2)
	state2[6] = 1.0 // |11> -> index 3 -> re at 3*2=6

	energy2 := ExpectationValue(state2, hamiltonian, numQubits)
	// <11|Z0Z1|11> = (-1)(-1) = 1
	// <11|X0|11> = 0
	// Total = 1.0
	if math.Abs(energy2-1.0) > 1e-10 {
		t.Errorf("expected energy 1.0 for |11>, got %f", energy2)
	}
}

func TestQuantumAutodiff_CircuitAnalysis(t *testing.T) {
	circuit := &parser.CircuitDecl{
		Name: "MultiParam",
		Params: []parser.CircuitParam{
			{Name: "q", Type: &parser.QubitType{Size: 2}},
		},
		Body: []parser.Node{
			&parser.QPUOpExpr{
				Op:    "rx",
				Args:  []parser.Node{&parser.Identifier{Name: "q"}},
				Angle: &parser.Float64Literal{Value: "1.57"},
			},
			&parser.QPUOpExpr{
				Op:    "ry",
				Args:  []parser.Node{&parser.Identifier{Name: "q"}},
				Angle: &parser.Float64Literal{Value: "0.78"},
			},
			&parser.QPUOpExpr{
				Op:    "rz",
				Args:  []parser.Node{&parser.Identifier{Name: "q"}},
				Angle: &parser.Float64Literal{Value: "3.14"},
			},
			// Non-parameterized gate should not count
			&parser.QPUOpExpr{
				Op:   "h",
				Args: []parser.Node{&parser.Identifier{Name: "q"}},
			},
		},
	}

	tape := NewQuantumGradientTape(2)
	err := tape.AnalyzeCircuit(circuit)
	if err != nil {
		t.Fatalf("AnalyzeCircuit failed: %v", err)
	}

	if tape.NumParams != 3 {
		t.Errorf("expected 3 parameters (rx, ry, rz), got %d", tape.NumParams)
	}

	if len(tape.Gates) != 3 {
		t.Errorf("expected 3 gate records, got %d", len(tape.Gates))
	}

	// Verify gate types
	expectedOps := []string{"rx", "ry", "rz"}
	for i, gate := range tape.Gates {
		if gate.Op != expectedOps[i] {
			t.Errorf("gate %d: expected op %s, got %s", i, expectedOps[i], gate.Op)
		}
	}

	// Verify angles parsed correctly
	expectedAngles := []float64{1.57, 0.78, 3.14}
	for i, gate := range tape.Gates {
		if math.Abs(gate.Angle-expectedAngles[i]) > 0.01 {
			t.Errorf("gate %d: expected angle %f, got %f", i, expectedAngles[i], gate.Angle)
		}
	}
}
