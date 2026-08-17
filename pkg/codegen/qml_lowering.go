package codegen

import (
	"fmt"
	"karkain/pkg/parser"
	"strings"
)

// ============================================================
// Quantum-Classical Hybrid Optimizer — lowers parameterized
// quantum circuits into WGSL compute or C99 evaluation loops
// for VQE / QNN training.
// ============================================================

// HybridBackend selects the target for VQE loop codegen
type HybridBackend int

const (
	HybridBackendWGSL HybridBackend = iota
	HybridBackendC99
)

// HybridVQELoopConfig configures code generation for a hybrid VQE loop
type HybridVQELoopConfig struct {
	Backend       HybridBackend
	MaxIter       int
	LearningRate  float64
	Tolerance     float64
	NumParams     int
	NumQubits     int
	CircuitName   string
	WorkgroupSize int
}

// DefaultHybridConfig returns sensible defaults
func DefaultHybridConfig() HybridVQELoopConfig {
	return HybridVQELoopConfig{
		Backend:       HybridBackendWGSL,
		MaxIter:       100,
		LearningRate:  0.1,
		Tolerance:     1e-6,
		WorkgroupSize: 64,
	}
}

// QMLLoweringGenerator generates hybrid quantum-classical optimizer code
type QMLLoweringGenerator struct {
	buf    strings.Builder
	errors []string
}

func NewQMLLoweringGenerator() *QMLLoweringGenerator {
	return &QMLLoweringGenerator{}
}

// GenerateVQELoop produces a full VQE optimization loop in the target backend
func (g *QMLLoweringGenerator) GenerateVQELoop(circuit *parser.CircuitDecl, config HybridVQELoopConfig) (string, error) {
	g.buf.Reset()
	g.errors = nil

	config.NumQubits = countCircuitQubits(circuit)
	config.CircuitName = circuit.Name
	if config.NumParams == 0 {
		config.NumParams = countParameterizedGates(circuit)
	}

	switch config.Backend {
	case HybridBackendWGSL:
		g.emitWGSLLoop(circuit, config)
	case HybridBackendC99:
		g.emitC99Loop(circuit, config)
	}

	if len(g.errors) > 0 {
		return "", fmt.Errorf("QML lowering errors: %s", strings.Join(g.errors, "\n"))
	}
	return g.buf.String(), nil
}

// ============================================================
// WGSL Backend — GPU-accelerated parameter-shift evaluation
// ============================================================

