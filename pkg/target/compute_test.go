package target

import (
	"strings"
	"testing"
)

func mustTarget(t *testing.T, name string) *ComputeTarget {
	t.Helper()
	ct := LookupComputeTarget(name)
	if ct == nil {
		t.Fatalf("compute target %q not registered", name)
	}
	return ct
}

func TestComputeTargetsRegistered(t *testing.T) {
	cts := ComputeTargets()
	if len(cts) != 6 {
		t.Fatalf("expected 6 compute targets, got %d", len(cts))
	}
	want := []string{"cpu", "gpu-experimental", "npu-experimental", "quantum-experimental", "simd", "wasm32-wasi"}
	for i, ct := range cts {
		if got := ct.Name; got != want[i] {
			t.Fatalf("target %d = %q, want %q (catalog not sorted deterministically)", i, got, want[i])
		}
		if ct.Capabilities.Len() == 0 {
			t.Errorf("%s: empty capability set", ct.Name)
		}
		if len(ct.KIRClasses.List()) == 0 {
			t.Errorf("%s: empty KIR class set", ct.Name)
		}
	}
	if got := mustTarget(t, "cpu").Maturity; got != MaturityImplemented {
		t.Errorf("cpu maturity = %v, want implemented", got)
	}
	if got := mustTarget(t, "simd").Maturity; got != MaturityImplemented {
		t.Errorf("simd maturity = %v, want implemented", got)
	}
	if got := mustTarget(t, "wasm32-wasi").Maturity; got != MaturityExperimental {
		t.Errorf("wasm32-wasi maturity = %v, want experimental", got)
	}
	if got := mustTarget(t, "gpu-experimental").Maturity; got != MaturityExperimental {
		t.Errorf("gpu maturity = %v, want experimental", got)
	}
	if got := mustTarget(t, "npu-experimental").Maturity; got != MaturityExperimental {
		t.Errorf("npu maturity = %v, want experimental", got)
	}
	if got := mustTarget(t, "quantum-experimental").Maturity; got != MaturityResearch {
		t.Errorf("quantum maturity = %v, want research", got)
	}
}

func TestCapabilitySetSemantics(t *testing.T) {
	s := NewCapabilitySet(CapIO, CapIntArithmetic)
	if !s.Has(CapIO) || s.Has(CapMatrixOps) {
		t.Error("Has semantics wrong")
	}
	if !s.SupportsAll(CapIO, CapIntArithmetic) {
		t.Error("SupportsAll should pass for present caps")
	}
	if s.SupportsAll(CapIO, CapMatrixOps) {
		t.Error("SupportsAll should fail on absent cap")
	}
	if got := s.Missing(CapIO, CapTensorOps, CapMatrixOps); len(got) != 2 {
		t.Errorf("Missing = %v, want 2", got)
	}
	u := s.Union(NewCapabilitySet(CapTensorOps))
	if !u.SupportsAll(CapIO, CapIntArithmetic, CapTensorOps) {
		t.Error("Union lost members")
	}
	if got := s.List()[0]; got != "int_arithmetic" {
		t.Errorf("List first = %q, want deterministic sort", got)
	}
}

func TestRequiredCapabilityMapping(t *testing.T) {
	cases := []struct {
		class KIRClass
		cap   Capability
		ok    bool
	}{
		{KIRPrint, CapIO, true},
		{KIRImport, CapIO, true},
		{KIRCImport, CapIO, true},
		{KIRFunc, CapFunctionCalls, true},
		{KIRKernel, CapKernelLaunch, true},
		{KIRGlobalID, CapKernelLaunch, true},
		{KIRMeasure, CapQuantumMeasurement, true},
		{KIRAlloc, CapAlloca, true},
		{KIRFree, CapAlloca, true},
		{KIRLet, "", false},
		{KIRIf, "", false},
	}
	for _, tc := range cases {
		cap, ok := RequiredCapabilityFor(tc.class)
		if ok != tc.ok || cap != tc.cap {
			t.Errorf("RequiredCapabilityFor(%s) = (%s,%v), want (%s,%v)", tc.class, cap, ok, tc.cap, tc.ok)
		}
	}
}

