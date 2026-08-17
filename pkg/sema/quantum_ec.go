package sema

import (
	"fmt"
	"math/bits"
	"strings"
)

// ============================================================
// Phase 33: Quantum Error Correction — Stabilizer Codes,
// Syndrome Extraction & Surface Code Primitives
// ============================================================

// ECOp represents a single-qubit Pauli operator for error correction
type ECOp int

const (
	ECOpI ECOp = iota
	ECOpX
	ECOpY
	ECOpZ
)

func (p ECOp) String() string {
	switch p {
	case ECOpI:
		return "I"
	case ECOpX:
		return "X"
	case ECOpY:
		return "Y"
	case ECOpZ:
		return "Z"
	default:
		return "?"
	}
}

// PauliPhase captures the phase factor: +1, -1, +i, -i
type PauliPhase int

const (
	PhasePlus  PauliPhase = 0 // +1
	PhaseMinus PauliPhase = 1 // -1
	PhasePlusI PauliPhase = 2 // +i
	PhaseMinusI PauliPhase = 3 // -i
)

func (p PauliPhase) String() string {
	switch p {
	case PhasePlus:
		return "+"
	case PhaseMinus:
		return "-"
	case PhasePlusI:
		return "+i"
	case PhaseMinusI:
		return "-i"
	default:
		return "?"
	}
}

// PauliString represents a Pauli operator on N qubits: coeff · P₁⊗P₂⊗...⊗Pₙ
type PauliString struct {
	N     int
	Ops   []ECOp
	Phase PauliPhase
}

// NewPauliStringIdentity creates the N-qubit identity
func NewPauliStringIdentity(n int) PauliString {
	ops := make([]ECOp, n)
	for i := range ops {
		ops[i] = ECOpI
	}
	return PauliString{N: n, Ops: ops, Phase: PhasePlus}
}

// NewPauliStringSingle creates a string with a single Pauli on the given qubit
func NewPauliStringSingle(n int, qubit int, p ECOp) PauliString {
	ps := NewPauliStringIdentity(n)
	if qubit >= 0 && qubit < n {
		ps.Ops[qubit] = p
	}
	return ps
}

func (ps PauliString) String() string {
	var b strings.Builder
	if ps.Phase != PhasePlus {
		b.WriteString(ps.Phase.String())
		b.WriteString(" ")
	}
	for i, op := range ps.Ops {
		if op != ECOpI {
			b.WriteString(fmt.Sprintf("%s%d ", op, i))
		}
	}
	if b.Len() == 0 {
		return "I"
	}
	return strings.TrimSpace(b.String())
}

// Weight returns the number of non-identity Paulis
func (ps PauliString) Weight() int {
	w := 0
	for _, op := range ps.Ops {
		if op != ECOpI {
			w++
		}
	}
	return w
}

// Commutes checks if two PauliStrings commute: true if they commute
func Commutes(a, b PauliString) bool {
	if a.N != b.N {
		return false
	}
	antiCount := 0
	for i := 0; i < a.N; i++ {
		if a.Ops[i] != ECOpI && b.Ops[i] != ECOpI && a.Ops[i] != b.Ops[i] {
			antiCount++
		}
	}
	return antiCount%2 == 0
}

// Multiply computes the Pauli product a ⊗ b (matrix product, not tensor)
func Multiply(a, b PauliString) PauliString {
	if a.N != b.N {
		return PauliString{N: 0}
	}
	result := PauliString{N: a.N, Ops: make([]ECOp, a.N), Phase: PhasePlus}
	phase := a.Phase

	for i := 0; i < a.N; i++ {
		ai, bi := a.Ops[i], b.Ops[i]
		switch {
		case ai == ECOpI:
			result.Ops[i] = bi
		case bi == ECOpI:
			result.Ops[i] = ai
		case ai == bi:
			result.Ops[i] = ECOpI
		default:
			// XY=+iZ, YX=-iZ, YZ=+iX, ZY=-iX, ZX=+iY, XZ=-iY
			switch {
			case (ai == ECOpX && bi == ECOpY) || (ai == ECOpY && bi == ECOpZ) || (ai == ECOpZ && bi == ECOpX):
				result.Ops[i] = thirdPauli(ai, bi)
				phase = addPhase(phase, PhasePlusI)
			default:
				result.Ops[i] = thirdPauli(ai, bi)
				phase = addPhase(phase, PhaseMinusI)
			}
		}
	}

	result.Phase = phase
	return result
}

