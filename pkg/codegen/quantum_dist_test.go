package codegen

import (
	"karkain/pkg/parser"
	"testing"
)

func TestQuantumDist_StateVectorPartitioning(t *testing.T) {
	tests := []struct {
		name        string
		totalQ      int
		numRanks    int
		wantLocalQ  int
		wantGlobalQ int
		wantChunk   int
	}{
		{"2 ranks, 28 qubits", 28, 2, 27, 1, 1 << 27},
		{"4 ranks, 30 qubits", 30, 4, 28, 2, 1 << 28},
		{"8 ranks, 32 qubits", 32, 8, 29, 3, 1 << 29},
		{"2 ranks, 2 qubits", 2, 2, 1, 1, 2},
		{"4 ranks, 4 qubits", 4, 4, 2, 2, 4},
		{"8 ranks, 6 qubits", 6, 8, 3, 3, 8},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := NewDistQuantumConfig(tt.totalQ, tt.numRanks)
			if err != nil {
				t.Fatalf("NewDistQuantumConfig(%d, %d) failed: %v", tt.totalQ, tt.numRanks, err)
			}
			if cfg.LocalQubits != tt.wantLocalQ {
				t.Errorf("LocalQubits = %d, want %d", cfg.LocalQubits, tt.wantLocalQ)
			}
			if cfg.GlobalQubits != tt.wantGlobalQ {
				t.Errorf("GlobalQubits = %d, want %d", cfg.GlobalQubits, tt.wantGlobalQ)
			}
			if cfg.ChunkSize != tt.wantChunk {
				t.Errorf("ChunkSize = %d, want %d", cfg.ChunkSize, tt.wantChunk)
			}
		})
	}

	// Verify index mapping round-trip
	t.Run("IndexRoundTrip", func(t *testing.T) {
		localQubits := 3
		numRanks := 4
		totalStates := 8 * 4 // 5 qubits = 32 states

		for globalIdx := 0; globalIdx < totalStates; globalIdx++ {
			rank, localIdx := GlobalToRank(globalIdx, localQubits)
			recovered := RankToGlobal(rank, localIdx, localQubits)
			if recovered != globalIdx {
				t.Errorf("round-trip failed: global=%d -> (rank=%d, local=%d) -> %d",
					globalIdx, rank, localIdx, recovered)
			}
			if rank < 0 || rank >= numRanks {
				t.Errorf("rank %d out of range [0, %d)", rank, numRanks)
			}
			if localIdx < 0 || localIdx >= (1<<uint(localQubits)) {
				t.Errorf("localIdx %d out of range", localIdx)
			}
		}
	})

	// Verify qubit classification
	t.Run("QubitClassification", func(t *testing.T) {
		localQubits := 3
		// Qubits 0,1,2 are local; 3,4 are global
		if IsGlobalQubit(0, localQubits) {
			t.Error("qubit 0 should be local")
		}
		if IsGlobalQubit(2, localQubits) {
			t.Error("qubit 2 should be local")
		}
		if !IsGlobalQubit(3, localQubits) {
			t.Error("qubit 3 should be global")
		}
		if !IsGlobalQubit(4, localQubits) {
			t.Error("qubit 4 should be global")
		}
	})

	// Verify gate classification
	t.Run("GateClassification", func(t *testing.T) {
		localQubits := 3
		if ClassifyGate(0, 1, localQubits) != GateLocal {
			t.Error("gate on qubits 0,1 should be local")
		}
		if ClassifyGate(0, 3, localQubits) != GateGlobal {
			t.Error("gate on qubits 0,3 should be global")
		}
		if ClassifyGate(3, 4, localQubits) != GateGlobal {
			t.Error("gate on qubits 3,4 should be global")
		}
	})

	// Verify exchange partner
	t.Run("ExchangePartner", func(t *testing.T) {
		localQubits := 2
		numRanks := 4
		// For target qubit 2 (global qubit 0), partner = rank XOR 1
		partner := ExchangePartner(0, 2, localQubits, numRanks)
		if partner != 1 {
			t.Errorf("partner for rank 0, target qubit 2: got %d, want 1", partner)
		}
		partner = ExchangePartner(1, 2, localQubits, numRanks)
		if partner != 0 {
			t.Errorf("partner for rank 1, target qubit 2: got %d, want 0", partner)
		}
	})
}

func TestQuantumDist_GlobalGateExchange(t *testing.T) {
	localQubits := 2
	numRanks := 4

	// Global CNOT: ctrl=qubit 2 (global), tgt=qubit 0 (local)
	// Rank 0 (binary 00): ctrl bit not set → no action
	// Rank 1 (binary 01): ctrl bit set → swap with partner on tgt
	// Rank 2 (binary 10): ctrl bit not set → no action
	// Rank 3 (binary 11): ctrl bit set → swap with partner on tgt

	for rank := 0; rank < numRanks; rank++ {
		ctrlBitVal := (rank >> 0) & 1 // qubit 2 is global bit 0
		partner := ExchangePartner(rank, 2, localQubits, numRanks)

		t.Run("rank"+string(rune('0'+rank)), func(t *testing.T) {
			if ctrlBitVal == 1 {
				// This rank has ctrl=1, should exchange with partner
				if partner == rank {
					t.Error("partner should be different from self")
				}
				if partner < 0 || partner >= numRanks {
					t.Errorf("partner %d out of range", partner)
				}
			}
		})
	}

	// Verify SWAP partner for two global qubits
	t.Run("SwapGlobal", func(t *testing.T) {
		// SWAP qubit 3, qubit 4 (both global)
		for rank := 0; rank < numRanks; rank++ {
			// For qubit 3 (global bit 1), partner = rank XOR 2
			partner3 := ExchangePartner(rank, 3, localQubits, numRanks)
			if partner3 == rank {
				t.Errorf("rank %d: partner for qubit 3 should not be self", rank)
			}
			// Verify involutive: partner's partner returns original
			partner3back := ExchangePartner(partner3, 3, localQubits, numRanks)
			if partner3back != rank {
				t.Errorf("rank %d: partner exchange not involutive", rank)
			}
		}
	})
}