func (g *QMLLoweringGenerator) emitWGSLLoop(circuit *parser.CircuitDecl, config HybridVQELoopConfig) {
	g.buf.WriteString("// Karkain Phase 30 — Hybrid Quantum-Classical VQE Loop (WGSL)\n")
	g.buf.WriteString(fmt.Sprintf("// Circuit: %s | Qubits: %d | Params: %d\n", circuit.Name, config.NumQubits, config.NumParams))
	g.buf.WriteString(fmt.Sprintf("// Max iterations: %d | Learning rate: %f | Tolerance: %e\n", config.MaxIter, config.LearningRate, config.Tolerance))
	g.buf.WriteString("\n")

	// Uniforms for VQE state
	g.buf.WriteString("struct VQEUniforms {\n")
	g.buf.WriteString("  iteration: u32,\n")
	g.buf.WriteString("  num_params: u32,\n")
	g.buf.WriteString("  num_qubits: u32,\n")
	g.buf.WriteString("  state_size: u32,\n")
	g.buf.WriteString("  learning_rate: f32,\n")
	g.buf.WriteString("  tolerance: f32,\n")
	g.buf.WriteString("  pad: vec2<f32>,\n")
	g.buf.WriteString("}\n\n")

	// Bindings
	g.buf.WriteString("@group(0) @binding(0) var<uniform> vqe: VQEUniforms;\n")
	g.buf.WriteString("@group(0) @binding(1) var<storage, read_write> params: array<f32>;\n")
	g.buf.WriteString("@group(0) @binding(2) var<storage, read_write> grads: array<f32>;\n")
	g.buf.WriteString("@group(0) @binding(3) var<storage, read_write> state_buf: array<f32>;\n\n")

	// Parameter-shift evaluation kernel (runs 2 * num_params state-vector simulations)
	g.buf.WriteString("@compute @workgroup_size(1)\n")
	g.buf.WriteString("fn param_shift_eval(@builtin(global_invocation_id) gid: vec3<u32>) {\n")
	g.buf.WriteString("  let param_idx: u32 = gid.x;\n")
	g.buf.WriteString("  if (param_idx >= vqe.num_params) { return; }\n")
	g.buf.WriteString("\n")
	g.buf.WriteString("  let shift: f32 = 1.5707963; // π/2\n")
	g.buf.WriteString("\n")
	g.buf.WriteString("  // Evaluate E(θ + π/2)\n")
	g.buf.WriteString("  var p_plus = params;\n")
	g.buf.WriteString("  p_plus[param_idx] = p_plus[param_idx] + shift;\n")
	g.buf.WriteString("  // ... run circuit with p_plus -> e_plus\n")
	g.buf.WriteString("  let e_plus: f32 = 0.0; // placeholder: circuit_eval(p_plus)\n")
	g.buf.WriteString("\n")
	g.buf.WriteString("  // Evaluate E(θ - π/2)\n")
	g.buf.WriteString("  var p_minus = params;\n")
	g.buf.WriteString("  p_minus[param_idx] = p_minus[param_idx] - shift;\n")
	g.buf.WriteString("  // ... run circuit with p_minus -> e_minus\n")
	g.buf.WriteString("  let e_minus: f32 = 0.0; // placeholder: circuit_eval(p_minus)\n")
	g.buf.WriteString("\n")
	g.buf.WriteString("  grads[param_idx] = (e_plus - e_minus) / 2.0;\n")
	g.buf.WriteString("}\n\n")

	// Gradient descent update kernel
	g.buf.WriteString("@compute @workgroup_size(64)\n")
	g.buf.WriteString("fn update_params(@builtin(global_invocation_id) gid: vec3<u32>) {\n")
	g.buf.WriteString("  let i: u32 = gid.x;\n")
	g.buf.WriteString("  if (i >= vqe.num_params) { return; }\n")
	g.buf.WriteString("  params[i] = params[i] - vqe.learning_rate * grads[i];\n")
	g.buf.WriteString("}\n\n")

	// Convergence check kernel
	g.buf.WriteString("@compute @workgroup_size(1)\n")
	g.buf.WriteString("fn check_convergence(@builtin(global_invocation_id) gid: vec3<u32>, @builtin(workgroup_id) wg: vec3<u32>) {\n")
	g.buf.WriteString("  if (gid.x != 0u) { return; }\n")
	g.buf.WriteString("  var max_grad: f32 = 0.0;\n")
	g.buf.WriteString("  for (var i: u32 = 0u; i < vqe.num_params; i = i + 1u) {\n")
	g.buf.WriteString("    let ag: f32 = abs(grads[i]);\n")
	g.buf.WriteString("    if (ag > max_grad) { max_grad = ag; }\n")
	g.buf.WriteString("  }\n")
	g.buf.WriteString("  // Write convergence flag to state_buf[0]\n")
	g.buf.WriteString("  state_buf[0] = select(1.0, 0.0, max_grad > vqe.tolerance);\n")
	g.buf.WriteString("}\n\n")

	// Dispatch loop comment
	g.buf.WriteString("// VQE iteration dispatch sequence:\n")
	g.buf.WriteString("// for iter in 0..maxIter:\n")
	g.buf.WriteString("//   1. dispatch param_shift_eval (1 workgroup)\n")
	g.buf.WriteString("//   2. barrier\n")
	g.buf.WriteString("//   3. dispatch update_params (ceil(num_params/64) workgroups)\n")
	g.buf.WriteString("//   4. barrier\n")
	g.buf.WriteString("//   5. dispatch check_convergence (1 workgroup)\n")
	g.buf.WriteString("//   6. read state_buf[0]; if 1.0 -> converged\n")
}