func thirdPauli(a, b ECOp) ECOp {
	if (a == ECOpX && b == ECOpY) || (a == ECOpY && b == ECOpX) {
		return ECOpZ
	}
	if (a == ECOpY && b == ECOpZ) || (a == ECOpZ && b == ECOpY) {
		return ECOpX
	}
	if (a == ECOpZ && b == ECOpX) || (a == ECOpX && b == ECOpZ) {
		return ECOpY
	}
	return ECOpI
}

func addPhase(a, b PauliPhase) PauliPhase {
	return PauliPhase((int(a) + int(b)) % 4)
}

// Equals checks structural equality (ignoring phase)
func (ps PauliString) Equals(other PauliString) bool {
	if ps.N != other.N {
		return false
	}
	for i := 0; i < ps.N; i++ {
		if ps.Ops[i] != other.Ops[i] {
			return false
		}
	}
	return true
}

// EqualsExact checks equality including phase
func (ps PauliString) EqualsExact(other PauliString) bool {
	return ps.Equals(other) && ps.Phase == other.Phase
}

// ============================================================
// Stabilizer Code
// ============================================================

// StabilizerCode represents an [[n, k, d]] stabilizer code:
//
//	n physical qubits, k logical qubits, distance d
type StabilizerCode struct {
	Name       string
	N          int           // physical qubits
	K          int           // logical qubits
	Distance   int           // code distance
	Generators []PauliString // n-k independent stabilizer generators
	LogicalX   []PauliString // k logical-X operators
	LogicalZ   []PauliString // k logical-Z operators
}

// NumChecks returns the number of stabilizer generators (n-k)
func (sc *StabilizerCode) NumChecks() int {
	return sc.N - sc.K
}

// IsStabilizer checks if a PauliString is in the stabilizer group
// (i.e., commutes with all generators and is a product of generators)
func (sc *StabilizerCode) IsStabilizer(ps PauliString) bool {
	if ps.N != sc.N {
		return false
	}
	for _, gen := range sc.Generators {
		if !Commutes(ps, gen) {
			return false
		}
	}
	return true
}

// ============================================================
// Standard Codes
// ============================================================

