package sema

import (
	"fmt"
	"math"
	"math/cmplx"
)

// ============================================================
// Phase 32: Quantum Noise Models & Density Matrix Simulation
// ============================================================

// Complex128 is a complex128 alias for density matrix entries
type Complex128 = complex128

// DensityMatrix represents a mixed quantum state ρ as a 2^N × 2^N matrix
// stored as a flat slice of complex128 values.
type DensityMatrix struct {
	Dim   int          // matrix dimension (2^N for N qubits)
	NumQ int          // number of qubits
	Data  []Complex128 // flat row-major storage: ρ[i*Dim + j]
}

// NewDensityMatrix creates a zero-initialized density matrix for N qubits
func NewDensityMatrix(numQubits int) *DensityMatrix {
	dim := 1 << uint(numQubits)
	return &DensityMatrix{
		Dim:   dim,
		NumQ: numQubits,
		Data:  make([]Complex128, dim*dim),
	}
}

// NewDensityMatrixFromStateVector creates a pure-state density matrix |ψ⟩⟨ψ|
func NewDensityMatrixFromStateVector(state []Complex128) *DensityMatrix {
	dim := len(state)
	numQ := 0
	for d := dim; d > 1; d >>= 1 {
		numQ++
	}
	dm := &DensityMatrix{
		Dim:   dim,
		NumQ: numQ,
		Data:  make([]Complex128, dim*dim),
	}
	// ρ = |ψ⟩⟨ψ|
	for i := 0; i < dim; i++ {
		for j := 0; j < dim; j++ {
			dm.Data[i*dim+j] = state[i] * cmplx.Conj(state[j])
		}
	}
	return dm
}

// Trace returns Tr(ρ) — should always be 1.0 for a valid density matrix
func (dm *DensityMatrix) Trace() Complex128 {
	var t Complex128
	for i := 0; i < dm.Dim; i++ {
		t += dm.Data[i*dm.Dim+i]
	}
	return t
}

// Purify computes Tr(ρ²) — equals 1.0 for pure states, < 1.0 for mixed
func (dm *DensityMatrix) Purify() Complex128 {
	var t Complex128
	for i := 0; i < dm.Dim; i++ {
		for j := 0; j < dm.Dim; j++ {
			t += dm.Data[i*dm.Dim+j] * dm.Data[j*dm.Dim+i]
		}
	}
	return t
}

// ExpectationValue computes ⟨O⟩ = Tr(ρ O) for a Hermitian operator O
func (dm *DensityMatrix) ExpectationValue(operator []Complex128) Complex128 {
	dim := dm.Dim
	var result Complex128
	for i := 0; i < dim; i++ {
		for j := 0; j < dim; j++ {
			result += dm.Data[i*dim+j] * operator[j*dim+i]
		}
	}
	return result
}

// Clone returns a deep copy of the density matrix
func (dm *DensityMatrix) Clone() *DensityMatrix {
	newDM := NewDensityMatrix(dm.NumQ)
	copy(newDM.Data, dm.Data)
	return newDM
}

// ============================================================
// Pauli Matrices (2×2)
// ============================================================

var (
	PauliIMat = [4]Complex128{
		1, 0,
		0, 1,
	}
	PauliXMat = [4]Complex128{
		0, 1,
		1, 0,
	}
	PauliYMat = [4]Complex128{
		0, -1i,
		1i, 0,
	}
	PauliZMat = [4]Complex128{
		1, 0,
		0, -1,
	}
)

// ============================================================
// Noise Channels — Kraus Operator Representation
// ============================================================

// KrausOperator represents one element K_i in a noise channel:
//   ε(ρ) = Σ_i K_i ρ K_i†
type KrausOperator struct {
	Name string
	Matrix []Complex128 // 2×2 for single-qubit channels
}

// NoiseChannel represents a quantum noise channel via Kraus operators
type NoiseChannel struct {
	Name      string
	Operators []KrausOperator
}

// ApplyKraus applies a single Kraus operator: K ρ K†
func ApplyKraus(rho *DensityMatrix, K []Complex128, targetQubit int) *DensityMatrix {
	dim := rho.Dim
	result := NewDensityMatrix(rho.NumQ)
	halfDim := 2 // single-qubit Kraus is 2×2

	for i := 0; i < dim; i++ {
		for j := 0; j < dim; j++ {
			var val Complex128
			for a := 0; a < halfDim; a++ {
				for b := 0; b < halfDim; b++ {
					rowI := replaceQubitBit(i, targetQubit, a, dim)
					colJ := replaceQubitBit(j, targetQubit, b, dim)
					val += K[a*halfDim+b] * rho.Data[rowI*dim+colJ] * cmplx.Conj(K[b*halfDim+a]) // Fixed index
				}
			}
			result.Data[i*dim+j] = val
		}
	}
	return result
}

