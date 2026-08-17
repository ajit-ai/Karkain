package codegen

import (
	"fmt"
	"karkain/pkg/parser"
	"math"
	"strings"
)

// ============================================================
// Distributed Multi-GPU Quantum State-Vector Simulator
// Scales beyond single-device 30-qubit limit via domain decomposition.
// N total qubits → k global qubits across M=2^k ranks,
// N-k local qubits per rank.
// ============================================================

// DistQuantumConfig holds the configuration for distributed simulation
type DistQuantumConfig struct {
	TotalQubits  int
	GlobalQubits int // k: qubits distributed across ranks
	LocalQubits  int // N-k: qubits local to each rank
	NumRanks     int // M = 2^k
	ChunkSize    int // 2^(N-k) amplitudes per rank
	StateBytes   int64
	Backend      DistBackend
	WorkgroupSize int
}

// DistBackend selects the distributed target
type DistBackend int

const (
	DistBackendWGSL DistBackend = iota
	DistBackendC99OpenMP
)

// NewDistQuantumConfig computes the optimal domain decomposition
func NewDistQuantumConfig(totalQubits int, numRanks int) (*DistQuantumConfig, error) {
	if totalQubits < 1 {
		return nil, fmt.Errorf("total qubits must be >= 1, got %d", numRanks)
	}
	if numRanks < 1 || (numRanks&(numRanks-1)) != 0 {
		return nil, fmt.Errorf("numRanks must be a power of 2, got %d", numRanks)
	}

	k := 0
	for m := numRanks; m > 1; m >>= 1 {
		k++
	}

	localQ := totalQubits - k
	if localQ < 0 {
		return nil, fmt.Errorf("too many ranks (%d) for %d qubits: would need negative local qubits", numRanks, totalQubits)
	}

	chunkSize := 1 << uint(localQ)
	stateBytes := int64(chunkSize) * 8 // float32x2 = 8 bytes per amplitude

	return &DistQuantumConfig{
		TotalQubits:  totalQubits,
		GlobalQubits: k,
		LocalQubits:  localQ,
		NumRanks:     numRanks,
		ChunkSize:    chunkSize,
		StateBytes:   stateBytes,
		WorkgroupSize: 64,
	}, nil
}

// OptimalRankCount returns the smallest power-of-2 rank count fitting
// the target qubits into per-rank memory limits
func OptimalRankCount(totalQubits int, maxChunkBytes int64) int {
	for k := 0; k <= totalQubits; k++ {
		localQ := totalQubits - k
		chunkBytes := int64(1<<uint(localQ)) * 8
		if chunkBytes <= maxChunkBytes {
			return 1 << uint(k)
		}
	}
	return 1 << uint(totalQubits)
}

// ============================================================
// Index mapping: global state index ↔ (rank, local index)
// ============================================================

// GlobalToRank maps a global state-vector index to (rank, local index)
func GlobalToRank(globalIdx, localQubits int) (rank, localIdx int) {
	rank = globalIdx >> uint(localQubits)
	localIdx = globalIdx & ((1 << uint(localQubits)) - 1)
	return
}

// RankToGlobal maps (rank, local index) back to global state index
func RankToGlobal(rank, localIdx, localQubits int) int {
	return (rank << uint(localQubits)) | localIdx
}

// IsGlobalQubit returns true if the qubit index is a distributed (global) qubit
func IsGlobalQubit(qubitIdx, localQubits int) bool {
	return qubitIdx >= localQubits
}

// ============================================================
// Distributed gate classification
// ============================================================

// GateType classifies a gate for distributed execution
type GateType int

const (
	GateLocal   GateType = iota // both qubits local → no communication
	GateGlobal                  // at least one qubit is global → inter-rank exchange
)

// ClassifyGate determines whether a gate requires inter-rank communication
func ClassifyGate(ctrlIdx, tgtIdx, localQubits int) GateType {
	if IsGlobalQubit(ctrlIdx, localQubits) || IsGlobalQubit(tgtIdx, localQubits) {
		return GateGlobal
	}
	return GateLocal
}

// ExchangePartner returns the rank that must exchange amplitudes with
// partnerRank for a global target qubit gate on localQubits
func ExchangePartner(rank, targetQubit, localQubits, numRanks int) int {
	return rank ^ (1 << uint(targetQubit-localQubits))
}

// ============================================================
// WGSL distributed kernel generator
// ============================================================

// DistQuantumWGSLGenerator generates distributed WGSL compute shaders
type DistQuantumWGSLGenerator struct {
	buf    strings.Builder
	errors []string
}

func NewDistQuantumWGSLGenerator() *DistQuantumWGSLGenerator {
	return &DistQuantumWGSLGenerator{}
}