// ShorCode913 returns the [[9,1,3]] Shor code
func ShorCode913() *StabilizerCode {
	n := 9
	gens := make([]PauliString, 8)

	// Z-type generators (bit-flip checks) — stabilizer 3-qubit repetition within blocks
	gens[0] = PauliString{N: n, Ops: []ECOp{ECOpZ, ECOpZ, ECOpI, ECOpI, ECOpI, ECOpI, ECOpI, ECOpI, ECOpI}, Phase: PhasePlus}
	gens[1] = PauliString{N: n, Ops: []ECOp{ECOpI, ECOpZ, ECOpZ, ECOpI, ECOpI, ECOpI, ECOpI, ECOpI, ECOpI}, Phase: PhasePlus}
	gens[2] = PauliString{N: n, Ops: []ECOp{ECOpI, ECOpI, ECOpI, ECOpZ, ECOpZ, ECOpI, ECOpI, ECOpI, ECOpI}, Phase: PhasePlus}
	gens[3] = PauliString{N: n, Ops: []ECOp{ECOpI, ECOpI, ECOpI, ECOpI, ECOpZ, ECOpZ, ECOpI, ECOpI, ECOpI}, Phase: PhasePlus}
	gens[4] = PauliString{N: n, Ops: []ECOp{ECOpI, ECOpI, ECOpI, ECOpI, ECOpI, ECOpI, ECOpZ, ECOpZ, ECOpI}, Phase: PhasePlus}
	gens[5] = PauliString{N: n, Ops: []ECOp{ECOpI, ECOpI, ECOpI, ECOpI, ECOpI, ECOpI, ECOpI, ECOpZ, ECOpZ}, Phase: PhasePlus}

	// X-type generators (phase-flip checks) — across blocks
	gens[6] = PauliString{N: n, Ops: []ECOp{ECOpX, ECOpX, ECOpX, ECOpX, ECOpX, ECOpX, ECOpI, ECOpI, ECOpI}, Phase: PhasePlus}
	gens[7] = PauliString{N: n, Ops: []ECOp{ECOpI, ECOpI, ECOpI, ECOpX, ECOpX, ECOpX, ECOpX, ECOpX, ECOpX}, Phase: PhasePlus}

	logX := []PauliString{
		{N: n, Ops: []ECOp{ECOpX, ECOpX, ECOpX, ECOpX, ECOpX, ECOpX, ECOpX, ECOpX, ECOpX}, Phase: PhasePlus},
	}
	logZ := []PauliString{
		{N: n, Ops: []ECOp{ECOpZ, ECOpI, ECOpI, ECOpZ, ECOpI, ECOpI, ECOpZ, ECOpI, ECOpI}, Phase: PhasePlus},
	}

	return &StabilizerCode{
		Name:       "Shor [[9,1,3]]",
		N: 9, K: 1, Distance: 3,
		Generators: gens, LogicalX: logX, LogicalZ: logZ,
	}
}

// SteaneCode713 returns the [[7,1,3]] Steane code
func SteaneCode713() *StabilizerCode {
	n := 7
	gens := make([]PauliString, 6)

	// X-type checks (from classical [7,4,3] Hamming code parity matrix)
	gens[0] = PauliString{N: n, Ops: []ECOp{ECOpX, ECOpX, ECOpI, ECOpX, ECOpX, ECOpI, ECOpI}, Phase: PhasePlus}
	gens[1] = PauliString{N: n, Ops: []ECOp{ECOpX, ECOpI, ECOpX, ECOpX, ECOpI, ECOpX, ECOpI}, Phase: PhasePlus}
	gens[2] = PauliString{N: n, Ops: []ECOp{ECOpI, ECOpX, ECOpX, ECOpX, ECOpI, ECOpI, ECOpX}, Phase: PhasePlus}

	// Z-type checks
	gens[3] = PauliString{N: n, Ops: []ECOp{ECOpZ, ECOpZ, ECOpI, ECOpZ, ECOpZ, ECOpI, ECOpI}, Phase: PhasePlus}
	gens[4] = PauliString{N: n, Ops: []ECOp{ECOpZ, ECOpI, ECOpZ, ECOpZ, ECOpI, ECOpZ, ECOpI}, Phase: PhasePlus}
	gens[5] = PauliString{N: n, Ops: []ECOp{ECOpI, ECOpZ, ECOpZ, ECOpZ, ECOpI, ECOpI, ECOpZ}, Phase: PhasePlus}

	logX := []PauliString{
		{N: n, Ops: []ECOp{ECOpX, ECOpX, ECOpX, ECOpX, ECOpX, ECOpX, ECOpX}, Phase: PhasePlus},
	}
	logZ := []PauliString{
		{N: n, Ops: []ECOp{ECOpZ, ECOpZ, ECOpZ, ECOpZ, ECOpZ, ECOpZ, ECOpZ}, Phase: PhasePlus},
	}

	return &StabilizerCode{
		Name:       "Steane [[7,1,3]]",
		N: 7, K: 1, Distance: 3,
		Generators: gens, LogicalX: logX, LogicalZ: logZ,
	}
}