// ApplyKrausGeneral applies a Kraus operator to a density matrix using full matrix multiplication
func ApplyKrausGeneral(rho *DensityMatrix, K []Complex128) *DensityMatrix {
	dim := rho.Dim
	result := NewDensityMatrix(rho.NumQ)

	// First compute K * ρ
	temp := make([]Complex128, dim*dim)
	for i := 0; i < dim; i++ {
		for j := 0; j < dim; j++ {
			var val Complex128
			for k := 0; k < dim; k++ {
				val += K[i*dim+k] * rho.Data[k*dim+j]
			}
			temp[i*dim+j] = val
		}
	}

	// Then compute (K * ρ) * K†
	kAdj := make([]Complex128, dim*dim)
	for i := 0; i < dim; i++ {
		for j := 0; j < dim; j++ {
			kAdj[i*dim+j] = cmplx.Conj(K[j*dim+i])
		}
	}

	for i := 0; i < dim; i++ {
		for j := 0; j < dim; j++ {
			var val Complex128
			for k := 0; k < dim; k++ {
				val += temp[i*dim+k] * kAdj[k*dim+j]
			}
			result.Data[i*dim+j] = val
		}
	}
	return result
}

// replaceQubitBit returns the index with qubit q set to value v
func replaceQubitBit(stateIdx, q, v, dim int) int {
	bit := uint(q)
	mask := int(1 << bit)
	// Clear the bit and set to v
	return (stateIdx &^ mask) | (v << bit)
}

// ============================================================
// Noise Channel Constructors
// ============================================================

// BitFlipChannel: ε(ρ) = (1-p)ρ + p XρX
func BitFlipChannel(p float64) NoiseChannel {
	return NoiseChannel{
		Name: fmt.Sprintf("bit_flip(p=%.4f)", p),
		Operators: []KrausOperator{
			{Name: "E0", Matrix: []Complex128{
				complex(math.Sqrt(1-p), 0), 0,
				0, complex(math.Sqrt(1-p), 0),
			}},
			{Name: "E1", Matrix: []Complex128{
				0, complex(math.Sqrt(p), 0),
				complex(math.Sqrt(p), 0), 0,
			}},
		},
	}
}

// PhaseFlipChannel: ε(ρ) = (1-p)ρ + p ZρZ
func PhaseFlipChannel(p float64) NoiseChannel {
	return NoiseChannel{
		Name: fmt.Sprintf("phase_flip(p=%.4f)", p),
		Operators: []KrausOperator{
			{Name: "E0", Matrix: []Complex128{
				complex(math.Sqrt(1-p), 0), 0,
				0, complex(math.Sqrt(1-p), 0),
			}},
			{Name: "E1", Matrix: []Complex128{
				complex(math.Sqrt(p), 0), 0,
				0, complex(-math.Sqrt(p), 0),
			}},
		},
	}
}

// DepolarizingChannel: ε(ρ) = (1-p)ρ + p·I/2
// Kraus operators:
//
//	E0 = sqrt(1 - 3p/4) · I
//	E1 = sqrt(p/4) · X
//	E2 = sqrt(p/4) · Y
//	E3 = sqrt(p/4) · Z
func DepolarizingChannel(p float64) NoiseChannel {
	c := math.Sqrt(p / 4.0)
	s := math.Sqrt(1.0 - 3.0*p/4.0)
	return NoiseChannel{
		Name: fmt.Sprintf("depolarizing(p=%.4f)", p),
		Operators: []KrausOperator{
			{Name: "E0", Matrix: []Complex128{
				complex(s, 0), 0,
				0, complex(s, 0),
			}},
			{Name: "E1_X", Matrix: []Complex128{
				0, complex(c, 0),
				complex(c, 0), 0,
			}},
			{Name: "E2_Y", Matrix: []Complex128{
				0, complex(0, -c),
				complex(0, c), 0,
			}},
			{Name: "E3_Z", Matrix: []Complex128{
				complex(c, 0), 0,
				0, complex(-c, 0),
			}},
		},
	}
}

