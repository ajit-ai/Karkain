package codegen

import (
	"fmt"
	"karkain/pkg/parser"
	"math"
	"strings"
)

// QuantumSimGenerator generates WGSL compute shaders for quantum state-vector simulation.
// For an N-qubit circuit, the state vector has 2^N complex amplitudes (float32x2).
type QuantumSimGenerator struct {
	buf     strings.Builder
	errors  []string
	numQubits int
	stateSize int
}

func NewQuantumSimGenerator() *QuantumSimGenerator {
	return &QuantumSimGenerator{}
}

// CircuitToWGSLOptions configures the WGSL output
type CircuitToWGSLOptions struct {
	WorkgroupSize int
}

// GenerateWGSLSimShader produces a full WGSL compute shader that simulates
// the given circuit declaration using a state-vector representation.
func (g *QuantumSimGenerator) GenerateWGSLSimShader(circuit *parser.CircuitDecl, opts CircuitToWGSLOptions) (string, error) {
	g.buf.Reset()
	g.errors = nil

	n, err := g.countQubits(circuit)
	if err != nil {
		return "", err
	}
	g.numQubits = n
	g.stateSize = 1 << n

	wgSize := opts.WorkgroupSize
	if wgSize <= 0 {
		wgSize = 64
	}

	// Validate qubit count
	if n > 30 {
		g.errors = append(g.errors, fmt.Sprintf("circuit exceeds 30 qubits (%d): state vector would require %d complex amplitudes, exceeding local GPU memory limits", n, g.stateSize))
		return "", fmt.Errorf("%s", g.errors[0])
	}

	g.emitHeader()
	g.emitTypes()
	g.emitUniforms()
	g.emitStorageBindings()
	g.emitMathHelpers()
	g.emitGateKernels(n)
	g.emitMeasurementKernel(n)
	g.emitMainPipeline(circuit, wgSize)

	return g.buf.String(), nil
}

// countQubits totals the qubit register size across all parameters.
func (g *QuantumSimGenerator) countQubits(circuit *parser.CircuitDecl) (int, error) {
	total := 0
	for _, p := range circuit.Params {
		if p.Type == nil || p.Type.Size <= 0 {
			total += 1
		} else {
			total += p.Type.Size
		}
	}
	if total == 0 {
		return 0, fmt.Errorf("circuit has no qubit parameters")
	}
	return total, nil
}

func (g *QuantumSimGenerator) emitHeader() {
	g.buf.WriteString("// Karkain Phase 29 — GPU Quantum State-Vector Simulator\n")
	g.buf.WriteString(fmt.Sprintf("// %d qubits, %d complex amplitudes\n", g.numQubits, g.stateSize))
	g.buf.WriteString("// Auto-generated WGSL compute shader\n\n")
}

func (g *QuantumSimGenerator) emitTypes() {
	g.buf.WriteString("struct Complex {\n")
	g.buf.WriteString("  re: f32,\n")
	g.buf.WriteString("  im: f32,\n")
	g.buf.WriteString("}\n\n")
}

func (g *QuantumSimGenerator) emitUniforms() {
	g.buf.WriteString("struct Uniforms {\n")
	g.buf.WriteString("  num_qubits: u32,\n")
	g.buf.WriteString("  state_size: u32,\n")
	g.buf.WriteString("  pad: vec2<u32>,\n")
	g.buf.WriteString("}\n\n")
	g.buf.WriteString("@group(0) @binding(0) var<uniform> uniforms: Uniforms;\n")
}

func (g *QuantumSimGenerator) emitStorageBindings() {
	g.buf.WriteString("@group(0) @binding(1) var<storage, read_write> state: array<Complex>;\n")
	g.buf.WriteString("@group(0) @binding(2) var<storage, read> gate_matrix: array<Complex>;\n")
	g.buf.WriteString("@group(0) @binding(3) var<storage, read_write> rng_state: array<u32>;\n\n")
}

