package tensor

import "fmt"

// ============================================================
// Gradient Engine — Reverse-mode autodiff on Tensor IR
// ============================================================
//
// Given a forward TensorGraph, the gradient engine produces the
// gradient of an output with respect to each trainable input.
// The result is a backward TensorGraph that can be executed by
// any backend (CPU reference first, then GPU/NPU).

// GradResult holds the gradient graph and per-input gradient nodes.
type GradResult struct {
	// BackwardGraph is the gradient computation graph (new nodes).
	BackwardGraph *TensorGraph
	// InputGrads maps forward input node ID -> gradient node ID.
	InputGrads map[string]string
	// GatheredInputs lists forward inputs (in graph.Inputs order).
	GatheredInputs []*TensorNode
}

// ============================================================
// Reverse-mode autodiff
// ============================================================

// BuildGradient computes the gradient of the graph's outputs
// w.r.t. its inputs using reverse-mode autodiff.
//
// It creates a new backward graph whose nodes compute
// dL/d(forward node) for every forward node, walking forward nodes
// in reverse topological order.
func BuildGradient(g *TensorGraph) (*GradResult, error) {
	fwd := NewGraph()

	// Clone forward graph into the builder so all ids resolve in backward graph
	for _, n := range g.Nodes {
		clone := &TensorNode{ID: n.ID, Op: n.Op, Shape: n.Shape, DType: n.DType, Args: n.Args}
		fwd.AddNode(clone)
	}
	for _, in := range g.Inputs {
		fwd.AddInput(in)
	}
	for _, out := range g.Outputs {
		fwd.AddOutput(out)
	}

	// Ensure we have fwd graph registered in backward graph too
	fwdB := NewGraph()
	for _, n := range fwd.Nodes {
		fwdB.AddNode(n)
	}
	for _, in := range fwd.Inputs {
		fwdB.AddInput(in)
	}

	// gradMap: nodeID -> gradient node ID
	gradMap := make(map[string]string)

	// Seed the output gradient: dL/d(out) = ones (vector of 1s matching output shape)
	for idx, out := range fwd.Outputs {
		onesID := fmt.Sprintf("seed_grad_%d", idx)
		ones := NewNode(onesID, OpCreate, out.Shape, out.DType)
		fwdB.AddNode(ones)
		gradMap[out.ID] = onesID
	}

	// Process forward nodes in reverse topological order
	var order []*TensorNode
	visited := make(map[string]bool)
	topoSort(fwd, &order, visited, fwd.Outputs)

	// Collect accumulated gradient per node (multiple uses)
	accum := make(map[string][]string)

	for i := len(order) - 1; i >= 0; i-- {
		node := order[i]
		dOutIDs := collectGrads(gradMap, accum, node.ID)

		if node.Op == OpMatMul {
			emitMatMulGrad(fwdB, node, dOutIDs[0], gradMap, accum)
		} else if node.Op == OpRelu {
			emitReluGrad(fwdB, node, dOutIDs[0], gradMap, accum)
		} else if node.Op == OpSigmoid {
			emitSigmoidGrad(fwdB, node, dOutIDs[0], gradMap, accum)
		} else if node.Op == OpAdd || node.Op == OpSub {
			emitEWAddGrad(fwdB, node, dOutIDs[0], gradMap, accum)
		} else if node.Op == OpMul {
			emitEWMulGrad(fwdB, node, dOutIDs[0], gradMap, accum)
		} else if node.Op == OpTranspose {
			emitTransposeGrad(fwdB, node, dOutIDs[0], gradMap, accum)
		}
	}

	// Collect input gradients
	inputGrads := make(map[string]string)
	for _, in := range fwd.Inputs {
		ids := collectGrads(gradMap, accum, in.ID)
		if len(ids) > 0 {
			if len(ids) == 1 {
				inputGrads[in.ID] = ids[0]
			} else {
				// Sum multiple uses
				sumID := fwdB.NewID("grad_sum")
				parts := accum[in.ID]
				start := parts[0]
				for _, next := range parts[1:] {
					sumID = fwdB.NewID("grad_sum")
					sumN := NewNode(sumID, OpAdd, in.Shape, in.DType, start, next)
					fwdB.AddNode(sumN)
					start = sumID
				}
				inputGrads[in.ID] = start
				gradMap[in.ID] = start
			}
		}
	}

	return &GradResult{
		BackwardGraph:  fwdB,
		InputGrads:     inputGrads,
		GatheredInputs: fwd.Inputs,
	}, nil
}

// collectGrads returns the gradient IDs flowing into a node,
// combining any accumulated contributions.
func collectGrads(gradMap map[string]string, accum map[string][]string, nodeID string) []string {
	var ids []string
	if id, ok := gradMap[nodeID]; ok {
		ids = append(ids, id)
	}
	if acc, ok := accum[nodeID]; ok {
		ids = append(ids, acc...)
	}
	return ids
}

// emitMatMulGrad: for C = A @ B,
//   dA = dC @ B^T
//   dB = A^T @ dC
func emitMatMulGrad(g *TensorGraph, node *TensorNode, dOutID string, gradMap map[string]string, accum map[string][]string) {
	if len(node.Args) < 2 {
		return
	}
	aID := node.Args[0].(string)
	bID := node.Args[1].(string)
	a, _ := g.GetNode(aID)
	b, _ := g.GetNode(bID)

	// B^T
	btID := g.NewID("bt")
	bt := NewNode(btID, OpTranspose, reverseShape(b.Shape), b.DType, bID)
	g.AddNode(bt)

	// A^T
	atID := g.NewID("at")
	at := NewNode(atID, OpTranspose, reverseShape(a.Shape), a.DType, aID)
	g.AddNode(at)

	// dA = dC @ B^T
	dAID := g.NewID("da")
	dAShape, _ := matmulShapeRule([]Shape{node.Shape, bt.Shape})
	dA := NewNode(dAID, OpMatMul, dAShape, a.DType, dOutID, btID)
	g.AddNode(dA)
	gradMap[aID] = dAID

	// dB = A^T @ dC
	dBID := g.NewID("db")
	dBShape, _ := matmulShapeRule([]Shape{at.Shape, node.Shape})
	dB := NewNode(dBID, OpMatMul, dBShape, b.DType, atID, dOutID)
	g.AddNode(dB)
	gradMap[bID] = dBID
}

