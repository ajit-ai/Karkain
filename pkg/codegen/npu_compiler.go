package codegen

import (
	"fmt"

	"karkain/pkg/backend"
	"karkain/pkg/npu"
	"karkain/pkg/npu/amd"
	"karkain/pkg/npu/apple"
	"karkain/pkg/npu/arm"
	"karkain/pkg/npu/intel"
	"karkain/pkg/npu/qualcomm"
	"karkain/pkg/tensor"

	cpubackend "karkain/pkg/backend/cpu"
)

// NPUOperation identifies a dispatchable operation on a @target(npu) function.
type NPUOperation string

const (
	// OpMatMul dispatches 2-D matrix multiplication (the first concrete NPU
	// operation — Phase 98).
	OpMatMul NPUOperation = "matmul"
)

// DispatchRequest describes an NPU-targeted operation and its inputs.
type DispatchRequest struct {
	// Op selects the operation to execute.
	Op NPUOperation
	// Left and Right hold the operands in row-major order.
	Left  []float64
	Right []float64
	// Matrix shapes: A is RowsA x ColsA, B is ColsA x ColsB, result RowsA x ColsB.
	RowsA int
	ColsA int
	ColsB int
	// DType is the element type. ElemUnknown selects the f32 default.
	DType tensor.ElemType
}

// DispatchResult is the outcome of an NPU dispatch.
type DispatchResult struct {
	// Backend is the name of the backend that executed the operation
	// ("npu-<vendor>" for an accelerator, "cpu" for the fallback).
	Backend string
	// Path is "npu" when an accelerator executed, "cpu" for the fallback.
	Path string
	// Values holds the result in row-major order.
	Values []float64
	// Metadata carries backend details (e.g., compiled blob descriptor).
	Metadata map[string]string
}

// NPUDispatcher routes NPU-targeted operations to an available accelerator
// backend and falls back to the CPU reference backend when none is available.
//
// Availability is decoupled from language semantics: @target(npu) functions
// always compile and always produce correct results. The CPU backend is the
// correctness oracle — every accelerator path must match its output.
//
// Vendor adapters are replaceable and additive. Adding a vendor never requires
// language, grammar, or pipeline changes; it only means supplying another
// npu.NPUBackend implementation to the dispatcher.
type NPUDispatcher struct {
	adapter npu.NPUBackend // active accelerator adapter (nil => CPU fallback)
	device  *npu.NPUDevice // backing device when an adapter is active
}

// standardAdapters lists the vendor NPU backends wired into the dispatcher.
// The order is deterministic (mirrors npu.DeviceDetector.Detect).
func standardAdapters() []npu.NPUBackend {
	return []npu.NPUBackend{
		intel.New(),
		qualcomm.New(),
		apple.New(),
		amd.New(),
		arm.New(),
	}
}

// NewNPUDispatcher builds a dispatcher that auto-detects the first available
// vendor NPU backend. On a machine with no NPU (the default development
// environment) it returns a CPU-only dispatcher.
func NewNPUDispatcher() *NPUDispatcher {
	for _, a := range standardAdapters() {
		if devs := a.DetectHardware(); len(devs) > 0 {
			return &NPUDispatcher{adapter: a, device: &devs[0]}
		}
	}
	return &NPUDispatcher{}
}

// NewNPUDispatcherWithBackend pins the dispatcher to a specific NPU backend.
// Used by tests to force NPU availability without physical hardware.
func NewNPUDispatcherWithBackend(b npu.NPUBackend) *NPUDispatcher {
	if devs := b.DetectHardware(); len(devs) > 0 {
		return &NPUDispatcher{adapter: b, device: &devs[0]}
	}
	return &NPUDispatcher{adapter: b}
}

// Available reports whether an accelerator backend can serve dispatches. When
// false every operation executes on the CPU fallback.
func (d *NPUDispatcher) Available() bool {
	return d.adapter != nil && d.device != nil
}

// BackendName returns the active backend's name, or "cpu" for the fallback.
func (d *NPUDispatcher) BackendName() string {
	if d.Available() {
		return d.adapter.Name()
	}
	return "cpu"
}

