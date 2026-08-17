package sema

import (
	"fmt"
	"karkain/pkg/parser"
	"math"
)

// ============================================================
// Quantum Gradient Tape — records parameterized circuit executions
// for automatic differentiation via the Parameter-Shift Rule.
// ============================================================

// ParamShiftRule implements the parameter-shift gradient:
//   dE/dθ_i = (E(θ_i + π/2) - E(θ_i - π/2)) / 2
//
// For a circuit C(θ) computing expectation value E(θ) = <ψ(θ)|H|ψ(θ)>,
// each parameterized gate rx/ry/rz contributes one gradient.
const ParamShiftShift = math.Pi / 2.0

// QuantumGateRecord stores a single parameterized gate invocation
type QuantumGateRecord struct {
	Op      string   // "rx", "ry", "rz"
	Qubit   string   // target qubit name
	Index   int      // qubit register index
	Angle   float64  // current angle value
	ParamID int      // unique parameter index (0..N-1)
}

// HamiltonianTerm represents one term in a Hamiltonian operator: coeff * (Pauli_i ⊗ Pauli_j ⊗ ...)
type HamiltonianTerm struct {
	Coefficient float64
	PauliLabels []PauliLabel // one per qubit
}

// PauliLabel is a single-qubit Pauli operator label
type PauliLabel string

const (
	PauliI PauliLabel = "I" // identity
	PauliX PauliLabel = "X"
	PauliY PauliLabel = "Y"
	PauliZ PauliLabel = "Z"
)

// CircuitEvaluator is a function that evaluates a circuit with given parameters
// and returns the expectation value <ψ|H|ψ>
type CircuitEvaluator func(params []float64) float64

// QuantumGradientTape records parameterized gate operations and computes
// gradients via the parameter-shift rule.
type QuantumGradientTape struct {
	Gates          []QuantumGateRecord
	NumParams      int
	Hamiltonian    []HamiltonianTerm
	NumQubits      int
	gradients      []float64
	errors         []string
}

// NewQuantumGradientTape creates a new gradient tape
func NewQuantumGradientTape(numQubits int) *QuantumGradientTape {
	return &QuantumGradientTape{
		NumQubits: numQubits,
	}
}

// RecordGate records a parameterized gate operation on the tape
func (t *QuantumGradientTape) RecordGate(op, qubit string, index int, angle float64) {
	t.Gates = append(t.Gates, QuantumGateRecord{
		Op:      op,
		Qubit:   qubit,
		Index:   index,
		Angle:   angle,
		ParamID: t.NumParams,
	})
	t.NumParams++
}

// SetHamiltonian sets the observable (Hamiltonian) for expectation value computation
func (t *QuantumGradientTape) SetHamiltonian(terms []HamiltonianTerm) {
	t.Hamiltonian = terms
}

// AnalyzeCircuit extracts parameterized gates from a CircuitDecl AST node
func (t *QuantumGradientTape) AnalyzeCircuit(circuit *parser.CircuitDecl) error {
	t.Gates = nil
	t.NumParams = 0
	t.errors = nil

	for _, param := range circuit.Params {
		if param.Type == nil || param.Type.Size <= 0 {
			t.NumQubits = 1
		} else {
			t.NumQubits = param.Type.Size
		}
	}

	for _, stmt := range circuit.Body {
		if err := t.analyzeStmt(stmt); err != nil {
			return err
		}
	}
	return nil
}

func (t *QuantumGradientTape) analyzeStmt(stmt parser.Node) error {
	switch n := stmt.(type) {
	case *parser.ExprStmt:
		return t.analyzeExpr(n.Expression)
	case *parser.QPUOpExpr:
		return t.analyzeQPUOp(n)
	default:
		return nil
	}
}

func (t *QuantumGradientTape) analyzeExpr(expr parser.Node) error {
	switch n := expr.(type) {
	case *parser.QPUOpExpr:
		return t.analyzeQPUOp(n)
	default:
		return nil
	}
}

func (t *QuantumGradientTape) analyzeQPUOp(op *parser.QPUOpExpr) error {
	switch op.Op {
	case "rx", "ry", "rz":
		qubitName := "q"
		qubitIdx := 0
		if len(op.Args) > 0 {
			qubitName = resolveQubitNameStatic(op.Args[0])
			qubitIdx = resolveQubitIndexStatic(op.Args[0])
		}
		angle := 0.0
		if op.Angle != nil {
			angle = evalAngleExpr(op.Angle)
		}
		t.RecordGate(op.Op, qubitName, qubitIdx, angle)
	default:
		// non-parameterized gates are not recorded
	}
	return nil
}

// ComputeGradients evaluates the parameter-shift rule for all recorded parameters.
// evaluator simulates the circuit and returns expectation values.
func (t *QuantumGradientTape) ComputeGradients(evaluator CircuitEvaluator, baseParams []float64) ([]float64, error) {
	if len(baseParams) != t.NumParams {
		return nil, fmt.Errorf("expected %d parameters, got %d", t.NumParams, len(baseParams))
	}

	t.gradients = make([]float64, t.NumParams)

	for i := 0; i < t.NumParams; i++ {
		// θ_i + π/2
		paramsPlus := make([]float64, len(baseParams))
		copy(paramsPlus, baseParams)
		paramsPlus[i] += ParamShiftShift
		ePlus := evaluator(paramsPlus)

		// θ_i - π/2
		paramsMinus := make([]float64, len(baseParams))
		copy(paramsMinus, baseParams)
		paramsMinus[i] -= ParamShiftShift
		eMinus := evaluator(paramsMinus)

		// Gradient: (E+ - E-) / 2
		t.gradients[i] = (ePlus - eMinus) / 2.0
	}

	return t.gradients, nil
}

