package sema

import (
	"karkain/pkg/parser"
	"testing"
)

func TestQuantumDist_LargeCircuitPlan(t *testing.T) {
	circuit := &parser.CircuitDecl{
		Name: "ThirtyTwoQubitCircuit",
		Params: []parser.CircuitParam{
			{Name: "q", Type: &parser.QubitType{Size: 32}},
		},
		ReturnType: &parser.BitType{Size: 32},
		Body: []parser.Node{
			// Local gates (qubits 0-28 are local with 8 ranks)
			&parser.QPUOpExpr{
				Op:   "h",
				Args: []parser.Node{&parser.QubitIndexExpr{Qubit: &parser.Identifier{Name: "q"}, Index: &parser.IntLiteral{Value: "0"}}},
			},
			&parser.QPUOpExpr{
				Op:   "ry",
				Args: []parser.Node{&parser.QubitIndexExpr{Qubit: &parser.Identifier{Name: "q"}, Index: &parser.IntLiteral{Value: "5"}}},
				Angle: &parser.Float64Literal{Value: "1.57"},
			},
			&parser.QPUOpExpr{
				Op: "cx",
				Args: []parser.Node{
					&parser.QubitIndexExpr{Qubit: &parser.Identifier{Name: "q"}, Index: &parser.IntLiteral{Value: "0"}},
					&parser.QubitIndexExpr{Qubit: &parser.Identifier{Name: "q"}, Index: &parser.IntLiteral{Value: "1"}},
				},
			},
			// Global gate: CNOT across partition (qubit 29 is global)
			&parser.QPUOpExpr{
				Op: "cx",
				Args: []parser.Node{
					&parser.QubitIndexExpr{Qubit: &parser.Identifier{Name: "q"}, Index: &parser.IntLiteral{Value: "28"}},
					&parser.QubitIndexExpr{Qubit: &parser.Identifier{Name: "q"}, Index: &parser.IntLiteral{Value: "29"}},
				},
			},
			// Measure
			&parser.QPUOpExpr{
				Op: "measure",
				Args: []parser.Node{
					&parser.QubitIndexExpr{Qubit: &parser.Identifier{Name: "q"}, Index: &parser.IntLiteral{Value: "0"}},
					&parser.QubitIndexExpr{Qubit: &parser.Identifier{Name: "q"}, Index: &parser.IntLiteral{Value: "29"}},
				},
			},
		},
	}

	planner := NewDistributedQuantumPlanner()

	// 8 ranks: 29 local qubits, 3 global qubits, chunk = 2^29 = 536870912 amplitudes
	// 536870912 * 8 = ~4 GiB per rank
	maxPerRank := int64(4 * 1024 * 1024 * 1024) // 4 GiB

	plan, err := planner.PlanDistributedExecution(circuit, maxPerRank)
	if err != nil {
		t.Fatalf("PlanDistributedExecution failed: %v", err)
	}

	// Verify partition
	if plan.NumQubits != 32 {
		t.Errorf("NumQubits = %d, want 32", plan.NumQubits)
	}
	if plan.LocalQubits != 29 {
		t.Errorf("LocalQubits = %d, want 29", plan.LocalQubits)
	}
	if plan.GlobalQubits != 3 {
		t.Errorf("GlobalQubits = %d, want 3", plan.GlobalQubits)
	}
	if plan.NumRanks != 8 {
		t.Errorf("NumRanks = %d, want 8", plan.NumRanks)
	}
	if plan.ChunkSize != 536870912 {
		t.Errorf("ChunkSize = %d, want 536870912", plan.ChunkSize)
	}

	// Verify total memory
	expectedTotal := int64(1<<32) * 8
	if plan.TotalBytes != expectedTotal {
		t.Errorf("TotalBytes = %d, want %d", plan.TotalBytes, expectedTotal)
	}

	// Verify gate schedule
	if len(plan.GateSchedule) != 5 {
		t.Errorf("expected 5 gate entries, got %d", len(plan.GateSchedule))
	}

	// First 3 gates should be local (qubits 0, 5, 0->1 are all < 29)
	localCount := 0
	globalCount := 0
	for _, entry := range plan.GateSchedule {
		if entry.IsLocal {
			localCount++
		} else {
			globalCount++
		}
	}

	if localCount != 3 {
		t.Errorf("expected 3 local gates, got %d", localCount)
	}
	if globalCount != 2 {
		t.Errorf("expected 2 global gates (CNOT 28->29, measure), got %d", globalCount)
	}

	// Validate plan consistency
	issues := planner.ValidatePlan(plan)
	if len(issues) > 0 {
		t.Errorf("validation issues: %v", issues)
	}

	// Verify communication volume for global gates
	totalCommKB := 0.0
	for _, entry := range plan.GateSchedule {
		totalCommKB += entry.ExchangeKB
	}
	if totalCommKB <= 0 {
		t.Error("expected positive communication volume for global gates")
	}
}