func (g *QuantumSimGenerator) emitMathHelpers() {
	g.buf.WriteString("fn complex_mul(a: Complex, b: Complex) -> Complex {\n")
	g.buf.WriteString("  return Complex(\n")
	g.buf.WriteString("    a.re * b.re - a.im * b.im,\n")
	g.buf.WriteString("    a.re * b.im + a.im * b.re\n")
	g.buf.WriteString("  );\n")
	g.buf.WriteString("}\n\n")

	g.buf.WriteString("fn complex_add(a: Complex, b: Complex) -> Complex {\n")
	g.buf.WriteString("  return Complex(a.re + b.re, a.im + b.im);\n")
	g.buf.WriteString("}\n\n")

	g.buf.WriteString("fn complex_conj(a: Complex) -> Complex {\n")
	g.buf.WriteString("  return Complex(a.re, -a.im);\n")
	g.buf.WriteString("}\n\n")

	g.buf.WriteString("fn complex_abs2(a: Complex) -> f32 {\n")
	g.buf.WriteString("  return a.re * a.re + a.im * a.im;\n")
	g.buf.WriteString("}\n\n")

	// xorshift32 PRNG for measurement collapse
	g.buf.WriteString("fn xorshift32(seed: ptr<function, u32>) -> f32 {\n")
	g.buf.WriteString("  var x: u32 = *seed;\n")
	g.buf.WriteString("  x ^= x << 13u;\n")
	g.buf.WriteString("  x ^= x >> 17u;\n")
	g.buf.WriteString("  x ^= x << 5u;\n")
	g.buf.WriteString("  *seed = x;\n")
	g.buf.WriteString("  return f32(x) / f32(0xFFFFFFFFu);\n")
	g.buf.WriteString("}\n\n")
}

func (g *QuantumSimGenerator) emitGateKernels(numQubits int) {
	g.emitApplyH()
	g.buf.WriteString("\n")
	g.emitApplyCNOT()
	g.buf.WriteString("\n")
	g.emitApplyRotation("ry", "y_rotation")
	g.buf.WriteString("\n")
	g.emitApplySwap()
	g.buf.WriteString("\n")
}

func (g *QuantumSimGenerator) emitApplyH() {
	g.buf.WriteString("// Hadamard gate: H|0> = (|0>+|1>)/sqrt(2), H|1> = (|0>-|1>)/sqrt(2)\n")
	g.buf.WriteString("@compute @workgroup_size(64)\n")
	g.buf.WriteString("fn apply_h(@builtin(global_invocation_id) gid: vec3<u32>) {\n")
	g.buf.WriteString("  let target: u32 = gid.x;\n")
	g.buf.WriteString("  if (target >= uniforms.state_size >> 1u) { return; }\n")
	g.buf.WriteString("  let bit: u32 = uniforms.num_qubits - 1u;\n")
	g.buf.WriteString("  let mask: u32 = 1u << bit;\n")
	g.buf.WriteString("  let i0: u32 = (target & ~mask);\n")
	g.buf.WriteString("  let i1: u32 = (target & ~mask) | mask;\n")
	g.buf.WriteString("  let a: Complex = state[i0];\n")
	g.buf.WriteString("  let b: Complex = state[i1];\n")
	g.buf.WriteString("  let inv_sqrt2: f32 = 0.7071067811865475;\n")
	g.buf.WriteString("  state[i0] = Complex(inv_sqrt2 * (a.re + b.re), inv_sqrt2 * (a.im + b.im));\n")
	g.buf.WriteString("  state[i1] = Complex(inv_sqrt2 * (a.re - b.re), inv_sqrt2 * (a.im - b.im));\n")
	g.buf.WriteString("}\n")
}

func (g *QuantumSimGenerator) emitApplyCNOT() {
	g.buf.WriteString("// CNOT gate: flip target if control qubit is |1>\n")
	g.buf.WriteString("@compute @workgroup_size(64)\n")
	g.buf.WriteString("fn apply_cnot(@builtin(global_invocation_id) gid: vec3<u32>) {\n")
	g.buf.WriteString("  let idx: u32 = gid.x;\n")
	g.buf.WriteString("  if (idx >= uniforms.state_size) { return; }\n")
	g.buf.WriteString("  let ctrl_bit: u32 = uniforms.num_qubits - 2u;\n")
	g.buf.WriteString("  let tgt_bit: u32 = uniforms.num_qubits - 1u;\n")
	g.buf.WriteString("  let ctrl_mask: u32 = 1u << ctrl_bit;\n")
	g.buf.WriteString("  let tgt_mask: u32 = 1u << tgt_bit;\n")
	g.buf.WriteString("  if ((idx & ctrl_mask) != 0u) {\n")
	g.buf.WriteString("    let partner: u32 = idx ^ tgt_mask;\n")
	g.buf.WriteString("    if (idx < partner) {\n")
	g.buf.WriteString("      let tmp: Complex = state[idx];\n")
	g.buf.WriteString("      state[idx] = state[partner];\n")
	g.buf.WriteString("      state[partner] = tmp;\n")
	g.buf.WriteString("    }\n")
	g.buf.WriteString("  }\n")
	g.buf.WriteString("}\n")
}