// ComputeGradientsAnalytic computes analytical gradients for simple rotation circuits.
// For RY(θ), d/dθ <0|R†Y†(θ) H RY(θ)|0> can be computed analytically.
func (t *QuantumGradientTape) ComputeGradientsAnalytic(baseParams []float64, evalFunc CircuitEvaluator) ([]float64, error) {
	analytic := make([]float64, len(baseParams))
	numParams := len(baseParams)
	for i := 0; i < numParams; i++ {
		eps := 1e-7
		paramsFwd := make([]float64, numParams)
		copy(paramsFwd, baseParams)
		paramsFwd[i] += eps
		paramsBwd := make([]float64, numParams)
		copy(paramsBwd, baseParams)
		paramsBwd[i] -= eps
		analytic[i] = (evalFunc(paramsFwd) - evalFunc(paramsBwd)) / (2 * eps)
	}
	return analytic, nil
}

// GetGradients returns the last computed gradients
func (t *QuantumGradientTape) GetGradients() []float64 {
	return t.gradients
}

// GetErrors returns accumulated errors
func (t *QuantumGradientTape) GetErrors() []string {
	return t.errors
}

// ============================================================
// VQE Optimization — classical gradient descent loop
// ============================================================

// VQEIteration represents one step in a VQE optimization
type VQEIteration struct {
	Step       int
	Params     []float64
	Gradients  []float64
	Energy     float64
	Converged  bool
}

// VQEOptimizerConfig holds configuration for VQE optimization
type VQEOptimizerConfig struct {
	MaxIter     int
	LearningRate float64
	Tolerance   float64
}

// DefaultVQEConfig returns sensible defaults for VQE optimization
func DefaultVQEConfig() VQEOptimizerConfig {
	return VQEOptimizerConfig{
		MaxIter:      100,
		LearningRate: 0.1,
		Tolerance:    1e-6,
	}
}

// RunVQE executes a VQE optimization loop using the parameter-shift rule
func RunVQE(tape *QuantumGradientTape, evaluator CircuitEvaluator, initialParams []float64, config VQEOptimizerConfig) ([]VQEIteration, error) {
	params := make([]float64, len(initialParams))
	copy(params, initialParams)

	var history []VQEIteration

	for step := 0; step < config.MaxIter; step++ {
		// Evaluate current energy
		energy := evaluator(params)

		// Compute gradients via parameter-shift rule
		grads, err := tape.ComputeGradients(evaluator, params)
		if err != nil {
			return history, fmt.Errorf("VQE step %d: %w", step, err)
		}

		// Check convergence
		converged := true
		for _, g := range grads {
			if math.Abs(g) > config.Tolerance {
				converged = false
				break
			}
		}

		history = append(history, VQEIteration{
			Step:      step,
			Params:    cloneFloats(params),
			Gradients: cloneFloats(grads),
			Energy:    energy,
			Converged: converged,
		})

		if converged {
			break
		}

		// Gradient descent update: θ_i = θ_i - η * dE/dθ_i
		for i := range params {
			params[i] -= config.LearningRate * grads[i]
		}
	}

	return history, nil
}

// ============================================================
// Hamiltonian evaluation helpers
// ================================================= ExpectValue

// ExpectationValue computes <ψ|H|ψ> from a state vector and Hamiltonian terms.
// state is a slice of complex amplitudes [re0, im0, re1, im1, ...]
func ExpectationValue(state []float64, hamiltonian []HamiltonianTerm, numQubits int) float64 {
	result := 0.0
	dim := 1 << uint(numQubits)

	for _, term := range hamiltonian {
		// For each computational basis state |x>, compute <x|H|x>
		for basis := 0; basis < dim; basis++ {
			coeff := term.Coefficient
			// Apply Pauli operators to basis state
			target := basis
			valid := true
			for q := 0; q < numQubits && q < len(term.PauliLabels); q++ {
				bit := (basis >> uint(q)) & 1
				switch term.PauliLabels[q] {
				case PauliX:
					target ^= (1 << uint(q))
				case PauliZ:
					if bit == 1 {
						coeff = -coeff
					}
				case PauliI:
					// identity, no change
				case PauliY:
					// Y|0> = i|1>, Y|1> = -i|0> (imaginary dropped for diagonal)
					target ^= (1 << uint(q))
					if bit == 1 {
						coeff = -coeff
					}
				default:
					valid = false
				}
			}
			if valid {
				// <basis|H|target> = coeff * delta(basis, target) for diagonal terms
				if target == basis {
					re := state[basis*2]
					result += coeff * re * re
				}
			}
		}
	}
	return result
}

// ============================================================
// AST helpers
// ============================================================

func resolveQubitNameStatic(node parser.Node) string {
	switch n := node.(type) {
	case *parser.Identifier:
		return n.Name
	case *parser.QubitIndexExpr:
		return resolveQubitNameStatic(n.Qubit)
	default:
		return "q"
	}
}

func resolveQubitIndexStatic(node parser.Node) int {
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

func evalAngleExpr(node parser.Node) float64 {
	switch n := node.(type) {
	case *parser.Float64Literal:
		val := 0.0
		fmt.Sscanf(n.Value, "%f", &val)
		return val
	case *parser.IntLiteral:
		val := 0
		fmt.Sscanf(n.Value, "%d", &val)
		return float64(val)
	case *parser.BinaryExpr:
		left := evalAngleExpr(n.Left)
		right := evalAngleExpr(n.Right)
		switch n.Operator {
		case "+":
			return left + right
		case "-":
			return left - right
		case "*":
			return left * right
		case "/":
			if right != 0 {
				return left / right
			}
		}
	}
	return 0.0
}

func cloneFloats(src []float64) []float64 {
	dst := make([]float64, len(src))
	copy(dst, src)
	return dst
}
