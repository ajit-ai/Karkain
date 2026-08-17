package sema

import (
	"math"
	"math/cmplx"
	"testing"
)

func TestQuantumNoise_DensityMatrixInit(t *testing.T) {
	dm := NewDensityMatrix(1)
	if dm.Dim != 2 {
		t.Errorf("expected dim 2, got %d", dm.Dim)
	}
	if dm.NumQ != 1 {
		t.Errorf("expected 1 qubit, got %d", dm.NumQ)
	}
	if len(dm.Data) != 4 {
		t.Errorf("expected 4 entries, got %d", len(dm.Data))
	}

	// Create from |0⟩ state vector
	state := []Complex128{1.0, 0.0}
	dm2 := NewDensityMatrixFromStateVector(state)
	trace := dm2.Trace()
	if math.Abs(real(trace)-1.0) > 1e-10 {
		t.Errorf("trace of |0⟩⟨0| should be 1.0, got %f", real(trace))
	}
	// |0⟩⟨0| should be [[1,0],[0,0]]
	if math.Abs(real(dm2.Data[0])-1.0) > 1e-10 {
		t.Errorf("ρ[0,0] should be 1.0, got %f", real(dm2.Data[0]))
	}
	if math.Abs(real(dm2.Data[1])) > 1e-10 {
		t.Errorf("ρ[0,1] should be 0, got %f", real(dm2.Data[1]))
	}
	if math.Abs(real(dm2.Data[3])) > 1e-10 {
		t.Errorf("ρ[1,1] should be 0, got %f", real(dm2.Data[3]))
	}
}

func TestQuantumNoise_BitFlipChannel(t *testing.T) {
	sim := NewNoisySimulator(1)

	// Apply bit flip with p=1.0 → should flip to |1⟩⟨1|
	channel := BitFlipChannel(1.0)
	sim.State = ApplyNoiseChannel(sim.State, channel, 0, sim.NumQubits)

	// ρ should now be |1⟩⟨1|
	trace := sim.State.Trace()
	if math.Abs(real(trace)-1.0) > 1e-10 {
		t.Errorf("trace should be 1.0 after bit flip, got %f", real(trace))
	}

	if math.Abs(real(sim.State.Data[3])-1.0) > 1e-10 {
		t.Errorf("ρ[1,1] should be 1.0 after full bit flip, got %f", real(sim.State.Data[3]))
	}

	if math.Abs(real(sim.State.Data[0])) > 1e-10 {
		t.Errorf("ρ[0,0] should be 0 after full bit flip, got %f", real(sim.State.Data[0]))
	}
}

func TestQuantumNoise_PhaseFlipChannel(t *testing.T) {
	invSqrt2 := 1.0 / math.Sqrt2
	state := []Complex128{complex(invSqrt2, 0), complex(invSqrt2, 0)}
	dm := NewDensityMatrixFromStateVector(state)

	if math.Abs(real(dm.Data[0])-0.5) > 1e-10 {
		t.Errorf("ρ[0,0] of |+⟩ should be 0.5, got %f", real(dm.Data[0]))
	}

	// Apply phase flip with p=1.0 → should flip to |−⟩⟨−|
	channel := PhaseFlipChannel(1.0)
	dm = ApplyNoiseChannel(dm, channel, 0, dm.NumQ)

	// |−⟩⟨−| = (|0⟩-|1⟩)(⟨0|-⟨1|)/2
	if math.Abs(real(dm.Data[1])-(-0.5)) > 1e-10 {
		t.Errorf("ρ[0,1] after full phase flip should be -0.5, got %f", real(dm.Data[1]))
	}
}

func TestQuantumNoise_DepolarizingChannel(t *testing.T) {
	sim := NewNoisySimulator(1)

	// Apply depolarizing with p=1.0 → should produce maximally mixed state I/2
	channel := DepolarizingChannel(1.0)
	sim.State = ApplyNoiseChannel(sim.State, channel, 0, sim.NumQubits)

	trace := sim.State.Trace()
	if math.Abs(real(trace)-1.0) > 1e-10 {
		t.Errorf("trace should be 1.0 after depolarizing, got %f", real(trace))
	}

	purity := real(sim.State.Purify())
	if math.Abs(purity-0.5) > 1e-10 {
		t.Errorf("purity of maximally mixed state should be 0.5, got %f", purity)
	}

	if math.Abs(real(sim.State.Data[0])-0.5) > 1e-10 {
		t.Errorf("ρ[0,0] of maximally mixed should be 0.5, got %f", real(sim.State.Data[0]))
	}
	if math.Abs(real(sim.State.Data[3])-0.5) > 1e-10 {
		t.Errorf("ρ[1,1] of maximally mixed should be 0.5, got %f", real(sim.State.Data[3]))
	}
}

