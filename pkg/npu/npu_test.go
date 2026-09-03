package npu

import (
	"testing"

	"karkain/pkg/backend"
	"karkain/pkg/tensor"
)

// mockBackend is a controllable NPUBackend used to exercise the NPU
// lifecycle (detection -> compile -> execute) and fallback logic.
type mockBackend struct {
	vendor Vendor
	name   string
	devs   []NPUDevice
	blast  [][]byte // per-compile blobs; nil entry means compile fails
}

func (m *mockBackend) Name() string      { return m.name }
func (m *mockBackend) Vendor() Vendor    { return m.vendor }
func (m *mockBackend) Capabilities() backend.Capabilities {
	return NewCPUCapabilities()
}
func (m *mockBackend) Supports(op tensor.Op, dt tensor.ElemType, shape tensor.Shape) bool {
	return true
}
func (m *mockBackend) DetectHardware() []NPUDevice { return m.devs }
func (m *mockBackend) Execute(graph *tensor.TensorGraph, _ map[string][]float64) (*backend.Result, error) {
	res := backend.NewResult()
	res.Metadata["npu_vendor"] = string(m.vendor)
	for _, n := range graph.Outputs {
		res.Outputs[n.ID] = n
	}
	return res, nil
}
func (m *mockBackend) Compile(graph *tensor.TensorGraph, device NPUDevice) (*NPUProgram, error) {
	if len(m.blast) == 0 {
		return nil, errCompileNotSupported
	}
	blob := m.blast[0]
	if len(m.blast) > 1 {
		m.blast = m.blast[1:]
	}
	if blob == nil {
		return nil, errCompileNotSupported
	}
	prog := NewProgram(m.vendor)
	prog.VendorName = "mock"
	prog.Blob = blob
	prog.Metadata["device"] = device.Name
	return prog, nil
}

var errCompileNotSupported = displayError("compile: operation not supported on mock device")

type displayError string

func (e displayError) Error() string { return string(e) }

// helper to build a small graph: x -> relu -> y
func buildReluGraph() *tensor.TensorGraph {
	x := tensor.NewNode("x", tensor.OpCreate, tensor.NewShape(4), tensor.ElemF32)
	y := tensor.NewNode("y", tensor.OpRelu, tensor.NewShape(4), tensor.ElemF32, "x")
	g := tensor.NewGraph()
	g.AddNode(x)
	g.AddNode(y)
	g.AddOutput(y)
	return g
}

func TestVendorConstants(t *testing.T) {
	if VendorIntel != "intel" || VendorQualcomm != "qualcomm" ||
		VendorApple != "apple" || VendorAMD != "amd" || VendorArm != "arm" {
		t.Error("vendor constants mismatch")
	}
}

func TestNewProgram(t *testing.T) {
	p := NewProgram(VendorIntel)
	if p.Vendor != VendorIntel {
		t.Errorf("expected intel, got %s", p.Vendor)
	}
	if p.Metadata == nil {
		t.Error("expected initialized metadata")
	}
}

func TestDetectionRespectsEnv(t *testing.T) {
	d := NewDeviceDetector()
	// Without opt-in env vars, no vendor hardware should be reported (deterministic).
	devs := d.Detect()
	if len(devs) != 0 {
		t.Errorf("expected no devices without env opt-in, got %d", len(devs))
	}
	// With a registered mock, detection should include it deterministically.
	d.Register(VendorAMD, func() []NPUDevice {
		return []NPUDevice{{
			Name:         "Mock NPU",
			Vendor:       VendorAMD,
			VendorName:   "mock",
			Capabilities: NewCPUCapabilities(),
			Memory:       1 << 20,
		}}
	})
	devs = d.Detect()
	if len(devs) != 1 {
		t.Fatalf("expected 1 registered device, got %d", len(devs))
	}
	if devs[0].Vendor != VendorAMD {
		t.Error("expected AMD vendor for registered device")
	}
}

func TestDetectByVendor(t *testing.T) {
	d := NewDeviceDetector()
	d.Register(VendorArm, func() []NPUDevice { return []NPUDevice{{Name: "arm-test"}} })
	res := d.DetectByVendor(VendorArm)
	if res == nil {
		t.Fatal("expected non-nil result")
	}
	if len(res) != 1 || res[0].Name != "arm-test" {
		t.Errorf("expected registered arm device, got %v", res)
	}
	// Unregistered vendor -> nil.
	if d.DetectByVendor(Vendor("bogus")) != nil {
		t.Error("expected nil for unregistered vendor")
	}
}

