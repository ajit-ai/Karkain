package target

import "karkain/pkg/tensor"

// npuCoreClasses lists the KIR v1 classes an NPU inference executor accepts.
// The NPU is deliberately narrower than the GPU: no function calls, no kernel
// launches and no async work — a single synchronous inference invocation over
// a device-resident tensor graph.
var npuCoreClasses = []KIRClass{
	KIRStruct, KIREnum, KIRVar, KIRLet, KIRConst, KIRReturn, KIRExprStmt,
	KIRBlock, KIRIf, KIRElse, KIRWhile, KIRFor, KIRForIn, KIRBreak,
	KIRContinue, KIRMatch, KIRCase, KIRAlloc, KIRFree,
}

// npuOps is the tensor.Op vocabulary the NPU executes natively: element-wise
// arithmetic, matrix product and the activation family.
var npuOps = []tensor.Op{
	tensor.OpCreate, tensor.OpConstant, tensor.OpLoad, tensor.OpStore,
	tensor.OpAdd, tensor.OpSub, tensor.OpMul, tensor.OpDiv,
	tensor.OpMatMul, tensor.OpReshape, tensor.OpTranspose, tensor.OpBroadcast,
	tensor.OpConcat, tensor.OpReduceSum, tensor.OpReduceMean,
	tensor.OpRelu, tensor.OpSigmoid, tensor.OpTanh, tensor.OpSoftmax,
	tensor.OpEqual, tensor.OpNotEqual, tensor.OpLess, tensor.OpGreater,
	tensor.OpCopy,
}

// npuTarget returns the Phase-124 NPU compute target. It is EXPERIMENTAL:
// the model and its lowering boundary exist, but no accelerator code is
// emitted yet. It reuses the existing NPU backend abstraction
// (pkg/npu) as the execution layer rather than duplicating an adapter stack.
func npuTarget() *ComputeTarget {
	host := Host()
	return &ComputeTarget{
		Name:           "npu-experimental",
		Family:         "npu",
		Triple:         &host,
		Maturity:       MaturityExperimental,
		Description:    "Single-invocation neural inference over tensor/matrix ops (model only; no vendor dependencies)",
		MemoryModel:    "device-only (single-invocation inference graph)",
		ExecutionModel: "synchronous single-invocation inference",
		Capabilities: NewCapabilitySet(
			CapFloatArithmetic, CapIntArithmetic, CapTensorOps, CapMatrixOps,
			CapReductionOps, CapNNActivations, CapDeviceMemory, CapAlloca,
		),
		KIRClasses: NewKIRClassSet(npuCoreClasses...),
		SupportedTensorOps: npuOps,
	}
}