// ============================================================
// C99 Backend — CPU VQE loop with OpenQASM-style evaluation
// ============================================================

func (g *QMLLoweringGenerator) emitC99Loop(circuit *parser.CircuitDecl, config HybridVQELoopConfig) {
	g.buf.WriteString("/* Karkain Phase 30 — Hybrid Quantum-Classical VQE Loop (C99) */\n")
	g.buf.WriteString(fmt.Sprintf("/* Circuit: %s | Qubits: %d | Params: %d */\n", circuit.Name, config.NumQubits, config.NumParams))
	g.buf.WriteString("\n")
	g.buf.WriteString("#include <math.h>\n")
	g.buf.WriteString("#include <stdio.h>\n")
	g.buf.WriteString("#include <stdlib.h>\n")
	g.buf.WriteString("#include <string.h>\n\n")

	// Constants
	g.buf.WriteString(fmt.Sprintf("#define NUM_QUBITS %d\n", config.NumQubits))
	g.buf.WriteString(fmt.Sprintf("#define STATE_SIZE %d\n", 1<<uint(config.NumQubits)))
	g.buf.WriteString(fmt.Sprintf("#define NUM_PARAMS %d\n", config.NumParams))
	g.buf.WriteString(fmt.Sprintf("#define MAX_ITER %d\n", config.MaxIter))
	g.buf.WriteString(fmt.Sprintf("#define LEARNING_RATE %f\n", config.LearningRate))
	g.buf.WriteString(fmt.Sprintf("#define TOLERANCE %e\n", config.Tolerance))
	g.buf.WriteString("#define PI 3.14159265358979323846\n")
	g.buf.WriteString("#define SHIFT (PI / 2.0)\n\n")

	// Complex type
	g.buf.WriteString("typedef struct { double re; double im; } complex_t;\n\n")

	// State vector
	g.buf.WriteString("static complex_t state[STATE_SIZE];\n")
	g.buf.WriteString("static double params[NUM_PARAMS];\n")
	g.buf.WriteString("static double grads[NUM_PARAMS];\n\n")

	// Circuit evaluation stub
	g.emitC99CircuitEval(circuit)

	// Parameter-shift gradient computation
	g.buf.WriteString("void compute_gradients(void (*eval_fn)(double*)) {\n")
	g.buf.WriteString("    double p_plus[NUM_PARAMS], p_minus[NUM_PARAMS];\n")
	g.buf.WriteString("    double e_plus, e_minus;\n")
	g.buf.WriteString("    for (int i = 0; i < NUM_PARAMS; i++) {\n")
	g.buf.WriteString("        memcpy(p_plus, params, sizeof(params));\n")
	g.buf.WriteString("        memcpy(p_minus, params, sizeof(params));\n")
	g.buf.WriteString("        p_plus[i] += SHIFT;\n")
	g.buf.WriteString("        p_minus[i] -= SHIFT;\n")
	g.buf.WriteString("        eval_fn(p_plus);  e_plus = measure_expectation();\n")
	g.buf.WriteString("        eval_fn(p_minus); e_minus = measure_expectation();\n")
	g.buf.WriteString("        grads[i] = (e_plus - e_minus) / 2.0;\n")
	g.buf.WriteString("    }\n")
	g.buf.WriteString("}\n\n")

	// Expectation value
	g.buf.WriteString("double measure_expectation(void) {\n")
	g.buf.WriteString("    double energy = 0.0;\n")
	g.buf.WriteString("    for (int i = 0; i < STATE_SIZE; i++) {\n")
	g.buf.WriteString("        double prob = state[i].re * state[i].re + state[i].im * state[i].im;\n")
	g.buf.WriteString("        energy += (double)(1 - 2 * ((i >> 0) & 1)) * prob; // Z_0 Pauli\n")
	g.buf.WriteString("    }\n")
	g.buf.WriteString("    return energy;\n")
	g.buf.WriteString("}\n\n")

	// VQE main loop
	g.buf.WriteString("int main(void) {\n")
	g.buf.WriteString("    // Initialize parameters\n")
	g.buf.WriteString("    for (int i = 0; i < NUM_PARAMS; i++) params[i] = 0.0;\n")
	g.buf.WriteString("\n")
	g.buf.WriteString("    for (int iter = 0; iter < MAX_ITER; iter++) {\n")
	g.buf.WriteString("        eval_circuit(params);\n")
	g.buf.WriteString("        double energy = measure_expectation();\n")
	g.buf.WriteString("        compute_gradients(eval_circuit);\n")
	g.buf.WriteString("\n")
	g.buf.WriteString("        // Check convergence\n")
	g.buf.WriteString("        double max_grad = 0.0;\n")
	g.buf.WriteString("        for (int i = 0; i < NUM_PARAMS; i++) {\n")
	g.buf.WriteString("            double ag = fabs(grads[i]);\n")
	g.buf.WriteString("            if (ag > max_grad) max_grad = ag;\n")
	g.buf.WriteString("        }\n")
	g.buf.WriteString("\n")
	g.buf.WriteString("        printf(\"Iter %d: E = %.8f, max|grad| = %.2e\\n\", iter, energy, max_grad);\n")
	g.buf.WriteString("\n")
	g.buf.WriteString("        if (max_grad < TOLERANCE) {\n")
	g.buf.WriteString("            printf(\"Converged at iteration %d\\n\", iter);\n")
	g.buf.WriteString("            break;\n")
	g.buf.WriteString("        }\n")
	g.buf.WriteString("\n")
	g.buf.WriteString("        // Gradient descent update\n")
	g.buf.WriteString("        for (int i = 0; i < NUM_PARAMS; i++)\n")
	g.buf.WriteString("            params[i] -= LEARNING_RATE * grads[i];\n")
	g.buf.WriteString("    }\n")
	g.buf.WriteString("    return 0;\n")
	g.buf.WriteString("}\n")
}

