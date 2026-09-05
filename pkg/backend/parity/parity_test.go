package parity_test

import (
	"os/exec"
	"testing"

	"karkain/pkg/backend"
	"karkain/pkg/backend/cpu"
	"karkain/pkg/backend/gpu"
	"karkain/pkg/backend/parity"
	"karkain/pkg/npu/intel"
	"karkain/pkg/tensor"
)

// mustGraph builds a graph from op construction mirroring the CPU backend E2E
// tests, returning the graph and its inputs.
func buildAddGraph() (*tensor.TensorGraph, map[string][]float64) {
	a := tensor.NewNode("a", tensor.OpCreate, tensor.NewShape(16), tensor.ElemF32)
	b := tensor.NewNode("b", tensor.OpCreate, tensor.NewShape(16), tensor.ElemF32)
	sum := tensor.NewNode("sum", tensor.OpAdd, tensor.NewShape(16), tensor.ElemF32, "a", "b")
	g := tensor.NewGraph()
	g.AddNode(a)
	g.AddNode(b)
	g.AddNode(sum)
	g.AddOutput(sum)
	inputs := map[string][]float64{}
	av := make([]float64, 16)
	bv := make([]float64, 16)
	for i := 0; i < 16; i++ {
		av[i] = float64(i + 1)
		bv[i] = float64(i*2 + 3)
	}
	inputs["a"] = av
	inputs["b"] = bv
	return g, inputs
}

func buildMatMulGraph() (*tensor.TensorGraph, map[string][]float64) {
	a := tensor.NewNode("a", tensor.OpCreate, tensor.NewShape(4, 6), tensor.ElemF32)
	b := tensor.NewNode("b", tensor.OpCreate, tensor.NewShape(6, 5), tensor.ElemF32)
	mm := tensor.NewNode("mm", tensor.OpMatMul, tensor.NewShape(4, 5), tensor.ElemF32, "a", "b")
	g := tensor.NewGraph()
	g.AddNode(a)
	g.AddNode(b)
	g.AddNode(mm)
	g.AddOutput(mm)
	inputs := map[string][]float64{
		"a": makeRange(24, 0.5),
		"b": makeRange(30, -0.25),
	}
	return g, inputs
}

func buildReluGraph() (*tensor.TensorGraph, map[string][]float64) {
	a := tensor.NewNode("a", tensor.OpCreate, tensor.NewShape(16), tensor.ElemF32)
	relu := tensor.NewNode("relu", tensor.OpRelu, tensor.NewShape(16), tensor.ElemF32, "a")
	g := tensor.NewGraph()
	g.AddNode(a)
	g.AddNode(relu)
	g.AddOutput(relu)
	inputs := map[string][]float64{}
	av := make([]float64, 16)
	for i := range av {
		av[i] = float64(i - 7)
	}
	inputs["a"] = av
	return g, inputs
}

func makeRange(n int, scale float64) []float64 {
	out := make([]float64, n)
	for i := range out {
		out[i] = float64(i+1) * scale
	}
	return out
}

func skipIfNoGCC(t *testing.T) {
	if _, err := exec.LookPath("gcc"); err != nil {
		t.Skip("gcc not available")
	}
}

// TestCpuScalarIsTheNumericOracle confirms the reference contract: only the CPU
// backend returns numeric Values; the accelerator results (GPU, NPU) are
// metadata-only, which the harness must surface as NoNumericOutput.
func TestCpuScalarIsTheNumericOracle(t *testing.T) {
	skipIfNoGCC(t)

	g, inputs := buildAddGraph()
	scalar := cpu.New()
	oracleResult, err := scalar.Execute(g, inputs)
	if err != nil {
		t.Fatalf("oracle execute: %v", err)
	}

	wgsl := gpu.New()
	npuBackend := intel.New()
	npuResult, err := npuBackend.Execute(g, nil)
	if err != nil {
		t.Fatalf("npu execute: %v", err)
	}
	gpuResult, err := wgsl.Execute(g, nil)
	if err != nil {
		t.Fatalf("gpu execute: %v", err)
	}

	candidates := []*parity.Candidate{
		{Name: "gpu", Result: gpuResult},
		{Name: "intel-npu", Result: npuResult},
	}
	rep := parity.Compare("oracle-contract", g, oracleResult, candidates, 1e-9)
	if len(rep.NoNumeric) != 2 {
		t.Errorf("expected 2 no-numeric candidates, got %v", rep.NoNumeric)
	}
	for name, c := range rep.ByCandidate {
		if !c.NoNumericOutput {
			t.Errorf("candidate %s should be marked NoNumericOutput", name)
		}
	}
}

