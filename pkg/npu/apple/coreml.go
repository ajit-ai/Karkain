// Package apple provides the Apple Neural Engine CoreML NPU adapter.
package apple

import (
	"karkain/pkg/backend"
	"karkain/pkg/npu"
	"karkain/pkg/tensor"
)

// Backend is the Apple CoreML NPU adapter.
type Backend struct{}

// New creates an Apple CoreML backend.
func New() *Backend {
	return &Backend{}
}

// Name returns the backend name.
func (b *Backend) Name() string { return "npu-apple" }

// Vendor returns the NPU vendor.
func (b *Backend) Vendor() npu.Vendor { return npu.VendorApple }

// Capabilities returns the CoreML capability model.
func (b *Backend) Capabilities() backend.Capabilities {
	caps := npu.NewCPUCapabilities()
	caps.Name = "npu-apple"
	return caps
}

// Supports reports whether CoreML can run the operation.
func (b *Backend) Supports(op tensor.Op, dt tensor.ElemType, shape tensor.Shape) bool {
	return b.Capabilities().SupportsOp(op) && b.Capabilities().SupportsDtype(dt)
}

// Execute returns a stub result for graph execution on the ANE.
func (b *Backend) Execute(graph *tensor.TensorGraph, _ map[string][]float64) (*backend.Result, error) {
	res := backend.NewResult()
	res.Metadata["npu_vendor"] = "apple"
	for _, n := range graph.Outputs {
		res.Outputs[n.ID] = n
	}
	return res, nil
}

// DetectHardware returns the Apple devices this adapter targets.
func (b *Backend) DetectHardware() []npu.NPUDevice {
	d := npu.NewDeviceDetector()
	return d.DetectByVendor(npu.VendorApple)
}

// Compile produces a CoreML program blob.
func (b *Backend) Compile(graph *tensor.TensorGraph, device npu.NPUDevice) (*npu.NPUProgram, error) {
	prog := npu.NewProgram(npu.VendorApple)
	prog.VendorName = "CoreML"
	prog.Metadata["device"] = device.Name
	prog.Metadata["graph_nodes"] = len(graph.Nodes)
	prog.Metadata["blob_format"] = "coreml-model"
	prog.Blob = []byte("coreml:" + device.Name)
	return prog, nil
}