func TestQuantumDist_PartitionConsistency(t *testing.T) {
	tests := []struct {
		name       string
		numQubits  int
		maxBytes   int64
		wantRanks  int
		wantLocal  int
		wantGlobal int
	}{
		{"26 qubits, 1GiB", 26, 1024 * 1024 * 1024, 1, 26, 0},
		{"28 qubits, 1GiB", 28, 1024 * 1024 * 1024, 2, 27, 1},
		{"30 qubits, 1GiB", 30, 1024 * 1024 * 1024, 8, 27, 3},
		{"32 qubits, 1GiB", 32, 1024 * 1024 * 1024, 32, 27, 5},
		{"32 qubits, 4GiB", 32, 4 * 1024 * 1024 * 1024, 8, 29, 3},
		{"40 qubits, 4GiB", 40, 4 * 1024 * 1024 * 1024, 2048, 29, 11},
		{"26 qubits, 256MiB", 26, 256 * 1024 * 1024, 2, 25, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ranks, local, global, chunk := StaticPartition(tt.numQubits, tt.maxBytes)
			if ranks != tt.wantRanks {
				t.Errorf("ranks = %d, want %d", ranks, tt.wantRanks)
			}
			if local != tt.wantLocal {
				t.Errorf("local = %d, want %d", local, tt.wantLocal)
			}
			if global != tt.wantGlobal {
				t.Errorf("global = %d, want %d", global, tt.wantGlobal)
			}
			expectedChunk := 1 << uint(local)
			if chunk != expectedChunk {
				t.Errorf("chunk = %d, want %d", chunk, expectedChunk)
			}
		})
	}
}

func TestQuantumDist_MemoryEstimation(t *testing.T) {
	tests := []struct {
		qubits      int
		expectedMem int64
	}{
		{1, 16},           // 2 * 8
		{10, 8192},        // 1024 * 8
		{20, 8388608},     // 1048576 * 8
		{26, 536870912},   // 67108864 * 8
		{30, 8589934592},  // 1073741824 * 8
		{32, 34359738368}, // 4294967296 * 8
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			mem := MemoryForQubits(tt.qubits)
			if mem != tt.expectedMem {
				t.Errorf("MemoryForQubits(%d) = %d, want %d", tt.qubits, mem, tt.expectedMem)
			}
		})
	}
}

