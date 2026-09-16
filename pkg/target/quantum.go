package target

// quantumCoreClasses lists the KIR v1 classes a quantum circuit executor
// accepts: classical scalar control flow plus circuit measurement. Function
// calls and kernel launches are intentionally absent — the lowering boundary
// rejects them deterministically. Display- and arithmetic-class libraries
// (import/cimport/print) belong to the host simulation harness.
var quantumCoreClasses = []KIRClass{
	KIRStruct, KIREnum, KIRVar, KIRLet, KIRConst, KIRReturn, KIRExprStmt,
	KIRBlock, KIRIf, KIRElse, KIRWhile, KIRBreak, KIRContinue, KIRMatch,
	KIRCase, KIRAlloc, KIRFree, KIRMeasure,
}

// quantumTarget returns the Phase-124 quantum compute target. It is RESEARCH:
// the specification model and its lowering boundary exist, but no quantum
// execution is performed. Gate primitives (h, x, y, z, cx, cz, t, s, reset,
// qinit) and measurement are the documented future operation vocabulary; the
// KIR v1 `measure` class already maps onto the lowering boundary today.
func quantumTarget() *ComputeTarget {
	host := Host()
	return &ComputeTarget{
		Name:           "quantum-experimental",
		Family:         "quantum",
		Triple:         &host,
		Maturity:       MaturityResearch,
		Description:    "Quantum circuit executor over gate primitives and measurement (spec model only)",
		MemoryModel:    "classical host plus a separate qubit register",
		ExecutionModel: "synchronous circuit execution with measurement to classical bits",
		Capabilities: NewCapabilitySet(
			CapQuantumGates, CapQuantumMeasurement, CapIntArithmetic, CapAlloca,
		),
		KIRClasses: NewKIRClassSet(quantumCoreClasses...),
		// No tensor vocabulary: gate/measurement primitives are its surface.
		SupportedTensorOps: nil,
	}
}