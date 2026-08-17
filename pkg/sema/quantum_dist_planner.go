package sema

import (
	"fmt"
	"karkain/pkg/parser"
	"math"
	"strings"
)

// ============================================================
// Distributed Quantum Execution Planner
// Computes memory footprint, communication overhead, and emits
// chunking strategy + communication schedules for multi-GPU
// state-vector simulation.
// ============================================================

// DistributedQuantumPlanner plans and validates distributed execution
type DistributedQuantumPlanner struct {
	errors []string
}

func NewDistributedQuantumPlanner() *DistributedQuantumPlanner {
	return &DistributedQuantumPlanner{}
}

// ExecutionPlan describes the full distributed execution strategy
type ExecutionPlan struct {
	NumQubits       int
	NumRanks        int
	GlobalQubits    int
	LocalQubits     int
	ChunkSize       int // amplitudes per rank
	PerRankBytes    int64
	TotalBytes      int64
	GateSchedule    []GateScheduleEntry
	CommunicationKB float64
	EstimatedSteps  int
	Warnings        []string
	Errors          []string
}

// GateScheduleEntry describes one gate's execution plan
type GateScheduleEntry struct {
	Op          string
	QubitIndices []int
	IsLocal     bool
	ExchangeKB  float64 // communication volume for this gate
	PartnerRanks []int  // ranks that must exchange
}

// PlanDistributedExecution computes a full execution plan for the circuit
func (p *DistributedQuantumPlanner) PlanDistributedExecution(circuit *parser.CircuitDecl, maxPerRankBytes int64) (*ExecutionPlan, error) {
	p.errors = nil

	numQubits := countPlannerQubits(circuit)
	if numQubits == 0 {
		return nil, fmt.Errorf("circuit '%s' has no qubit parameters", circuit.Name)
	}

	// Compute partitioning
	partition := computeOptimalPartition(int64(numQubits), maxPerRankBytes)

	plan := &ExecutionPlan{
		NumQubits:    numQubits,
		NumRanks:     partition.numRanks,
		GlobalQubits: partition.globalQ,
		LocalQubits:  partition.localQ,
		ChunkSize:    partition.chunkSize,
		PerRankBytes: int64(partition.chunkSize) * 8,
		TotalBytes:   int64(1<<uint(numQubits)) * 8,
	}

	// Validate memory constraints
	if plan.PerRankBytes > maxPerRankBytes {
		plan.Errors = append(plan.Errors, fmt.Sprintf(
			"per-rank memory %d bytes exceeds limit %d bytes for %d-qubit circuit with %d ranks",
			plan.PerRankBytes, maxPerRankBytes, numQubits, plan.NumRanks))
	}

	// Memory warnings
	if numQubits > 30 {
		totalGiB := float64(plan.TotalBytes) / (1024.0 * 1024.0 * 1024.0)
		plan.Warnings = append(plan.Warnings, fmt.Sprintf(
			"circuit requires %.2f GiB total state vector memory", totalGiB))
	}

	// Analyze circuit gates
	plan.GateSchedule = p.analyzeGates(circuit, partition)
	totalCommKB := 0.0
	for _, entry := range plan.GateSchedule {
		totalCommKB += entry.ExchangeKB
	}
	plan.CommunicationKB = totalCommKB

	// Estimate total execution steps (gate passes + exchange rounds)
	plan.EstimatedSteps = len(plan.GateSchedule) * 2 // each gate: local pass + optional exchange

	return plan, nil
}

// ValidatePlan checks the plan for correctness
func (p *DistributedQuantumPlanner) ValidatePlan(plan *ExecutionPlan) []string {
	var issues []string

	if plan.NumRanks < 1 || (plan.NumRanks&(plan.NumRanks-1)) != 0 {
		issues = append(issues, fmt.Sprintf("numRanks must be power of 2, got %d", plan.NumRanks))
	}

	if plan.LocalQubits+plan.GlobalQubits != plan.NumQubits {
		issues = append(issues, fmt.Sprintf("local(%d) + global(%d) != total(%d)",
			plan.LocalQubits, plan.GlobalQubits, plan.NumQubits))
	}

	expectedChunk := 1 << uint(plan.LocalQubits)
	if plan.ChunkSize != expectedChunk {
		issues = append(issues, fmt.Sprintf("chunk size %d != expected %d for %d local qubits",
			plan.ChunkSize, expectedChunk, plan.LocalQubits))
	}

	perRankBytes := int64(plan.ChunkSize) * 8
	if perRankBytes != plan.PerRankBytes {
		issues = append(issues, fmt.Sprintf("PerRankBytes mismatch: %d vs %d", perRankBytes, plan.PerRankBytes))
	}

	return issues
}

type partitionResult struct {
	numRanks   int
	globalQ    int
	localQ     int
	chunkSize  int
}

func computeOptimalPartition(totalQubits, maxPerRankBytes int64) partitionResult {
	tq := int(totalQubits)
	for k := 0; k <= tq; k++ {
		localQ := tq - k
		chunkBytes := int64(1<<uint(localQ)) * 8
		if chunkBytes <= maxPerRankBytes {
			return partitionResult{
				numRanks:  1 << uint(k),
				globalQ:   k,
				localQ:    localQ,
				chunkSize: 1 << uint(localQ),
			}
		}
	}
	// Fallback: fully distributed
	return partitionResult{
		numRanks:  1 << uint(tq),
		globalQ:   tq,
		localQ:    0,
		chunkSize: 1,
	}
}