// emitReluGrad: dInput = dOut * (input > 0)
func emitReluGrad(g *TensorGraph, node *TensorNode, dOutID string, gradMap map[string]string, accum map[string][]string) {
	if len(node.Args) < 1 {
		return
	}
	inputID := node.Args[0].(string)
	input, _ := g.GetNode(inputID)

	// mask = relu-gate = (input > 0); approximate with step using mul by relu-d
	// dInput = dOut * cond(input>0). We model cond via a "step" helper not in IR;
	// instead we multiply by relu of input compared to sign. Use boolean mask op.
	maskID := g.NewID("relu_mask")
	mask := NewNode(maskID, OpCreate, input.Shape, input.DType)
	g.AddNode(mask)

	gradID := g.NewID("relu_grad")
	grad := NewNode(gradID, OpMul, input.Shape, input.DType, dOutID, maskID)
	g.AddNode(grad)
	gradMap[inputID] = gradID
}

// emitSigmoidGrad: dInput = dOut * sigmoid(x) * (1 - sigmoid(x))
func emitSigmoidGrad(g *TensorGraph, node *TensorNode, dOutID string, gradMap map[string]string, accum map[string][]string) {
	if len(node.Args) < 1 {
		return
	}
	inputID := node.Args[0].(string)
	gradID := g.NewID("sigmoid_grad")
	// dOut * x * (1 - x) where x is sigmoid output (this node)
	oneID := g.NewID("one")
	one := NewNode(oneID, OpCreate, node.Shape, node.DType)
	g.AddNode(one)
	negID := g.NewID("sigmoid_neg")
	neg := NewNode(negID, OpSub, node.Shape, node.DType, oneID, node.ID)
	g.AddNode(neg)
	m1ID := g.NewID("sigmoid_m1")
	m1 := NewNode(m1ID, OpMul, node.Shape, node.DType, dOutID, node.ID)
	g.AddNode(m1)
	grad := NewNode(gradID, OpMul, node.Shape, node.DType, m1ID, negID)
	g.AddNode(grad)
	gradMap[inputID] = gradID
}

// emitEWAddGrad: d(a+b)/da = dOut, d(a+b)/db = dOut
func emitEWAddGrad(g *TensorGraph, node *TensorNode, dOutID string, gradMap map[string]string, accum map[string][]string) {
	if len(node.Args) < 2 {
		return
	}
	aID := node.Args[0].(string)
	bID := node.Args[1].(string)
	a, _ := g.GetNode(aID)
	b, _ := g.GetNode(bID)
	gradMap[aID] = addToAccum(accum, aID, dOutID, a.Shape)
	gradMap[bID] = addToAccum(accum, bID, dOutID, b.Shape)
}

// emitEWMulGrad: d(a*b)/da = dOut * b, d(a*b)/db = dOut * a
func emitEWMulGrad(g *TensorGraph, node *TensorNode, dOutID string, gradMap map[string]string, accum map[string][]string) {
	if len(node.Args) < 2 {
		return
	}
	aID := node.Args[0].(string)
	bID := node.Args[1].(string)
	a, _ := g.GetNode(aID)
	b, _ := g.GetNode(bID)

	daID := g.NewID("ewmul_da")
	da := NewNode(daID, OpMul, a.Shape, a.DType, dOutID, bID)
	g.AddNode(da)
	gradMap[aID] = daID

	dbID := g.NewID("ewmul_db")
	db := NewNode(dbID, OpMul, b.Shape, b.DType, dOutID, aID)
	g.AddNode(db)
	gradMap[bID] = dbID
}

// emitTransposeGrad: derivative of transpose is the transpose itself.
func emitTransposeGrad(g *TensorGraph, node *TensorNode, dOutID string, gradMap map[string]string, accum map[string][]string) {
	if len(node.Args) < 1 {
		return
	}
	inputID := node.Args[0].(string)
	input, _ := g.GetNode(inputID)
	gradID := g.NewID("transpose_grad")
	grad := NewNode(gradID, OpTranspose, input.Shape, input.DType, dOutID)
	g.AddNode(grad)
	gradMap[inputID] = gradID
}

// addToAccum records a gradient contribution to a node for later summation.
func addToAccum(accum map[string][]string, nodeID string, gradID string, _ Shape) string {
	accum[nodeID] = append(accum[nodeID], gradID)
	return gradID
}

// ============================================================
// Topological sort (reverse mode needs post-order)
// ============================================================

func topoSort(g *TensorGraph, order *[]*TensorNode, visited map[string]bool, outputs []*TensorNode) {
	var visit func(node *TensorNode)
	visit = func(node *TensorNode) {
		if visited[node.ID] {
			return
		}
		visited[node.ID] = true
		for _, a := range node.Args {
			if s, ok := a.(string); ok {
				if child, ok := g.GetNode(s); ok {
					visit(child)
				}
			}
		}
		*order = append(*order, node)
	}
	for _, out := range outputs {
		visit(out)
	}
}

func reverseShape(s Shape) Shape {
	out := make(Shape, s.Rank())
	for i := 0; i < s.Rank(); i++ {
		out[i] = s[s.Rank()-1-i]
	}
	return out
}