func (g *QuantumSimGenerator) emitApplyRotation(name string, fnName string) {
	g.buf.WriteString(fmt.Sprintf("// %s rotation gate: 2x2 rotation matrix applied to target qubit\n", strings.ToUpper(name)))
	g.buf.WriteString("@compute @workgroup_size(64)\n")
	g.buf.WriteString(fmt.Sprintf("fn %s(@builtin(global_invocation_id) gid: vec3<u32>, angle: f32) {\n", fnName))
	g.buf.WriteString("  let target: u32 = gid.x;\n")
	g.buf.WriteString("  if (target >= uniforms.state_size >> 1u) { return; }\n")
	g.buf.WriteString("  let bit: u32 = uniforms.num_qubits - 1u;\n")
	g.buf.WriteString("  let mask: u32 = 1u << bit;\n")
	g.buf.WriteString("  let i0: u32 = (target & ~mask);\n")
	g.buf.WriteString("  let i1: u32 = (target & ~mask) | mask;\n")
	g.buf.WriteString("  let c: f32 = cos(angle);\n")
	g.buf.WriteString("  let s: f32 = sin(angle);\n")
	g.buf.WriteString("  let a: Complex = state[i0];\n")
	g.buf.WriteString("  let b: Complex = state[i1];\n")
	g.buf.WriteString("  state[i0] = Complex(c * a.re - s * b.re, c * a.im - s * b.im);\n")
	g.buf.WriteString("  state[i1] = Complex(s * a.re + c * b.re, s * a.im + c * b.im);\n")
	g.buf.WriteString("}\n")
}

func (g *QuantumSimGenerator) emitApplySwap() {
	g.buf.WriteString("// SWAP gate: swap two qubit amplitudes\n")
	g.buf.WriteString("@compute @workgroup_size(64)\n")
	g.buf.WriteString("fn apply_swap(@builtin(global_invocation_id) gid: vec3<u32>) {\n")
	g.buf.WriteString("  let idx: u32 = gid.x;\n")
	g.buf.WriteString("  if (idx >= uniforms.state_size) { return; }\n")
	g.buf.WriteString("  let bit_a: u32 = uniforms.num_qubits - 2u;\n")
	g.buf.WriteString("  let bit_b: u32 = uniforms.num_qubits - 1u;\n")
	g.buf.WriteString("  let mask_a: u32 = 1u << bit_a;\n")
	g.buf.WriteString("  let mask_b: u32 = 1u << bit_b;\n")
	g.buf.WriteString("  let has_a: bool = (idx & mask_a) != 0u;\n")
	g.buf.WriteString("  let has_b: bool = (idx & mask_b) != 0u;\n")
	g.buf.WriteString("  if (has_a != has_b) {\n")
	g.buf.WriteString("    let partner: u32 = idx ^ mask_a ^ mask_b;\n")
	g.buf.WriteString("    if (idx < partner) {\n")
	g.buf.WriteString("      let tmp: Complex = state[idx];\n")
	g.buf.WriteString("      state[idx] = state[partner];\n")
	g.buf.WriteString("      state[partner] = tmp;\n")
	g.buf.WriteString("    }\n")
	g.buf.WriteString("  }\n")
	g.buf.WriteString("}\n")
}

