package npu

import (
	"fmt"
	"strings"

	"karkain/pkg/tensor"
)

// FusionKind identifies a fused operation pattern.
type FusionKind string

const (
	// FusedLinear = MatMul + Add(Bias) + ReLU (typical MLP/DNN block).
	FusedLinear FusionKind = "fused_linear"
	// FusedConv = Conv2d + ReLU (conv + activation).
	FusedConv FusionKind = "fused_conv"
)

// FusedOp describes one fused operation produced by the fusion pass.
type FusedOp struct {
	Kind    FusionKind
	Root    *tensor.TensorNode // the terminating op of the pattern (e.g. matmul for FusedLinear)
	Nodes   []*tensor.TensorNode // all nodes consumed by the fusion (in order)
	Output  *tensor.TensorNode // the fused output node
	Extra   []*tensor.TensorNode // auxiliary inputs (e.g. bias), possibly optional
}

// FusionResult carries the set of fused ops for a graph.
type FusionResult struct {
	Ops        []*FusedOp
	Remainder  []*tensor.TensorNode // unfused nodes still needing individual kernels
}

// stableMeta returns a small metadata summary for a node (used to make fusion
// deterministic and testable).
func stableMeta(n *tensor.TensorNode) string {
	return fmt.Sprintf("%s:%s", n.Op, n.ID)
}

// detectFusedLinear checks whether a matmul node is directly followed by a
// bias add and then a ReLU, returning a FusedOp if so.
func detectFusedLinear(g *tensor.TensorGraph, mm *tensor.TensorNode) *FusedOp {
	if mm.Op != tensor.OpMatMul {
		return nil
	}
	// Find consumers of mm (nodes whose Args reference mm.ID).
	var add *tensor.TensorNode
	for _, n := range g.Nodes {
		if n.Op != tensor.OpAdd {
			continue
		}
		for _, a := range n.Args {
			if s, ok := a.(string); ok && s == mm.ID {
				add = n
				break
			}
		}
		if add != nil {
			break
		}
	}
	if add == nil {
		return nil
	}

	// Find a ReLU consuming the add.
	for _, n := range g.Nodes {
		if n.Op != tensor.OpRelu {
			continue
		}
		for _, a := range n.Args {
			if s, ok := a.(string); ok && s == add.ID {
				// Fused linear: matmul -> add -> relu
				return &FusedOp{
					Kind:   FusedLinear,
					Root:   mm,
					Nodes:  []*tensor.TensorNode{mm, add, n},
					Output: n,
				}
			}
		}
	}
	return nil
}

// detectFusedConv detects a conv2d->relu pattern. Since Tensor IR currently
// lowers conv through matmul (no standalone conv op), this is conservative and
// always returns nil until a dedicated conv op exists; kept for the optimizer's
// forward-looking architecture.
func detectFusedConv(_ *tensor.TensorGraph, _ *tensor.TensorNode) *FusedOp {
	return nil
}

// FusionPass performs operator fusion over a tensor graph.
type FusionPass struct {
	enabledKinds map[FusionKind]bool
}

// NewFusionPass creates a fusion pass with all fusions enabled.
func NewFusionPass() *FusionPass {
	return &FusionPass{
		enabledKinds: map[FusionKind]bool{
			FusedLinear: true,
			FusedConv:   true,
		},
	}
}

// Enable / Disable toggle individual fusions.
func (f *FusionPass) Enable(k FusionKind)  { f.enabledKinds[k] = true }
func (f *FusionPass) Disable(k FusionKind) { f.enabledKinds[k] = false }

// Run performs fusion over the graph and returns the fused ops plus any
// nodes that remain as-is. Only nodes whose pattern is a contiguous
// matmul->add->relu chain are absorbed; all others fall through to Remainder.
func (f *FusionPass) Run(g *tensor.TensorGraph) *FusionResult {
	result := &FusionResult{}
	covered := make(map[string]bool)

	for _, n := range g.Nodes {
		if covered[n.ID] {
			continue
		}
		var fused *FusedOp
		if f.enabledKinds[FusedLinear] {
			fused = detectFusedLinear(g, n)
		}
		if fused == nil && f.enabledKinds[FusedConv] {
			fused = detectFusedConv(g, n)
		}
		if fused != nil {
			result.Ops = append(result.Ops, fused)
			for _, fn := range fused.Nodes {
				covered[fn.ID] = true
			}
			continue
		}
		result.Remainder = append(result.Remainder, n)
	}
	return result
}

// Describe returns a human-readable summary of the fusion result.
func (r *FusionResult) Describe() string {
	var sb strings.Builder
	for _, op := range r.Ops {
		sb.WriteString(fmt.Sprintf("%s(", op.Kind))
		parts := make([]string, len(op.Nodes))
		for i, n := range op.Nodes {
			parts[i] = n.Op.String()
		}
		sb.WriteString(strings.Join(parts, " + "))
		sb.WriteString(")\n")
	}
	return sb.String()
}