func TestClassifyKIRLine(t *testing.T) {
	cases := []struct {
		line  string
		class KIRClass
	}{
		{"func main (params: ) line: 1", KIRFunc},
		{"kernel blas (params: ) line: 2", KIRKernel},
		{"  print 42 line: 3", KIRPrint},
		{"let a = (array 1 2 3) line: 4", KIRLet},
		{"global_id 0 line: 5", KIRGlobalID},
		{"(call measure q0 0) line: 6", KIRMeasure},
		{"  alloc i32 16 line: 7", KIRAlloc},
		{"x line: 8", KIRExprStmt},
		{"KIR v1", KIRUnknown},
		{"source: test.kark", KIRUnknown},
		{"", KIRUnknown},
	}
	for _, tc := range cases {
		if got := ClassifyKIRLine(tc.line); got != tc.class {
			t.Errorf("ClassifyKIRLine(%q) = %s, want %s", tc.line, got, tc.class)
		}
	}
}

func kirProgram(lines ...string) []string {
	return append([]string{"KIR v1", "source: test.kark"}, lines...)
}

func TestLoweringScenarios(t *testing.T) {
	printProg := kirProgram(
		"func main (params: ) line: 1",
		"  print 42 line: 2",
		"  return line: 3",
	)
	kernelProg := kirProgram(
		"kernel add_kernel (params: ) line: 1",
		"  let a = (array 1 2 3) line: 2",
		"  global_id 0 line: 3",
		"  alloc i32 16 line: 4",
		"  return line: 5",
	)
	measureProg := kirProgram(
		"let q0 = (array 1 0) line: 1",
		"(call measure q0 0) line: 2",
	)

	t.Run("cpu accepts host IO program", func(t *testing.T) {
		plan, err := mustTarget(t, "cpu").Lower(printProg)
		if err != nil {
			t.Fatalf("cpu should lower print program: %v", err)
		}
		if !plan.Required.Has(CapIO) {
			t.Error("plan must require CapIO for a print program")
		}
		if len(plan.Nodes) != 3 {
			t.Errorf("plan nodes = %d, want 3", len(plan.Nodes))
		}
	})

	t.Run("gpu lowers kernel program", func(t *testing.T) {
		plan, err := mustTarget(t, "gpu-experimental").Lower(kernelProg)
		if err != nil {
			t.Fatalf("gpu should lower kernel program: %v", err)
		}
		if !plan.Required.Has(CapKernelLaunch) {
			t.Error("kernel program must require CapKernelLaunch")
		}
		if !contains(plan.SupportedOps, "matmul") {
			t.Errorf("gpu native tensor ops should include matmul, got %v", plan.SupportedOps)
		}
	})

	t.Run("gpu rejects host IO", func(t *testing.T) {
		_, err := mustTarget(t, "gpu-experimental").Lower(printProg)
		ue, ok := err.(*UnsupportedError)
		if !ok {
			t.Fatalf("expected *UnsupportedError, got %T", err)
		}
		if !strings.Contains(ue.Error(), "gpu-experimental") || ue.Class != KIRPrint {
			t.Errorf("bad diagnostic: %v", ue)
		}
	})

	t.Run("npu rejects function calls", func(t *testing.T) {
		_, err := mustTarget(t, "npu-experimental").Lower(printProg)
		ue, ok := err.(*UnsupportedError)
		if !ok {
			t.Fatalf("expected *UnsupportedError, got %T", err)
		}
		if ue.Class != KIRFunc || !strings.Contains(ue.Error(), "npu-experimental") {
			t.Errorf("bad diagnostic: %v", ue)
		}
		want := "error[K124] target 'npu-experimental' cannot lower 'func main (params: ) line: 1' (func); target does not provide function calls"
		if got := ue.Error(); got != want {
			t.Errorf("diagnostic not deterministic\n got: %s\nwant: %s", got, want)
		}
	})

	t.Run("npu accepts func-free tensor surface", func(t *testing.T) {
		prog := kirProgram(
			"let a = (array 1 2 3) line: 1",
			"let b = (array 4 5 6) line: 2",
			"let s = (call reduce_sum a) line: 3",
		)
		plan, err := mustTarget(t, "npu-experimental").Lower(prog)
		if err != nil {
			t.Fatalf("npu should lower func-free program: %v", err)
		}
		if len(plan.Nodes) != 3 {
			t.Errorf("plan nodes = %d, want 3", len(plan.Nodes))
		}
		if !contains(plan.SupportedOps, "reduce_sum") {
			t.Errorf("npu should expose reduce_sum, got %v", plan.SupportedOps)
		}
	})

	t.Run("quantum accepts measurement", func(t *testing.T) {
		plan, err := mustTarget(t, "quantum-experimental").Lower(measureProg)
		if err != nil {
			t.Fatalf("quantum should lower measurement program: %v", err)
		}
		if !plan.Required.Has(CapQuantumMeasurement) {
			t.Error("measurement program must require quantum_measurement")
		}
		if len(plan.SupportedOps) != 0 {
			t.Errorf("quantum must not claim tensor ops, got %v", plan.SupportedOps)
		}
	})

	t.Run("quantum rejects host IO", func(t *testing.T) {
		_, err := mustTarget(t, "quantum-experimental").Lower(printProg)
		ue, ok := err.(*UnsupportedError)
		if !ok || ue.Class != KIRFunc {
			t.Fatalf("expected function-call boundary UnsupportedError, got %v", err)
		}
	})
}

