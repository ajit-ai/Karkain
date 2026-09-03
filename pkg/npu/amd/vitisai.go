// Package amd provides the AMD VitisAI (XDNA/Ryzen AI) NPU adapter.
package amd

import (
	"karkain/pkg/backend"
	"karkain/pkg/npu"
	"karkain/pkg/tensor"
)

// Backend is the AMD XDNA/VitisAI NPU adapter.
type Backend struct{}

// New creates an AMD VitisAI backend.
func New() *Backend {
	return &Backend{}
}

// Name returns the backend name.
func (b *Backend) Name() string { return "npu-amd" }

// Vendor returns the NPU vendor.
func (b *Backend) Vendor() npu.Vendor { return npu.VendorAMD }

// Capabilities returns the VitisAI capability model.
func (b *Backend) Capabilities() backend.Capabilities {
	caps := npu.NewCPUCapabilities()
	caps.Name = "npu-amd"
	return caps
}

// Supports reports whether VitisAI can run the operation.
func (b *Backend) Supports(op tensor.Op, dt tensor.ElemType, shape tensor.Shape) bool {
	return b.Capabilities().SupportsOp(op) && b.Capabilities().SupportsDtype(dt)
}

// Execute returns a stub result for graph execution on XDNA.
func (b *Backend) Execute(graph *tensor.TensorGraph, _ map[string][]float64) (*backend.Result, error) {
	res := backend.NewResult()
	res.Metadata["npu_vendor"] = "amd"
	for _, n := range graph.Outputs {
		res.Outputs[n.ID] = n
	}
	return res, nil
}

// DetectHardware returns the AMD devices this adapter targets.
func (b *Backend) DetectHardware() []npu.NPUDevice {
	d := npu.NewDeviceDetector()
	return d.DetectByVendor(npu.VendorAMD)
}

// Compile produces a VitisAI program blob.
func (b *Backend) Compile(graph *tensor.TensorGraph, device npu.NPUDevice) (*npu.NPUProgram, error) {
	prog := npu.NewProgram(npu.VendorAMD)
	prog.VendorName = "VitisAI"
	prog.Metadata["device"] = device.Name
	prog.Metadata["graph_nodes"] = len(graph.Nodes)
	prog.Metadata["blob_format"] = "vitisai-xclbin"
	prog.Blob = []byte("vitisai:" + device.Name)
	return prog, nil
}
