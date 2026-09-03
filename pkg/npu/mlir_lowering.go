package npu

import (
	"fmt"

	"karkain/pkg/tensor"
)

// LowerToMLIR lowers a Karkain Tensor IR graph to the Karkain-owned MLIR dialect.
// Each graph node becomes one dialect op; fused regions may be emitted as a
// single fused dialect op via the fusion pass.
type LowerToMLIR struct {
	module *MLIRModule
}

// NewLowerToMLIR creates a lowering context for a function.
func NewLowerToMLIR(funcName string) *LowerToMLIR {
	return &LowerToMLIR{module: NewMLIRModule(funcName)}
}

// Module returns the accumulated MLIR module.
func (l *LowerToMLIR) Module() *MLIRModule {
	return l.module
}

// opName maps a Tensor IR op to a dialect op name.
func opName(op tensor.Op) string {
	return KarkainMLIRDialect + "." + op.String()
}

// Lower lowers the whole graph (topologically: Nodes slice order) to ops.
// Graph output nodes are registered as function results.
func (l *LowerToMLIR) Lower(g *tensor.TensorGraph) (*MLIRModule, error) {
	outputs := map[string]bool{}
	for _, n := range g.Nodes {
		var operands []string
		for _, a := range n.Args {
			if s, ok := a.(string); ok {
				operands = append(operands, s)
			}
		}
		l.module.Add(&MLIROp{
			Name:     opName(n.Op),
			Operands: operands,
			Result:   n.ID,
			Attrs: map[string]string{
				"shape": shapeAttr(n.Shape),
				"dtype": n.DType.String(),
			},
		})
	}
	// Gather output node IDs.
	var results []string
	for _, o := range g.Outputs {
		outputs[o.ID] = true
		results = append(results, o.ID)
	}
	for _, r := range results {
		l.module.AddResult(r)
	}
	if len(results) == 0 {
		return nil, fmt.Errorf("graph has no outputs")
	}
	return l.module, nil
}

// shapeAttr renders a shape as an MLIR array attribute like [3, 4].
func shapeAttr(s tensor.Shape) string {
	out := "["
	for i, d := range s {
		if i > 0 {
			out += ", "
		}
		if d.IsStatic() {
			out += fmt.Sprintf("%d", d.Size)
		} else {
			out += "?"
		}
	}
	out += "]"
	return out
}
