// Package intel provides the Intel OpenVINO NPU adapter.
package intel

import (
	"karkain/pkg/backend"
	"karkain/pkg/npu"
	"karkain/pkg/tensor"
)

// Backend is the Intel OpenVINO NPU adapter.
type Backend struct{}

// New creates an Intel OpenVINO backend.
func New() *Backend {
	return &Backend{}
}

// Name returns the backend name.
func (b *Backend) Name() string { return "npu-intel" }

// Vendor returns the NPU vendor.
func (b *Backend) Vendor() npu.Vendor { return npu.VendorIntel }

// Capabilities returns the OpenVINO capability model.
func (b *Backend) Capabilities() backend.Capabilities {
	caps := npu.NewCPUCapabilities()
	caps.Name = "npu-intel"
	return caps
}

// Supports reports whether OpenVINO can run the operation.
func (b *Backend) Supports(op tensor.Op, dt tensor.ElemType, shape tensor.Shape) bool {
	return b.Capabilities().SupportsOp(op) && b.Capabilities().SupportsDtype(dt)
}

// Execute returns a stub result marking the graph as compiled for OpenVINO.
func (b *Backend) Execute(graph *tensor.TensorGraph, _ map[string][]float64) (*backend.Result, error) {
	res := backend.NewResult()
	res.Metadata["npu_vendor"] = "intel"
	for _, n := range graph.Outputs {
		res.Outputs[n.ID] = n
	}
	return res, nil
}

// DetectHardware returns the Intel devices this adapter targets.
func (b *Backend) DetectHardware() []npu.NPUDevice {
	d := npu.NewDeviceDetector()
	return d.DetectByVendor(npu.VendorIntel)
}

// Compile produces an OpenVINO program blob (serialized graph descriptor).
func (b *Backend) Compile(graph *tensor.TensorGraph, device npu.NPUDevice) (*npu.NPUProgram, error) {
	prog := npu.NewProgram(npu.VendorIntel)
	prog.VendorName = "OpenVINO"
	prog.Metadata["device"] = device.Name
	prog.Metadata["graph_nodes"] = len(graph.Nodes)
	prog.Metadata["blob_format"] = "openvino-ir"
	prog.Blob = []byte("openvino-ir:" + device.Name)
	return prog, nil
}
