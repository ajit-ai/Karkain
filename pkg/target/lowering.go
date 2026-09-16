package target

import (
	"fmt"
	"strings"
)

// KIRNode is one classified KIR v1 text line under the lowering boundary.
type KIRNode struct {
	Class KIRClass
	Text  string
}

// ReviewProgram classifies every line of a KIR v1 text program. Lines without
// a statement suffix are structural ("KIR v1", "source: ...") and are dropped
// so callers can lower the executable body directly.
func ReviewProgram(lines []string) []KIRNode {
	var out []KIRNode
	for _, line := range lines {
		t := strings.TrimSpace(line)
		if t == "" || t == "KIR v1" || strings.HasPrefix(t, "source: ") {
			continue
		}
		out = append(out, KIRNode{Class: ClassifyKIRLine(line), Text: t})
	}
	return out
}

// LowerPlan is the deterministic result of lowering a KIR program onto a
// compute target: the classified operations, the capability requirements of
// that program, and the tensor operations the target executes natively.
type LowerPlan struct {
	TargetName   string
	Nodes        []KIRNode
	Required     *CapabilitySet
	SupportedOps []string
}

// RequiredCapabilityFor returns the capability a KIR class needs on every
// accelerator target (I/O, function calls, kernel launch, measurement,
// allocation).
func RequiredCapabilityFor(class KIRClass) (Capability, bool) {
	switch class {
	case KIRImport, KIRCImport, KIRPrint:
		return CapIO, true
	case KIRFunc:
		return CapFunctionCalls, true
	case KIRKernel, KIRGlobalID:
		return CapKernelLaunch, true
	case KIRMeasure:
		return CapQuantumMeasurement, true
	case KIRAlloc, KIRFree:
		return CapAlloca, true
	}
	return "", false
}

// UnsupportedError is the deterministic diagnostic the lowering boundary
// produces for an operation a compute target cannot lower.
type UnsupportedError struct {
	TargetName string
	Op         string
	Class      KIRClass
	Hint       string
}

// Error renders the Phase-124 diagnostic contract. Every unsupported
// operation names the op text, its KIR class and the target, so callers and
// tests can assert on the exact boundary ("npu-experimental cannot lower
// 'func main (...)' (func); target does not provide function calls").
func (e *UnsupportedError) Error() string {
	return fmt.Sprintf("error[K124] target '%s' cannot lower '%s' (%s); %s",
		e.TargetName, e.Op, e.Class, e.Hint)
}

func capabilityHint(cap Capability) string {
	switch cap {
	case CapIO:
		return "host I/O is performed on the host, not the accelerator"
	case CapFunctionCalls:
		return "target does not provide function calls"
	case CapKernelLaunch:
		return "target does not provide kernel launches"
	case CapQuantumMeasurement:
		return "target does not provide quantum measurement"
	case CapAlloca:
		return "target does not provide heap allocation"
	}
	return "target does not claim this capability"
}

// Lower validates a KIR v1 text program against the target and returns a
// plan, or the first unsupported operation as an UnsupportedError. Validation
// is deterministic: identical input always yields the same plan or error.
func (c *ComputeTarget) Lower(kirLines []string) (*LowerPlan, error) {
	nodes := ReviewProgram(kirLines)
	required := NewCapabilitySet()
	for _, n := range nodes {
		if cap, ok := RequiredCapabilityFor(n.Class); ok {
			if !c.Capabilities.Has(cap) {
				return nil, &UnsupportedError{
					TargetName: c.Name,
					Op:         n.Text,
					Class:      n.Class,
					Hint:       capabilityHint(cap),
				}
			}
			required.Add(cap)
		}
		if !c.KIRClasses.Has(n.Class) {
			return nil, &UnsupportedError{
				TargetName: c.Name,
				Op:         n.Text,
				Class:      n.Class,
				Hint:       capabilityHint(requiredHintCapability(n.Class)),
			}
		}
	}
	return &LowerPlan{
		TargetName:   c.Name,
		Nodes:        nodes,
		Required:     required,
		SupportedOps: c.TensorOpNames(),
	}, nil
}

// requiredHintCapability returns the capability tied to a class for hint text
// when the class is rejected but does not itself require a capability.
func requiredHintCapability(class KIRClass) Capability {
	switch class {
	case KIRKernel, KIRGlobalID:
		return CapKernelLaunch
	case KIRImport, KIRCImport, KIRPrint:
		return CapIO
	case KIRFunc:
		return CapFunctionCalls
	case KIRMeasure:
		return CapQuantumMeasurement
	}
	return ""
}