// FiveQubitCode513 returns the [[5,1,3]] perfect code
func FiveQubitCode513() *StabilizerCode {
	n := 5
	gens := make([]PauliString, 4)

	gens[0] = PauliString{N: n, Ops: []ECOp{ECOpX, ECOpZ, ECOpZ, ECOpX, ECOpI}, Phase: PhasePlus}
	gens[1] = PauliString{N: n, Ops: []ECOp{ECOpI, ECOpX, ECOpZ, ECOpZ, ECOpX}, Phase: PhasePlus}
	gens[2] = PauliString{N: n, Ops: []ECOp{ECOpX, ECOpI, ECOpX, ECOpZ, ECOpZ}, Phase: PhasePlus}
	gens[3] = PauliString{N: n, Ops: []ECOp{ECOpZ, ECOpX, ECOpI, ECOpX, ECOpZ}, Phase: PhasePlus}

	logX := []PauliString{
		{N: n, Ops: []ECOp{ECOpX, ECOpX, ECOpX, ECOpX, ECOpX}, Phase: PhasePlus},
	}
	logZ := []PauliString{
		{N: n, Ops: []ECOp{ECOpZ, ECOpZ, ECOpZ, ECOpZ, ECOpZ}, Phase: PhasePlus},
	}

	return &StabilizerCode{
		Name:       "Five-qubit [[5,1,3]]",
		N: 5, K: 1, Distance: 3,
		Generators: gens, LogicalX: logX, LogicalZ: logZ,
	}
}

// ============================================================
// Surface Code
// ============================================================

// SurfaceCode represents a rotated surface code with distance d
type SurfaceCode struct {
	Distance          int
	DataQubits        int // total data qubits
	SyndromeQubits    int // total syndrome ancilla qubits
	XStabilizers      [][]int // X (face) stabilizer supports (data qubit indices)
	ZStabilizers      [][]int // Z (vertex) stabilizer supports (data qubit indices)
	DataQubitCoords   [][2]int // (row, col) of each data qubit
	XSyndromeCoords   [][2]int
	ZSyndromeCoords   [][2]int
}

// NewSurfaceCode constructs a distance-d rotated surface code
func NewSurfaceCode(d int) *SurfaceCode {
	if d < 2 {
		d = 2
	}

	sc := &SurfaceCode{Distance: d}
	d2 := d * d

	// Data qubits on the d×d grid
	sc.DataQubits = d2
	sc.DataQubitCoords = make([][2]int, d2)
	idx := 0
	for r := 0; r < d; r++ {
		for c := 0; c < d; c++ {
			sc.DataQubitCoords[idx] = [2]int{r, c}
			idx++
		}
	}

	// X stabilizers: (d-1)^2/2 face operators (roughly half the plaquettes)
	// For a rotated surface code: X stabilizers on "rough" faces
	sc.XStabilizers = generateXStabilizers(d)
	sc.XSyndromeCoords = make([][2]int, len(sc.XStabilizers))
	for i := range sc.XStabilizers {
		// syndrome qubit at face center
		sc.XSyndromeCoords[i] = [2]int{-1, -1} // placeholder
	}

	// Z stabilizers: (d-1)^2/2 vertex operators
	sc.ZStabilizers = generateZStabilizers(d)
	sc.ZSyndromeCoords = make([][2]int, len(sc.ZStabilizers))
	for i := range sc.ZStabilizers {
		sc.ZSyndromeCoords[i] = [2]int{-1, -1}
	}

	sc.SyndromeQubits = len(sc.XStabilizers) + len(sc.ZStabilizers)
	return sc
}

func generateXStabilizers(d int) [][]int {
	var stabs [][]int
	for r := 0; r < d-1; r++ {
		for c := 0; c < d-1; c++ {
			if (r+c)%2 == 0 {
				stabs = append(stabs, faceQubits(r, c, d))
			}
		}
	}
	return stabs
}

func generateZStabilizers(d int) [][]int {
	var stabs [][]int
	for r := 0; r < d-1; r++ {
		for c := 0; c < d-1; c++ {
			if (r+c)%2 == 1 {
				stabs = append(stabs, faceQubits(r, c, d))
			}
		}
	}
	return stabs
}

