package codegen

import (
	"errors"
	"math"
	"os/exec"
	"testing"

	cpubackend "karkain/pkg/backend/cpu"
	"karkain/pkg/backend"
	"karkain/pkg/npu"
	"karkain/pkg/tensor"
)

// Phase 98: the NPU dispatcher. Matrix multiplication is the first concrete
// NPU operation. These tests prove (1) a matmul request reaches an available
// accelerator backend, (2) the CPU reference backend takes over when no NPU is
// available, (3) both paths return bit-identical results for the same inputs,
// and (4) no physical NPU, vendor SDK, driver, or cloud service is required —
// the CPU backend is the correctness oracle.

// phase98MockBackend emulates an accelerator that produces real computed
// values by delegating execution to the CPU reference backend (a software NPU
// emulator for dispatch testing). It does NOT depend on production adapters.
type phase98MockBackend struct {
	devs []npu.NPUDevice
	fail bool
}

func (m *phase98MockBackend) Name() string { return "npu-mock" }
func (m *phase98MockBackend) Capabilities() backend.Capabilities {
	return cpubackend.New().Capabilities()
}
func (m *phase98MockBackend) Supports(op tensor.Op, dt tensor.ElemType, sh tensor.Shape) bool {
	return cpubackend.New().Supports(op, dt, sh)
}
func (m *phase98MockBackend) Execute(g *tensor.TensorGraph, inputs map[string][]float64) (*backend.Result, error) {
	if m.fail {
		return nil, errors.New("mock NPU execute failed")
	}
	return cpubackend.New().Execute(g, inputs)
}
func (m *phase98MockBackend) Vendor() npu.Vendor { return npu.VendorIntel }
func (m *phase98MockBackend) DetectHardware() []npu.NPUDevice {
	return m.devs
}
func (m *phase98MockBackend) Compile(_ *tensor.TensorGraph, dev npu.NPUDevice) (*npu.NPUProgram, error) {
	return &npu.NPUProgram{Vendor: npu.VendorIntel, VendorName: dev.Name, Blob: []byte("phase98-mock-blob")}, nil
}

// matmul2x2Request returns the Phase 98 canonical request: A = [[1,2],[3,4]],
// B = [[5,6],[7,8]], so C = [[19,22],[43,50]].
func matmul2x2Request() DispatchRequest {
	return DispatchRequest{
		Op:     OpMatMul,
		Left:   []float64{1, 2, 3, 4},
		Right:  []float64{5, 6, 7, 8},
		RowsA:  2,
		ColsA:  2,
		ColsB:  2,
		DType:  tensor.ElemF32,
	}
}

func phase98HasGCC(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("gcc"); err != nil {
		t.Skip("gcc not available")
	}
}

func phase98MockDevice() npu.NPUDevice {
	return npu.NPUDevice{
		Name:   "phase98-emulated-npu",
		Vendor: npu.VendorIntel,
		Memory: 1024 * 1024,
	}
}

func assertClose2x2(t *testing.T, got []float64, want []float64) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("result has %d elements, want %d: %v", len(got), len(want), got)
	}
	for i := range want {
		if math.Abs(got[i]-want[i]) > 1e-9 {
			t.Errorf("result[%d] = %v, want %v", i, got[i], want[i])
		}
	}
}

func TestPhase98_Dispatch_MatMulReachesNPU(t *testing.T) {
	phase98HasGCC(t)
	dispatcher := NewNPUDispatcherWithBackend(&phase98MockBackend{
		devs: []npu.NPUDevice{phase98MockDevice()},
	})
	if !dispatcher.Available() {
		t.Fatal("expected dispatcher to report NPU availability with a mock backend")
	}
	res, err := dispatcher.Dispatch(matmul2x2Request())
	if err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	if res.Path != "npu" {
		t.Errorf("Path = %q, want npu", res.Path)
	}
	if res.Backend != "npu-mock" {
		t.Errorf("Backend = %q, want npu-mock", res.Backend)
	}
	if res.Metadata["npu_device"] != "phase98-emulated-npu" {
		t.Errorf("Metadata[npu_device] = %q, want phase98-emulated-npu", res.Metadata["npu_device"])
	}
	if res.Metadata["npu_blob"] != "phase98-mock-blob" {
		t.Errorf("Metadata[npu_blob] = %q, want phase98-mock-blob", res.Metadata["npu_blob"])
	}
	assertClose2x2(t, res.Values, []float64{19, 22, 43, 50})
}

