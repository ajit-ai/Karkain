// Package qualcomm provides the Qualcomm QNN/HTP NPU adapter.
package qualcomm

import (
	"karkain/pkg/backend"
	"karkain/pkg/npu"
	"karkain/pkg/tensor"
)

// Backend is the Qualcomm QNN/HTP NPU adapter.
type Backend struct{}

// New creates a Qualcomm QNN backend.
func New() *Backend {
	return &Backend{}
}

// Name returns the backend name.
func (b *Backend) Name() string { return "npu-qualcomm" }

// Vendor returns the NPU vendor.
func (b *Backend) Vendor() npu.Vendor { return npu.VendorQualcomm }

// Capabilities returns the QNN capability model.
func (b *Backend) Capabilities() backend.Capabilities {
	caps := npu.NewCPUCapabilities()
	caps.Name = "npu-qualcomm"
	return caps
}

// Supports reports whether QNN can run the operation.
func (b *Backend) Supports(op tensor.Op, dt tensor.ElemType, shape tensor.Shape) bool {
	return b.Capabilities().SupportsOp(op) && b.Capabilities().SupportsDtype(dt)
}

// Execute returns a stub result for graph execution on QNN.
func (b *Backend) Execute(graph *tensor.TensorGraph, _ map[string][]float64) (*backend.Result, error) {
	res := backend.NewResult()
	res.Metadata["npu_vendor"] = "qualcomm"
	for _, n := range graph.Outputs {
		res.Outputs[n.ID] = n
	}
	return res, nil
}

// DetectHardware returns the Qualcomm devices this adapter targets.
func (b *Backend) DetectHardware() []npu.NPUDevice {
	d := npu.NewDeviceDetector()
	return d.DetectByVendor(npu.VendorQualcomm)
}

// Compile produces a QNN program blob.
func (b *Backend) Compile(graph *tensor.TensorGraph, device npu.NPUDevice) (*npu.NPUProgram, error) {
	prog := npu.NewProgram(npu.VendorQualcomm)
	prog.VendorName = "QNN/HTP"
	prog.Metadata["device"] = device.Name
	prog.Metadata["graph_nodes"] = len(graph.Nodes)
	prog.Metadata["blob_format"] = "qnn-context-binary"
	prog.Blob = []byte("qnn:" + device.Name)
	return prog, nil
}
