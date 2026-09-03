package npu

import (
	"karkain/pkg/backend"
	"karkain/pkg/tensor"
)

// NewCPUCapabilities returns the capability model shared by CPU-backed
// reference execution. NPU adapters may further restrict this based on
// hardware features.
func NewCPUCapabilities() backend.Capabilities {
	return backend.Capabilities{
		Name: "npu",
		SupportedDtypes: []tensor.ElemType{
			tensor.ElemF32, tensor.ElemF64, tensor.ElemI32,
		},
		MaxRank:        8,
		MemoryLimit:    1 << 30, // 1 GiB (soft reference cap)
		AlignmentReqs:  16,
		SupportedOps: []tensor.Op{
			tensor.OpCreate, tensor.OpAdd, tensor.OpSub, tensor.OpMul,
			tensor.OpDiv, tensor.OpMatMul, tensor.OpRelu, tensor.OpSigmoid,
			tensor.OpTanh, tensor.OpSoftmax, tensor.OpTranspose,
		},
		SupportedLayouts: []tensor.Layout{tensor.LayoutRowMajor},
	}
}

// HasVendorCapabilities reports whether the NPU device exposes at least one
// compute operation (a realistic accelerator signature, rather than a pure
// memory-movement device).
func HasVendorCapabilities(d NPUDevice) bool {
	for _, op := range d.Capabilities.SupportedOps {
		switch op {
		case tensor.OpMatMul, tensor.OpRelu, tensor.OpSoftmax,
			tensor.OpSigmoid, tensor.OpTanh, tensor.OpAdd, tensor.OpMul,
			tensor.OpReduceSum, tensor.OpReduceMean:
			return true
		}
	}
	return false
}