// Capabilities reports the active backend's capability model. With no NPU it
// reports the CPU reference capabilities (which support every Phase 98 op).
func (d *NPUDispatcher) Capabilities() backend.Capabilities {
	if d.Available() {
		return d.adapter.Capabilities()
	}
	return cpubackend.New().Capabilities()
}

// Dispatch routes an operation to the NPU backend when one is available and
// capable; otherwise it executes on the CPU reference backend. Both paths
// return the same semantically-identical result.
func (d *NPUDispatcher) Dispatch(req DispatchRequest) (*DispatchResult, error) {
	switch req.Op {
	case OpMatMul:
		return d.dispatchMatMul(req)
	default:
		return nil, fmt.Errorf("npu dispatch: unsupported operation %q", req.Op)
	}
}

// dispatchMatMul executes C = A @ B (Phase 98's canonical NPU operation).
func (d *NPUDispatcher) dispatchMatMul(req DispatchRequest) (*DispatchResult, error) {
	if req.RowsA <= 0 || req.ColsA <= 0 || req.ColsB <= 0 {
		return nil, fmt.Errorf("npu dispatch: invalid matmul dimensions %dx%d and %dx%d",
			req.RowsA, req.ColsA, req.ColsA, req.ColsB)
	}
	dtype := req.DType
	if dtype == tensor.ElemUnknown {
		dtype = tensor.ElemF32
	}
	rowsA, colsA, colsB := req.RowsA, req.ColsA, req.ColsB
	if len(req.Left) != rowsA*colsA {
		return nil, fmt.Errorf("npu dispatch: left operand has %d elements, want %d", len(req.Left), rowsA*colsA)
	}
	if len(req.Right) != colsA*colsB {
		return nil, fmt.Errorf("npu dispatch: right operand has %d elements, want %d", len(req.Right), colsA*colsB)
	}

	// Build the tensor graph C = A @ B (Karkain-owned Tensor IR).
	aNode := tensor.NewNode("A", tensor.OpCreate, tensor.NewShape(rowsA, colsA), dtype)
	bNode := tensor.NewNode("B", tensor.OpCreate, tensor.NewShape(colsA, colsB), dtype)
	cNode := tensor.NewNode("C", tensor.OpMatMul, tensor.NewShape(rowsA, colsB), dtype, "A", "B")
	g := tensor.NewGraph()
	g.AddNode(aNode)
	g.AddNode(bNode)
	g.AddNode(cNode)
	g.AddInput(aNode)
	g.AddInput(bNode)
	g.AddOutput(cNode)
	inputs := map[string][]float64{"A": req.Left, "B": req.Right}

	// NPU path: an accelerator is available and supports matrix multiplication.
	// Compile/execute failures are environmental (missing driver, vendor SDK
	// quirk, device contention), never caller errors — so they degrade to the
	// CPU fallback below. Only a backend that produced computed values owns the
	// result; anything else delegates so @target(npu) always yields a correct
	// program.
	if d.Available() && d.adapter.Supports(tensor.OpMatMul, dtype, tensor.NewShape(rowsA, colsA)) {
		prog, cErr := d.adapter.Compile(g, *d.device)
		var values []float64
		if cErr == nil {
			if res, eErr := d.adapter.Execute(g, inputs); eErr == nil {
				values = res.Values["C"]
			}
		}
		if len(values) > 0 {
			return &DispatchResult{
				Backend: d.adapter.Name(),
				Path:    "npu",
				Values:  values,
				Metadata: map[string]string{
					"npu_vendor": string(d.adapter.Vendor()),
					"npu_device": d.device.Name,
					"npu_blob":   string(prog.Blob),
				},
			}, nil
		}
	}

	// CPU fallback (always available and correct).
	res, err := cpubackend.New().Execute(g, inputs)
	if err != nil {
		return nil, fmt.Errorf("npu dispatch cpu fallback: %w", err)
	}
	return &DispatchResult{
		Backend:  "cpu",
		Path:     "cpu",
		Values:   res.Values["C"],
		Metadata: nil,
	}, nil
}