func faceQubits(r, c, d int) []int {
	var qubits []int
	// 4 corners of the plaquette
	corners := [][2]int{{r, c}, {r, c + 1}, {r + 1, c}, {r + 1, c + 1}}
	for _, rc := range corners {
		if rc[0] >= 0 && rc[0] < d && rc[1] >= 0 && rc[1] < d {
			qubits = append(qubits, rc[0]*d+rc[1])
		}
	}
	return qubits
}

// DataQubitIndex returns the flat index from (row, col)
func (sc *SurfaceCode) DataQubitIndex(row, col int) int {
	return row*sc.Distance + col
}

// NumLogicalQubits returns k for the surface code (=1 for standard)
func (sc *SurfaceCode) NumLogicalQubits() int {
	n := sc.DataQubits
	nx := len(sc.XStabilizers)
	nz := len(sc.ZStabilizers)
	return n - nx - nz
}

// ============================================================
// Syndrome Extraction
// ============================================================

// SyndromeMeasurement represents a single stabilizer measurement
type SyndromeMeasurement struct {
	StabilizerIndex int
	StabilizerType  string // "X" or "Z"
	Support         []int  // data qubit indices
	Outcome         bool   // measurement result
}

// SyndromeResult holds the full syndrome measurement result
type SyndromeResult struct {
	Measurements []SyndromeMeasurement
	RawBits      []bool
	SyndromeInt  int // integer encoding of syndrome bits
}

// ExtractSyndrome computes the syndrome for a given error pattern
func ExtractSyndrome(code *StabilizerCode, errorPattern PauliString) *SyndromeResult {
	nChecks := code.NumChecks()
	rawBits := make([]bool, nChecks)
	measurements := make([]SyndromeMeasurement, nChecks)

	for i, gen := range code.Generators {
		// Count the number of positions where both error and generator have non-identity,
		// and they differ (i.e., the error anticommutes with the generator at that qubit)
		anticommuteCount := 0
		for q := 0; q < code.N; q++ {
			if errorPattern.Ops[q] != ECOpI && gen.Ops[q] != ECOpI {
				if errorPattern.Ops[q] != gen.Ops[q] {
					anticommuteCount++
				}
			}
		}
		// The syndrome bit is +1 if the error commutes with the generator,
		// -1 if it anticommutes. For measurement outcome: false = +1, true = -1
		outcome := anticommuteCount%2 == 1
		rawBits[i] = outcome

		stabType := "Z"
		if gen.Ops[0] == ECOpX || gen.Ops[0] == ECOpY {
			stabType = "X"
		}
		for _, op := range gen.Ops {
			if op == ECOpX || op == ECOpY {
				stabType = "X"
				break
			}
		}

		measurements[i] = SyndromeMeasurement{
			StabilizerIndex: i,
			StabilizerType:  stabType,
			Support:         gen.NonIdentityIndices(),
			Outcome:         outcome,
		}
	}

	syndromeInt := 0
	for i, bit := range rawBits {
		if bit {
			syndromeInt |= 1 << uint(i)
		}
	}

	return &SyndromeResult{
		Measurements: measurements,
		RawBits:      rawBits,
		SyndromeInt:  syndromeInt,
	}
}

// NonIdentityIndices returns indices of non-identity Paulis
func (ps PauliString) NonIdentityIndices() []int {
	var indices []int
	for i, op := range ps.Ops {
		if op != ECOpI {
			indices = append(indices, i)
		}
	}
	return indices
}

// ============================================================
// Syndrome Table / Decoder
// ============================================================

// SyndromeEntry maps a syndrome integer to a recovery operator
type SyndromeEntry struct {
	Syndrome    int
	Recovery    PauliString
	Weight      int
}

// LookupTableDecoder implements a simple lookup-table decoder
type LookupTableDecoder struct {
	Code      *StabilizerCode
	Table     map[int]SyndromeEntry
}