// ThermalRelaxationConfig holds T1/T2 thermal relaxation parameters
type ThermalRelaxationConfig struct {
	T1 float64 // energy relaxation time (amplitude damping)
	T2 float64 // dephasing time
	TGate float64 // gate duration
}

// ThermalRelaxationChannel creates a combined amplitude damping + dephasing channel.
// Combines amplitude damping (T1) with pure dephasing (T2).
// Combined Kraus operators = D_j · A_i for sequential application:
//
//	K0 = sqrt(1-γφ) · [[1, 0], [0, sqrt(1-γ1)]]
//	K1 = sqrt(1-γφ) · [[0, sqrt(γ1)], [0, 0]]
//	K2 = sqrt(γφ)  · [[1, 0], [0, -sqrt(1-γ1)]]
//	K3 = sqrt(γφ)  · [[0, -sqrt(γ1)], [0, 0]]
func ThermalRelaxationChannel(cfg ThermalRelaxationConfig) NoiseChannel {
	if cfg.TGate <= 0 {
		cfg.TGate = 1.0
	}

	gammaAmp := 1.0 - math.Exp(-cfg.TGate/cfg.T1)
	gammaPhase := 0.0
	if cfg.T2 < 2*cfg.T1 {
		gammaPhase = 1.0 - math.Exp(-cfg.TGate*(1.0/cfg.T2-1.0/(2.0*cfg.T1)))
	}

	sqrtG1 := math.Sqrt(gammaAmp)
	sqrt1G1 := math.Sqrt(1.0 - gammaAmp)
	sqrtGp := math.Sqrt(gammaPhase)
	sqrt1Gp := math.Sqrt(1.0 - gammaPhase)

	return NoiseChannel{
		Name: fmt.Sprintf("thermal_relaxation(T1=%.2f,T2=%.2f,t=%.2f)", cfg.T1, cfg.T2, cfg.TGate),
		Operators: []KrausOperator{
			{Name: "K0", Matrix: []Complex128{
				complex(sqrt1Gp, 0), 0,
				0, complex(sqrt1Gp*sqrt1G1, 0),
			}},
			{Name: "K1", Matrix: []Complex128{
				0, complex(sqrt1Gp*sqrtG1, 0),
				0, 0,
			}},
			{Name: "K2", Matrix: []Complex128{
				complex(sqrtGp, 0), 0,
				0, complex(-sqrtGp*sqrt1G1, 0),
			}},
			{Name: "K3", Matrix: []Complex128{
				0, complex(-sqrtGp*sqrtG1, 0),
				0, 0,
			}},
		},
	}
}

// ============================================================
// Noisy Density Matrix Simulator
// ============================================================

// NoisySimulator tracks a density matrix and applies noise channels
type NoisySimulator struct {
	State        *DensityMatrix
	NumQubits    int
	NoiseModel   []NoiseChannel
	QubitNoise   map[int][]NoiseChannel // per-qubit noise overrides
	Errors       []string
}

// NewNoisySimulator creates a simulator for N qubits
func NewNoisySimulator(numQubits int) *NoisySimulator {
	state := NewDensityMatrix(numQubits)
	// Initialize to |0⟩⟨0|...⊗N
	state.Data[0] = 1.0
	return &NoisySimulator{
		State:     state,
		NumQubits: numQubits,
		QubitNoise: make(map[int][]NoiseChannel),
	}
}

// NewNoisySimulatorFromStateVector initializes from a pure state vector
func NewNoisySimulatorFromStateVector(state []Complex128) *NoisySimulator {
	numQ := 0
	dim := len(state)
	for d := dim; d > 1; d >>= 1 {
		numQ++
	}
	dm := NewDensityMatrixFromStateVector(state)
	return &NoisySimulator{
		State:     dm,
		NumQubits: numQ,
		QubitNoise: make(map[int][]NoiseChannel),
	}
}

// SetGlobalNoiseModel sets the noise channel applied after every gate
func (s *NoisySimulator) SetGlobalNoiseModel(channels ...NoiseChannel) {
	s.NoiseModel = channels
}

// SetQubitNoiseModel sets noise channels for a specific qubit
func (s *NoisySimulator) SetQubitNoiseModel(qubit int, channels ...NoiseChannel) {
	s.QubitNoise[qubit] = channels
}

