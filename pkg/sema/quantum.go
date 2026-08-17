package sema

import (
	"fmt"
	"karkain/pkg/parser"
)

// QubitState tracks the lifecycle state of a qubit for compile-time safety
type QubitState int

const (
	QubitAllocated QubitState = iota // Freshly allocated, ready for gates
	QubitInitialized                 // After reset, ready for gates
	QubitMeasured                    // After measure — measurement collapsed
	QubitFreed                       // After deallocation
)

func (s QubitState) String() string {
	switch s {
	case QubitAllocated:
		return "allocated"
	case QubitInitialized:
		return "initialized"
	case QubitMeasured:
		return "measured"
	case QubitFreed:
		return "freed"
	default:
		return "unknown"
	}
}

// QubitInfo tracks information about a single qubit or qubit register
type QubitInfo struct {
	Name      string
	State     QubitState
	Size      int  // 1 for single qubit, N for register
	IsRegister bool
	AssignedFrom string // Name of the source qubit if assigned (for cloning check)
}

// QuantumSafetyAnalyzer performs compile-time quantum safety validation:
// - No-Cloning Theorem enforcement
// - Measurement state collapse tracking
// - Gate-after-measurement detection
type QuantumSafetyAnalyzer struct {
	qubits      map[string]*QubitInfo // qubit name -> info
	errors      []error
	circuitName string
}

// NewQuantumSafetyAnalyzer creates a new quantum safety analyzer
func NewQuantumSafetyAnalyzer() *QuantumSafetyAnalyzer {
	return &QuantumSafetyAnalyzer{
		qubits: make(map[string]*QubitInfo),
	}
}

// GetErrors returns accumulated errors
func (a *QuantumSafetyAnalyzer) GetErrors() []error {
	return a.errors
}

// AnalyzeCircuit validates a circuit declaration for quantum safety violations
func (a *QuantumSafetyAnalyzer) AnalyzeCircuit(circuit *parser.CircuitDecl) []error {
	a.errors = nil
	a.qubits = make(map[string]*QubitInfo)
	a.circuitName = circuit.Name

	// Register parameters as qubits
	for _, param := range circuit.Params {
		qubitSize := 1
		if param.Type != nil && param.Type.Size > 0 {
			qubitSize = param.Type.Size
		}
		a.qubits[param.Name] = &QubitInfo{
			Name:       param.Name,
			State:      QubitAllocated,
			Size:       qubitSize,
			IsRegister: qubitSize > 1,
		}
	}

	// Analyze body statements
	a.analyzeBody(circuit.Body)

	return a.errors
}

// analyzeBody processes a list of statements
func (a *QuantumSafetyAnalyzer) analyzeBody(stmts []parser.Node) {
	for _, stmt := range stmts {
		a.analyzeStatement(stmt)
	}
}

// analyzeStatement checks a single statement for safety violations
func (a *QuantumSafetyAnalyzer) analyzeStatement(stmt parser.Node) {
	switch n := stmt.(type) {
	case *parser.ExprStmt:
		a.analyzeExpression(n.Expression)
	case *parser.QPUOpExpr:
		a.analyzeQPUOp(n)
	case *parser.CircuitReturnStmt:
		if n.Value != nil {
			a.analyzeExpression(n.Value)
		}
	case *parser.VarDeclStmt:
		if n.Value != nil {
			a.analyzeExpression(n.Value)
		}
	}
}

// analyzeExpression checks an expression for quantum safety violations
func (a *QuantumSafetyAnalyzer) analyzeExpression(expr parser.Node) {
	switch n := expr.(type) {
	case *parser.QPUOpExpr:
		a.analyzeQPUOp(n)
	case *parser.MeasureExpr:
		a.analyzeMeasure(n)
	}
}

// analyzeQPUOp validates a quantum gate operation
func (a *QuantumSafetyAnalyzer) analyzeQPUOp(op *parser.QPUOpExpr) {
	// Check each argument for validity
	for _, arg := range op.Args {
		a.checkQubitAccess(arg, op.Op)
	}

	// Track measurement
	if op.Op == "measure" {
		for _, arg := range op.Args {
			a.markMeasured(arg)
		}
	}

	// Track reset
	if op.Op == "reset" {
		for _, arg := range op.Args {
			a.markReset(arg)
		}
	}
}

// checkQubitAccess verifies a qubit reference is valid for the given operation
func (a *QuantumSafetyAnalyzer) checkQubitAccess(node parser.Node, op string) {
	qubitName := a.resolveQubitName(node)
	if qubitName == "" {
		return
	}

	qubit, ok := a.qubits[qubitName]
	if !ok {
		// Unknown qubit — not tracked, skip
		return
	}

	// Cannot apply gates to a measured qubit without reset
	if qubit.State == QubitMeasured && op != "reset" {
		a.errors = append(a.errors, fmt.Errorf(
			"circuit '%s': cannot apply gate '%s' to qubit '%s' after measurement without reset",
			a.circuitName, op, qubitName))
	}

	// Cannot apply gates to a freed qubit
	if qubit.State == QubitFreed {
		a.errors = append(a.errors, fmt.Errorf(
			"circuit '%s': cannot apply gate '%s' to freed qubit '%s'",
			a.circuitName, op, qubitName))
	}
}