func (g *QuantumSimGenerator) emitMeasurementKernel(numQubits int) {
	g.buf.WriteString("// Measurement: probabilistic collapse via pseudo-random sampling\n")
	g.buf.WriteString("// Iterates through basis states accumulating probability, collapses when\n")
	g.buf.WriteString("// cumulative probability exceeds the random threshold\n")
	g.buf.WriteString("@compute @workgroup_size(1)\n")
	g.buf.WriteString("fn measure_collapse(@builtin(global_invocation_id) gid: vec3<u32>) {\n")
	g.buf.WriteString("  if (gid.x != 0u) { return; }\n")
	g.buf.WriteString("  let rand: f32 = xorshift32(&rng_state[0]);\n")
	g.buf.WriteString("  var cumulative: f32 = 0.0;\n")
	g.buf.WriteString("  for (var i: u32 = 0u; i < uniforms.state_size; i = i + 1u) {\n")
	g.buf.WriteString("    cumulative = cumulative + complex_abs2(state[i]);\n")
	g.buf.WriteString("    if (cumulative >= rand) {\n")
	g.buf.WriteString("      // Collapse to basis state i: set this amplitude to 1, rest to 0\n")
	g.buf.WriteString("      for (var j: u32 = 0u; j < uniforms.state_size; j = j + 1u) {\n")
	g.buf.WriteString("        if (j == i) {\n")
	g.buf.WriteString("          state[j] = Complex(1.0, 0.0);\n")
	g.buf.WriteString("        } else {\n")
	g.buf.WriteString("          state[j] = Complex(0.0, 0.0);\n")
	g.buf.WriteString("        }\n")
	g.buf.WriteString("      }\n")
	g.buf.WriteString("      return;\n")
	g.buf.WriteString("    }\n")
	g.buf.WriteString("  }\n")
	g.buf.WriteString("  // Fallback: collapse to last state\n")
	g.buf.WriteString("  state[uniforms.state_size - 1u] = Complex(1.0, 0.0);\n")
	g.buf.WriteString("}\n\n")

	// Normalization kernel: re-normalize state vector after non-unitary ops
	g.buf.WriteString("// Normalize: divide all amplitudes by sqrt(sum of |a_i|^2)\n")
	g.buf.WriteString("@compute @workgroup_size(64)\n")
	g.buf.WriteString("fn normalize_state(@builtin(global_invocation_id) gid: vec3<u32>) {\n")
	g.buf.WriteString("  let idx: u32 = gid.x;\n")
	g.buf.WriteString("  if (idx != 0u) { return; }\n")
	g.buf.WriteString("  var norm2: f32 = 0.0;\n")
	g.buf.WriteString("  for (var i: u32 = 0u; i < uniforms.state_size; i = i + 1u) {\n")
	g.buf.WriteString("    norm2 = norm2 + complex_abs2(state[i]);\n")
	g.buf.WriteString("  }\n")
	g.buf.WriteString("  let inv_norm: f32 = 1.0 / sqrt(norm2);\n")
	g.buf.WriteString("  // Broadcast normalization factor via first element, apply in a second pass\n")
	g.buf.WriteString("  state[0] = Complex(state[0].re * inv_norm, state[0].im * inv_norm);\n")
	g.buf.WriteString("}\n\n")

	// Full normalization pass — applies the factor broadcast in state[0].re
	g.buf.WriteString("@compute @workgroup_size(64)\n")
	g.buf.WriteString("fn normalize_apply(@builtin(global_invocation_id) gid: vec3<u32>) {\n")
	g.buf.WriteString("  let idx: u32 = gid.x;\n")
	g.buf.WriteString("  if (idx >= uniforms.state_size) { return; }\n")
	g.buf.WriteString("  let inv_norm: f32 = state[0].re;\n")
	g.buf.WriteString("  if (idx == 0u) { return; }\n")
	g.buf.WriteString("  state[idx] = Complex(state[idx].re * inv_norm, state[idx].im * inv_norm);\n")
	g.buf.WriteString("}\n")
}

func (g *QuantumSimGenerator) emitMainPipeline(circuit *parser.CircuitDecl, wgSize int) {
	g.buf.WriteString("// ---- Gate dispatch sequence for circuit: " + circuit.Name + " ----\n")
	g.buf.WriteString("// In production, each @dispatch call maps to a GPU queue.submit\n")
	g.buf.WriteString("// with appropriate buffer writes for angles and gate indices.\n\n")

	// Walk circuit body and emit dispatch comments/structure
	g.emitDispatchSequence(circuit.Body, wgSize)

	if len(g.errors) > 0 {
		return
	}

	// Emit init kernel: |0...0> state
	g.buf.WriteString("@compute @workgroup_size(64)\n")
	g.buf.WriteString("fn init_state(@builtin(global_invocation_id) gid: vec3<u32>) {\n")
	g.buf.WriteString("  let idx: u32 = gid.x;\n")
	g.buf.WriteString("  if (idx >= uniforms.state_size) { return; }\n")
	g.buf.WriteString("  if (idx == 0u) {\n")
	g.buf.WriteString("    state[0] = Complex(1.0, 0.0);\n")
	g.buf.WriteString("  } else {\n")
	g.buf.WriteString("    state[idx] = Complex(0.0, 0.0);\n")
	g.buf.WriteString("  }\n")
	g.buf.WriteString("}\n")

	// Probability readback kernel
	g.buf.WriteString("@compute @workgroup_size(64)\n")
	g.buf.WriteString("fn compute_probabilities(@builtin(global_invocation_id) gid: vec3<u32>) {\n")
	g.buf.WriteString("  let idx: u32 = gid.x;\n")
	g.buf.WriteString("  if (idx >= uniforms.state_size) { return; }\n")
	g.buf.WriteString("  // Overwrite state with probability amplitudes (real only)\n")
	g.buf.WriteString("  state[idx] = Complex(complex_abs2(state[idx]), 0.0);\n")
	g.buf.WriteString("}\n")
}

