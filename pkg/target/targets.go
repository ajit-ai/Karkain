package target

// scalarKIRClasses returns the KIR v1 classes every scalar/host-family
// executor accepts: the full portable surface plus host I/O. Kernel and
// measurement surface is accelerator-only.
func scalarKIRClasses() *KIRClassSet {
	return NewKIRClassSet(
		KIRImport, KIRCImport, KIRFunc, KIRStruct, KIREnum, KIRVar, KIRLet,
		KIRConst, KIRReturn, KIRPrint, KIRExprStmt, KIRBlock, KIRIf, KIRElse,
		KIRWhile, KIRFor, KIRForIn, KIRBreak, KIRContinue, KIRMatch, KIRCase,
		KIRAlloc, KIRFree,
	)
}

// hostScalarCaps is the capability set shared by the scalar host executors.
func hostScalarCaps(extra ...Capability) *CapabilitySet {
	return NewCapabilitySet(append([]Capability{
		CapFloatArithmetic, CapIntArithmetic, CapFunctionCalls, CapIO,
		CapAlloca,
	}, extra...)...)
}

// cpuTarget returns the reference CPU compute target: the host triple, fully
// implemented and exercised by the normal toolchain.
func cpuTarget() *ComputeTarget {
	host := Host()
	return &ComputeTarget{
		Name:           "cpu",
		Family:         "cpu",
		Triple:         &host,
		Maturity:       MaturityImplemented,
		Description:    "Reference scalar CPU executor (the default lowering surface)",
		MemoryModel:    "host (single address space)",
		ExecutionModel: "synchronous scalar execution",
		Capabilities:   hostScalarCaps(),
		KIRClasses:     scalarKIRClasses(),
	}
}

// simdTarget returns the SIMD/lane-vector compute target: the CPU surface
// extended with lane-vector arithmetic (Phase 106 @simd_* surface).
func simdTarget() *ComputeTarget {
	host := Host()
	return &ComputeTarget{
		Name:           "simd",
		Family:         "simd",
		Triple:         &host,
		Maturity:       MaturityImplemented,
		Description:    "CPU executor with lane-vector arithmetic (Phase 106 @simd_* surface)",
		MemoryModel:    "host (single address space)",
		ExecutionModel: "synchronous scalar and lane-vector execution",
		Capabilities:   hostScalarCaps(CapSIMDVectorOps),
		KIRClasses:     scalarKIRClasses(),
	}
}

// wasmTarget returns the wasm32-wasi compute target (Phase 108).
func wasmTarget() *ComputeTarget {
	t := Target{Arch: ArchWasm32, OS: OSWasi}
	return &ComputeTarget{
		Name:           "wasm32-wasi",
		Family:         "wasm",
		Triple:         &t,
		Maturity:       MaturityExperimental,
		Description:    "WebAssembly/WASI executor (Phase 108 Karkain-owned wasm backend)",
		MemoryModel:    "host sandbox (linear memory)",
		ExecutionModel: "synchronous scalar execution under a WASI runtime",
		Capabilities:   hostScalarCaps(),
		KIRClasses:     scalarKIRClasses(),
	}
}