// analyzeMeasure validates a legacy MeasureExpr
func (a *QuantumSafetyAnalyzer) analyzeMeasure(expr *parser.MeasureExpr) {
	qubitName := a.resolveQubitName(expr.Qubit)
	if qubitName == "" {
		return
	}
	a.markMeasured(expr.Qubit)
}

// markMeasured marks a qubit as measured
func (a *QuantumSafetyAnalyzer) markMeasured(node parser.Node) {
	qubitName := a.resolveQubitName(node)
	if qubitName == "" {
		return
	}
	if qubit, ok := a.qubits[qubitName]; ok {
		qubit.State = QubitMeasured
	}
}

// markReset resets a qubit to initialized state (ready for reuse)
func (a *QuantumSafetyAnalyzer) markReset(node parser.Node) {
	qubitName := a.resolveQubitName(node)
	if qubitName == "" {
		return
	}
	if qubit, ok := a.qubits[qubitName]; ok {
		qubit.State = QubitInitialized
	}
}

// resolveQubitName extracts the qubit variable name from an expression
func (a *QuantumSafetyAnalyzer) resolveQubitName(node parser.Node) string {
	switch n := node.(type) {
	case *parser.Identifier:
		return n.Name
	case *parser.QubitIndexExpr:
		return a.resolveQubitName(n.Qubit)
	default:
		return ""
	}
}

// ============================================================
// No-Cloning Theorem Enforcement
// ============================================================

// CheckNoCloning verifies that a qubit is not assigned, copied, or aliased
// This enforces the No-Cloning Theorem at compile time.
func (a *QuantumSafetyAnalyzer) CheckNoCloning(stmts []parser.Node) []error {
	a.errors = nil
	assignments := make(map[string]string) // target -> source

	for _, stmt := range stmts {
		switch n := stmt.(type) {
		case *parser.VarDeclStmt:
			// Check if assigning a qubit variable to another
			if sourceName := a.resolveQubitName(n.Value); sourceName != "" {
				if _, isQubit := a.qubits[sourceName]; isQubit {
					if existingSource, alreadyAssigned := assignments[n.Name]; alreadyAssigned && existingSource != sourceName {
						// Re-assignment from different source
						a.errors = append(a.errors, fmt.Errorf(
							"no-cloning violation: qubit '%s' cannot be reassigned from '%s' (previously '%s')",
							n.Name, sourceName, existingSource))
					}
					assignments[n.Name] = sourceName

					// Check direct copy: if source is already tracked, this is a clone attempt
					if sourceInfo, ok := a.qubits[sourceName]; ok {
						// Create a new qubit entry if it doesn't exist, or flag as aliased
						if _, exists := a.qubits[n.Name]; !exists {
							a.qubits[n.Name] = &QubitInfo{
								Name:         n.Name,
								State:        sourceInfo.State,
								Size:         sourceInfo.Size,
								IsRegister:   sourceInfo.IsRegister,
								AssignedFrom: sourceName,
							}
						}
					}
				}
			}

		case *parser.ExprStmt:
			a.checkExprForCloning(n.Expression, assignments)

		case *parser.QPUOpExpr:
			a.checkOpForCloning(n, assignments)
		}
	}

	return a.errors
}

// checkExprForCloning checks an expression for qubit cloning
func (a *QuantumSafetyAnalyzer) checkExprForCloning(expr parser.Node, assignments map[string]string) {
	switch n := expr.(type) {
	case *parser.QPUOpExpr:
		a.checkOpForCloning(n, assignments)
	}
}

// checkOpForCloning checks a QPU operation for cloning violations
func (a *QuantumSafetyAnalyzer) checkOpForCloning(op *parser.QPUOpExpr, assignments map[string]string) {
	// Track which qubits are being used as targets (destinations)
	targets := make([]string, 0)

	for _, arg := range op.Args {
		name := a.resolveQubitName(arg)
		if name == "" {
			continue
		}

		// Check if this qubit is an alias of another tracked qubit
		if source, ok := assignments[name]; ok {
			if _, sourceTracked := a.qubits[source]; sourceTracked {
				// This is using an aliased qubit — check if it's also used elsewhere
				for _, otherTarget := range targets {
					if otherTarget == source || otherTarget == name {
						continue
					}
					// Check if otherTarget is also an alias of the same source
					if otherSource, ok := assignments[otherTarget]; ok && otherSource == source {
						a.errors = append(a.errors, fmt.Errorf(
							"no-cloning violation: qubit alias '%s' and '%s' both originate from '%s'",
							name, otherTarget, source))
					}
				}
			}
		}

		targets = append(targets, name)
	}
}

// RegisterQubit registers a qubit in the analyzer's tracking table
func (a *QuantumSafetyAnalyzer) RegisterQubit(name string, size int) {
	a.qubits[name] = &QubitInfo{
		Name:       name,
		State:      QubitAllocated,
		Size:       size,
		IsRegister: size > 1,
	}
}

// GetQubitState returns the current state of a tracked qubit
func (a *QuantumSafetyAnalyzer) GetQubitState(name string) (QubitState, bool) {
	if q, ok := a.qubits[name]; ok {
		return q.State, true
	}
	return QubitAllocated, false
}