func TestQuantumDist_ScheduleFormatting(t *testing.T) {
	plan := &ExecutionPlan{
		NumQubits:    32,
		NumRanks:     8,
		GlobalQubits: 3,
		LocalQubits:  29,
		ChunkSize:    536870912,
		PerRankBytes: 4294967296,
		TotalBytes:   34359738368,
		GateSchedule: []GateScheduleEntry{
			{Op: "h", QubitIndices: []int{0}, IsLocal: true, ExchangeKB: 0},
			{Op: "cx", QubitIndices: []int{0, 1}, IsLocal: true, ExchangeKB: 0},
			{Op: "cx", QubitIndices: []int{28, 29}, IsLocal: false, ExchangeKB: 4194304.0},
		},
		CommunicationKB: 4194304.0,
		EstimatedSteps:  6,
	}

	text := FormatSchedule(plan)

	if len(text) == 0 {
		t.Fatal("FormatSchedule returned empty string")
	}

	// Verify key content
	expectedFragments := []string{
		"32 qubits",
		"8 ranks",
		"Global qubits: 3",
		"Local qubits: 29",
		"Gate schedule:",
		"h [0] LOCAL",
		"cx [0 1] LOCAL",
		"cx [28 29] GLOBAL",
	}

	for _, frag := range expectedFragments {
		found := false
		for i := 0; i <= len(text)-len(frag); i++ {
			if text[i:i+len(frag)] == frag {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected '%s' in formatted schedule:\n%s", frag, text)
		}
	}
}

func TestQuantumDist_CircuitAnalysisIntegration(t *testing.T) {
	circuit := &parser.CircuitDecl{
		Name: "MixedGates",
		Params: []parser.CircuitParam{
			{Name: "q", Type: &parser.QubitType{Size: 4}},
		},
		Body: []parser.Node{
			&parser.QPUOpExpr{
				Op:   "h",
				Args: []parser.Node{&parser.QubitIndexExpr{Qubit: &parser.Identifier{Name: "q"}, Index: &parser.IntLiteral{Value: "0"}}},
			},
			&parser.QPUOpExpr{
				Op:   "ry",
				Args: []parser.Node{&parser.QubitIndexExpr{Qubit: &parser.Identifier{Name: "q"}, Index: &parser.IntLiteral{Value: "1"}}},
				Angle: &parser.Float64Literal{Value: "0.5"},
			},
			&parser.QPUOpExpr{
				Op: "cx",
				Args: []parser.Node{
					&parser.QubitIndexExpr{Qubit: &parser.Identifier{Name: "q"}, Index: &parser.IntLiteral{Value: "0"}},
					&parser.QubitIndexExpr{Qubit: &parser.Identifier{Name: "q"}, Index: &parser.IntLiteral{Value: "2"}},
				},
			},
			&parser.QPUOpExpr{
				Op: "swap",
				Args: []parser.Node{
					&parser.QubitIndexExpr{Qubit: &parser.Identifier{Name: "q"}, Index: &parser.IntLiteral{Value: "2"}},
					&parser.QubitIndexExpr{Qubit: &parser.Identifier{Name: "q"}, Index: &parser.IntLiteral{Value: "3"}},
				},
			},
		},
	}

	planner := NewDistributedQuantumPlanner()

	// With 2 ranks, qubits 0-2 are local, qubit 3 is global
	// maxPerRank = 2^3 * 8 = 64 bytes
	plan, err := planner.PlanDistributedExecution(circuit, 64)
	if err != nil {
		t.Fatalf("PlanDistributedExecution failed: %v", err)
	}

	if plan.NumRanks != 2 {
		t.Errorf("NumRanks = %d, want 2", plan.NumRanks)
	}
	if plan.LocalQubits != 3 {
		t.Errorf("LocalQubits = %d, want 3", plan.LocalQubits)
	}
	if plan.GlobalQubits != 1 {
		t.Errorf("GlobalQubits = %d, want 1", plan.GlobalQubits)
	}

	// Verify gate classifications
	// h(0): local, ry(1): local, cx(0,2): local, swap(2,3): global (qubit 3 is global)
	localCount := 0
	globalCount := 0
	for _, entry := range plan.GateSchedule {
		if entry.IsLocal {
			localCount++
		} else {
			globalCount++
		}
	}
	if localCount != 3 {
		t.Errorf("expected 3 local gates, got %d", localCount)
	}
	if globalCount != 1 {
		t.Errorf("expected 1 global gate (swap with qubit 3), got %d", globalCount)
	}
}