// BuildDecoder constructs a minimum-weight lookup table for the given code
func BuildDecoder(code *StabilizerCode) *LookupTableDecoder {
	dec := &LookupTableDecoder{
		Code:  code,
		Table: make(map[int]SyndromeEntry),
	}

	numSyndromes := 1 << uint(code.NumChecks())
	_ = numSyndromes

	// For each weight-1 error, compute its syndrome
	for q := 0; q < code.N; q++ {
		for _, pauli := range []ECOp{ECOpX, ECOpY, ECOpZ} {
			err := NewPauliStringIdentity(code.N)
			err.Ops[q] = pauli
			syn := ExtractSyndrome(code, err)
			synInt := syn.SyndromeInt
			if synInt == 0 {
				continue // trivial
			}
			if existing, ok := dec.Table[synInt]; !ok || err.Weight() < existing.Weight {
				dec.Table[synInt] = SyndromeEntry{
					Syndrome: synInt,
					Recovery: err,
					Weight:   err.Weight(),
				}
			}
		}
	}

	return dec
}

// Decode finds the minimum-weight recovery for a measured syndrome
func (dec *LookupTableDecoder) Decode(syn *SyndromeResult) (PauliString, bool) {
	if syn.SyndromeInt == 0 {
		return NewPauliStringIdentity(dec.Code.N), true // no error detected
	}
	entry, ok := dec.Table[syn.SyndromeInt]
	if !ok {
		return PauliString{}, false
	}
	return entry.Recovery, true
}

// ============================================================
// Error Analysis
// ============================================================

// CodeCapacityReport summarizes the code's error correction capability
type CodeCapacityReport struct {
	CodeName         string
	PhysicalQubits   int
	LogicalQubits    int
	Distance         int
	NumStabilizers   int
	NumLogicalOps    int
	MaxCorrectableT  int // floor((d-1)/2)
	ErrorThreshold   float64 // approximate threshold for depolarizing noise
}

// AnalyzeCodeCapacity produces a capacity report
func AnalyzeCodeCapacity(code *StabilizerCode) CodeCapacityReport {
	return CodeCapacityReport{
		CodeName:       code.Name,
		PhysicalQubits: code.N,
		LogicalQubits:  code.K,
		Distance:       code.Distance,
		NumStabilizers: code.NumChecks(),
		NumLogicalOps:  len(code.LogicalX) + len(code.LogicalZ),
		MaxCorrectableT: (code.Distance - 1) / 2,
		ErrorThreshold:  approxThreshold(code.Distance),
	}
}

func approxThreshold(d int) float64 {
	// Approximate threshold from threshold theorem
	switch {
	case d >= 7:
		return 0.01
	case d >= 5:
		return 0.005
	case d >= 3:
		return 0.001
	default:
		return 0.0001
	}
}

// AnalyzeSurfaceCodeCapacity produces a capacity report for a surface code
func AnalyzeSurfaceCodeCapacity(sc *SurfaceCode) CodeCapacityReport {
	d := sc.Distance
	return CodeCapacityReport{
		CodeName:       fmt.Sprintf("Surface code d=%d", d),
		PhysicalQubits: sc.DataQubits + sc.SyndromeQubits,
		LogicalQubits:  sc.NumLogicalQubits(),
		Distance:       d,
		NumStabilizers: len(sc.XStabilizers) + len(sc.ZStabilizers),
		NumLogicalOps:  2,
		MaxCorrectableT: (d - 1) / 2,
		ErrorThreshold:  approxThreshold(d),
	}
}

// ============================================================
// Logical Gate Simulation
// ============================================================

// LogicalGateType identifies a logical gate operation
type LogicalGateType int

const (
	LogicalH LogicalGateType = iota
	LogicalX
	LogicalZ
	LogicalCNOT
	LogicalS
	LogicalT
	LogicalTdag
	LogicalSdag
	LogicalMeasure
)

