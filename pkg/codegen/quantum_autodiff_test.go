package codegen

import (
	"karkain/pkg/parser"
	"math"
	"strings"
	"testing"
)

func TestQuantumAutodiff_OpenQASMEmitWithGrad(t *testing.T) {
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
			// Rx(θ_1) on q[1]
			&parser.QPUOpExpr{
				Op:    "rx",
				Args:  []parser.Node{&parser.QubitIndexExpr{Qubit: &parser.Identifier{Name: "q"}, Index: &parser.IntLiteral{Value: "1"}}},
				Angle: &parser.Float64Literal{Value: "0.0"},
			},
			// CX entangling gate
			&parser.QPUOpExpr{
				Op: "cx",
				Args: []parser.Node{
					&parser.QubitIndexExpr{Qubit: &parser.Identifier{Name: "q"}, Index: &parser.IntLiteral{Value: "0"}},
					&parser.QubitIndexExpr{Qubit: &parser.Identifier{Name: "q"}, Index: &parser.IntLiteral{Value: "1"}},
				},
			},
			// Rz(θ_2) on q[0]
			&parser.QPUOpExpr{
				Op:    "rz",
				Args:  []parser.Node{&parser.QubitIndexExpr{Qubit: &parser.Identifier{Name: "q"}, Index: &parser.IntLiteral{Value: "0"}}},
				Angle: &parser.Float64Literal{Value: "0.0"},
			},
			// Measure
			&parser.QPUOpExpr{
				Op: "measure",
				Args: []parser.Node{
					&parser.QubitIndexExpr{Qubit: &parser.Identifier{Name: "q"}, Index: &parser.IntLiteral{Value: "0"}},
					&parser.QubitIndexExpr{Qubit: &parser.Identifier{Name: "q"}, Index: &parser.IntLiteral{Value: "1"}},
				},
			},
		},
	}

	gen := NewOpenQASMParamShiftGenerator()
	qasm, err := gen.GenerateParamShiftProgram(circuit, 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify header
	if !strings.Contains(qasm, "OPENQASM 3.0;") {
		t.Error("expected OPENQASM 3.0 header")
	}

	// Verify parameter inputs
	if !strings.Contains(qasm, "input float theta[3];") {
		t.Errorf("expected 'input float theta[3];', got:\n%s", qasm)
	}

	// Verify qubit register
	if !strings.Contains(qasm, "qubit[2] q;") {
		t.Errorf("expected 'qubit[2] q;', got:\n%s", qasm)
	}

	// Verify parameterized gates use theta bindings
	if !strings.Contains(qasm, "ry(theta[0])") {
		t.Errorf("expected 'ry(theta[0])' parameter binding, got:\n%s", qasm)
	}
	if !strings.Contains(qasm, "rx(theta[1])") {
		t.Errorf("expected 'rx(theta[1])' parameter binding, got:\n%s", qasm)
	}
	if !strings.Contains(qasm, "rz(theta[2])") {
		t.Errorf("expected 'rz(theta[2])' parameter binding, got:\n%s", qasm)
	}

	// Verify non-parameterized gate
	if !strings.Contains(qasm, "cx ") {
		t.Errorf("expected CX gate, got:\n%s", qasm)
	}

	// Verify measurement
	if !strings.Contains(qasm, "measure") {
		t.Errorf("expected measurement, got:\n%s", qasm)
	}

	// Verify parameter-shift comments
	if !strings.Contains(qasm, "Parameter-shift gradient evaluation") {
		t.Error("expected parameter-shift gradient evaluation comments")
	}
	if !strings.Contains(qasm, "π/2") {
		t.Error("expected π/2 shift description in comments")
	}
}

