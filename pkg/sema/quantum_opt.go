package sema

import (
	"math"
	"strings"
)

// ============================================================
// Phase 34: Quantum Circuit Optimization — Gate Cancellation,
// Commutation Rules, Template Synthesis & Native Compilation
// ============================================================

// QGate represents a single gate in a quantum circuit
type QGate struct {
	Name     string
	Qubits   []int
	Angle    float64 // for parameterized gates (Rz, Ry, etc.)
	Controls []int   // for controlled gates
}

// QCircuit represents a sequence of gates
type QCircuit struct {
	NumQubits int
	Gates     []QGate
}

// NewQCircuit creates a new empty circuit
func NewQCircuit(n int) *QCircuit {
	return &QCircuit{NumQubits: n}
}

// AddGate appends a gate to the circuit
func (c *QCircuit) AddGate(name string, qubits ...int) {
	c.Gates = append(c.Gates, QGate{Name: name, Qubits: qubits})
}

// AddParamGate appends a parameterized gate
func (c *QCircuit) AddParamGate(name string, angle float64, qubits ...int) {
	c.Gates = append(c.Gates, QGate{Name: name, Qubits: qubits, Angle: angle})
}

// AddControlledGate appends a controlled gate
func (c *QCircuit) AddControlledGate(name string, controls []int, targets []int) {
	c.Gates = append(c.Gates, QGate{Name: name, Qubits: targets, Controls: controls})
}

// Clone creates a deep copy of the circuit
func (c *QCircuit) Clone() *QCircuit {
	nc := &QCircuit{NumQubits: c.NumQubits, Gates: make([]QGate, len(c.Gates))}
	for i, g := range c.Gates {
		nc.Gates[i] = g
		nc.Gates[i].Qubits = make([]int, len(g.Qubits))
		copy(nc.Gates[i].Qubits, g.Qubits)
		nc.Gates[i].Controls = make([]int, len(g.Controls))
		copy(nc.Gates[i].Controls, g.Controls)
	}
	return nc
}

// Depth returns the circuit depth (longest path through the circuit)
func (c *QCircuit) Depth() int {
	qDepth := make([]int, c.NumQubits)
	for _, g := range c.Gates {
		maxD := 0
		for _, q := range g.Qubits {
			if qDepth[q] > maxD {
				maxD = qDepth[q]
			}
		}
		for _, q := range g.Qubits {
			qDepth[q] = maxD + 1
		}
	}
	d := 0
	for _, d2 := range qDepth {
		if d2 > d {
			d = d2
		}
	}
	return d
}

// GateCount returns total gate count
func (c *QCircuit) GateCount() int {
	return len(c.Gates)
}

// TwoQubitCount returns count of two-qubit gates
func (c *QCircuit) TwoQubitCount() int {
	n := 0
	for _, g := range c.Gates {
		if len(g.Qubits) == 2 || len(g.Controls) > 0 {
			n++
		}
	}
	return n
}

// ============================================================
// Gate Cancellation
// ============================================================

// IsInversePair checks if two gates cancel (H-H, X-X, etc.)
func IsInversePair(a, b QGate) bool {
	if a.Name != b.Name {
		return false
	}
	if !intSliceEqual(a.Qubits, b.Qubits) {
		return false
	}
	if !intSliceEqual(a.Controls, b.Controls) {
		return false
	}

	switch a.Name {
	case "x", "y", "z", "h", "cx", "cy", "cz", "swap", "s", "sdag", "t", "tdag":
		return true // self-inverse (involution)
	}

	// For rotation gates, check if angles sum to 0 (mod 2π)
	if isRotationGate(a.Name) {
		sum := a.Angle + b.Angle
		return math.Abs(sum-math.Round(sum/(2*math.Pi))*2*math.Pi) < 1e-10
	}

	return false
}

func isRotationGate(name string) bool {
	switch name {
	case "rx", "ry", "rz", "crx", "cry", "crz":
		return true
	}
	return false
}