func (g *QMLLoweringGenerator) emitC99CircuitEval(circuit *parser.CircuitDecl) {
	g.buf.WriteString("void eval_circuit(double *p) {\n")
	g.buf.WriteString("    // Initialize |0...0>\n")
	g.buf.WriteString("    memset(state, 0, sizeof(state));\n")
	g.buf.WriteString("    state[0].re = 1.0;\n\n")

	paramIdx := 0
	for _, stmt := range circuit.Body {
		if qpu, ok := stmt.(*parser.QPUOpExpr); ok {
			g.emitC99Gate(qpu, &paramIdx)
		} else if expr, ok := stmt.(*parser.ExprStmt); ok {
			if qpu, ok := expr.Expression.(*parser.QPUOpExpr); ok {
				g.emitC99Gate(qpu, &paramIdx)
			}
		}
	}
	g.buf.WriteString("}\n\n")
}

func (g *QMLLoweringGenerator) emitC99Gate(op *parser.QPUOpExpr, paramIdx *int) {
	qubitIdx := 0
	if len(op.Args) > 0 {
		qubitIdx = resolveQubitIndexStaticCG(op.Args[0])
	}

	switch op.Op {
	case "h":
		g.buf.WriteString(fmt.Sprintf("    // H on qubit %d\n", qubitIdx))
		g.buf.WriteString(fmt.Sprintf("    apply_h_c99(%d);\n", qubitIdx))
	case "x":
		g.buf.WriteString(fmt.Sprintf("    apply_x_c99(%d);\n", qubitIdx))
	case "rx":
		g.buf.WriteString(fmt.Sprintf("    apply_rx_c99(p[%d], %d);\n", *paramIdx, qubitIdx))
		*paramIdx++
	case "ry":
		g.buf.WriteString(fmt.Sprintf("    apply_ry_c99(p[%d], %d);\n", *paramIdx, qubitIdx))
		*paramIdx++
	case "rz":
		g.buf.WriteString(fmt.Sprintf("    apply_rz_c99(p[%d], %d);\n", *paramIdx, qubitIdx))
		*paramIdx++
	case "cx":
		ctrlIdx := 0
		if len(op.Args) > 1 {
			ctrlIdx = resolveQubitIndexStaticCG(op.Args[0])
			qubitIdx = resolveQubitIndexStaticCG(op.Args[1])
		}
		g.buf.WriteString(fmt.Sprintf("    apply_cx_c99(%d, %d);\n", ctrlIdx, qubitIdx))
	case "measure":
		g.buf.WriteString("    // measurement (collapse)\n")
	default:
		g.buf.WriteString(fmt.Sprintf("    // %s gate\n", op.Op))
	}
}