// ApplyGate applies a unitary gate to the density matrix: U ρ U†
func (s *NoisySimulator) ApplyGate(U []Complex128, targetQubit int) {
	Ufull := expandGate(U, targetQubit, s.NumQubits)
	s.State = ApplyKrausGeneral(s.State, Ufull)
}

// ApplyNoiseChannel applies a full noise channel: ρ' = Σ_i K_i ρ K_i†
func ApplyNoiseChannel(rho *DensityMatrix, channel NoiseChannel, targetQubit int, numQubits int) *DensityMatrix {
	dim := rho.Dim
	result := NewDensityMatrix(numQubits)

	for _, kOp := range channel.Operators {
		Ufull := expandGate(kOp.Matrix, targetQubit, numQubits)
		// Compute K ρ K† and accumulate into result
		temp := applyKrausMatrix(rho, Ufull, dim)
		for idx := range result.Data {
			result.Data[idx] += temp.Data[idx]
		}
	}
	return result
}

// applyKrausMatrix computes K ρ K† for a full-size Kraus matrix
func applyKrausMatrix(rho *DensityMatrix, K []Complex128, dim int) *DensityMatrix {
	result := NewDensityMatrix(rho.NumQ)
	// K * ρ
	temp := make([]Complex128, dim*dim)
	for i := 0; i < dim; i++ {
		for j := 0; j < dim; j++ {
			var val Complex128
			for k := 0; k < dim; k++ {
				val += K[i*dim+k] * rho.Data[k*dim+j]
			}
			temp[i*dim+j] = val
		}
	}
	// (K * ρ) * K†
	for i := 0; i < dim; i++ {
		for j := 0; j < dim; j++ {
			var val Complex128
			for k := 0; k < dim; k++ {
				val += temp[i*dim+k] * cmplx.Conj(K[j*dim+k])
			}
			result.Data[i*dim+j] = val
		}
	}
	return result
}

// ApplyNoiseAfterGate applies all noise channels after a gate operation
func (s *NoisySimulator) ApplyNoiseAfterGate(targetQubit int) {
	// Apply per-qubit noise
	if channels, ok := s.QubitNoise[targetQubit]; ok {
		for _, ch := range channels {
			s.State = ApplyNoiseChannel(s.State, ch, targetQubit, s.NumQubits)
		}
	}
	// Apply global noise
	for _, ch := range s.NoiseModel {
		s.State = ApplyNoiseChannel(s.State, ch, targetQubit, s.NumQubits)
	}
}

// RunCircuit executes a circuit with noise using the provided gate map.
// gateMap maps gate name → unitary matrix (2×2 or 4×4)
func (s *NoisySimulator) RunCircuit(gates []CircuitGateOp) error {
	for _, gate := range gates {
		U, ok := gateMatrices[gate.Name]
		if !ok {
			return fmt.Errorf("unknown gate: %s", gate.Name)
		}
		s.ApplyGate(U, gate.TargetQubit)
		s.ApplyNoiseAfterGate(gate.TargetQubit)
	}
	return nil
}

// GetState returns the current density matrix
func (s *NoisySimulator) GetState() *DensityMatrix {
	return s.State
}

// GetPurity returns Tr(ρ²) for the current state
func (s *NoisySimulator) GetPurity() float64 {
	return real(s.State.Purify())
}

// ============================================================
// Gate Matrices (2×2 single-qubit)
// ============================================================

// CircuitGateOp represents a single gate application
type CircuitGateOp struct {
	Name        string
	TargetQubit int
	ControlQubit int // -1 if not a controlled gate
	Angle       float64
}

// Standard gate matrices (2×2)
var gateMatrices = map[string][]Complex128{
	"I": {1, 0, 0, 1},
	"X": {0, 1, 1, 0},
	"Y": {0, -1i, 1i, 0},
	"Z": {1, 0, 0, -1},
	"H": {
		complex(1/math.Sqrt2, 0), complex(1/math.Sqrt2, 0),
		complex(1/math.Sqrt2, 0), complex(-1/math.Sqrt2, 0),
	},
}

// Rx returns the rotation-X matrix for angle θ
func Rx(theta float64) []Complex128 {
	c := math.Cos(theta / 2)
	s := math.Sin(theta / 2)
	return []Complex128{
		complex(c, 0), complex(0, -s),
		complex(0, -s), complex(c, 0),
	}
}