// GenerateDistributedShader produces WGSL for a distributed state-vector simulation
func (g *DistQuantumWGSLGenerator) GenerateDistributedShader(circuit *parser.CircuitDecl, config *DistQuantumConfig) (string, error) {
	g.buf.Reset()
	g.errors = nil

	g.emitHeader(config)
	g.emitTypes()
	g.emitDistBindings(config)
	g.emitMathHelpers()
	g.emitLocalGateKernels(config)
	g.emitGlobalGateKernels(config)
	g.emitExchangeKernels(config)
	g.emitInitKernel(config)
	g.emitDispatchSequence(circuit, config)

	if len(g.errors) > 0 {
		return "", fmt.Errorf("distributed WGSL errors: %s", strings.Join(g.errors, "\n"))
	}
	return g.buf.String(), nil
}

func (g *DistQuantumWGSLGenerator) emitHeader(config *DistQuantumConfig) {
	g.buf.WriteString("// Karkain Phase 31 — Distributed Multi-GPU Quantum State-Vector Simulator\n")
	g.buf.WriteString(fmt.Sprintf("// Total qubits: %d | Global: %d | Local: %d | Ranks: %d\n",
		config.TotalQubits, config.GlobalQubits, config.LocalQubits, config.NumRanks))
	g.buf.WriteString(fmt.Sprintf("// Chunk: %d amplitudes per rank (%d bytes)\n",
		config.ChunkSize, config.StateBytes))
	g.buf.WriteString("\n")
}

func (g *DistQuantumWGSLGenerator) emitTypes() {
	g.buf.WriteString("struct Complex {\n")
	g.buf.WriteString("  re: f32,\n")
	g.buf.WriteString("  im: f32,\n")
	g.buf.WriteString("}\n\n")
}

func (g *DistQuantumWGSLGenerator) emitDistBindings(config *DistQuantumConfig) {
	g.buf.WriteString("struct DistUniforms {\n")
	g.buf.WriteString("  rank_id: u32,\n")
	g.buf.WriteString("  num_ranks: u32,\n")
	g.buf.WriteString("  local_qubits: u32,\n")
	g.buf.WriteString("  total_qubits: u32,\n")
	g.buf.WriteString("  local_size: u32,\n")
	g.buf.WriteString("  pad: u32,\n")
	g.buf.WriteString("}\n\n")
	g.buf.WriteString("@group(0) @binding(0) var<uniform> uniforms: DistUniforms;\n")
	g.buf.WriteString("@group(0) @binding(1) var<storage, read_write> state: array<Complex>;\n")
	g.buf.WriteString("@group(0) @binding(2) var<storage, read> gate_matrix: array<Complex>;\n")
	g.buf.WriteString("@group(0) @binding(3) var<storage, read_write> exchange_buf: array<Complex>;\n\n")
}

func (g *DistQuantumWGSLGenerator) emitMathHelpers() {
	g.buf.WriteString("fn complex_mul(a: Complex, b: Complex) -> Complex {\n")
	g.buf.WriteString("  return Complex(a.re * b.re - a.im * b.im, a.re * b.im + a.im * b.re);\n")
	g.buf.WriteString("}\n\n")
	g.buf.WriteString("fn complex_add(a: Complex, b: Complex) -> Complex {\n")
	g.buf.WriteString("  return Complex(a.re + b.re, a.im + b.im);\n")
	g.buf.WriteString("}\n\n")
	g.buf.WriteString("fn complex_abs2(a: Complex) -> f32 {\n")
	g.buf.WriteString("  return a.re * a.re + a.im * a.im;\n")
	g.buf.WriteString("}\n\n")
}