func TestQuantumAutodiff_VQECodeGenWGSL(t *testing.T) {
	circuit := &parser.CircuitDecl{
		Name: "VQECircuit",
		Params: []parser.CircuitParam{
			{Name: "q", Type: &parser.QubitType{Size: 2}},
		},
		ReturnType: &parser.BitType{Size: 2},
		Body: []parser.Node{
			&parser.QPUOpExpr{
				Op:    "ry",
				Args:  []parser.Node{&parser.QubitIndexExpr{Qubit: &parser.Identifier{Name: "q"}, Index: &parser.IntLiteral{Value: "0"}}},
				Angle: &parser.Float64Literal{Value: "0.0"},
			},
			&parser.QPUOpExpr{
				Op: "cx",
				Args: []parser.Node{
					&parser.QubitIndexExpr{Qubit: &parser.Identifier{Name: "q"}, Index: &parser.IntLiteral{Value: "0"}},
					&parser.QubitIndexExpr{Qubit: &parser.Identifier{Name: "q"}, Index: &parser.IntLiteral{Value: "1"}},
				},
			},
			&parser.QPUOpExpr{
				Op:    "rz",
				Args:  []parser.Node{&parser.QubitIndexExpr{Qubit: &parser.Identifier{Name: "q"}, Index: &parser.IntLiteral{Value: "1"}}},
				Angle: &parser.Float64Literal{Value: "0.0"},
			},
		},
	}

	gen := NewQMLLoweringGenerator()
	wgsl, err := gen.GenerateVQELoop(circuit, HybridVQELoopConfig{
		Backend:       HybridBackendWGSL,
		MaxIter:       50,
		LearningRate:  0.1,
		Tolerance:     1e-4,
		WorkgroupSize: 64,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify VQE header
	if !strings.Contains(wgsl, "Phase 30") {
		t.Error("expected Phase 30 header")
	}
	if !strings.Contains(wgsl, "VQE Loop (WGSL)") {
		t.Error("expected VQE Loop (WGSL) in header")
	}

	// Verify VQE uniforms
	if !strings.Contains(wgsl, "struct VQEUniforms") {
		t.Error("expected VQEUniforms struct")
	}
	if !strings.Contains(wgsl, "iteration: u32") {
		t.Error("expected iteration field in VQEUniforms")
	}
	if !strings.Contains(wgsl, "learning_rate: f32") {
		t.Error("expected learning_rate field")
	}
	if !strings.Contains(wgsl, "tolerance: f32") {
		t.Error("expected tolerance field")
	}

	// Verify parameter buffer
	if !strings.Contains(wgsl, "var<storage, read_write> params: array<f32>") {
		t.Error("expected params storage buffer")
	}
	if !strings.Contains(wgsl, "var<storage, read_write> grads: array<f32>") {
		t.Error("expected grads storage buffer")
	}

	// Verify kernel functions
	if !strings.Contains(wgsl, "fn param_shift_eval(") {
		t.Error("expected param_shift_eval kernel")
	}
	if !strings.Contains(wgsl, "fn update_params(") {
		t.Error("expected update_params kernel")
	}
	if !strings.Contains(wgsl, "fn check_convergence(") {
		t.Error("expected check_convergence kernel")
	}

	// Verify π/2 shift constant
	if !strings.Contains(wgsl, "1.5707963") {
		t.Error("expected π/2 shift constant in param_shift_eval")
	}

	// Verify gradient formula
	if !strings.Contains(wgsl, "(e_plus - e_minus) / 2.0") {
		t.Error("expected (e_plus - e_minus) / 2.0 gradient formula")
	}

	// Verify gradient descent update
	if !strings.Contains(wgsl, "params[i] = params[i] - vqe.learning_rate * grads[i]") {
		t.Error("expected gradient descent update rule")
	}

	// Verify convergence check
	if !strings.Contains(wgsl, "max_grad") {
		t.Error("expected max_grad in convergence check")
	}
	if !strings.Contains(wgsl, "vqe.tolerance") {
		t.Error("expected tolerance comparison in convergence check")
	}
}

func TestQuantumAutodiff_VQECodeGenC99(t *testing.T) {
	circuit := &parser.CircuitDecl{
		Name: "SimpleRy",
		Params: []parser.CircuitParam{
			{Name: "q", Type: &parser.QubitType{Size: 1}},
		},
		Body: []parser.Node{
			&parser.QPUOpExpr{
				Op:    "ry",
				Args:  []parser.Node{&parser.Identifier{Name: "q"}},
				Angle: &parser.Float64Literal{Value: "0.0"},
			},
		},
	}

	gen := NewQMLLoweringGenerator()
	c99, err := gen.GenerateVQELoop(circuit, HybridVQELoopConfig{
		Backend:       HybridBackendC99,
		MaxIter:       20,
		LearningRate:  0.05,
		Tolerance:     1e-5,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify C99 header
	if !strings.Contains(c99, "Phase 30") {
		t.Error("expected Phase 30 header")
	}
	if !strings.Contains(c99, "VQE Loop (C99)") {
		t.Error("expected VQE Loop (C99) in header")
	}

	// Verify C99 includes
	if !strings.Contains(c99, "#include <math.h>") {
		t.Error("expected math.h include")
	}
	if !strings.Contains(c99, "#include <stdio.h>") {
		t.Error("expected stdio.h include")
	}

	// Verify constants
	if !strings.Contains(c99, "#define NUM_QUBITS 1") {
		t.Error("expected NUM_QUBITS 1")
	}
	if !strings.Contains(c99, "#define STATE_SIZE 2") {
		t.Error("expected STATE_SIZE 2 for 1 qubit")
	}
	if !strings.Contains(c99, "#define NUM_PARAMS 1") {
		t.Error("expected NUM_PARAMS 1")
	}
	if !strings.Contains(c99, "#define MAX_ITER 20") {
		t.Error("expected MAX_ITER 20")
	}

	// Verify complex type
	if !strings.Contains(c99, "typedef struct { double re; double im; } complex_t;") {
		t.Error("expected complex_t typedef")
	}

	// Verify circuit evaluation function
	if !strings.Contains(c99, "void eval_circuit(double *p)") {
		t.Error("expected eval_circuit function")
	}

	// Verify RY gate evaluation
	if !strings.Contains(c99, "apply_ry_c99(p[0], 0);") {
		t.Errorf("expected apply_ry_c99 with param binding, got:\n%s", c99)
	}

	// Verify gradient computation
	if !strings.Contains(c99, "void compute_gradients") {
		t.Error("expected compute_gradients function")
	}
	if !strings.Contains(c99, "PI / 2.0") {
		t.Error("expected PI/2 shift in gradient computation")
	}

	// Verify main VQE loop
	if !strings.Contains(c99, "for (int iter = 0; iter < MAX_ITER; iter++)") {
		t.Error("expected VQE iteration loop")
	}
	if !strings.Contains(c99, "params[i] -= LEARNING_RATE * grads[i]") {
		t.Error("expected gradient descent update in main loop")
	}
	if !strings.Contains(c99, "max_grad < TOLERANCE") {
		t.Error("expected convergence check")
	}
}

func TestQuantumAutodiff_QubitCountInOutput(t *testing.T) {
	circuit := &parser.CircuitDecl{
		Name: "FiveQubitCircuit",
		Params: []parser.CircuitParam{
			{Name: "q", Type: &parser.QubitType{Size: 5}},
		},
		Body: []parser.Node{
			&parser.QPUOpExpr{
				Op:    "ry",
				Args:  []parser.Node{&parser.Identifier{Name: "q"}},
				Angle: &parser.Float64Literal{Value: "0.0"},
			},
		},
	}

	gen := NewQMLLoweringGenerator()
	c99, err := gen.GenerateVQELoop(circuit, HybridVQELoopConfig{Backend: HybridBackendC99})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(c99, "#define NUM_QUBITS 5") {
		t.Error("expected 5 qubits in output")
	}
	if !strings.Contains(c99, "#define STATE_SIZE 32") {
		t.Error("expected STATE_SIZE 32 for 5 qubits")
	}

	// Also test WGSL output
	wgsl, err := gen.GenerateVQELoop(circuit, HybridVQELoopConfig{Backend: HybridBackendWGSL})
	if err != nil {
		t.Fatalf("unexpected WGSL error: %v", err)
	}
	if !strings.Contains(wgsl, "Qubits: 5") {
		t.Errorf("expected 5 qubits in WGSL header, got:\n%s", wgsl)
	}
}

func TestQuantumAutodiff_AnalyticalVsParameterShift(t *testing.T) {
	// Verify that the parameter-shift gradient matches finite-difference analytic gradient
	// For RY(θ), E = cos(θ), dE/dθ = -sin(θ)
	// Parameter-shift: (cos(θ+π/2) - cos(θ-π/2)) / 2 = (-sin(θ) - sin(θ)) / 2 = -sin(θ) ✓

	evaluator := func(params []float64) float64 {
		return math.Cos(params[0])
	}

	theta := math.Pi / 3.0

	// Parameter-shift gradient
	shift := math.Pi / 2.0
	ePlus := evaluator([]float64{theta + shift})
	eMinus := evaluator([]float64{theta - shift})
	paramShiftGrad := (ePlus - eMinus) / 2.0

	// Analytical gradient
	analyticGrad := -math.Sin(theta)

	if math.Abs(paramShiftGrad-analyticGrad) > 1e-10 {
		t.Errorf("parameter-shift gradient mismatch: got %f, expected %f", paramShiftGrad, analyticGrad)
	}

	// Also verify via finite differences
	eps := 1e-8
	fdGrad := (evaluator([]float64{theta + eps}) - evaluator([]float64{theta - eps})) / (2 * eps)
	if math.Abs(paramShiftGrad-fdGrad) > 1e-4 {
		t.Errorf("parameter-shift vs finite-difference mismatch: shift=%f, fd=%f", paramShiftGrad, fdGrad)
	}
}
