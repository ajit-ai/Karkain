package target

import "sort"

// Capability is a vendor-neutral execution capability a compute target can
// offer. Capabilities are deliberately coarse: they describe what a program
// may express when a KIR program is lowered to that target, never how a
// particular hardware vendor implements it.
//
// Phase 124 introduces the capability vocabulary as shared target/KIR
// infrastructure: every compute target (CPU, SIMD, WASM, GPU, NPU, quantum)
// declares the capabilities it genuinely provides, and the lowering boundary
// in lowering.go refuses programs whose required capabilities a target does
// not own.
type Capability string

const (
	// CapFloatArithmetic covers IEEE-754 scalar float arithmetic.
	CapFloatArithmetic Capability = "float_arithmetic"
	// CapIntArithmetic covers integer scalar arithmetic.
	CapIntArithmetic Capability = "int_arithmetic"
	// CapTensorOps covers tensor-shaped operations (create/load/store/reshape).
	CapTensorOps Capability = "tensor_ops"
	// CapMatrixOps covers matrix operations (matmul, transpose, broadcast).
	CapMatrixOps Capability = "matrix_ops"
	// CapReductionOps covers axis reductions (reduce_sum, reduce_mean).
	CapReductionOps Capability = "reduction_ops"
	// CapNNActivations covers activation functions (relu, sigmoid, tanh, softmax).
	CapNNActivations Capability = "nn_activations"
	// CapSIMDVectorOps covers SIMD lane-vector arithmetic.
	CapSIMDVectorOps Capability = "simd_vector_ops"
	// CapKernelLaunch covers data-parallel kernel launches with work-group ids.
	CapKernelLaunch Capability = "kernel_launch"
	// CapAsyncExecution covers asynchronous execution (launch without join).
	CapAsyncExecution Capability = "async_execution"
	// CapHostDeviceMemory covers a host/device memory model with transfers.
	CapHostDeviceMemory Capability = "host_device_memory"
	// CapDeviceMemory covers device-local-only memory.
	CapDeviceMemory Capability = "device_memory"
	// CapFunctionCalls covers general first-class function calls and recursion.
	CapFunctionCalls Capability = "function_calls"
	// CapIO covers host I/O (print/read).
	CapIO Capability = "io"
	// CapQuantumGates covers single/multi-qubit gate primitives.
	CapQuantumGates Capability = "quantum_gates"
	// CapQuantumMeasurement covers qubit measurement to a classical bit.
	CapQuantumMeasurement Capability = "quantum_measurement"
	// CapAlloca covers explicit heap allocation/free.
	CapAlloca Capability = "allocation"
)

// Maturity classifies how production-ready a compute target is. It is a
// statement about the Karkain-side surface, never about external hardware.
type Maturity int

const (
	// MaturityImplemented targets are exercised by the normal toolchain today.
	MaturityImplemented Maturity = iota
	// MaturityExperimental targets are modelled and lowering-checked but their
	// execution backends are not wired into the build pipeline yet.
	MaturityExperimental
	// MaturityResearch targets are modelled for specification purposes only.
	MaturityResearch
)

// String renders a Maturity as a stable lowercase label.
func (m Maturity) String() string {
	switch m {
	case MaturityImplemented:
		return "implemented"
	case MaturityExperimental:
		return "experimental"
	case MaturityResearch:
		return "research"
	}
	return "unknown"
}

// CapabilitySet is an unordered set of Capabilities with a sorted listing so
// diagnostics and catalogs are deterministic.
type CapabilitySet struct {
	m map[Capability]bool
}

// NewCapabilitySet builds a set from the given capabilities.
func NewCapabilitySet(caps ...Capability) *CapabilitySet {
	s := &CapabilitySet{m: make(map[Capability]bool, len(caps))}
	for _, c := range caps {
		s.m[c] = true
	}
	return s
}

// Has reports whether cap is present.
func (s *CapabilitySet) Has(cap Capability) bool {
	if s == nil || s.m == nil {
		return false
	}
	return s.m[cap]
}

// Add inserts cap and returns the set (for chaining).
func (s *CapabilitySet) Add(cap Capability) *CapabilitySet {
	if s.m == nil {
		s.m = make(map[Capability]bool)
	}
	s.m[cap] = true
	return s
}

// Union returns a new set containing this set's members and other's.
func (s *CapabilitySet) Union(other *CapabilitySet) *CapabilitySet {
	out := NewCapabilitySet()
	if s != nil {
		for c := range s.m {
			out.m[c] = true
		}
	}
	if other != nil {
		for c := range other.m {
			out.m[c] = true
		}
	}
	return out
}

// SupportsAll reports whether every listed capability is present.
func (s *CapabilitySet) SupportsAll(caps ...Capability) bool {
	for _, c := range caps {
		if !s.Has(c) {
			return false
		}
	}
	return true
}

// Missing returns the capabilities in caps that are absent, in input order.
func (s *CapabilitySet) Missing(caps ...Capability) []Capability {
	var out []Capability
	for _, c := range caps {
		if !s.Has(c) {
			out = append(out, c)
		}
	}
	return out
}

// List returns the members as sorted strings.
func (s *CapabilitySet) List() []string {
	if s == nil || s.m == nil {
		return nil
	}
	out := make([]string, 0, len(s.m))
	for c := range s.m {
		out = append(out, string(c))
	}
	sort.Strings(out)
	return out
}

// Len returns the number of members.
func (s *CapabilitySet) Len() int {
	if s == nil || s.m == nil {
		return 0
	}
	return len(s.m)
}