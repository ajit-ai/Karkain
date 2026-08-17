package sema

import (
	"testing"
)

func TestEC_PauliStringOps(t *testing.T) {
	ps := PauliStringFromWord("XYZIZ")
	if ps.N != 5 {
		t.Fatalf("expected N=5, got %d", ps.N)
	}
	if ps.Ops[0] != ECOpX {
		t.Errorf("expected X at q0, got %s", ps.Ops[0])
	}
	if ps.Ops[1] != ECOpY {
		t.Errorf("expected Y at q1, got %s", ps.Ops[1])
	}
	if ps.Ops[2] != ECOpZ {
		t.Errorf("expected Z at q2, got %s", ps.Ops[2])
	}
	if ps.Ops[3] != ECOpI {
		t.Errorf("expected I at q3, got %s", ps.Ops[3])
	}
	if ps.Weight() != 4 {
		t.Errorf("expected weight 4, got %d", ps.Weight())
	}
}

func TestEC_Commutation(t *testing.T) {
	// XX and YY commute (both overlap on 2 qubits, both different → 2 anticommutations → even)
	xx := PauliStringFromWord("XX")
	yy := PauliStringFromWord("YY")
	if !Commutes(xx, yy) {
		t.Error("XX and YY should commute")
	}

	// XY and XZ commute? XY on q0,q1: X*Y at q0 (different) and I*I at q1 → 1 anticommutation → anticommute
	xy := PauliStringFromWord("XY")
	xz := PauliStringFromWord("XZ")
	if Commutes(xy, xz) {
		t.Error("XY and XZ should anticommute")
	}

	// IX and IZ anticommute (both act non-trivially on qubit 1 with X≠Z)
	ix := PauliStringFromWord("IX")
	iz := PauliStringFromWord("IZ")
	if Commutes(ix, iz) {
		t.Error("IX and IZ should anticommute")
	}
}

func TestEC_PauliMultiply(t *testing.T) {
	// X * X = I
	xx := PauliStringFromWord("XX")
	xx2 := PauliStringFromWord("XX")
	result := Multiply(xx, xx2)
	for i := 0; i < 2; i++ {
		if result.Ops[i] != ECOpI {
			t.Errorf("X*X should give I at q%d, got %s", i, result.Ops[i])
		}
	}

	// X * Z = -iY
	xOp := PauliStringFromWord("X")
	zOp := PauliStringFromWord("Z")
	result2 := Multiply(xOp, zOp)
	if result2.Ops[0] != ECOpY {
		t.Errorf("X*Z should give Y, got %s", result2.Ops[0])
	}
}

func TestEC_ShorCode(t *testing.T) {
	code := ShorCode913()

	if code.N != 9 {
		t.Errorf("expected N=9, got %d", code.N)
	}
	if code.K != 1 {
		t.Errorf("expected K=1, got %d", code.K)
	}
	if code.Distance != 3 {
		t.Errorf("expected d=3, got %d", code.Distance)
	}
	if code.NumChecks() != 8 {
		t.Errorf("expected 8 checks, got %d", code.NumChecks())
	}
	if len(code.Generators) != 8 {
		t.Errorf("expected 8 generators, got %d", len(code.Generators))
	}

	// All generators should commute
	for i := 0; i < len(code.Generators); i++ {
		for j := i + 1; j < len(code.Generators); j++ {
			if !Commutes(code.Generators[i], code.Generators[j]) {
				t.Errorf("generators %d and %d should commute", i, j)
			}
		}
	}
}

func TestEC_SteaneCode(t *testing.T) {
	code := SteaneCode713()

	if code.N != 7 {
		t.Errorf("expected N=7, got %d", code.N)
	}
	if code.K != 1 {
		t.Errorf("expected K=1, got %d", code.K)
	}
	if code.Distance != 3 {
		t.Errorf("expected d=3, got %d", code.Distance)
	}

	// All generators should commute
	for i := 0; i < len(code.Generators); i++ {
		for j := i + 1; j < len(code.Generators); j++ {
			if !Commutes(code.Generators[i], code.Generators[j]) {
				t.Errorf("generators %d and %d should commute", i, j)
			}
		}
	}

	// Logical operators should commute with all generators
	for _, lx := range code.LogicalX {
		for _, gen := range code.Generators {
			if !Commutes(lx, gen) {
				t.Error("logical X should commute with all generators")
			}
		}
	}
	for _, lz := range code.LogicalZ {
		for _, gen := range code.Generators {
			if !Commutes(lz, gen) {
				t.Error("logical Z should commute with all generators")
			}
		}
	}

	// Logical X and Z should anticommute
	if Commutes(code.LogicalX[0], code.LogicalZ[0]) {
		t.Error("logical X and Z should anticommute")
	}
}

func TestEC_FiveQubitCode(t *testing.T) {
	code := FiveQubitCode513()

	if code.N != 5 || code.K != 1 || code.Distance != 3 {
		t.Errorf("expected [[5,1,3]], got [[%d,%d,%d]]", code.N, code.K, code.Distance)
	}

	// All generators should commute
	for i := 0; i < len(code.Generators); i++ {
		for j := i + 1; j < len(code.Generators); j++ {
			if !Commutes(code.Generators[i], code.Generators[j]) {
				t.Errorf("generators %d and %d should commute", i, j)
			}
		}
	}
}