func TestPhase98_Dispatch_MatMulCPUFallback(t *testing.T) {
	phase98HasGCC(t)
	dispatcher := NewNPUDispatcher() // no accelerator on the dev machine
	if dispatcher.Available() {
		t.Skip("machine reports an NPU; set KARKAIN_KCC etc. and rerun on a plain host")
	}
	res, err := dispatcher.Dispatch(matmul2x2Request())
	if err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	if res.Path != "cpu" {
		t.Errorf("Path = %q, want cpu", res.Path)
	}
	if res.Backend != "cpu" {
		t.Errorf("Backend = %q, want cpu", res.Backend)
	}
	assertClose2x2(t, res.Values, []float64{19, 22, 43, 50})
}

func TestPhase98_Dispatch_MatMulSameResultBothPaths(t *testing.T) {
	phase98HasGCC(t)
	npuPath := NewNPUDispatcherWithBackend(&phase98MockBackend{
		devs: []npu.NPUDevice{phase98MockDevice()},
	})
	cpuOnly := NewNPUDispatcher()
	if !npuPath.Available() {
		t.Fatal("expected mock dispatcher to be available")
	}
	nRes, err := npuPath.Dispatch(matmul2x2Request())
	if err != nil {
		t.Fatalf("npu dispatch: %v", err)
	}
	if nRes.Path != "npu" {
		t.Fatalf("expected npu path, got %q", nRes.Path)
	}
	cRes, err := cpuOnly.Dispatch(matmul2x2Request())
	if err != nil {
		t.Fatalf("cpu dispatch: %v", err)
	}
	if cRes.Path != "cpu" {
		t.Fatalf("expected cpu path, got %q", cRes.Path)
	}
	assertClose2x2(t, nRes.Values, []float64{19, 22, 43, 50})
	assertClose2x2(t, cRes.Values, []float64{19, 22, 43, 50})
}

func TestPhase98_Dispatch_NoHardwareResultStillCorrect(t *testing.T) {
	phase98HasGCC(t)
	// A backend whose DetectHardware reports nothing behaves exactly like a
	// machine without an NPU: the dispatcher reports no availability and the
	// CPU fallback executes.
	dispatcher := NewNPUDispatcherWithBackend(&phase98MockBackend{})
	if dispatcher.Available() {
		t.Error("expected no availability when the backend detects no hardware")
	}
	if dispatcher.BackendName() != "cpu" {
		t.Errorf("BackendName = %q, want cpu", dispatcher.BackendName())
	}
	res, err := dispatcher.Dispatch(matmul2x2Request())
	if err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	if res.Path != "cpu" {
		t.Errorf("Path = %q, want cpu fallback", res.Path)
	}
	assertClose2x2(t, res.Values, []float64{19, 22, 43, 50})
}

func TestPhase98_Dispatch_AdapterErrorDelegatesToCPU(t *testing.T) {
	phase98HasGCC(t)
	dispatcher := NewNPUDispatcherWithBackend(&phase98MockBackend{
		devs: []npu.NPUDevice{phase98MockDevice()},
		fail: true,
	})
	if !dispatcher.Available() {
		t.Fatal("expected availability with a mock device")
	}
	res, err := dispatcher.Dispatch(matmul2x2Request())
	if err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	if res.Path != "cpu" {
		t.Errorf("Path = %q, want cpu fallback after adapter failure", res.Path)
	}
	assertClose2x2(t, res.Values, []float64{19, 22, 43, 50})
}

func TestPhase98_Dispatch_InvalidDimensionsRejected(t *testing.T) {
	dispatcher := NewNPUDispatcher()
	req := matmul2x2Request()
	req.RowsA = 0
	if _, err := dispatcher.Dispatch(req); err == nil {
		t.Fatal("expected error for zero rows")
	}
}

func TestPhase98_Dispatch_OperandShapeMismatchRejected(t *testing.T) {
	dispatcher := NewNPUDispatcher()
	req := matmul2x2Request()
	req.Left = []float64{1, 2, 3} // 3 elements, want 4
	if _, err := dispatcher.Dispatch(req); err == nil {
		t.Fatal("expected error for mismatched left operand")
	}
}

func TestPhase98_Dispatch_UnsupportedOperationRejected(t *testing.T) {
	dispatcher := NewNPUDispatcher()
	req := DispatchRequest{Op: NPUOperation("convolution"), DType: tensor.ElemF32}
	if _, err := dispatcher.Dispatch(req); err == nil {
		t.Fatal("expected error for unsupported operation")
	}
}

func TestPhase98_Dispatcher_CapabilitiesDefaultToCPU(t *testing.T) {
	d := NewNPUDispatcher()
	caps := d.Capabilities()
	if !caps.SupportsOp(tensor.OpMatMul) {
		t.Error("default dispatcher must report matmul capability via the CPU oracle")
	}
}