func TestQuantumNoise_ThermalRelaxation(t *testing.T) {
	channel := ThermalRelaxationChannel(ThermalRelaxationConfig{
		T1:    50.0,
		T2:    30.0,
		TGate: 35.0,
	})

	if len(channel.Operators) != 4 {
		t.Errorf("expected 4 Kraus operators for thermal relaxation, got %d", len(channel.Operators))
	}

	// Start with |1⟩ state
	sim := NewNoisySimulatorFromStateVector([]Complex128{0.0, 1.0})

	sim.State = ApplyNoiseChannel(sim.State, channel, 0, sim.NumQubits)

	trace := sim.State.Trace()
	if math.Abs(real(trace)-1.0) > 1e-10 {
		t.Errorf("trace should be 1.0 after thermal relaxation, got %f", real(trace))
	}

	if real(sim.State.Data[0]) < 0.001 {
		t.Errorf("expected some population transfer to |0⟩, got ρ[0,0]=%f", real(sim.State.Data[0]))
	}
}

func TestQuantumNoise_ApplyGateAndNoise(t *testing.T) {
	sim := NewNoisySimulator(1)

	// Set depolarizing noise with p=0.1
	sim.SetGlobalNoiseModel(DepolarizingChannel(0.1))

	// Apply Hadamard
	sim.ApplyGate(gateMatrices["H"], 0)
	sim.ApplyNoiseAfterGate(0)

	// State should be approximately |+⟩ but with noise
	trace := sim.State.Trace()
	if math.Abs(real(trace)-1.0) > 1e-10 {
		t.Errorf("trace should be 1.0 after gate+noise, got %f", real(trace))
	}

	// Purity should be less than 1 due to noise
	purity := sim.GetPurity()
	if purity > 1.0+1e-10 {
		t.Errorf("purity should be ≤ 1.0, got %f", purity)
	}
	// For p=0.1 depolarizing, purity drops
	if purity > 0.99 {
		t.Errorf("purity should decrease with noise, got %f", purity)
	}
}

func TestQuantumNoise_QubitSpecificNoise(t *testing.T) {
	sim := NewNoisySimulator(2)

	// Set noise only on qubit 0
	sim.SetQubitNoiseModel(0, BitFlipChannel(0.5))

	// Apply X gate to qubit 0 + noise
	sim.ApplyGate(gateMatrices["X"], 0)
	sim.ApplyNoiseAfterGate(0)

	trace := sim.State.Trace()
	if math.Abs(real(trace)-1.0) > 1e-10 {
		t.Errorf("trace should be 1.0, got %f", real(trace))
	}

	// Purity should be reduced
	purity := sim.GetPurity()
	if purity > 0.99 {
		t.Errorf("purity should be reduced by qubit-specific noise, got %f", purity)
	}
}

func TestQuantumNoise_PauliMatrices(t *testing.T) {
	// Verify Pauli matrices are unitary
	if !IsUnitary(PauliIMat[:], 2, 1e-10) {
		t.Error("Pauli I should be unitary")
	}
	if !IsUnitary(PauliXMat[:], 2, 1e-10) {
		t.Error("Pauli X should be unitary")
	}
	if !IsUnitary(PauliYMat[:], 2, 1e-10) {
		t.Error("Pauli Y should be unitary")
	}
	if !IsUnitary(PauliZMat[:], 2, 1e-10) {
		t.Error("Pauli Z should be unitary")
	}

	// Verify H is unitary
	if !IsUnitary(gateMatrices["H"], 2, 1e-10) {
		t.Error("H should be unitary")
	}

	// Verify Rx, Ry, Rz are unitary for various angles
	angles := []float64{0, math.Pi / 4, math.Pi / 2, math.Pi, 2 * math.Pi}
	for _, theta := range angles {
		if !IsUnitary(Rx(theta), 2, 1e-10) {
			t.Errorf("Rx(%.4f) should be unitary", theta)
		}
		if !IsUnitary(Ry(theta), 2, 1e-10) {
			t.Errorf("Ry(%.4f) should be unitary", theta)
		}
		if !IsUnitary(Rz(theta), 2, 1e-10) {
			t.Errorf("Rz(%.4f) should be unitary", theta)
		}
	}
}