func TestQuantumDist_LargeCircuitPlan(t *testing.T) {
	// Test WGSL generation for a 32-qubit circuit distributed across 8 ranks
	circuit := &parser.CircuitDecl{
		Name: "LargeCircuit",
		Params: []parser.CircuitParam{
			{Name: "q", Type: &parser.QubitType{Size: 32}},
		},
		ReturnType: &parser.BitType{Size: 32},
		Body: []parser.Node{
			&parser.QPUOpExpr{
				Op:   "h",
				Args: []parser.Node{&parser.QubitIndexExpr{Qubit: &parser.Identifier{Name: "q"}, Index: &parser.IntLiteral{Value: "0"}}},
			},
			&parser.QPUOpExpr{
				Op: "cx",
				Args: []parser.Node{
					&parser.QubitIndexExpr{Qubit: &parser.Identifier{Name: "q"}, Index: &parser.IntLiteral{Value: "0"}},
					&parser.QubitIndexExpr{Qubit: &parser.Identifier{Name: "q"}, Index: &parser.IntLiteral{Value: "1"}},
				},
			},
			// Global gate: CNOT across partition boundary (qubit 29 is global)
			&parser.QPUOpExpr{
				Op: "cx",
				Args: []parser.Node{
					&parser.QubitIndexExpr{Qubit: &parser.Identifier{Name: "q"}, Index: &parser.IntLiteral{Value: "28"}},
					&parser.QubitIndexExpr{Qubit: &parser.Identifier{Name: "q"}, Index: &parser.IntLiteral{Value: "29"}},
				},
			},
		},
	}

	cfg, err := NewDistQuantumConfig(32, 8)
	if err != nil {
		t.Fatalf("NewDistQuantumConfig failed: %v", err)
	}

	gen := NewDistQuantumWGSLGenerator()
	wgsl, err := gen.GenerateDistributedShader(circuit, cfg)
	if err != nil {
		t.Fatalf("GenerateDistributedShader failed: %v", err)
	}

	// Verify header
	if !containsStr(wgsl, "Phase 31") {
		t.Error("expected Phase 31 header")
	}
	if !containsStr(wgsl, "Total qubits: 32") {
		t.Error("expected 32 total qubits")
	}
	if !containsStr(wgsl, "Ranks: 8") {
		t.Error("expected 8 ranks")
	}

	// Verify local gate kernels exist
	if !containsStr(wgsl, "fn local_apply_h(") {
		t.Error("expected local_apply_h kernel")
	}
	if !containsStr(wgsl, "fn local_cnot(") {
		t.Error("expected local_cnot kernel")
	}
	if !containsStr(wgsl, "fn local_ry(") {
		t.Error("expected local_ry kernel")
	}

	// Verify global gate kernels exist
	if !containsStr(wgsl, "fn global_cnot_ctrl_global(") {
		t.Error("expected global_cnot_ctrl_global kernel")
	}
	if !containsStr(wgsl, "fn global_cnot_tgt_global(") {
		t.Error("expected global_cnot_tgt_global kernel")
	}
	if !containsStr(wgsl, "fn global_hadamard(") {
		t.Error("expected global_hadamard kernel")
	}

	// Verify exchange kernels
	if !containsStr(wgsl, "fn pack_exchange(") {
		t.Error("expected pack_exchange kernel")
	}
	if !containsStr(wgsl, "fn unpack_merge(") {
		t.Error("expected unpack_merge kernel")
	}
	if !containsStr(wgsl, "fn local_norm_sq(") {
		t.Error("expected local_norm_sq kernel")
	}

	// Verify dispatch annotations distinguish local vs global
	if !containsStr(wgsl, "qubit 0 is local") {
		t.Error("qubit 0 dispatch should be local")
	}
	if !containsStr(wgsl, "ctrl=28 local, tgt=29 global") {
		t.Error("CNOT 28->29 should be classified as ctrl-local, tgt-global")
	}

	// Verify dist_init_state
	if !containsStr(wgsl, "fn dist_init_state(") {
		t.Error("expected dist_init_state kernel")
	}

	// Verify Uniforms has rank_id and num_ranks
	if !containsStr(wgsl, "rank_id: u32") {
		t.Error("expected rank_id in uniforms")
	}
	if !containsStr(wgsl, "num_ranks: u32") {
		t.Error("expected num_ranks in uniforms")
	}
}

func containsStr(s, substr string) bool {
	return len(s) >= len(substr) && (substr == "" || findSubstring(s, substr))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
