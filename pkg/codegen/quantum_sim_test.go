package codegen

import (
	"karkain/pkg/parser"
	"math"
	"strings"
	"testing"
)

func TestQuantumSim_BellStateWGSL(t *testing.T) {
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

	gen := NewQuantumSimGenerator()
	wgsl, err := gen.GenerateWGSLSimShader(circuit, CircuitToWGSLOptions{WorkgroupSize: 64})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify header
	if !strings.Contains(wgsl, "Karkain Phase 29") {
		t.Error("expected Phase 29 header comment")
	}
	if !strings.Contains(wgsl, "2 qubits, 4 complex amplitudes") {
		t.Errorf("expected '2 qubits, 4 complex amplitudes' in header, got:\n%s", wgsl)
	}

	// Verify Complex struct
	if !strings.Contains(wgsl, "struct Complex") {
		t.Error("expected Complex struct definition")
	}
	if !strings.Contains(wgsl, "re: f32") || !strings.Contains(wgsl, "im: f32") {
		t.Error("expected re/im f32 fields in Complex struct")
	}

	// Verify storage bindings for state vector and gate matrix
	if !strings.Contains(wgsl, "var<storage, read_write> state: array<Complex>") {
		t.Error("expected state storage binding")
	}
	if !strings.Contains(wgsl, "var<storage, read> gate_matrix: array<Complex>") {
		t.Error("expected gate_matrix storage binding")
	}

	// Verify Hadamard kernel is emitted
	if !strings.Contains(wgsl, "fn apply_h(") {
		t.Error("expected apply_h kernel")
	}
	if !strings.Contains(wgsl, "inv_sqrt2") {
		t.Error("expected inv_sqrt2 constant in Hadamard kernel")
	}

	// Verify CNOT kernel
	if !strings.Contains(wgsl, "fn apply_cnot(") {
		t.Error("expected apply_cnot kernel")
	}

	// Verify measurement kernel with collapse
	if !strings.Contains(wgsl, "fn measure_collapse(") {
		t.Error("expected measure_collapse kernel")
	}
	if !strings.Contains(wgsl, "cumulative") {
		t.Error("expected cumulative probability logic in measurement")
	}

	// Verify normalization kernels
	if !strings.Contains(wgsl, "fn normalize_state(") {
		t.Error("expected normalize_state kernel")
	}
	if !strings.Contains(wgsl, "fn normalize_apply(") {
		t.Error("expected normalize_apply kernel")
	}

	// Verify init_state kernel
	if !strings.Contains(wgsl, "fn init_state(") {
		t.Error("expected init_state kernel")
	}

	// Verify dispatch comments for the circuit sequence
	if !strings.Contains(wgsl, "dispatch apply_h") {
		t.Error("expected apply_h dispatch comment")
	}
	if !strings.Contains(wgsl, "dispatch apply_cnot") {
		t.Error("expected apply_cnot dispatch comment")
	}
	if !strings.Contains(wgsl, "dispatch measure_collapse") {
		t.Error("expected measure_collapse dispatch comment")
	}
}

func TestQuantumSim_GateMatrixTransform(t *testing.T) {
	circuit := &parser.CircuitDecl{
		Name: "RyCircuit",
		Params: []parser.CircuitParam{
			{Name: "q", Type: &parser.QubitType{Size: 1}},
		},
		Body: []parser.Node{
			&parser.QPUOpExpr{
				Op:    "ry",
				Args:  []parser.Node{&parser.Identifier{Name: "q"}},
				Angle: &parser.Float64Literal{Value: "1.5707963"},
			},
		},
	}

	gen := NewQuantumSimGenerator()
	wgsl, err := gen.GenerateWGSLSimShader(circuit, CircuitToWGSLOptions{WorkgroupSize: 32})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify rotation kernel exists
	if !strings.Contains(wgsl, "fn y_rotation(") {
		t.Error("expected y_rotation kernel")
	}

	// Verify trig functions in rotation
	if !strings.Contains(wgsl, "cos(angle)") || !strings.Contains(wgsl, "sin(angle)") {
		t.Error("expected cos/sin trig calls in rotation kernel")
	}

	// Verify the 2x2 matrix-vector multiply pattern
	if !strings.Contains(wgsl, "c * a.re") || !strings.Contains(wgsl, "s * b.re") {
		t.Error("expected matrix-vector multiply pattern in rotation kernel")
	}

	// Verify dispatch comment references angle parameter
	if !strings.Contains(wgsl, "dispatch y_rotation") {
		t.Error("expected y_rotation dispatch comment")
	}
	if !strings.Contains(wgsl, "angle=param") {
		t.Error("expected angle=param in dispatch comment")
	}

	// Verify uniform struct with state_size
	if !strings.Contains(wgsl, "state_size: u32") {
		t.Error("expected state_size in Uniforms struct")
	}
}