// ============================================================
// OpenQASM generation with parameter bindings for gradient steps
// ============================================================

// OpenQASMParamShiftGenerator generates OpenQASM 3.0 with parameter
// binding support for gradient evaluation
type OpenQASMParamShiftGenerator struct {
	buf    strings.Builder
	errors []string
}

func NewOpenQASMParamShiftGenerator() *OpenQASMParamShiftGenerator {
	return &OpenQASMParamShiftGenerator{}
}

// GenerateParamShiftProgram emits an OpenQASM 3.0 program with param shift
func (g *OpenQASMParamShiftGenerator) GenerateParamShiftProgram(circuit *parser.CircuitDecl, numParams int) (string, error) {
	g.buf.Reset()
	g.errors = nil

	g.buf.WriteString("// Karkain Phase 30 — Parameterized Circuit with Parameter-Shift Gradient\n")
	g.buf.WriteString("OPENQASM 3.0;\n")
	g.buf.WriteString("include \"stdgates.inc\";\n")
	g.buf.WriteString("\n")

	// Declare parameter variables
	for i := 0; i < numParams; i++ {
		g.buf.WriteString(fmt.Sprintf("input float theta[%d];\n", numParams))
	}

	numQubits := countCircuitQubits(circuit)
	g.buf.WriteString(fmt.Sprintf("qubit[%d] q;\n", numQubits))

	returnTypeSize := 0
	if circuit.ReturnType != nil {
		returnTypeSize = circuit.ReturnType.Size
	}
	if returnTypeSize > 0 {
		g.buf.WriteString(fmt.Sprintf("bit[%d] c;\n", returnTypeSize))
	}
	g.buf.WriteString("\n")

	// Emit circuit body with parameter bindings
	paramIdx := 0
	for _, stmt := range circuit.Body {
		if qpu, ok := stmt.(*parser.QPUOpExpr); ok {
			g.emitQASMGate(qpu, &paramIdx)
		} else if expr, ok := stmt.(*parser.ExprStmt); ok {
			if qpu, ok := expr.Expression.(*parser.QPUOpExpr); ok {
				g.emitQASMGate(qpu, &paramIdx)
			}
		}
	}

	// Parameter-shift evaluation program
	g.buf.WriteString("\n// Parameter-shift gradient evaluation\n")
	g.buf.WriteString("// For each parameter i:\n")
	g.buf.WriteString("//   1. Set theta[i] += π/2, run circuit, measure expectation E_plus\n")
	g.buf.WriteString("//   2. Set theta[i] -= π/2, run circuit, measure expectation E_minus\n")
	g.buf.WriteString("//   3. grad[i] = (E_plus - E_minus) / 2\n")

	if len(g.errors) > 0 {
		return "", fmt.Errorf("OpenQASM param-shift errors: %s", strings.Join(g.errors, "\n"))
	}
	return g.buf.String(), nil
}

