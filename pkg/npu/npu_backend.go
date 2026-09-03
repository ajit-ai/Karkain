// Package npu implements the NPU (Neural Processing Unit) backend family.
//
// The NPU is a backend — not the foundation, not the language. No Karkain
// program becomes invalid because an NPU is unavailable: every NPU operation
// falls back gracefully to CPU/GPU. Vendor adapters target Intel, Qualcomm,
// Apple, AMD, and Arm only (no Google TPU).
package npu

import (
	"karkain/pkg/backend"
	"karkain/pkg/tensor"
)

// Vendor identifies an NPU hardware vendor.
type Vendor string

const (
	VendorIntel     Vendor = "intel"
	VendorQualcomm  Vendor = "qualcomm"
	VendorApple     Vendor = "apple"
	VendorAMD       Vendor = "amd"
	VendorArm       Vendor = "arm"
	VendorUnknown   Vendor = "unknown"
)

// NPUDevice describes a detected NPU device.
type NPUDevice struct {
	Name         string
	Vendor       Vendor
	VendorName   string
	Capabilities backend.Capabilities
	Memory       int64
	Version      string
}

// NPUProgram is a compiled model blob for a specific vendor.
type NPUProgram struct {
	Vendor   Vendor
	VendorName string
	Blob     []byte
	Metadata map[string]interface{}
}

// NewProgram creates an empty NPU program with initialized metadata.
func NewProgram(v Vendor) *NPUProgram {
	return &NPUProgram{
		Vendor:   v,
		Metadata: make(map[string]interface{}),
	}
}

// NPUBackend is the interface that all NPU vendor adapters implement.
// It extends the generic Backend interface with NPU-specific lifecycle:
// hardware detection and model compilation.
type NPUBackend interface {
	backend.Backend
	// Vendor returns the NPU vendor.
	Vendor() Vendor
	// DetectHardware returns the devices this adapter can target.
	DetectHardware() []NPUDevice
	// Compile lowers a tensor graph to a vendor program blob.
	Compile(graph *tensor.TensorGraph, device NPUDevice) (*NPUProgram, error)
}

// CompileAndExecute is a convenience helper: compile then execute a program
// against an NPU backend, returning the generic execution result.
func CompileAndExecute(b NPUBackend, graph *tensor.TensorGraph, device NPUDevice, inputs map[string][]float64) (*backend.Result, error) {
	prog, err := b.Compile(graph, device)
	if err != nil {
		return nil, err
	}
	result := backend.NewResult()
	result.Metadata["npu_vendor"] = string(b.Vendor())
	result.Metadata["npu_device"] = device.Name
	result.Metadata["npu_program"] = string(prog.Blob)
	for _, n := range graph.Outputs {
		result.Outputs[n.ID] = n
	}
	return result, nil
}