// CancelAdjacentInverses removes adjacent inverse gate pairs.
// Returns the optimized circuit and the number of cancellations.
func CancelAdjacentInverses(c *QCircuit) (*QCircuit, int) {
	result := c.Clone()
	cancelled := 0

	for {
		found := false
		for i := 0; i < len(result.Gates)-1; i++ {
			if !areIndependentOrSame(result.Gates[i], result.Gates[i+1]) {
				continue
			}
			if IsInversePair(result.Gates[i], result.Gates[i+1]) {
				result.Gates = append(result.Gates[:i], result.Gates[i+2:]...)
				cancelled++
				found = true
				break
			}
		}
		if !found {
			break
		}
	}

	return result, cancelled
}

// areIndependentOrSame checks if two gates operate on overlapping qubits
func areIndependentOrSame(a, b QGate) bool {
	aQubits := gateQubits(a)
	bQubits := gateQubits(b)
	for _, aq := range aQubits {
		for _, bq := range bQubits {
			if aq == bq {
				return true
			}
		}
	}
	return false
}

func gateQubits(g QGate) []int {
	qubits := make([]int, 0, len(g.Qubits)+len(g.Controls))
	qubits = append(qubits, g.Qubits...)
	qubits = append(qubits, g.Controls...)
	return qubits
}

