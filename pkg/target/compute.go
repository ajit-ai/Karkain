package target

import (
	"sort"

	"karkain/pkg/tensor"
)

// ComputeTarget models one accelerator/executor the KIR lowering boundary can
// target. It is the shared Phase-124 abstraction the GPU, NPU and quantum
// implementations extend (compute targets, capabilities and compiler target
// selection/dispatch). Compute targets deliberately reuse the existing target
// identity model (Target/triples), the existing tensor operation vocabulary
// (tensor.Op) and the KIR v1 classes (kir.go); they introduce no new IR and no
// vendor/hardware dependencies.
type ComputeTarget struct {
	// Name is the stable kebab-case identifier used by `karkain target` and
	// the lowering boundary (e.g. "gpu-experimental"). Never empty.
	Name string
	// Family groups related executors ("cpu", "simd", "wasm", "gpu", "npu",
	// "quantum").
	Family string
	// Triple is the host-identity target the accelerator code is compiled for
	// when it is lowered through the host C pipeline, or nil for executors
	// that are not reachable from a native host triple yet.
	Triple *Target
	// Maturity is the honest readiness label of the Karkain-side surface.
	Maturity Maturity
	// Description is a one-line, vendor-neutral summary shown in the catalog.
	Description string
	// MemoryModel describes where values live (host, device, host-device).
	MemoryModel string
	// ExecutionModel describes how work is scheduled and joined.
	ExecutionModel string
	// Capabilities is the vendor-neutral capability set the target provides.
	Capabilities *CapabilitySet
	// KIRClasses is the set of KIR v1 classes the target accepts on its
	// lowering boundary. Anything else is a deterministic UnsupportedError.
	KIRClasses *KIRClassSet
	// SupportedTensorOps is the reused tensor.Op vocabulary the target
	// executes natively (empty for scalar/quantum executors).
	SupportedTensorOps []tensor.Op
}

// Supports reports whether the target provides every listed capability.
func (c *ComputeTarget) Supports(caps ...Capability) bool {
	return c.Capabilities.SupportsAll(caps...)
}

// SupportsKIR reports whether the target accepts every listed KIR class.
func (c *ComputeTarget) SupportsKIR(classes ...KIRClass) bool {
	return c.KIRClasses.SupportsAll(classes...)
}

// MissingKIR returns the classes the target does not accept, in input order.
func (c *ComputeTarget) MissingKIR(classes ...KIRClass) []KIRClass {
	return c.KIRClasses.Missing(classes...)
}

// SupportedTensorOpName returns the tensor.Op.String of op ("add", "matmul",
// ...), or the empty string when op is not in the target's native set.
func (c *ComputeTarget) SupportedTensorOpName(op tensor.Op) string {
	for _, known := range c.SupportedTensorOps {
		if known == op {
			return op.String()
		}
	}
	return ""
}

// TensorOpNames returns the supported tensor operations as sorted strings.
func (c *ComputeTarget) TensorOpNames() []string {
	out := make([]string, 0, len(c.SupportedTensorOps))
	for _, op := range c.SupportedTensorOps {
		out = append(out, op.String())
	}
	sort.Strings(out)
	return out
}

// KIRClassSet is an unordered set of KIRClass values with a sorted listing.
type KIRClassSet struct {
	m map[KIRClass]bool
}

// NewKIRClassSet builds a set from the given classes.
func NewKIRClassSet(classes ...KIRClass) *KIRClassSet {
	s := &KIRClassSet{m: make(map[KIRClass]bool, len(classes))}
	for _, c := range classes {
		s.m[c] = true
	}
	return s
}

// Has reports whether class is present.
func (s *KIRClassSet) Has(class KIRClass) bool {
	if s == nil || s.m == nil {
		return false
	}
	return s.m[class]
}

// SupportsAll reports whether every class is present.
func (s *KIRClassSet) SupportsAll(classes ...KIRClass) bool {
	for _, c := range classes {
		if !s.Has(c) {
			return false
		}
	}
	return true
}

// Missing returns the classes not present, in input order.
func (s *KIRClassSet) Missing(classes ...KIRClass) []KIRClass {
	var out []KIRClass
	for _, c := range classes {
		if !s.Has(c) {
			out = append(out, c)
		}
	}
	return out
}

// List returns the members as sorted strings.
func (s *KIRClassSet) List() []string {
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