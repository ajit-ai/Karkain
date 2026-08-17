package codegen

import (
	"karkain/pkg/parser"
	"strings"
	"testing"
)

func TestOpenPulse_DefcalGeneration(t *testing.T) {
	circuit := &parser.CircuitDecl{
		Name: "RyCircuit",
		Params: []parser.CircuitParam{
			{Name: "q", Type: &parser.QubitType{Size: 1}},
		},
		ReturnType: &parser.BitType{Size: 1},
		Body: []parser.Node{
			&parser.QPUOpExpr{
				Op:    "ry",
				Args:  []parser.Node{&parser.Identifier{Name: "q"}},
				Angle: &parser.Float64Literal{Value: "1.5708"},
			},
			&parser.QPUOpExpr{
				Op:   "measure",
				Args: []parser.Node{&parser.Identifier{Name: "q"}},
			},
		},
	}

	calibrations := []PulseGateCalibration{
		{
			GateName:  "ry",
			Qubit:     0,
			Amplitude: 0.3,
			Duration:  160,
			Sigma:     40.0,
			Envelope:  EnvelopeDrag,
			DragCoeff: 0.2,
		},
		{
			GateName:  "x",
			Qubit:     0,
			Amplitude: 0.31,
			Duration:  160,
			Sigma:     40.0,
			Envelope:  EnvelopeDrag,
			DragCoeff: 0.18,
		},
	}

	gen := NewOpenPulseGenerator(DefaultPulseBackend())
	qasm, err := gen.GeneratePulseProgram(circuit, calibrations)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify header
	if !strings.Contains(qasm, "OPENQASM 3.0;") {
		t.Error("expected OPENQASM 3.0 header")
	}
	if !strings.Contains(qasm, "stdpulse.inc") {
		t.Error("expected stdpulse.inc include")
	}

	// Verify frame definitions
	if !strings.Contains(qasm, "frame q0_drive") {
		t.Error("expected q0_drive frame definition")
	}
	if !strings.Contains(qasm, "newframe(hardware通道") {
		t.Error("expected newframe hardware call")
	}

	// Verify defcal blocks
	if !strings.Contains(qasm, "defcal ry(angle[angle]) $0") {
		t.Errorf("expected defcal ry block, got:\n%s", qasm)
	}
	if !strings.Contains(qasm, "defcal x $0") {
		t.Errorf("expected defcal x block, got:\n%s", qasm)
	}

	// Verify pulse play calls with envelope
	if !strings.Contains(qasm, "play(") {
		t.Error("expected play() call in defcal")
	}
	if !strings.Contains(qasm, "drag(amp=") {
		t.Errorf("expected drag envelope in defcal, got:\n%s", qasm)
	}
	if !strings.Contains(qasm, "duration=160 dt") {
		t.Errorf("expected duration=160 dt, got:\n%s", qasm)
	}
	if !strings.Contains(qasm, "sigma=40 dt") {
		t.Errorf("expected sigma=40 dt, got:\n%s", qasm)
	}
	if !strings.Contains(qasm, "beta=0.2") {
		t.Errorf("expected drag beta=0.2, got:\n%s", qasm)
	}

	// Verify circuit body
	if !strings.Contains(qasm, "ry(1.5708) $0") {
		t.Errorf("expected ry gate in circuit body, got:\n%s", qasm)
	}
	if !strings.Contains(qasm, "measure $0 -> c[0]") {
		t.Errorf("expected measure in circuit body, got:\n%s", qasm)
	}
}

func TestOpenPulse_MultipleQubits(t *testing.T) {
	circuit := &parser.CircuitDecl{
		Name: "BellPair",
		Params: []parser.CircuitParam{
			{Name: "q", Type: &parser.QubitType{Size: 2}},
		},
		ReturnType: &parser.BitType{Size: 2},
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
			&parser.QPUOpExpr{
				Op: "measure",
				Args: []parser.Node{
					&parser.QubitIndexExpr{Qubit: &parser.Identifier{Name: "q"}, Index: &parser.IntLiteral{Value: "0"}},
					&parser.QubitIndexExpr{Qubit: &parser.Identifier{Name: "q"}, Index: &parser.IntLiteral{Value: "1"}},
				},
			},
		},
	}

	gen := NewOpenPulseGenerator(DefaultPulseBackend())
	qasm, err := gen.GeneratePulseProgram(circuit, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify frames for both qubits
	if !strings.Contains(qasm, "frame q0_drive") {
		t.Error("expected q0_drive frame")
	}
	if !strings.Contains(qasm, "frame q1_drive") {
		t.Error("expected q1_drive frame")
	}
	if !strings.Contains(qasm, "frame q0_measure") {
		t.Error("expected q0_measure frame")
	}

	// Verify gates reference correct qubit indices
	if !strings.Contains(qasm, "h $0") {
		t.Errorf("expected h $0, got:\n%s", qasm)
	}
	if !strings.Contains(qasm, "cx $0, $1") {
		t.Errorf("expected cx $0, $1, got:\n%s", qasm)
	}
	if !strings.Contains(qasm, "measure $0 -> c[0]") {
		t.Error("expected measure $0 -> c[0]")
	}
	if !strings.Contains(qasm, "measure $1 -> c[1]") {
		t.Error("expected measure $1 -> c[1]")
	}
}