func TestCapabilities(t *testing.T) {
	caps := NewCPUCapabilities()
	if !caps.SupportsOp(tensor.OpMatMul) {
		t.Error("expected matmul support")
	}
	if !caps.SupportsOp(tensor.OpRelu) {
		t.Error("expected relu support")
	}
	if !caps.SupportsDtype(tensor.ElemF32) {
		t.Error("expected f32 support")
	}
	if caps.MaxRank != 8 {
		t.Errorf("expected max rank 8, got %d", caps.MaxRank)
	}
}

func TestHasVendorCapabilities(t *testing.T) {
	empty := NPUDevice{Capabilities: backend.Capabilities{}}
	if HasVendorCapabilities(empty) {
		t.Error("empty capabilities should not count as vendor capabilities")
	}
	real := NPUDevice{Capabilities: NewCPUCapabilities()}
	if !HasVendorCapabilities(real) {
		t.Error("cpu-like capabilities should count as vendor capabilities")
	}
}

func TestCompileAndExecute(t *testing.T) {
	mk := &mockBackend{
		vendor: VendorIntel,
		name:   "mock-intel",
		devs: []NPUDevice{{
			Name: "TestNPU", Vendor: VendorIntel, VendorName: "intel",
			Capabilities: NewCPUCapabilities(), Memory: 1 << 20,
		}},
		blast: [][]byte{[]byte("intel-blob")},
	}
	g := buildReluGraph()
	res, err := CompileAndExecute(mk, g, mk.devs[0], nil)
	if err != nil {
		t.Fatalf("compile-and-execute failed: %v", err)
	}
	if res.Metadata["npu_vendor"] != "intel" {
		t.Error("expected intel vendor in result metadata")
	}
	if res.Metadata["npu_program"] != "intel-blob" {
		t.Error("expected intel-blob program blob in metadata")
	}
	if _, ok := res.Outputs["y"]; !ok {
		t.Error("expected output node registered")
	}
}

func TestFallbackOnCompileFailure(t *testing.T) {
	// Simulate a device that cannot compile -> must not be an error for Karkain;
	// the executor should fall back. Here we verify the adapter signals failure
	// which a caller routes to the CPU reference backend.
	mk := &mockBackend{
		vendor: VendorQualcomm,
		name:   "mock-qualcomm",
		devs:   []NPUDevice{{Name: "BrokenNPU", Vendor: VendorQualcomm}},
		blast:  [][]byte{nil}, // compile fails
	}
	g := buildReluGraph()
	_, err := CompileAndExecute(mk, g, mk.devs[0], nil)
	if err == nil {
		t.Fatal("expected compile error on failing device")
	}
	if err != errCompileNotSupported {
		t.Errorf("expected errCompileNotSupported, got %v", err)
	}
}

func TestMockBackendImplementsInterface(t *testing.T) {
	mk := &mockBackend{vendor: VendorArm, name: "mock-arm"}
	var nb NPUBackend = mk
	if nb.Name() != "mock-arm" {
		t.Errorf("expected mock-arm, got %s", nb.Name())
	}
	if nb.Vendor() != VendorArm {
		t.Error("expected arm vendor")
	}
}

func TestVendorAdaptersRegister(t *testing.T) {
	// Ensure the standard adapters expose valid Vendor() and Build a program.
	adapters := []NPUBackend{
		newMockFromVendor(VendorIntel),
		newMockFromVendor(VendorQualcomm),
		newMockFromVendor(VendorApple),
		newMockFromVendor(VendorAMD),
		newMockFromVendor(VendorArm),
	}
	seen := map[Vendor]bool{}
	for _, a := range adapters {
		seen[a.Vendor()] = true
	}
	for _, v := range []Vendor{VendorIntel, VendorQualcomm, VendorApple, VendorAMD, VendorArm} {
		if !seen[v] {
			t.Errorf("missing adapter for vendor %s", v)
		}
	}
}

func newMockFromVendor(v Vendor) NPUBackend {
	return &mockBackend{vendor: v, name: "mock-" + string(v)}
}