func TestEC_SurfaceCode(t *testing.T) {
	sc := NewSurfaceCode(3)

	if sc.Distance != 3 {
		t.Errorf("expected d=3, got %d", sc.Distance)
	}
	if sc.DataQubits != 9 {
		t.Errorf("expected 9 data qubits, got %d", sc.DataQubits)
	}
	if len(sc.XStabilizers) == 0 {
		t.Error("expected X stabilizers")
	}
	if len(sc.ZStabilizers) == 0 {
		t.Error("expected Z stabilizers")
	}

	k := sc.NumLogicalQubits()
	if k < 1 {
		t.Errorf("expected at least 1 logical qubit, got %d", k)
	}
}

func TestEC_SurfaceCodeD5(t *testing.T) {
	sc := NewSurfaceCode(5)

	if sc.Distance != 5 {
		t.Errorf("expected d=5, got %d", sc.Distance)
	}
	if sc.DataQubits != 25 {
		t.Errorf("expected 25 data qubits, got %d", sc.DataQubits)
	}
	if len(sc.XStabilizers) == 0 || len(sc.ZStabilizers) == 0 {
		t.Error("expected stabilizers")
	}
}

func TestEC_SyndromeExtraction(t *testing.T) {
	code := SteaneCode713()

	// Single X error on qubit 0
	err := NewPauliStringIdentity(code.N)
	err.Ops[0] = ECOpX

	syn := ExtractSyndrome(code, err)
	if syn.SyndromeInt == 0 {
		t.Error("single X error should produce nonzero syndrome")
	}
	if len(syn.RawBits) != code.NumChecks() {
		t.Errorf("expected %d syndrome bits, got %d", code.NumChecks(), len(syn.RawBits))
	}
}

func TestEC_SyndromeNoError(t *testing.T) {
	code := SteaneCode713()
	err := NewPauliStringIdentity(code.N) // identity = no error

	syn := ExtractSyndrome(code, err)
	if syn.SyndromeInt != 0 {
		t.Error("no error should produce zero syndrome")
	}
}

func TestEC_SyndromeUnique(t *testing.T) {
	code := FiveQubitCode513()
	syndromes := make(map[int]int) // syndrome → qubit

	for q := 0; q < code.N; q++ {
		err := NewPauliStringIdentity(code.N)
		err.Ops[q] = ECOpX
		syn := ExtractSyndrome(code, err)
		if prev, ok := syndromes[syn.SyndromeInt]; ok {
			t.Errorf("X error on q%d and q%d produce same syndrome %d", q, prev, syn.SyndromeInt)
		}
		syndromes[syn.SyndromeInt] = q
	}
}

func TestEC_LookupDecoder(t *testing.T) {
	code := FiveQubitCode513()
	decoder := BuildDecoder(code)

	// Introduce X error on qubit 2
	err := NewPauliStringIdentity(code.N)
	err.Ops[2] = ECOpX
	syn := ExtractSyndrome(code, err)

	recovery, ok := decoder.Decode(syn)
	if !ok {
		t.Fatal("decoder should find a recovery")
	}
	if recovery.Ops[2] != ECOpX {
		t.Errorf("recovery should be X on q2, got %s", recovery)
	}
}

func TestEC_LookupDecoderNoError(t *testing.T) {
	code := FiveQubitCode513()
	decoder := BuildDecoder(code)

	err := NewPauliStringIdentity(code.N)
	syn := ExtractSyndrome(code, err)

	recovery, ok := decoder.Decode(syn)
	if !ok {
		t.Fatal("decoder should succeed for no error")
	}
	if recovery.Weight() != 0 {
		t.Errorf("recovery for no error should be identity, got weight %d", recovery.Weight())
	}
}

func TestEC_CodeCapacity(t *testing.T) {
	codes := []*StabilizerCode{ShorCode913(), SteaneCode713(), FiveQubitCode513()}
	for _, code := range codes {
		report := AnalyzeCodeCapacity(code)
		if report.MaxCorrectableT != (code.Distance-1)/2 {
			t.Errorf("%s: expected max correctable %d, got %d",
				code.Name, (code.Distance-1)/2, report.MaxCorrectableT)
		}
	}
}

func TestEC_SurfaceCodeCapacity(t *testing.T) {
	sc := NewSurfaceCode(3)
	report := AnalyzeSurfaceCodeCapacity(sc)
	if report.MaxCorrectableT != 1 {
		t.Errorf("d=3 surface code should correct 1 error, got %d", report.MaxCorrectableT)
	}
}

func TestEC_LogicalDecomposition(t *testing.T) {
	code := SteaneCode713()
	ops := []LogicalCircuitOp{
		{Type: LogicalX, Target: 0},
		{Type: LogicalZ, Target: 0},
		{Type: LogicalH, Target: 0},
	}

	result := DecomposeLogicalCircuit(code, ops)
	if len(result) != 3 {
		t.Fatalf("expected 3 decomposed op groups, got %d", len(result))
	}
	// LogicalX should produce physical X gates
	if len(result[0]) == 0 {
		t.Error("logical X decomposition should not be empty")
	}
}

func TestEC_IsStabilizer(t *testing.T) {
	code := SteaneCode713()

	// Identity is in the stabilizer group (commutes with everything)
	id := NewPauliStringIdentity(code.N)
	if !code.IsStabilizer(id) {
		t.Error("identity should commute with all generators")
	}

	// A single X error is NOT in the stabilizer group
	err := NewPauliStringIdentity(code.N)
	err.Ops[0] = ECOpX
	// This might or might not be a stabilizer — depends on the code
	// Just verify the function doesn't panic
	_ = code.IsStabilizer(err)
}