func (g *OpenQASMParamShiftGenerator) emitQASMGate(op *parser.QPUOpExpr, paramIdx *int) {
	qubitStr := "q"
	if len(op.Args) > 0 {
		qubitStr = formatQubitArgCG(op.Args[0])
	}

	switch op.Op {
	case "h":
		g.buf.WriteString(fmt.Sprintf("h %s;\n", qubitStr))
	case "x":
		g.buf.WriteString(fmt.Sprintf("x %s;\n", qubitStr))
	case "y":
		g.buf.WriteString(fmt.Sprintf("y %s;\n", qubitStr))
	case "z":
		g.buf.WriteString(fmt.Sprintf("z %s;\n", qubitStr))
	case "rx":
		g.buf.WriteString(fmt.Sprintf("rx(theta[%d]) %s;\n", *paramIdx, qubitStr))
		*paramIdx++
	case "ry":
		g.buf.WriteString(fmt.Sprintf("ry(theta[%d]) %s;\n", *paramIdx, qubitStr))
		*paramIdx++
	case "rz":
		g.buf.WriteString(fmt.Sprintf("rz(theta[%d]) %s;\n", *paramIdx, qubitStr))
		*paramIdx++
	case "cx":
		ctrl := qubitStr
		tgt := "q"
		if len(op.Args) > 1 {
			ctrl = formatQubitArgCG(op.Args[0])
			tgt = formatQubitArgCG(op.Args[1])
		}
		g.buf.WriteString(fmt.Sprintf("cx %s, %s;\n", ctrl, tgt))
	case "measure":
		for i, arg := range op.Args {
			g.buf.WriteString(fmt.Sprintf("c[%d] = measure %s;\n", i, formatQubitArgCG(arg)))
		}
	}
}

// ============================================================
// Shared helpers
// ============================================================

func countCircuitQubits(circuit *parser.CircuitDecl) int {
	total := 0
	for _, p := range circuit.Params {
		if p.Type == nil || p.Type.Size <= 0 {
			total += 1
		} else {
			total += p.Type.Size
		}
	}
	if total == 0 {
		return 1
	}
	return total
}

func countParameterizedGates(circuit *parser.CircuitDecl) int {
	count := 0
	for _, stmt := range circuit.Body {
		switch n := stmt.(type) {
		case *parser.QPUOpExpr:
			if n.Op == "rx" || n.Op == "ry" || n.Op == "rz" {
				count++
			}
		case *parser.ExprStmt:
			if qpu, ok := n.Expression.(*parser.QPUOpExpr); ok {
				if qpu.Op == "rx" || qpu.Op == "ry" || qpu.Op == "rz" {
					count++
				}
			}
		}
	}
	return count
}

func resolveQubitIndexStaticCG(node parser.Node) int {
	switch n := node.(type) {
	case *parser.QubitIndexExpr:
		if lit, ok := n.Index.(*parser.IntLiteral); ok {
			val := 0
			fmt.Sscanf(lit.Value, "%d", &val)
			return val
		}
		return 0
	case *parser.Identifier:
		return 0
	default:
		return 0
	}
}

func formatQubitArgCG(node parser.Node) string {
	switch n := node.(type) {
	case *parser.Identifier:
		return n.Name
	case *parser.QubitIndexExpr:
		qubitName := formatQubitArgCG(n.Qubit)
		if lit, ok := n.Index.(*parser.IntLiteral); ok {
			return fmt.Sprintf("%s[%s]", qubitName, lit.Value)
		}
		return qubitName
	default:
		return "q"
	}
}

// GetErrors returns accumulated errors
func (g *QMLLoweringGenerator) GetErrors() []string {
	return g.errors
}