func TestLoweringDeterminism(t *testing.T) {
	prog := kirProgram("func main (params: ) line: 1", "  print 42 line: 2")
	a, errA := mustTarget(t, "gpu-experimental").Lower(prog)
	b, errB := mustTarget(t, "gpu-experimental").Lower(prog)
	if errA.Error() != errB.Error() {
		t.Error("unsupported diagnostics differ across runs")
	}
	if (a != nil) != (b != nil) {
		t.Error("plan/catalog presence differs across runs")
	}
}

func TestRegistrySemantics(t *testing.T) {
	r := NewRegistry()
	host := Host()
	first := &ComputeTarget{Name: "cpu", Family: "cpu", Triple: &host, Capabilities: NewCapabilitySet(), KIRClasses: NewKIRClassSet()}
	if err := r.Register(first); err != nil {
		t.Fatalf("first register failed: %v", err)
	}
	dup := &ComputeTarget{Name: "cpu", Family: "cpu", Triple: &host, Capabilities: NewCapabilitySet(), KIRClasses: NewKIRClassSet()}
	if err := r.Register(dup); err == nil {
		t.Error("registering a duplicate name must fail")
	}
	if err := r.Register(&ComputeTarget{Name: "", Capabilities: NewCapabilitySet(), KIRClasses: NewKIRClassSet()}); err == nil {
		t.Error("registering an empty name must fail")
	}
	ct := &ComputeTarget{Name: "custom", Family: "custom", Capabilities: NewCapabilitySet(CapIO), KIRClasses: NewKIRClassSet(KIRPrint)}
	if err := r.Register(ct); err != nil {
		t.Fatalf("register failed: %v", err)
	}
	if r.Lookup("custom") != ct {
		t.Error("Lookup did not return the registered target")
	}
	if r.Lookup("missing") != nil {
		t.Error("Lookup(missing) must be nil")
	}
	if got := r.ByFamily("custom"); len(got) != 1 || got[0].Name != "custom" {
		t.Errorf("ByFamily(custom) = %v", got)
	}
}

func contains(xs []string, want string) bool {
	for _, x := range xs {
		if x == want {
			return true
		}
	}
	return false
}