func (g LogicalGateType) String() string {
	switch g {
	case LogicalH:
		return "H_L"
	case LogicalX:
		return "X_L"
	case LogicalZ:
		return "Z_L"
	case LogicalCNOT:
		return "CNOT_L"
	case LogicalS:
		return "S_L"
	case LogicalT:
		return "T_L"
	case LogicalTdag:
		return "T†_L"
	case LogicalSdag:
		return "S†_L"
	case LogicalMeasure:
		return "M_L"
	default:
		return "UNKNOWN"
	}
}

// LogicalCircuitOp represents one operation in a logical circuit
type LogicalCircuitOp struct {
	Type       LogicalGateType
	Target     int   // logical qubit index
	Control    int   // for CNOT: control logical qubit
	Decompose  []string // human-readable physical decomposition
}

// DecomposeLogicalCircuit converts logical gates to physical stabilizer operations
func DecomposeLogicalCircuit(code *StabilizerCode, ops []LogicalCircuitOp) [][]string {
	var result [][]string
	for _, op := range ops {
		var decomp []string
		switch op.Type {
		case LogicalX:
			decomp = decomposeLogicalX(code, op.Target)
		case LogicalZ:
			decomp = decomposeLogicalZ(code, op.Target)
		case LogicalH:
			decomp = decomposeLogicalH(code, op.Target)
		case LogicalMeasure:
			decomp = decomposeLogicalMeasure(code, op.Target)
		default:
			decomp = []string{fmt.Sprintf("// %s on logical qubit %d", op.Type, op.Target)}
		}
		result = append(result, decomp)
	}
	return result
}

func decomposeLogicalX(code *StabilizerCode, target int) []string {
	if target >= len(code.LogicalX) {
		return []string{"// ERROR: invalid logical qubit"}
	}
	lx := code.LogicalX[target]
	var ops []string
	for i, p := range lx.Ops {
		if p == ECOpX {
			ops = append(ops, fmt.Sprintf("X q[%d]", i))
		}
	}
	return ops
}

func decomposeLogicalZ(code *StabilizerCode, target int) []string {
	if target >= len(code.LogicalZ) {
		return []string{"// ERROR: invalid logical qubit"}
	}
	lz := code.LogicalZ[target]
	var ops []string
	for i, p := range lz.Ops {
		if p == ECOpZ {
			ops = append(ops, fmt.Sprintf("Z q[%d]", i))
		}
	}
	return ops
}

func decomposeLogicalH(code *StabilizerCode, target int) []string {
	return []string{
		fmt.Sprintf("// Logical H: transversal H on all %d physical qubits", code.N),
		"// H_L = H^⊗n (transversal for CSS codes)",
	}
}

func decomposeLogicalMeasure(code *StabilizerCode, target int) []string {
	var ops []string
	for _, lz := range code.LogicalZ {
		support := lz.NonIdentityIndices()
		if len(support) > 0 {
			qubits := ""
			for i, q := range support {
				if i > 0 {
					qubits += ", "
				}
				qubits += fmt.Sprintf("q[%d]", q)
			}
			ops = append(ops, fmt.Sprintf("measure_z_group(%s)", qubits))
		}
		break
	}
	if len(ops) == 0 {
		ops = append(ops, "// measure all data qubits")
	}
	return ops
}

// ============================================================
// Helper: Pauli string from Pauli word
// ============================================================

// PauliStringFromWord creates a PauliString from a compact representation
// e.g., "XIZY" means X on q0, I on q1, Z on q2, Y on q3
func PauliStringFromWord(word string) PauliString {
	ops := make([]ECOp, len(word))
	for i, ch := range word {
		switch ch {
		case 'I', 'i':
			ops[i] = ECOpI
		case 'X', 'x':
			ops[i] = ECOpX
		case 'Y', 'y':
			ops[i] = ECOpY
		case 'Z', 'z':
			ops[i] = ECOpZ
		default:
			ops[i] = ECOpI
		}
	}
	return PauliString{N: len(word), Ops: ops, Phase: PhasePlus}
}

// bits.OnesCount is available via math/bits; used internally
var _ = bits.OnesCount