func (g *QuantumSimGenerator) emitDispatchSequence(stmts []parser.Node, wgSize int) {
	for _, stmt := range stmts {
		switch n := stmt.(type) {
		case *parser.QPUOpExpr:
			g.emitDispatchComment(n, wgSize)
		case *parser.ExprStmt:
			if qpu, ok := n.Expression.(*parser.QPUOpExpr); ok {
				g.emitDispatchComment(qpu, wgSize)
			}
		}
	}
}

func (g *QuantumSimGenerator) emitDispatchComment(op *parser.QPUOpExpr, wgSize int) {
	totalGroups := (g.stateSize + wgSize - 1) / wgSize
	switch op.Op {
	case "h":
		g.buf.WriteString(fmt.Sprintf("// dispatch apply_h: workgroups=%d (target=qubit 0)\n", totalGroups))
	case "cx":
		g.buf.WriteString(fmt.Sprintf("// dispatch apply_cnot: workgroups=%d (ctrl=qubit 0, tgt=qubit 1)\n", totalGroups))
	case "ry":
		g.buf.WriteString(fmt.Sprintf("// dispatch y_rotation: workgroups=%d (angle=param, target=qubit 0)\n", totalGroups))
	case "swap":
		g.buf.WriteString(fmt.Sprintf("// dispatch apply_swap: workgroups=%d (qubit_a=0, qubit_b=1)\n", totalGroups))
	case "measure":
		g.buf.WriteString("// dispatch measure_collapse: workgroups=1 (single-threaded collapse)\n")
		g.buf.WriteString("// dispatch normalize_state: workgroups=1\n")
		g.buf.WriteString(fmt.Sprintf("// dispatch normalize_apply: workgroups=%d\n", totalGroups))
	case "reset":
		g.buf.WriteString("// dispatch measure_collapse + re-init target qubit\n")
	case "x", "y", "z":
		g.buf.WriteString(fmt.Sprintf("// dispatch apply_%s: workgroups=%d\n", op.Op, totalGroups))
	case "rx", "rz":
		g.buf.WriteString(fmt.Sprintf("// dispatch %s_rotation: workgroups=%d\n", op.Op, totalGroups))
	default:
		g.buf.WriteString(fmt.Sprintf("// dispatch unknown gate '%s': workgroups=%d\n", op.Op, totalGroups))
	}
}

// GetErrors returns accumulated generation errors
func (g *QuantumSimGenerator) GetErrors() []string {
	return g.errors
}

// EstimateGPUMemoryBytes returns estimated GPU buffer size for the state vector
func EstimateGPUMemoryBytes(numQubits int) int64 {
	stateSize := int64(1) << uint(numQubits)
	complexSize := int64(8) // 2 x float32 = 8 bytes
	return stateSize * complexSize
}

// ValidateQubitCount returns an error if the qubit count exceeds the GPU limit
func ValidateQubitCount(n int) error {
	const maxQubits = 30
	if n > maxQubits {
		mem := EstimateGPUMemoryBytes(n)
		return fmt.Errorf("circuit with %d qubits requires %d bytes (%d MiB) for state vector, exceeding %d-qubit local GPU limit",
			n, mem, mem/(1024*1024), maxQubits)
	}
	return nil
}

// MaxQubitsForMemory returns the maximum qubit count fitting in maxBytes
func MaxQubitsForMemory(maxBytes int64) int {
	for n := 1; n <= 30; n++ {
		if EstimateGPUMemoryBytes(n) > maxBytes {
			return n - 1
		}
	}
	return 30
}

// init ensures math functions used are referenced
var _ = math.Sin
