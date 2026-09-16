package target

import "karkain/pkg/tensor"

// gpuCoreClasses lists the KIR v1 classes a GPU data-parallel executor
// accepts: full scalar control flow plus the kernel/work-group surface.
// Host I/O and quantum measurement are deliberately absent — both belong to
// the host, and the lowering boundary reports them deterministically.
var gpuCoreClasses = []KIRClass{
	KIRFunc, KIRKernel, KIRStruct, KIREnum, KIRVar, KIRLet, KIRConst,
	KIRReturn, KIRExprStmt, KIRBlock, KIRIf, KIRElse, KIRWhile, KIRFor,
	KIRForIn, KIRBreak, KIRContinue, KIRMatch, KIRCase, KIRAlloc, KIRFree,
	KIRGlobalID,
}

// gpuOps is the reused tensor.Op vocabulary the GPU target executes natively.
var gpuOps = []tensor.Op{
	tensor.OpCreate, tensor.OpConstant, tensor.OpLoad, tensor.OpStore,
	tensor.OpAdd, tensor.OpSub, tensor.OpMul, tensor.OpDiv, tensor.OpMod,
	tensor.OpMatMul, tensor.OpReshape, tensor.OpTranspose, tensor.OpBroadcast,
	tensor.OpSlice, tensor.OpConcat, tensor.OpReduceSum, tensor.OpReduceMean,
	tensor.OpRelu, tensor.OpSigmoid, tensor.OpTanh, tensor.OpSoftmax,
	tensor.OpEqual, tensor.OpNotEqual, tensor.OpLess, tensor.OpGreater,
	tensor.OpCopy,
}

// gpuTarget returns the Phase-124 GPU compute target. It is EXPERIMENTAL: the
// model and its lowering boundary exist, but no accelerator code is emitted
// by the build pipeline yet. It never assumes a vendor, SDK, or hardware.
func gpuTarget() *ComputeTarget {
	host := Host()
	return &ComputeTarget{
		Name:           "gpu-experimental",
		Family:         "gpu",
		Triple:         &host,
		Maturity:       MaturityExperimental,
		Description:    "Data-parallel kernel executor over tensor/matrix ops (model only; no vendor dependencies)",
		MemoryModel:    "host-device (plain buffers copied across the host/device boundary)",
		ExecutionModel: "async data-parallel work-groups with explicit kernel launch",
		Capabilities: NewCapabilitySet(
			CapFloatArithmetic, CapIntArithmetic, CapTensorOps, CapMatrixOps,
			CapReductionOps, CapNNActivations, CapSIMDVectorOps,
			CapKernelLaunch, CapAsyncExecution, CapHostDeviceMemory,
			CapFunctionCalls, CapAlloca,
		),
		KIRClasses: NewKIRClassSet(gpuCoreClasses...),
		SupportedTensorOps: gpuOps,
	}
}