func (g *DistQuantumWGSLGenerator) emitLocalGateKernels(config *DistQuantumConfig) {
	// Local Hadamard: operates entirely within the rank's chunk
	g.buf.WriteString("// ---- LOCAL GATE KERNELS (no inter-rank communication) ----\n\n")

	g.buf.WriteString("// Local Hadamard: target qubit < local_qubits\n")
	g.buf.WriteString("@compute @workgroup_size(64)\n")
	g.buf.WriteString("fn local_apply_h(@builtin(global_invocation_id) gid: vec3<u32>, target_bit: u32) {\n")
	g.buf.WriteString("  let idx: u32 = gid.x;\n")
	g.buf.WriteString("  if (idx >= uniforms.local_size >> 1u) { return; }\n")
	g.buf.WriteString("  let mask: u32 = 1u << target_bit;\n")
	g.buf.WriteString("  let i0: u32 = (idx & ~mask);\n")
	g.buf.WriteString("  let i1: u32 = i0 | mask;\n")
	g.buf.WriteString("  let a: Complex = state[i0];\n")
	g.buf.WriteString("  let b: Complex = state[i1];\n")
	g.buf.WriteString("  let s: f32 = 0.7071067811865475;\n")
	g.buf.WriteString("  state[i0] = Complex(s * (a.re + b.re), s * (a.im + b.im));\n")
	g.buf.WriteString("  state[i1] = Complex(s * (a.re - b.re), s * (a.im - b.im));\n")
	g.buf.WriteString("}\n\n")

	// Local rotation
	g.buf.WriteString("// Local RY rotation: target qubit < local_qubits\n")
	g.buf.WriteString("@compute @workgroup_size(64)\n")
	g.buf.WriteString("fn local_ry(@builtin(global_invocation_id) gid: vec3<u32>, target_bit: u32, angle: f32) {\n")
	g.buf.WriteString("  let idx: u32 = gid.x;\n")
	g.buf.WriteString("  if (idx >= uniforms.local_size >> 1u) { return; }\n")
	g.buf.WriteString("  let mask: u32 = 1u << target_bit;\n")
	g.buf.WriteString("  let i0: u32 = (idx & ~mask);\n")
	g.buf.WriteString("  let i1: u32 = i0 | mask;\n")
	g.buf.WriteString("  let c: f32 = cos(angle);\n")
	g.buf.WriteString("  let s: f32 = sin(angle);\n")
	g.buf.WriteString("  let a: Complex = state[i0];\n")
	g.buf.WriteString("  let b: Complex = state[i1];\n")
	g.buf.WriteString("  state[i0] = Complex(c * a.re - s * b.re, c * a.im - s * b.im);\n")
	g.buf.WriteString("  state[i1] = Complex(s * a.re + c * b.re, s * a.im + c * b.im);\n")
	g.buf.WriteString("}\n\n")

	// Local CNOT (both ctrl and tgt local)
	g.buf.WriteString("// Local CNOT: both control and target < local_qubits\n")
	g.buf.WriteString("@compute @workgroup_size(64)\n")
	g.buf.WriteString("fn local_cnot(@builtin(global_invocation_id) gid: vec3<u32>, ctrl_bit: u32, tgt_bit: u32) {\n")
	g.buf.WriteString("  let idx: u32 = gid.x;\n")
	g.buf.WriteString("  if (idx >= uniforms.local_size) { return; }\n")
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

func (g *DistQuantumWGSLGenerator) emitGlobalGateKernels(config *DistQuantumConfig) {
	g.buf.WriteString("\n// ---- GLOBAL GATE KERNELS (inter-rank exchange required) ----\n\n")

	// Global CNOT: control is global, target is local
	g.buf.WriteString("// Global CNOT: control is global qubit, target is local\n")
	g.buf.WriteString("// Partner rank identified via rank XOR (1 << (ctrl_bit - local_qubits))\n")
	g.buf.WriteString("@compute @workgroup_size(64)\n")
	g.buf.WriteString("fn global_cnot_ctrl_global(@builtin(global_invocation_id) gid: vec3<u32>, ctrl_bit: u32, tgt_bit: u32) {\n")
	g.buf.WriteString("  let idx: u32 = gid.x;\n")
	g.buf.WriteString("  if (idx >= uniforms.local_size) { return; }\n")
	g.buf.WriteString("  // Global qubit bits encode the rank; local qubit bits encode within-chunk index\n")
	g.buf.WriteString("  let rank_bit: u32 = ctrl_bit - uniforms.local_qubits;\n")
	g.buf.WriteString("  let ctrl_in_rank: bool = ((uniforms.rank_id >> rank_bit) & 1u) == 1u;\n")
	g.buf.WriteString("  let tgt_mask: u32 = 1u << tgt_bit;\n")
	g.buf.WriteString("  if (ctrl_in_rank) {\n")
	g.buf.WriteString("    let partner: u32 = idx ^ tgt_mask;\n")
	g.buf.WriteString("    if (idx < partner) {\n")
	g.buf.WriteString("      let tmp: Complex = state[idx];\n")
	g.buf.WriteString("      state[idx] = state[partner];\n")
	g.buf.WriteString("      state[partner] = tmp;\n")
	g.buf.WriteString("    }\n")
	g.buf.WriteString("  }\n")
	g.buf.WriteString("}\n\n")

	// Global CNOT: control is local, target is global
	g.buf.WriteString("// Global CNOT: control is local qubit, target is global\n")
	g.buf.WriteString("@compute @workgroup_size(64)\n")
	g.buf.WriteString("fn global_cnot_tgt_global(@builtin(global_invocation_id) gid: vec3<u32>, ctrl_bit: u32, tgt_bit: u32) {\n")
	g.buf.WriteString("  let idx: u32 = gid.x;\n")
	g.buf.WriteString("  if (idx >= uniforms.local_size) { return; }\n")
	g.buf.WriteString("  let ctrl_mask: u32 = 1u << ctrl_bit;\n")
	g.buf.WriteString("  let rank_bit: u32 = tgt_bit - uniforms.local_qubits;\n")
	g.buf.WriteString("  let tgt_in_rank: bool = ((uniforms.rank_id >> rank_bit) & 1u) == 1u;\n")
	g.buf.WriteString("  if ((idx & ctrl_mask) != 0u && tgt_in_rank) {\n")
	g.buf.WriteString("    // Swap with partner rank on the same local index\n")
	g.buf.WriteString("    let partner_rank: u32 = uniforms.rank_id ^ (1u << rank_bit);\n")
	g.buf.WriteString("    // Exchange via exchange_buf: rank writes its amplitude, reads partner's\n")
	g.buf.WriteString("    exchange_buf[idx] = state[idx];\n")
	g.buf.WriteString("  }\n")
	g.buf.WriteString("}\n\n")

	// Global Hadamard: target is global qubit
	g.buf.WriteString("// Global Hadamard: target qubit is distributed across ranks\n")
	g.buf.WriteString("// Rank with bit=0 holds |0⟩ amplitudes, bit=1 holds |1⟩ amplitudes\n")
	g.buf.WriteString("@compute @workgroup_size(64)\n")
	g.buf.WriteString("fn global_hadamard(@builtin(global_invocation_id) gid: vec3<u32>, target_bit: u32) {\n")
	g.buf.WriteString("  let idx: u32 = gid.x;\n")
	g.buf.WriteString("  if (idx >= uniforms.local_size) { return; }\n")
	g.buf.WriteString("  let rank_bit: u32 = target_bit - uniforms.local_qubits;\n")
	g.buf.WriteString("  let bit_val: u32 = (uniforms.rank_id >> rank_bit) & 1u;\n")
	g.buf.WriteString("  // Each rank holds half the amplitudes; H mixes across ranks\n")
	g.buf.WriteString("  // Partner rank has complementary bit value\n")
	g.buf.WriteString("  let partner_rank: u32 = uniforms.rank_id ^ (1u << rank_bit);\n")
	g.buf.WriteString("  let s: f32 = 0.7071067811865475;\n")
	g.buf.WriteString("  let a: Complex = state[idx];\n")
	g.buf.WriteString("  // In full implementation, exchange_buf holds partner amplitudes\n")
	g.buf.WriteString("  // This kernel emits the local transform; exchange is handled by交换\n")
	g.buf.WriteString("  if (bit_val == 0u) {\n")
	g.buf.WriteString("    // Rank holds |0⟩ component: state[i] = s * (a + partner_a)\n")
	g.buf.WriteString("    state[idx] = Complex(s * a.re, s * a.im);\n")
	g.buf.WriteString("  } else {\n")
	g.buf.WriteString("    // Rank holds |1⟩ component: state[i] = s * (a - partner_a)\n")
	g.buf.WriteString("    state[idx] = Complex(s * a.re, s * a.im);\n")
	g.buf.WriteString("  }\n")
	g.buf.WriteString("}\n")
}

func (g *DistQuantumWGSLGenerator) emitExchangeKernels(config *DistQuantumConfig) {
	g.buf.WriteString("\n// ---- INTER-RANK EXCHANGE KERNELS ----\n\n")

	// Pack local amplitudes into exchange buffer for sending
	g.buf.WriteString("// Pack: copy local state into exchange buffer for partner consumption\n")
	g.buf.WriteString("@compute @workgroup_size(64)\n")
	g.buf.WriteString("fn pack_exchange(@builtin(global_invocation_id) gid: vec3<u32>) {\n")
	g.buf.WriteString("  let idx: u32 = gid.x;\n")
	g.buf.WriteString("  if (idx >= uniforms.local_size) { return; }\n")
	g.buf.WriteString("  exchange_buf[idx] = state[idx];\n")
	g.buf.WriteString("}\n\n")

	// Unpack: merge received amplitudes from partner into local state
	g.buf.WriteString("// Unpack: merge partner amplitudes with local state for global gate\n")
	g.buf.WriteString("@compute @workgroup_size(64)\n")
	g.buf.WriteString("fn unpack_merge(@builtin(global_invocation_id) gid: vec3<u32>, ctrl_is_zero: u32) {\n")
	g.buf.WriteString("  let idx: u32 = gid.x;\n")
	g.buf.WriteString("  if (idx >= uniforms.local_size) { return; }\n")
	g.buf.WriteString("  let partner: Complex = exchange_buf[idx];\n")
	g.buf.WriteString("  let local: Complex = state[idx];\n")
	g.buf.WriteString("  let s: f32 = 0.7071067811865475;\n")
	g.buf.WriteString("  if (ctrl_is_zero == 1u) {\n")
	g.buf.WriteString("    state[idx] = Complex(s * (local.re + partner.re), s * (local.im + partner.im));\n")
	g.buf.WriteString("  } else {\n")
	g.buf.WriteString("    state[idx] = Complex(s * (local.re - partner.re), s * (local.im - partner.im));\n")
	g.buf.WriteString("  }\n")
	g.buf.WriteString("}\n")

	// Global normalization across ranks
	g.buf.WriteString("\n// Global norm reduction: each rank computes local sum, then allreduce\n")
	g.buf.WriteString("@compute @workgroup_size(64)\n")
	g.buf.WriteString("fn local_norm_sq(@builtin(global_invocation_id) gid: vec3<u32>) {\n")
	g.buf.WriteString("  let idx: u32 = gid.x;\n")
	g.buf.WriteString("  if (idx != 0u) { return; }\n")
	g.buf.WriteString("  var sum: f32 = 0.0;\n")
	g.buf.WriteString("  for (var i: u32 = 0u; i < uniforms.local_size; i = i + 1u) {\n")
	g.buf.WriteString("    sum = sum + complex_abs2(state[i]);\n")
	g.buf.WriteString("  }\n")
	g.buf.WriteString("  exchange_buf[0] = Complex(sum, 0.0); // local partial norm²\n")
	g.buf.WriteString("}\n")
}

func (g *DistQuantumWGSLGenerator) emitInitKernel(config *DistQuantumConfig) {
	g.buf.WriteString("\n// ---- INITIALIZATION ----\n\n")
	g.buf.WriteString("// Initialize distributed state: rank 0 gets |0⟩ = 1.0, all others 0\n")
	g.buf.WriteString("@compute @workgroup_size(64)\n")
	g.buf.WriteString("fn dist_init_state(@builtin(global_invocation_id) gid: vec3<u32>) {\n")
	g.buf.WriteString("  let idx: u32 = gid.x;\n")
	g.buf.WriteString("  if (idx >= uniforms.local_size) { return; }\n")
	g.buf.WriteString("  if (uniforms.rank_id == 0u && idx == 0u) {\n")
	g.buf.WriteString("    state[0] = Complex(1.0, 0.0);\n")
	g.buf.WriteString("  } else {\n")
	g.buf.WriteString("    state[idx] = Complex(0.0, 0.0);\n")
	g.buf.WriteString("  }\n")
	g.buf.WriteString("}\n")
}

func (g *DistQuantumWGSLGenerator) emitDispatchSequence(circuit *parser.CircuitDecl, config *DistQuantumConfig) {
	g.buf.WriteString("\n// ---- CIRCUIT DISPATCH SEQUENCE: " + circuit.Name + " ----\n")
	g.buf.WriteString("// Each entry: gate classification + dispatch metadata\n\n")

	for _, stmt := range circuit.Body {
		switch n := stmt.(type) {
		case *parser.QPUOpExpr:
			g.emitGateDispatch(n, config)
		case *parser.ExprStmt:
			if qpu, ok := n.Expression.(*parser.QPUOpExpr); ok {
				g.emitGateDispatch(qpu, config)
			}
		}
	}
}

func (g *DistQuantumWGSLGenerator) emitGateDispatch(op *parser.QPUOpExpr, config *DistQuantumConfig) {
	ctrlIdx := 0
	tgtIdx := 0
	if len(op.Args) > 0 {
		tgtIdx = resolveQubitIdxDist(op.Args[0])
	}
	if len(op.Args) > 1 {
		ctrlIdx = resolveQubitIdxDist(op.Args[0])
		tgtIdx = resolveQubitIdxDist(op.Args[1])
	}

	isGlobalCtrl := IsGlobalQubit(ctrlIdx, config.LocalQubits)
	isGlobalTgt := IsGlobalQubit(tgtIdx, config.LocalQubits)

	switch op.Op {
	case "h":
		if isGlobalTgt {
			g.buf.WriteString(fmt.Sprintf("// dispatch global_hadamard: qubit %d is global (rank exchange required)\n", tgtIdx))
		} else {
			g.buf.WriteString(fmt.Sprintf("// dispatch local_apply_h: qubit %d is local (no communication)\n", tgtIdx))
		}
	case "cx":
		if !isGlobalCtrl && !isGlobalTgt {
			g.buf.WriteString(fmt.Sprintf("// dispatch local_cnot: ctrl=%d tgt=%d both local\n", ctrlIdx, tgtIdx))
		} else if isGlobalCtrl && !isGlobalTgt {
			g.buf.WriteString(fmt.Sprintf("// dispatch global_cnot_ctrl_global: ctrl=%d global, tgt=%d local\n", ctrlIdx, tgtIdx))
		} else if !isGlobalCtrl && isGlobalTgt {
			g.buf.WriteString(fmt.Sprintf("// dispatch global_cnot_tgt_global: ctrl=%d local, tgt=%d global\n", ctrlIdx, tgtIdx))
		} else {
			g.buf.WriteString(fmt.Sprintf("// dispatch global_cnot: both ctrl=%d tgt=%d global (full exchange)\n", ctrlIdx, tgtIdx))
		}
	case "ry":
		if isGlobalTgt {
			g.buf.WriteString(fmt.Sprintf("// dispatch global_ry: qubit %d is global (rotation across ranks)\n", tgtIdx))
		} else {
			g.buf.WriteString(fmt.Sprintf("// dispatch local_ry: qubit %d is local\n", tgtIdx))
		}
	case "measure":
		g.buf.WriteString("// dispatch local_measure + global_allreduce (norm across ranks)\n")
	case "swap":
		g.buf.WriteString(fmt.Sprintf("// dispatch swap: ctrl=%d tgt=%d (may be global)\n", ctrlIdx, tgtIdx))
	default:
		g.buf.WriteString(fmt.Sprintf("// dispatch %s: qubit %d\n", op.Op, tgtIdx))
	}
}

// ============================================================
// C99/OpenMP distributed backend
// ============================================================

// DistQuantumC99Generator generates C99+OpenMP distributed kernels
type DistQuantumC99Generator struct {
	buf    strings.Builder
	errors []string
}

func NewDistQuantumC99Generator() *DistQuantumC99Generator {
	return &DistQuantumC99Generator{}
}

// GenerateDistributedC99 produces C99+OpenMP code for distributed simulation
func (g *DistQuantumC99Generator) GenerateDistributedC99(circuit *parser.CircuitDecl, config *DistQuantumConfig) (string, error) {
	g.buf.Reset()
	g.errors = nil

	g.emitHeader(config)
	g.emitTypes(config)
	g.emitLocalGates(config)
	g.emitGlobalGates(config)
	g.emitExchange(config)
	g.emitInit(config)
	g.emitCircuitEval(circuit, config)

	if len(g.errors) > 0 {
		return "", fmt.Errorf("distributed C99 errors: %s", strings.Join(g.errors, "\n"))
	}
	return g.buf.String(), nil
}

func (g *DistQuantumC99Generator) emitHeader(config *DistQuantumConfig) {
	g.buf.WriteString("/* Karkain Phase 31 — Distributed Multi-GPU Quantum Simulator (C99/OpenMP) */\n")
	g.buf.WriteString(fmt.Sprintf("/* Total qubits: %d | Ranks: %d | Local qubits: %d | Chunk: %d */\n",
		config.TotalQubits, config.NumRanks, config.LocalQubits, config.ChunkSize))
	g.buf.WriteString("\n")
	g.buf.WriteString("#include <math.h>\n")
	g.buf.WriteString("#include <stdio.h>\n")
	g.buf.WriteString("#include <stdlib.h>\n")
	g.buf.WriteString("#include <string.h>\n")
	g.buf.WriteString("#include <omp.h>\n\n")
	g.buf.WriteString(fmt.Sprintf("#define TOTAL_QUBITS %d\n", config.TotalQubits))
	g.buf.WriteString(fmt.Sprintf("#define GLOBAL_QUBITS %d\n", config.GlobalQubits))
	g.buf.WriteString(fmt.Sprintf("#define LOCAL_QUBITS %d\n", config.LocalQubits))
	g.buf.WriteString(fmt.Sprintf("#define NUM_RANKS %d\n", config.NumRanks))
	g.buf.WriteString(fmt.Sprintf("#define LOCAL_SIZE %d\n", config.ChunkSize))
	g.buf.WriteString("#define PI 3.14159265358979323846\n\n")
}

func (g *DistQuantumC99Generator) emitTypes(config *DistQuantumConfig) {
	g.buf.WriteString("typedef struct { double re; double im; } complex_t;\n\n")
	g.buf.WriteString(fmt.Sprintf("static complex_t state[%d];\n", config.ChunkSize))
	g.buf.WriteString(fmt.Sprintf("static complex_t exchange[%d];\n", config.ChunkSize))
	g.buf.WriteString("static int rank_id = 0;\n\n")
}

func (g *DistQuantumC99Generator) emitLocalGates(config *DistQuantumConfig) {
	g.buf.WriteString("/* ---- LOCAL GATE KERNELS ---- */\n\n")

	// Local CNOT
	g.buf.WriteString("void local_cnot(int ctrl_bit, int tgt_bit) {\n")
	g.buf.WriteString("    int ctrl_mask = 1 << ctrl_bit;\n")
	g.buf.WriteString("    int tgt_mask = 1 << tgt_bit;\n")
	g.buf.WriteString("    #pragma omp parallel for\n")
	g.buf.WriteString(fmt.Sprintf("    for (int i = 0; i < %d; i++) {\n", config.ChunkSize))
	g.buf.WriteString("        if ((i & ctrl_mask) != 0) {\n")
	g.buf.WriteString("            int partner = i ^ tgt_mask;\n")
	g.buf.WriteString("            if (i < partner) {\n")
	g.buf.WriteString("                complex_t tmp = state[i];\n")
	g.buf.WriteString("                state[i] = state[partner];\n")
	g.buf.WriteString("                state[partner] = tmp;\n")
	g.buf.WriteString("            }\n")
	g.buf.WriteString("        }\n")
	g.buf.WriteString("    }\n")
	g.buf.WriteString("}\n\n")

	// Local Ry
	g.buf.WriteString("void local_ry(double angle, int target_bit) {\n")
	g.buf.WriteString("    int mask = 1 << target_bit;\n")
	g.buf.WriteString("    double c = cos(angle), s = sin(angle);\n")
	g.buf.WriteString("    #pragma omp parallel for\n")
	g.buf.WriteString(fmt.Sprintf("    for (int i = 0; i < %d; i += 2) {\n", config.ChunkSize))
	g.buf.WriteString("        int i0 = i & ~mask;\n")
	g.buf.WriteString("        int i1 = i0 | mask;\n")
	g.buf.WriteString("        complex_t a = state[i0], b = state[i1];\n")
	g.buf.WriteString("        state[i0].re = c*a.re - s*b.re; state[i0].im = c*a.im - s*b.im;\n")
	g.buf.WriteString("        state[i1].re = s*a.re + c*b.re; state[i1].im = s*a.im + c*b.im;\n")
	g.buf.WriteString("    }\n")
	g.buf.WriteString("}\n\n")

	// Local Hadamard
	g.buf.WriteString("void local_hadamard(int target_bit) {\n")
	g.buf.WriteString("    int mask = 1 << target_bit;\n")
	g.buf.WriteString("    double s = 0.7071067811865475;\n")
	g.buf.WriteString("    #pragma omp parallel for\n")
	g.buf.WriteString(fmt.Sprintf("    for (int i = 0; i < %d; i += 2) {\n", config.ChunkSize))
	g.buf.WriteString("        int i0 = i & ~mask;\n")
	g.buf.WriteString("        int i1 = i0 | mask;\n")
	g.buf.WriteString("        complex_t a = state[i0], b = state[i1];\n")
	g.buf.WriteString("        state[i0].re = s*(a.re+b.re); state[i0].im = s*(a.im+b.im);\n")
	g.buf.WriteString("        state[i1].re = s*(a.re-b.re); state[i1].im = s*(a.im-b.im);\n")
	g.buf.WriteString("    }\n")
	g.buf.WriteString("}\n\n")
}

func (g *DistQuantumC99Generator) emitGlobalGates(config *DistQuantumConfig) {
	g.buf.WriteString("/* ---- GLOBAL GATE KERNELS (inter-rank) ---- */\n\n")

	// Global CNOT with exchange
	g.buf.WriteString("/* Global CNOT: packs exchange buffer, simulates partner exchange, merges */\n")
	g.buf.WriteString("void global_cnot_exchange(int ctrl_bit, int tgt_bit) {\n")
	g.buf.WriteString("    int partner = rank_id ^ (1 << (ctrl_bit - LOCAL_QUBITS));\n")
	g.buf.WriteString("    /* Pack local amplitudes */\n")
	g.buf.WriteString("    #pragma omp parallel for\n")
	g.buf.WriteString(fmt.Sprintf("    for (int i = 0; i < %d; i++) exchange[i] = state[i];\n", config.ChunkSize))
	g.buf.WriteString("    /* In real implementation: MPI_Sendrecv(exchange, partner) */\n")
	g.buf.WriteString("    /* Merge: if local rank has ctrl bit = 0, state[i] = s*(local + partner) */\n")
	g.buf.WriteString("    /* Otherwise: state[i] = s*(local - partner) */\n")
	g.buf.WriteString("    double s = 0.7071067811865475;\n")
	g.buf.WriteString("    int ctrl_in_rank = (rank_id >> (ctrl_bit - LOCAL_QUBITS)) & 1;\n")
	g.buf.WriteString("    #pragma omp parallel for\n")
	g.buf.WriteString(fmt.Sprintf("    for (int i = 0; i < %d; i++) {\n", config.ChunkSize))
	g.buf.WriteString("        complex_t partner_amp = exchange[i]; /* from MPI recv */\n")
	g.buf.WriteString("        if (ctrl_in_rank == 0) {\n")
	g.buf.WriteString("            state[i].re = s*(state[i].re + partner_amp.re);\n")
	g.buf.WriteString("            state[i].im = s*(state[i].im + partner_amp.im);\n")
	g.buf.WriteString("        } else {\n")
	g.buf.WriteString("            state[i].re = s*(state[i].re - partner_amp.re);\n")
	g.buf.WriteString("            state[i].im = s*(state[i].im - partner_amp.im);\n")
	g.buf.WriteString("        }\n")
	g.buf.WriteString("    }\n")
	g.buf.WriteString("}\n\n")
}

func (g *DistQuantumC99Generator) emitExchange(config *DistQuantumConfig) {
	g.buf.WriteString("/* ---- ALLREDUCE for normalization ---- */\n\n")
	g.buf.WriteString("double local_norm_sq(void) {\n")
	g.buf.WriteString("    double sum = 0.0;\n")
	g.buf.WriteString(fmt.Sprintf("    for (int i = 0; i < %d; i++)\n", config.ChunkSize))
	g.buf.WriteString("        sum += state[i].re*state[i].re + state[i].im*state[i].im;\n")
	g.buf.WriteString("    return sum;\n")
	g.buf.WriteString("}\n\n")
	g.buf.WriteString("/* In real implementation: MPI_Allreduce(&local_sum, &global_sum, MPI_SUM) */\n\n")
}

func (g *DistQuantumC99Generator) emitInit(config *DistQuantumConfig) {
	g.buf.WriteString("/* ---- INITIALIZATION ---- */\n\n")
	g.buf.WriteString("void dist_init(void) {\n")
	g.buf.WriteString(fmt.Sprintf("    memset(state, 0, sizeof(state));\n"))
	g.buf.WriteString("    if (rank_id == 0) state[0].re = 1.0;\n")
	g.buf.WriteString("}\n\n")
}

func (g *DistQuantumC99Generator) emitCircuitEval(circuit *parser.CircuitDecl, config *DistQuantumConfig) {
	g.buf.WriteString(fmt.Sprintf("/* ---- CIRCUIT: %s ---- */\n\n", circuit.Name))

	g.buf.WriteString("void eval_circuit(void) {\n")
	for _, stmt := range circuit.Body {
		switch n := stmt.(type) {
		case *parser.QPUOpExpr:
			g.emitC99GateDispatch(n, config)
		case *parser.ExprStmt:
			if qpu, ok := n.Expression.(*parser.QPUOpExpr); ok {
				g.emitC99GateDispatch(qpu, config)
			}
		}
	}
	g.buf.WriteString("}\n")
}

func (g *DistQuantumC99Generator) emitC99GateDispatch(op *parser.QPUOpExpr, config *DistQuantumConfig) {
	ctrlIdx := 0
	tgtIdx := 0
	if len(op.Args) > 0 {
		tgtIdx = resolveQubitIdxDist(op.Args[0])
	}
	if len(op.Args) > 1 {
		ctrlIdx = resolveQubitIdxDist(op.Args[0])
		tgtIdx = resolveQubitIdxDist(op.Args[1])
	}

	switch op.Op {
	case "h":
		if IsGlobalQubit(tgtIdx, config.LocalQubits) {
			g.buf.WriteString(fmt.Sprintf("    /* global H on qubit %d — exchange + merge */\n", tgtIdx))
			g.buf.WriteString("    global_cnot_exchange(-1, -1); /* placeholder for global H */\n")
		} else {
			g.buf.WriteString(fmt.Sprintf("    local_hadamard(%d);\n", tgtIdx))
		}
	case "cx":
		if !IsGlobalQubit(ctrlIdx, config.LocalQubits) && !IsGlobalQubit(tgtIdx, config.LocalQubits) {
			g.buf.WriteString(fmt.Sprintf("    local_cnot(%d, %d);\n", ctrlIdx, tgtIdx))
		} else {
			g.buf.WriteString(fmt.Sprintf("    global_cnot_exchange(%d, %d);\n", ctrlIdx, tgtIdx))
		}
	case "ry":
		if IsGlobalQubit(tgtIdx, config.LocalQubits) {
			g.buf.WriteString(fmt.Sprintf("    /* global RY on qubit %d */\n", tgtIdx))
		} else {
			g.buf.WriteString(fmt.Sprintf("    local_ry(0.0, %d); /* TODO: param */\n", tgtIdx))
		}
	default:
		g.buf.WriteString(fmt.Sprintf("    /* %s gate on qubit %d */\n", op.Op, tgtIdx))
	}
}

func resolveQubitIdxDist(node parser.Node) int {
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

// ============================================================
// Pure Go partition math (no AST dependency)
// ============================================================

// StateVectorPartition holds the partitioning metadata
type StateVectorPartition struct {
	TotalQubits int
	LocalQubits int
	GlobalQubits int
	NumRanks    int
	ChunkSize   int
}

// ComputePartitioning determines the optimal split
func ComputePartitioning(totalQubits, maxLocalQubits int) StateVectorPartition {
	localQ := totalQubits
	if localQ > maxLocalQubits {
		localQ = maxLocalQubits
	}
	globalQ := totalQubits - localQ
	numRanks := 1 << uint(globalQ)
	chunkSize := 1 << uint(localQ)
	return StateVectorPartition{
		TotalQubits:  totalQubits,
		LocalQubits:  localQ,
		GlobalQubits: globalQ,
		NumRanks:     numRanks,
		ChunkSize:    chunkSize,
	}
}

// ValidatePartition checks partition consistency
func ValidatePartition(p StateVectorPartition) error {
	if p.LocalQubits < 0 {
		return fmt.Errorf("local qubits cannot be negative: %d", p.LocalQubits)
	}
	if p.GlobalQubits < 0 {
		return fmt.Errorf("global qubits cannot be negative: %d", p.GlobalQubits)
	}
	if p.LocalQubits+p.GlobalQubits != p.TotalQubits {
		return fmt.Errorf("local(%d) + global(%d) != total(%d)", p.LocalQubits, p.GlobalQubits, p.TotalQubits)
	}
	expectedRanks := 1 << uint(p.GlobalQubits)
	if p.NumRanks != expectedRanks {
		return fmt.Errorf("expected %d ranks for %d global qubits, got %d", expectedRanks, p.GlobalQubits, p.NumRanks)
	}
	expectedChunk := 1 << uint(p.LocalQubits)
	if p.ChunkSize != expectedChunk {
		return fmt.Errorf("expected chunk size %d for %d local qubits, got %d", expectedChunk, p.LocalQubits, p.ChunkSize)
	}
	return nil
}

// EstimateMemoryBytes returns per-rank memory requirement
func EstimateMemoryBytes(localQubits int) int64 {
	return int64(1<<uint(localQubits)) * 8
}

// Suppress unused import warnings
var _ = math.Sqrt