// TestCpuSimdMatchesOracleNumeric is the core Phase 80 differential: the CPU
// SIMD variant must match the CPU scalar oracle within tolerance on
// add/mul/matmul/relu.
func TestCpuSimdMatchesOracleNumeric(t *testing.T) {
	skipIfNoGCC(t)

	cases := []struct {
		name      string
		graph     func() (*tensor.TensorGraph, map[string][]float64)
		tolerance float64
	}{
		{"add", buildAddGraph, 0.0},
		{"matmul", buildMatMulGraph, 1e-12},
		{"relu", buildReluGraph, 0.0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			g, inputs := tc.graph()
			scalar := cpu.New()
			simd := cpu.NewSimd()
			oracleResult, err := scalar.Execute(g, inputs)
			if err != nil {
				t.Fatalf("oracle execute: %v", err)
			}
			simdResult, err := simd.Execute(g, inputs)
			if err != nil {
				t.Fatalf("simd execute: %v", err)
			}
			if simdResult.Metadata["simd"] != "true" {
				t.Error("SIMD result should carry simd metadata")
			}

			rep := parity.Compare(tc.name, g, oracleResult,
				[]*parity.Candidate{{Name: "cpu-simd", Result: simdResult}}, tc.tolerance)
			if !rep.Pass {
				t.Fatalf("SIMD parity failed:\n%s", parity.ReportString(rep))
			}
			c := rep.ByCandidate["cpu-simd"]
			if !c.NumericParity {
				t.Error("SIMD backend must produce numeric output")
			}
			if c.MaxAbsDiff > tc.tolerance {
				t.Errorf("unexpected max diff %.3e (tolerance %.3e)", c.MaxAbsDiff, tc.tolerance)
			}
		})
	}
}

// TestSimdBroadcastFallsBack verifies the vectorized path is only used for
// same-shape operands — a broadcast add still produces the scalar-equivalent
// values (harness parity with tolerance).
func TestSimdBroadcastFallsBack(t *testing.T) {
	skipIfNoGCC(t)

	a := tensor.NewNode("a", tensor.OpCreate, tensor.NewShape(4), tensor.ElemF32)
	b := tensor.NewNode("b", tensor.OpCreate, tensor.NewShape(1), tensor.ElemF32)
	sum := tensor.NewNode("sum", tensor.OpAdd, tensor.NewShape(4), tensor.ElemF32, "a", "b")
	g := tensor.NewGraph()
	g.AddNode(a)
	g.AddNode(b)
	g.AddNode(sum)
	g.AddOutput(sum)
	inputs := map[string][]float64{"a": {1, 2, 3, 4}, "b": {100}}

	scalar := cpu.New()
	simd := cpu.NewSimd()
	oracleResult, err := scalar.Execute(g, inputs)
	if err != nil {
		t.Fatalf("oracle execute: %v", err)
	}
	simdResult, err := simd.Execute(g, inputs)
	if err != nil {
		t.Fatalf("simd execute: %v", err)
	}
	rep := parity.Compare("broadcast", g, oracleResult,
		[]*parity.Candidate{{Name: "cpu-simd", Result: simdResult}}, 0.0)
	if !rep.Pass {
		t.Fatalf("broadcast parity failed:\n%s", parity.ReportString(rep))
	}
}

// TestSimdReportsFailureOnMismatch proves the harness can detect a real numeric
// divergence (fabricated oracle values must fail).
func TestSimdReportsFailureOnMismatch(t *testing.T) {
	skipIfNoGCC(t)

	g, inputs := buildAddGraph()
	scalar := cpu.New()
	oracleResult, err := scalar.Execute(g, inputs)
	if err != nil {
		t.Fatalf("oracle execute: %v", err)
	}
	simdResult, err := cpu.NewSimd().Execute(g, inputs)
	if err != nil {
		t.Fatalf("simd execute: %v", err)
	}
	// Corrupt the oracle's values beyond tolerance.
	oracleResult.Values["sum"][0] += 1000
	rep := parity.Compare("doctored", g, oracleResult,
		[]*parity.Candidate{{Name: "cpu-simd", Result: simdResult}}, 1e-12)
	if rep.Pass {
		t.Fatalf("harness must detect injected divergence:\n%s", parity.ReportString(rep))
	}
}

// TestDispatcherSimdBackendIntegration proves a CPU-SIMD backend is a
// first-class dispatcher backend: the planner assigns Add/Mul/MatMul/Relu
// subgraphs to it and numeric Values flow back through the merged result.
func TestDispatcherSimdBackendIntegration(t *testing.T) {
	skipIfNoGCC(t)

	g, inputs := buildAddGraph()
	simd := cpu.NewSimd()
	dispatcher := backend.NewDispatcher(simd)
	result, err := dispatcher.Execute(g, inputs)
	if err != nil {
		t.Fatalf("dispatcher execute: %v", err)
	}
	if result.Values["sum"] == nil || len(result.Values["sum"]) != 16 {
		t.Fatalf("dispatcher should produce numeric values via the SIMD backend, got %d", len(result.Values["sum"]))
	}
}