func TestOpenPulse_GaussianEnvelope(t *testing.T) {
	circuit := &parser.CircuitDecl{
		Name: "XGate",
		Params: []parser.CircuitParam{
			{Name: "q", Type: &parser.QubitType{Size: 1}},
		},
		Body: []parser.Node{
			&parser.QPUOpExpr{
				Op:   "x",
				Args: []parser.Node{&parser.Identifier{Name: "q"}},
			},
		},
	}

	calibrations := []PulseGateCalibration{
		{
			GateName:  "x",
			Qubit:     0,
			Amplitude: 0.5,
			Duration:  160,
			Sigma:     40.0,
			Envelope:  EnvelopeGaussian,
		},
	}

	gen := NewOpenPulseGenerator(DefaultPulseBackend())
	qasm, err := gen.GeneratePulseProgram(circuit, calibrations)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(qasm, "defcal x $0") {
		t.Errorf("expected defcal x block, got:\n%s", qasm)
	}
	if !strings.Contains(qasm, "gaussian(amp=") {
		t.Errorf("expected gaussian envelope, got:\n%s", qasm)
	}
}

func TestOpenPulse_PulseSequence(t *testing.T) {
	circuit := &parser.CircuitDecl{
		Name: "SeqCircuit",
		Params: []parser.CircuitParam{
			{Name: "q", Type: &parser.QubitType{Size: 1}},
		},
		Body: []parser.Node{
			&parser.QPUOpExpr{
				Op:   "x",
				Args: []parser.Node{&parser.Identifier{Name: "q"}},
			},
			&parser.QPUOpExpr{
				Op:    "ry",
				Args:  []parser.Node{&parser.Identifier{Name: "q"}},
				Angle: &parser.Float64Literal{Value: "0.5"},
			},
		},
	}

	calibrations := []PulseGateCalibration{
		{GateName: "x", Qubit: 0, Amplitude: 0.3, Duration: 160, Sigma: 40, Envelope: EnvelopeDrag, DragCoeff: 0.2},
		{GateName: "ry", Qubit: 0, Amplitude: 0.3, Duration: 160, Sigma: 40, Envelope: EnvelopeDrag, DragCoeff: 0.2},
	}

	seq := GeneratePulseSequence(circuit, calibrations)

	if len(seq.Operations) != 2 {
		t.Fatalf("expected 2 pulse operations, got %d", len(seq.Operations))
	}

	// First op should be play for X gate
	if seq.Operations[0].Type != "play" {
		t.Errorf("expected play operation, got %s", seq.Operations[0].Type)
	}
	if seq.Operations[0].Frame != "q0_drive" {
		t.Errorf("expected q0_drive frame, got %s", seq.Operations[0].Frame)
	}
	if !strings.Contains(seq.Operations[0].Waveform, "drag") {
		t.Errorf("expected drag waveform, got %s", seq.Operations[0].Waveform)
	}
}

func TestOpenPulse_CalibrationTable(t *testing.T) {
	calibrations := []PulseGateCalibration{
		{GateName: "x", Qubit: 0, Amplitude: 0.31, Duration: 160, Sigma: 40, Envelope: EnvelopeDrag, DragCoeff: 0.2},
		{GateName: "rx", Qubit: 0, Amplitude: 0.30, Duration: 160, Sigma: 40, Envelope: EnvelopeDrag, DragCoeff: 0.18},
		{GateName: "cx", Qubit: 0, Amplitude: 0.45, Duration: 300, Sigma: 60, Envelope: EnvelopeGaussian},
	}

	ct := NewCalibrationTableGenerator()
	table := ct.GenerateCalibrationTable(calibrations)

	if !strings.Contains(table, "calibration_table") {
		t.Error("expected calibration_table header")
	}
	if !strings.Contains(table, "gate: x, qubit: 0") {
		t.Error("expected x gate calibration")
	}
	if !strings.Contains(table, "amplitude: 0.310000") {
		t.Errorf("expected amplitude 0.310000, got:\n%s", table)
	}
	if !strings.Contains(table, "duration: 160 dt") {
		t.Errorf("expected duration 160 dt, got:\n%s", table)
	}
	if !strings.Contains(table, "drag_beta: 0.2") {
		t.Errorf("expected drag_beta: 0.2, got:\n%s", table)
	}
	if !strings.Contains(table, "gate: cx") {
		t.Error("expected cx gate calibration")
	}
}

func TestOpenPulse_StandardCalibrationSet(t *testing.T) {
	cals := StandardCalibrationSet()

	if len(cals) < 10 {
		t.Errorf("expected at least 10 calibrations, got %d", len(cals))
	}

	// Verify we have calibrations for both qubits
	hasQ0 := false
	hasQ1 := false
	for _, cal := range cals {
		if cal.Qubit == 0 {
			hasQ0 = true
		}
		if cal.Qubit == 1 {
			hasQ1 = true
		}
	}
	if !hasQ0 {
		t.Error("expected qubit 0 calibrations")
	}
	if !hasQ1 {
		t.Error("expected qubit 1 calibrations")
	}

	// Verify all have valid envelopes
	for _, cal := range cals {
		if cal.Duration <= 0 {
			t.Errorf("gate %s qubit %d: duration should be positive", cal.GateName, cal.Qubit)
		}
		if cal.Amplitude <= 0 {
			t.Errorf("gate %s qubit %d: amplitude should be positive", cal.GateName, cal.Qubit)
		}
	}
}