func TestQuantumNoise_NoiseModelReport(t *testing.T) {
	gates := []CircuitGateOp{
		{Name: "h", TargetQubit: 0},
		{Name: "cx", TargetQubit: 1, ControlQubit: 0},
		{Name: "x", TargetQubit: 0},
	}

	model := []NoiseChannel{DepolarizingChannel(0.01)}
	qubitNoise := map[int][]NoiseChannel{
		0: {BitFlipChannel(0.005)},
	}

	report := AnalyzeNoiseModel(gates, model, qubitNoise)

	if report.TotalGates != 3 {
		t.Errorf("expected 3 total gates, got %d", report.TotalGates)
	}
	if report.NoisyGates != 3 {
		t.Errorf("expected 3 noisy gates, got %d", report.NoisyGates)
	}
	if len(report.Channels) != 2 {
		t.Errorf("expected 2 unique channels, got %d", len(report.Channels))
	}
}

func TestQuantumNoise_PurityConservation(t *testing.T) {
	// Verify that pure states remain pure through unitary evolution
	state := []Complex128{complex(1/math.Sqrt2, 0), complex(0, 1/math.Sqrt2)}
	sim := NewNoisySimulatorFromStateVector(state)

	// Apply multiple unitary gates (no noise)
	sim.ApplyGate(gateMatrices["H"], 0)
	sim.ApplyGate(Rx(math.Pi/3), 0)
	sim.ApplyGate(gateMatrices["X"], 0)

	// Purity should remain 1.0 (pure state stays pure under unitary evolution)
	purity := sim.GetPurity()
	if math.Abs(purity-1.0) > 1e-10 {
		t.Errorf("purity should be 1.0 for unitary evolution, got %f", purity)
	}
}

func TestQuantumNoise_NormalizeDensityMatrix(t *testing.T) {
	dm := NewDensityMatrix(1)
	dm.Data[0] = 2.0
	dm.Data[3] = 3.0
	NormalizeDensityMatrix(dm)

	trace := dm.Trace()
	if math.Abs(real(trace)-1.0) > 1e-10 {
		t.Errorf("trace should be 1.0 after normalization, got %f", real(trace))
	}

	// ρ[0,0] should be 0.4
	if math.Abs(real(dm.Data[0])-0.4) > 1e-10 {
		t.Errorf("ρ[0,0] should be 0.4, got %f", real(dm.Data[0]))
	}
}

func TestQuantumNoise_CloneDensityMatrix(t *testing.T) {
	dm := NewDensityMatrix(1)
	dm.Data[0] = 0.7
	dm.Data[3] = 0.3

	clone := dm.Clone()

	// Modify original
	dm.Data[0] = 0.5

	// Clone should be unaffected
	if math.Abs(real(clone.Data[0])-0.7) > 1e-10 {
		t.Errorf("clone should not be affected by original modification, got %f", real(clone.Data[0]))
	}
}

func TestQuantumNoise_ExpectationValue(t *testing.T) {
	// |0⟩ state → ⟨Z⟩ = 1
	state := []Complex128{1.0, 0.0}
	dm := NewDensityMatrixFromStateVector(state)

	zExp := dm.ExpectationValue(PauliZMat[:])
	if math.Abs(real(zExp)-1.0) > 1e-10 {
		t.Errorf("⟨0|Z|0⟩ should be 1.0, got %f", real(zExp))
	}

	// |1⟩ state → ⟨Z⟩ = -1
	state1 := []Complex128{0.0, 1.0}
	dm1 := NewDensityMatrixFromStateVector(state1)
	zExp1 := dm1.ExpectationValue(PauliZMat[:])
	if cmplx.Abs(zExp1-(-1.0)) > 1e-10 {
		t.Errorf("⟨1|Z|1⟩ should be -1.0, got %f", real(zExp1))
	}
}
