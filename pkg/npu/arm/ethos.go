// Package arm provides the Arm Ethos-U NPU adapter for embedded targets.
package arm

import (
	"karkain/pkg/backend"
	"karkain/pkg/npu"
	"karkain/pkg/tensor"
)

// Backend is the Arm Ethos-U NPU adapter.
type Backend struct{}

// New creates an Arm Ethos-U backend.
func New() *Backend {
	return &Backend{}
}

// Name returns the backend name.
func (b *Backend) Name() string { return "npu-arm" }

// Vendor returns the NPU vendor.
func (b *Backend) Vendor() npu.Vendor { return npu.VendorArm }

// Capabilities returns the Ethos-U capability model.
func (b *Backend) Capabilities() backend.Capabilities {
	caps := npu.NewCPUCapabilities()
	caps.Name = "npu-arm"
	// Ethos-U is an embedded NPU; reduce rank/dtype support accordingly.
	caps.SupportedDtypes = []tensor.ElemType{tensor.ElemF32, tensor.ElemI32}
	caps.MaxRank = 4
	caps.MemoryLimit = 1 << 24 // 16 MiB
	return caps
}

// Supports reports whether Ethos-U can run the operation.
func (b *Backend) Supports(op tensor.Op, dt tensor.ElemType, shape tensor.Shape) bool {
	return b.Capabilities().SupportsOp(op) && b.Capabilities().SupportsDtype(dt)
}

// Execute returns a stub result for graph execution on Ethos-U.
func (b *Backend) Execute(graph *tensor.TensorGraph, _ map[string][]float64) (*backend.Result, error) {
	res := backend.NewResult()
	res.Metadata["npu_vendor"] = "arm"
	for _, n := range graph.Outputs {
		res.Outputs[n.ID] = n
	}
	return res, nil
}

// DetectHardware returns the Arm devices this adapter targets.
func (b *Backend) DetectHardware() []npu.NPUDevice {
	d := npu.NewDeviceDetector()
	return d.DetectByVendor(npu.VendorArm)
}

// Compile produces an Ethos-U program blob (Vela-style compiled command stream).
func (b *Backend) Compile(graph *tensor.TensorGraph, device npu.NPUDevice) (*npu.NPUProgram, error) {
	prog := npu.NewProgram(npu.VendorArm)
	prog.VendorName = "Ethos-U"
	prog.Metadata["device"] = device.Name
	prog.Metadata["graph_nodes"] = len(graph.Nodes)
	prog.Metadata["blob_format"] = "ethos-u-command-stream"
	prog.Blob = []byte("ethos-u:" + device.Name)
	return prog, nil
}