func (p *DistributedQuantumPlanner) analyzeGates(circuit *parser.CircuitDecl, partition partitionResult) []GateScheduleEntry {
	var entries []GateScheduleEntry
	localQ := partition.localQ

	for _, stmt := range circuit.Body {
		switch n := stmt.(type) {
		case *parser.QPUOpExpr:
			entries = append(entries, p.classifyGate(n, localQ, partition.numRanks))
		case *parser.ExprStmt:
			if qpu, ok := n.Expression.(*parser.QPUOpExpr); ok {
				entries = append(entries, p.classifyGate(qpu, localQ, partition.numRanks))
			}
		}
	}
	return entries
}

func (p *DistributedQuantumPlanner) classifyGate(op *parser.QPUOpExpr, localQ, numRanks int) GateScheduleEntry {
	indices := extractQubitIndices(op)
	entry := GateScheduleEntry{
		Op:           op.Op,
		QubitIndices: indices,
	}

	// Check if any qubit is global
	allLocal := true
	for _, idx := range indices {
		if idx >= localQ {
			allLocal = false
			break
		}
	}

	entry.IsLocal = allLocal

	if !allLocal {
		// Estimate communication: for each global qubit, exchange ~chunk_size * 8 bytes
		numGlobalQubits := 0
		for _, idx := range indices {
			if idx >= localQ {
				numGlobalQubits++
			}
		}
		chunkBytes := float64(int64(1)<<uint(localQ)) * 8.0
		entry.ExchangeKB = chunkBytes * float64(numGlobalQubits) / 1024.0

		// Compute partner ranks
		for _, idx := range indices {
			if idx >= localQ {
				partner := numRanks // placeholder — real partner depends on rank
				entry.PartnerRanks = append(entry.PartnerRanks, partner)
			}
		}
	}

	return entry
}

func extractQubitIndices(op *parser.QPUOpExpr) []int {
	var indices []int
	for _, arg := range op.Args {
		idx := resolveQubitIdxPlanner(arg)
		if idx >= 0 {
			indices = append(indices, idx)
		}
	}
	return indices
}

func resolveQubitIdxPlanner(node parser.Node) int {
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
		return -1
	}
}

func countPlannerQubits(circuit *parser.CircuitDecl) int {
	total := 0
	for _, p := range circuit.Params {
		if p.Type == nil || p.Type.Size <= 0 {
			total += 1
		} else {
			total += p.Type.Size
		}
	}
	return total
}

// ============================================================
// Communication schedule formatting
// ============================================================

// FormatSchedule produces a human-readable execution plan
func FormatSchedule(plan *ExecutionPlan) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("=== Distributed Execution Plan: %d qubits, %d ranks ===\n", plan.NumQubits, plan.NumRanks))
	b.WriteString(fmt.Sprintf("  Global qubits: %d | Local qubits: %d\n", plan.GlobalQubits, plan.LocalQubits))
	b.WriteString(fmt.Sprintf("  Chunk size: %d amplitudes | Per-rank: %d bytes\n", plan.ChunkSize, plan.PerRankBytes))
	b.WriteString(fmt.Sprintf("  Total state vector: %d bytes (%.2f MiB)\n", plan.TotalBytes, float64(plan.TotalBytes)/(1024*1024)))
	b.WriteString(fmt.Sprintf("  Total communication: %.2f KB\n", plan.CommunicationKB))
	b.WriteString(fmt.Sprintf("  Estimated steps: %d\n", plan.EstimatedSteps))

	if len(plan.Warnings) > 0 {
		b.WriteString("  Warnings:\n")
		for _, w := range plan.Warnings {
			b.WriteString(fmt.Sprintf("    - %s\n", w))
		}
	}
	if len(plan.Errors) > 0 {
		b.WriteString("  Errors:\n")
		for _, e := range plan.Errors {
			b.WriteString(fmt.Sprintf("    - %s\n", e))
		}
	}

	b.WriteString("  Gate schedule:\n")
	for i, entry := range plan.GateSchedule {
		locStr := "LOCAL"
		if !entry.IsLocal {
			locStr = "GLOBAL"
		}
		commStr := ""
		if entry.ExchangeKB > 0 {
			commStr = fmt.Sprintf(" [exchange: %.2f KB]", entry.ExchangeKB)
		}
		b.WriteString(fmt.Sprintf("    %d: %s %v %s%s\n", i, entry.Op, entry.QubitIndices, locStr, commStr))
	}

	return b.String()
}

// ============================================================
// Static helper functions
// ============================================================

// StaticPartition computes partition for a given qubit count and max memory
func StaticPartition(numQubits int, maxPerRankBytes int64) (numRanks, localQ, globalQ, chunkSize int) {
	for k := 0; k <= numQubits; k++ {
		lq := numQubits - k
		chunkBytes := int64(1<<uint(lq)) * 8
		if chunkBytes <= maxPerRankBytes {
			return 1 << uint(k), lq, k, 1 << uint(lq)
		}
	}
	return 1 << uint(numQubits), 0, numQubits, 1
}

// MemoryForQubits returns total memory bytes for N qubits
func MemoryForQubits(n int) int64 {
	return int64(1<<uint(n)) * 8
}

// Suppress unused import warnings
var _ = math.Abs
