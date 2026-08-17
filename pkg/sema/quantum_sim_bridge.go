package sema

import (
	"fmt"
	"karkain/pkg/parser"
	"strings"
)

// QuantumSimBridge validates CircuitDecl nodes and converts them into
// a WGSL pipeline descriptor suitable for the QuantumSimGenerator.
type QuantumSimBridge struct {
	errors []string
}

func NewQuantumSimBridge() *QuantumSimBridge {
	return &QuantumSimBridge{}
}

// WGSLPipelineDescriptor describes the GPU pipeline configuration
// needed to execute a quantum circuit simulation.
type WGSLPipelineDescriptor struct {
	CircuitName    string
	NumQubits      int
	StateSize      int
	EstimateBytes  int64
	MaxQubits      int
	WorkgroupSize  int
	GateSequence   []GateDispatch
	NeedsNormalize bool
	NeedsCollapse  bool
}

// GateDispatch describes a single GPU dispatch for a gate operation
type GateDispatch struct {
	Op         string
	KernelName string
	TargetBits []int
	AngleExpr  string
	Workgroups int
}

// ValidateAndDescribe analyzes a CircuitDecl and returns a pipeline descriptor.
// It emits warnings for high qubit counts and errors for invalid circuits.
func (b *QuantumSimBridge) ValidateAndDescribe(circuit *parser.CircuitDecl) (*WGSLPipelineDescriptor, error) {
	b.errors = nil

	numQubits, err := countTotalQubits(circuit)
	if err != nil {
		return nil, err
	}

	// Validate qubit count against GPU memory limits
	if err := validateQubitCountBridge(numQubits); err != nil {
		b.errors = append(b.errors, err.Error())
	}

	stateSize := 1 << uint(numQubits)
	estimateBytes := int64(stateSize) * 8 // float32x2

	desc := &WGSLPipelineDescriptor{
		CircuitName:   circuit.Name,
		NumQubits:     numQubits,
		StateSize:     stateSize,
		EstimateBytes: estimateBytes,
		MaxQubits:     30,
		WorkgroupSize: 64,
		GateSequence:  b.analyzeGateSequence(circuit.Body, numQubits),
	}

	// Check if measurement appears in circuit (needs collapse + normalize)
	for _, gate := range desc.GateSequence {
		if gate.Op == "measure" {
			desc.NeedsCollapse = true
			desc.NeedsNormalize = true
			break
		}
	}

	if len(b.errors) > 0 {
		return desc, fmt.Errorf("quantum bridge validation: %s", strings.Join(b.errors, "; "))
	}
	return desc, nil
}

func countTotalQubits(circuit *parser.CircuitDecl) (int, error) {
	total := 0
	for _, p := range circuit.Params {
		if p.Type == nil || p.Type.Size <= 0 {
			total += 1
		} else {
			total += p.Type.Size
		}
	}
	if total == 0 {
		return 0, fmt.Errorf("circuit '%s' has no qubit parameters", circuit.Name)
	}
	return total, nil
}

func validateQubitCountBridge(n int) error {
	const maxQubits = 30
	if n > maxQubits {
		estimateBytes := int64(1<<uint(n)) * 8
		return fmt.Errorf("WARNING: circuit with %d qubits requires ~%d MiB GPU local memory for state vector (limit: %d qubits / ~%d MiB)",
			n, estimateBytes/(1024*1024), maxQubits, int64(1<<uint(maxQubits))*8/(1024*1024))
	}
	if n > 25 {
		estimateBytes := int64(1<<uint(n)) * 8
		return fmt.Errorf("WARNING: circuit with %d qubits will use ~%d MiB GPU memory — consider reducing qubit count",
			n, estimateBytes/(1024*1024))
	}
	return nil
}

func (b *QuantumSimBridge) analyzeGateSequence(stmts []parser.Node, numQubits int) []GateDispatch {
	var seq []GateDispatch
	for _, stmt := range stmts {
		switch n := stmt.(type) {
		case *parser.QPUOpExpr:
			seq = append(seq, b.describeGateDispatch(n, numQubits))
		case *parser.ExprStmt:
			if qpu, ok := n.Expression.(*parser.QPUOpExpr); ok {
				seq = append(seq, b.describeGateDispatch(qpu, numQubits))
			}
		}
	}
	return seq
}

func (b *QuantumSimBridge) describeGateDispatch(op *parser.QPUOpExpr, numQubits int) GateDispatch {
	stateSize := 1 << uint(numQubits)
	wgSize := 64
	totalWG := (stateSize + wgSize - 1) / wgSize

	gd := GateDispatch{
		Op:         op.Op,
		TargetBits: b.resolveTargetBits(op, numQubits),
		Workgroups: totalWG,
	}

	switch op.Op {
	case "h":
		gd.KernelName = "apply_h"
	case "cx":
		gd.KernelName = "apply_cnot"
	case "ry":
		gd.KernelName = "y_rotation"
	case "swap":
		gd.KernelName = "apply_swap"
	case "measure":
		gd.KernelName = "measure_collapse"
		gd.Workgroups = 1
	case "reset":
		gd.KernelName = "reset_qubit"
		gd.Workgroups = 1
	case "x":
		gd.KernelName = "apply_x"
	case "y":
		gd.KernelName = "apply_y"
	case "z":
		gd.KernelName = "apply_z"
	case "rx":
		gd.KernelName = "rx_rotation"
	case "rz":
		gd.KernelName = "rz_rotation"
	case "cz":
		gd.KernelName = "apply_cz"
	default:
		gd.KernelName = "unknown_" + op.Op
	}

	return gd
}

func (b *QuantumSimBridge) resolveTargetBits(op *parser.QPUOpExpr, numQubits int) []int {
	var bits []int
	for _, arg := range op.Args {
		idx := resolveQubitIndex(arg)
		if idx >= 0 && idx < numQubits {
			bits = append(bits, idx)
		} else {
			bits = append(bits, -1) // unresolved
		}
	}
	return bits
}

func resolveQubitIndex(node parser.Node) int {
	switch n := node.(type) {
	case *parser.QubitIndexExpr:
		if lit, ok := n.Index.(*parser.IntLiteral); ok {
			val := 0
			fmt.Sscanf(lit.Value, "%d", &val)
			return val
		}
	case *parser.Identifier:
		return 0
	}
	return -1
}

// GetErrors returns accumulated validation errors
func (b *QuantumSimBridge) GetErrors() []string {
	return b.errors
}