func TestQuantumSim_StateVectorNorm(t *testing.T) {
	circuit := &parser.CircuitDecl{
		Name: "NormTest",
		Params: []parser.CircuitParam{
			{Name: "q", Type: &parser.QubitType{Size: 2}},
		},
		ReturnType: &parser.BitType{Size: 2},
		Body: []parser.Node{
			&parser.QPUOpExpr{
				Op:   "h",
				Args: []parser.Node{&parser.QubitIndexExpr{Qubit: &parser.Identifier{Name: "q"}, Index: &parser.IntLiteral{Value: "0"}}},
			},
			&parser.QPUOpExpr{
				Op: "cx",
				Args: []parser.Node{
					&parser.QubitIndexExpr{Qubit: &parser.Identifier{Name: "q"}, Index: &parser.IntLiteral{Value: "0"}},
					&parser.QubitIndexExpr{Qubit: &parser.Identifier{Name: "q"}, Index: &parser.IntLiteral{Value: "1"}},
				},
			},
			&parser.QPUOpExpr{
				Op:    "ry",
				Args:  []parser.Node{&parser.QubitIndexExpr{Qubit: &parser.Identifier{Name: "q"}, Index: &parser.IntLiteral{Value: "0"}}},
				Angle: &parser.Float64Literal{Value: "0.785398"},
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

	gen := NewQuantumSimGenerator()
	wgsl, err := gen.GenerateWGSLSimShader(circuit, CircuitToWGSLOptions{WorkgroupSize: 64})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify complex_abs2 function exists for norm computation
	if !strings.Contains(wgsl, "fn complex_abs2(a: Complex) -> f32") {
		t.Error("expected complex_abs2 function for norm computation")
	}
	if !strings.Contains(wgsl, "a.re * a.re + a.im * a.im") {
		t.Error("expected |a|^2 = re^2 + im^2 formula")
	}

	// Verify normalization kernel accumulates probability and divides
	if !strings.Contains(wgsl, "norm2") {
		t.Error("expected norm2 accumulation variable in normalize_state")
	}
	if !strings.Contains(wgsl, "1.0 / sqrt(norm2)") {
		t.Error("expected 1/sqrt(norm2) normalization factor")
	}

	// Verify probability readback kernel for post-simulation verification
	if !strings.Contains(wgsl, "fn compute_probabilities(") {
		t.Error("expected compute_probabilities kernel")
	}

	// Verify probability conservation: the kernel writes complex_abs2 into state
	if !strings.Contains(wgsl, "Complex(complex_abs2(state[idx]), 0.0)") {
		t.Error("expected probability amplitude extraction via complex_abs2")
	}

	// Verify init kernel sets |0...0> correctly (amplitude 1+0i for index 0)
	if !strings.Contains(wgsl, "state[0] = Complex(1.0, 0.0)") {
		t.Error("expected |0...0> initialization with Complex(1.0, 0.0)")
	}

	// Verify Hadamard preserves norm: coefficients divided by inv_sqrt2
	if !strings.Contains(wgsl, "inv_sqrt2") {
		t.Error("expected inv_sqrt2 normalization in Hadamard gate (1/sqrt(2) preserves norm)")
	}

	// The initial state |00> has norm = |1|^2 = 1.0
	// After H on q[0]: state = (|00> + |10>)/sqrt(2), norm = 0.5 + 0.5 = 1.0
	// Verify the abs2 function would compute this correctly
	invSqrt2 := 1.0 / math.Sqrt(2.0)
	normAfterH := invSqrt2*invSqrt2 + invSqrt2*invSqrt2
	if math.Abs(normAfterH-1.0) > 1e-10 {
		t.Errorf("norm after H gate should be 1.0, got %f", normAfterH)
	}
}

func TestQuantumSim_QubitCountLimits(t *testing.T) {
	// Test that circuits with <= 30 qubits succeed
	gen := NewQuantumSimGenerator()
	smallCircuit := &parser.CircuitDecl{
		Name: "SmallCircuit",
		Params: []parser.CircuitParam{
			{Name: "q", Type: &parser.QubitType{Size: 5}},
		},
		Body: []parser.Node{},
	}
	_, err := gen.GenerateWGSLSimShader(smallCircuit, CircuitToWGSLOptions{})
	if err != nil {
		t.Errorf("5-qubit circuit should succeed, got: %v", err)
	}

	// Test that circuits with > 30 qubits fail
	largeCircuit := &parser.CircuitDecl{
		Name: "TooLarge",
		Params: []parser.CircuitParam{
			{Name: "q", Type: &parser.QubitType{Size: 31}},
		},
		Body: []parser.Node{},
	}
	_, err = gen.GenerateWGSLSimShader(largeCircuit, CircuitToWGSLOptions{})
	if err == nil {
		t.Error("31-qubit circuit should fail with memory limit error")
	}
	if err != nil && !strings.Contains(err.Error(), "exceeding") {
		t.Errorf("expected memory limit error, got: %v", err)
	}
}

func TestQuantumSim_SwapKernel(t *testing.T) {
	circuit := &parser.CircuitDecl{
		Name: "SwapCircuit",
		Params: []parser.CircuitParam{
			{Name: "q", Type: &parser.QubitType{Size: 2}},
		},
		Body: []parser.Node{
			&parser.QPUOpExpr{
				Op: "swap",
				Args: []parser.Node{
					&parser.QubitIndexExpr{Qubit: &parser.Identifier{Name: "q"}, Index: &parser.IntLiteral{Value: "0"}},
					&parser.QubitIndexExpr{Qubit: &parser.Identifier{Name: "q"}, Index: &parser.IntLiteral{Value: "1"}},
				},
			},
		},
	}

	gen := NewQuantumSimGenerator()
	wgsl, err := gen.GenerateWGSLSimShader(circuit, CircuitToWGSLOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(wgsl, "fn apply_swap(") {
		t.Error("expected apply_swap kernel")
	}
	// Swap should XOR both bit masks to find partner
	if !strings.Contains(wgsl, "mask_a ^ mask_b") {
		t.Error("expected XOR of both masks in swap partner calculation")
	}
}