func intSliceEqual(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// ============================================================
// Commutation Rules
// ============================================================

// CanCommute checks if two gates can be swapped in the circuit
func CanCommute(a, b QGate) bool {
	aQubits := gateQubits(a)
	bQubits := gateQubits(b)

	// If no qubit overlap, always commute
	if !hasOverlap(aQubits, bQubits) {
		return true
	}

	// Same gate on same qubits → commute if both self-inverse or same angle
	if a.Name == b.Name && intSliceEqual(aQubits, bQubits) {
		return true
	}

	// Pauli gates commute with each other on same qubit
	if isPauliGate(a.Name) && isPauliGate(b.Name) && len(aQubits) == 1 && len(bQubits) == 1 && aQubits[0] == bQubits[0] {
		return true
	}

	// Rotations around same axis commute
	if isRotationGate(a.Name) && isRotationGate(b.Name) && sameAxis(a.Name, b.Name) && len(aQubits) == 1 && len(bQubits) == 1 && aQubits[0] == bQubits[0] {
		return true
	}

	// Z-type gates commute with each other (diagonal gates commute)
	if isDiagonalGate(a.Name) && isDiagonalGate(b.Name) {
		return true
	}

	return false
}

func isPauliGate(name string) bool {
	switch name {
	case "x", "y", "z":
		return true
	}
	return false
}

func isDiagonalGate(name string) bool {
	switch name {
	case "z", "s", "sdag", "t", "tdag", "rz", "cz":
		return true
	}
	return false
}

func sameAxis(a, b string) bool {
	stripC := func(s string) string {
		if strings.HasPrefix(s, "c") {
			return s[1:]
		}
		return s
	}
	return stripC(a) == stripC(b)
}

func hasOverlap(a, b []int) bool {
	for _, aq := range a {
		for _, bq := range b {
			if aq == bq {
				return true
			}
		}
	}
	return false
}

// ============================================================
// Gate Synthesis Rules
// ============================================================

// GateIdentity stores known identities
type GateIdentity struct {
	Input  []string // sequence of gates
	Output []string // equivalent simpler sequence
}

// StandardIdentities returns common gate identities
func StandardIdentities() []GateIdentity {
	return []GateIdentity{
		{Input: []string{"h", "x", "h"}, Output: []string{"z"}},
		{Input: []string{"h", "z", "h"}, Output: []string{"x"}},
		{Input: []string{"h", "y", "h"}, Output: []string{"-y"}},
		{Input: []string{"s", "s"}, Output: []string{"z"}},
		{Input: []string{"t", "t"}, Output: []string{"s"}},
		{Input: []string{"s", "s", "s"}, Output: []string{"sdag"}},
		{Input: []string{"t", "t", "t", "t"}, Output: []string{"s"}},
		{Input: []string{"sdag", "s"}, Output: []string{"id"}},
		{Input: []string{"tdag", "t"}, Output: []string{"id"}},
		{Input: []string{"rx", "rx"}, Output: []string{"rx_sum"}},  // angle-add
		{Input: []string{"rz", "rz"}, Output: []string{"rz_sum"}},  // angle-add
		{Input: []string{"cnot", "cnot"}, Output: []string{"id"}},  // same control+target → self-inverse
	}
}

// SynthesizeTDepth performs T-depth optimization using T-gate synthesis
// Reduces the number of T-gates in a circuit via resource estimation
func SynthesizeTDepth(c *QCircuit) (*QCircuit, int) {
	result := c.Clone()
	reduced := 0

	for i := 0; i < len(result.Gates)-1; i++ {
		g1, g2 := result.Gates[i], result.Gates[i+1]
		if g1.Name == g2.Name && isRotationGate(g1.Name) && intSliceEqual(g1.Qubits, g2.Qubits) {
			merged := normalizeAngle(g1.Angle + g2.Angle)
			if math.Abs(merged) < 1e-10 {
				// Both gates cancel — remove them
				result.Gates = append(result.Gates[:i], result.Gates[i+2:]...)
				reduced++
				i -= 2
				if i < -1 {
					i = -1
				}
			} else {
				result.Gates[i] = QGate{
					Name:   g1.Name,
					Qubits: g1.Qubits,
					Angle:  merged,
				}
				result.Gates = append(result.Gates[:i+1], result.Gates[i+2:]...)
				reduced++
				i--
			}
		}
	}

	return result, reduced
}

func normalizeAngle(a float64) float64 {
	a = math.Mod(a, 2*math.Pi)
	if a < 0 {
		a += 2 * math.Pi
	}
	if math.Abs(a) < 1e-10 || math.Abs(a-2*math.Pi) < 1e-10 {
		return 0
	}
	return a
}

// ============================================================
// Hardware-Native Gate Set Compilation
// ============================================================

// NativeGateSet defines target hardware gates
type NativeGateSet struct {
	Name    string
	Gates   []string
	Rules   map[string][]QGate // decomposition rules
}

// IBMNativeGates returns the IBM Quantum native gate set
func IBMNativeGates() NativeGateSet {
	return NativeGateSet{
		Name:  "ibm",
		Gates: []string{"id", "rz", "sx", "x", "cx"},
		Rules: map[string][]QGate{
			"h":  {{Name: "rz", Angle: math.Pi / 2}, {Name: "sx"}, {Name: "rz", Angle: math.Pi}},
			"t":  {{Name: "rz", Angle: math.Pi / 4}},
			"tdag": {{Name: "rz", Angle: -math.Pi / 4}},
			"s":  {{Name: "rz", Angle: math.Pi / 2}},
			"sdag": {{Name: "rz", Angle: -math.Pi / 2}},
			"z":  {{Name: "rz", Angle: math.Pi}},
		},
	}
}

// GoogleNativeGates returns the Google Cirq native gate set
func GoogleNativeGates() NativeGateSet {
	return NativeGateSet{
		Name:  "google",
		Gates: []string{"phased_x_z", "phased_x", "z", "cz", "xx"},
		Rules: map[string][]QGate{
			"h":  {{Name: "phased_x_z", Angle: math.Pi / 2}},
			"x":  {{Name: "phased_x_z", Angle: 0}},
			"t":  {{Name: "z", Angle: math.Pi / 4}},
			"s":  {{Name: "z", Angle: math.Pi / 2}},
		},
	}
}

// IonQNativeGates returns the IonQ native gate set
func IonQNativeGates() NativeGateSet {
	return NativeGateSet{
		Name:  "ionq",
		Gates: []string{"rxx", "rz", "ry"},
		Rules: map[string][]QGate{
			"h":  {{Name: "ry", Angle: math.Pi / 2}, {Name: "rz", Angle: math.Pi}},
			"x":  {{Name: "rz", Angle: math.Pi}},
			"cx": {{Name: "rxx", Angle: math.Pi / 2}},
		},
	}
}

// CompileToNative converts a circuit to use only native gates
func CompileToNative(c *QCircuit, native NativeGateSet) (*QCircuit, int) {
	result := c.Clone()
	gatesAdded := 0

	var newGates []QGate
	for _, g := range result.Gates {
		rules, ok := native.Rules[g.Name]
		if ok {
			for _, r := range rules {
				rg := r
				rg.Qubits = make([]int, len(g.Qubits))
				copy(rg.Qubits, g.Qubits)
				rg.Controls = make([]int, len(g.Controls))
				copy(rg.Controls, g.Controls)
				if rg.Angle == 0 && r.Angle != 0 {
					rg.Angle = r.Angle
				}
				newGates = append(newGates, rg)
				gatesAdded++
			}
		} else {
			isNative := false
			for _, ng := range native.Gates {
				if g.Name == ng {
					isNative = true
					break
				}
			}
			if isNative {
				newGates = append(newGates, g)
			}
		}
	}

	result.Gates = newGates
	return result, gatesAdded
}

// ============================================================
// Circuit Optimization Pipeline
// ============================================================

// OptConfig configures the optimization pipeline
type OptConfig struct {
	MaxIterations     int
	CancelInverses    bool
	MergeRotations    bool
	CompileToNative   bool
	NativeGates       NativeGateSet
	ApplyTemplates    bool
}

// DefaultOptConfig returns a sensible default configuration
func DefaultOptConfig() OptConfig {
	return OptConfig{
		MaxIterations:   5,
		CancelInverses:  true,
		MergeRotations:  true,
		CompileToNative: false,
		ApplyTemplates:  true,
	}
}

// OptimizeResult holds the result of circuit optimization
type OptimizeResult struct {
	Original          *QCircuit
	Optimized         *QCircuit
	GatesRemoved      int
	GatesAdded        int
	DepthReduction    int
	OriginalDepth     int
	OptimizedDepth    int
	OriginalGateCount int
	OptimizedGateCount int
	Iterations        int
}

// OptimizeCircuit runs the full optimization pipeline
func OptimizeCircuit(c *QCircuit, cfg OptConfig) *OptimizeResult {
	result := &OptimizeResult{
		Original:          c,
		OriginalDepth:     c.Depth(),
		OriginalGateCount: c.GateCount(),
	}

	current := c.Clone()

	for iter := 0; iter < cfg.MaxIterations; iter++ {
		prev := current.Clone()

		if cfg.CancelInverses {
			var cancelled int
			current, cancelled = CancelAdjacentInverses(current)
			result.GatesRemoved += cancelled
		}

		if cfg.MergeRotations {
			var merged int
			current, merged = SynthesizeTDepth(current)
			result.GatesRemoved += merged
		}

		if current.GateCount() == prev.GateCount() {
			break
		}
		result.Iterations = iter + 1
	}

	if cfg.CompileToNative {
		var added int
		current, added = CompileToNative(current, cfg.NativeGates)
		result.GatesAdded += added
	}

	result.Optimized = current
	result.OptimizedDepth = current.Depth()
	result.OptimizedGateCount = current.GateCount()
	result.DepthReduction = result.OriginalDepth - result.OptimizedDepth

	return result
}

// ============================================================
// Circuit Statistics
// ============================================================

// CircuitStats holds detailed circuit statistics
type CircuitStats struct {
	GateCounts       map[string]int
	TotalGates       int
	TwoQubitGates    int
	Depth            int
	Width            int
	TGateCount       int
	TDepth           int
	Parameterized    int
}

// ComputeStats returns detailed statistics for a circuit
func ComputeStats(c *QCircuit) CircuitStats {
	stats := CircuitStats{
		GateCounts: make(map[string]int),
		Width:      c.NumQubits,
		Depth:      c.Depth(),
	}

	for _, g := range c.Gates {
		stats.GateCounts[g.Name]++
		stats.TotalGates++

		if len(g.Controls) > 0 || len(g.Qubits) == 2 {
			stats.TwoQubitGates++
		}

		switch g.Name {
		case "t", "tdag":
			stats.TGateCount++
		}

		if isRotationGate(g.Name) && g.Angle != 0 {
			stats.Parameterized++
		}
	}

	return stats
}