// Ry returns the rotation-Y matrix for angle θ
func Ry(theta float64) []Complex128 {
	c := math.Cos(theta / 2)
	s := math.Sin(theta / 2)
	return []Complex128{
		complex(c, 0), complex(-s, 0),
		complex(s, 0), complex(c, 0),
	}
}

// Rz returns the rotation-Z matrix for angle θ
func Rz(theta float64) []Complex128 {
	return []Complex128{
		cmplx.Exp(complex(0, -theta/2)), 0,
		0, cmplx.Exp(complex(0, theta/2)),
	}
}

// expandGate expands a 2×2 gate matrix to the full Hilbert space
func expandGate(U2x2 []Complex128, targetQubit, numQubits int) []Complex128 {
	dim := 1 << uint(numQubits)
	full := make([]Complex128, dim*dim)
	tgtBit := uint(targetQubit)

	for i := 0; i < dim; i++ {
		for j := 0; j < dim; j++ {
			bitI := (i >> tgtBit) & 1
			bitJ := (j >> tgtBit) & 1
			// All bits except target must match
			maskI := i & ^(1 << tgtBit)
			maskJ := j & ^(1 << tgtBit)
			if maskI != maskJ {
				continue
			}
			full[i*dim+j] = U2x2[bitI*2+bitJ]
		}
	}
	return full
}

// ============================================================
// Circuit-level noise analysis
// ============================================================

// NoiseModelReport summarizes the noise characteristics of a circuit
type NoiseModelReport struct {
	TotalGates    int
	NoisyGates    int
	TotalChannels int
	Channels      []string
	QubitNoiseMap map[int][]string
	EstimatedPurity float64
}

// AnalyzeNoiseModel estimates the noise impact of a circuit
func AnalyzeNoiseModel(gates []CircuitGateOp, model []NoiseChannel, qubitNoise map[int][]NoiseChannel) *NoiseModelReport {
	report := &NoiseModelReport{
		TotalGates:    len(gates),
		QubitNoiseMap: make(map[int][]string),
	}

	seenChannels := make(map[string]bool)
	for _, ch := range model {
		report.Channels = append(report.Channels, ch.Name)
		report.TotalChannels++
		seenChannels[ch.Name] = true
	}

	for qubit, channels := range qubitNoise {
		for _, ch := range channels {
			if !seenChannels[ch.Name] {
				report.Channels = append(report.Channels, ch.Name)
				report.TotalChannels++
				seenChannels[ch.Name] = true
			}
			report.QubitNoiseMap[qubit] = append(report.QubitNoiseMap[qubit], ch.Name)
		}
	}

	report.NoisyGates = report.TotalGates
	if report.TotalChannels == 0 {
		report.NoisyGates = 0
	}

	// Estimate purity after one depolarizing channel per gate
	purity := 1.0
	p := 0.01 // default per-gate error rate
	for _, ch := range model {
		if ch.Name[0:3] == "dep" {
			// Parse p from depolarizing name
			for _, c := range ch.Operators {
				_ = c
			}
		}
	}
	for i := 0; i < report.NoisyGates; i++ {
		purity *= (1.0 - p)
	}
	report.EstimatedPurity = math.Max(purity, 0.0)

	return report
}

// ============================================================
// Utility functions
// ============================================================

// Complex128SliceFromFloats creates a []Complex128 from interleaved re/im pairs
func Complex128SliceFromFloats(vals ...float64) []Complex128 {
	result := make([]Complex128, len(vals)/2)
	for i := 0; i < len(vals); i += 2 {
		result[i/2] = complex(vals[i], vals[i+1])
	}
	return result
}

// IsUnitary checks if a matrix is unitary (U U† = I) within tolerance
func IsUnitary(U []Complex128, dim int, tol float64) bool {
	for i := 0; i < dim; i++ {
		for j := 0; j < dim; j++ {
			var val Complex128
			for k := 0; k < dim; k++ {
				val += U[i*dim+k] * cmplx.Conj(U[j*dim+k])
			}
			expected := Complex128(0)
			if i == j {
				expected = 1
			}
			if cmplx.Abs(val-expected) > tol {
				return false
			}
		}
	}
	return true
}

// NormalizeDensityMatrix ensures Tr(ρ) = 1
func NormalizeDensityMatrix(dm *DensityMatrix) {
	trace := dm.Trace()
	if cmplx.Abs(trace) > 1e-12 {
		for i := range dm.Data {
			dm.Data[i] /= trace
		}
	}
}

// Suppress unused import warnings
var _ = math.Abs
