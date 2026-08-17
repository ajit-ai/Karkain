package sema

import (
	"fmt"
	"karkain/pkg/parser"
)

// ============================================================
// Tensor Shape & Dimension Types
// ============================================================

// TensorShape represents the shape (dimensions) of a tensor
type TensorShape struct {
	Dims []int // e.g., [32, 3, 224, 224]
}

// NewTensorShape creates a shape from dimension slice
func NewTensorShape(dims ...int) TensorShape {
	return TensorShape{Dims: dims}
}

// NumElements returns the total number of elements
func (s TensorShape) NumElements() int {
	if len(s.Dims) == 0 {
		return 0
	}
	product := 1
	for _, d := range s.Dims {
		product *= d
	}
	return product
}

// Ndims returns the rank (number of dimensions)
func (s TensorShape) Ndims() int {
	return len(s.Dims)
}

// String returns a human-readable shape like [32, 128]
func (s TensorShape) String() string {
	result := "["
	for i, d := range s.Dims {
		if i > 0 {
			result += ", "
		}
		result += fmt.Sprintf("%d", d)
	}
	result += "]"
	return result
}

// Equal checks if two shapes are identical
func (s TensorShape) Equal(other TensorShape) bool {
	if len(s.Dims) != len(other.Dims) {
		return false
	}
	for i := range s.Dims {
		if s.Dims[i] != other.Dims[i] {
			return false
		}
	}
	return true
}

// ============================================================
// DAG Node Types
// ============================================================

// ADNode is a node in the autodiff computation graph (DAG)
type ADNode struct {
	ID       int
	Name     string
	Op       string         // "input", "matmul", "relu", "softmax", "conv2d", "transpose", "add"
	Inputs   []*ADNode      // Input edges (forward direction)
	Grad     *ADNode        // Backward derivative node
	Shape    TensorShape    // Shape of the output tensor
	Metadata map[string]int // Op-specific metadata (e.g., axis for softmax, stride for conv2d)
}

// ADGraph is the full autodiff computation graph
type ADGraph struct {
	Nodes    []*ADNode
	Inputs   []*ADNode // Parameter nodes
	Outputs  []*ADNode // Output nodes
	nodeID   int
}

// NewADGraph creates a new empty autodiff graph
func NewADGraph() *ADGraph {
	return &ADGraph{
		Nodes:   make([]*ADNode, 0),
		Inputs:  make([]*ADNode, 0),
		Outputs: make([]*ADNode, 0),
		nodeID:  0,
	}
}

// NewInputNode creates an input (leaf) node in the graph
func (g *ADGraph) NewInputNode(name string, shape TensorShape) *ADNode {
	n := &ADNode{
		ID:       g.nodeID,
		Name:     name,
		Op:       "input",
		Shape:    shape,
		Metadata: make(map[string]int),
	}
	g.nodeID++
	g.Nodes = append(g.Nodes, n)
	g.Inputs = append(g.Inputs, n)
	return n
}

// NewOpNode creates an operation node in the graph
func (g *ADGraph) NewOpNode(op string, inputs []*ADNode, shape TensorShape) *ADNode {
	n := &ADNode{
		ID:       g.nodeID,
		Name:     fmt.Sprintf("%s_%d", op, g.nodeID),
		Op:       op,
		Inputs:   inputs,
		Shape:    shape,
		Metadata: make(map[string]int),
	}
	g.nodeID++
	g.Nodes = append(g.Nodes, n)
	return n
}

// ============================================================
// Shape Checker — validates tensor op dimension alignment
// ============================================================

// ShapeChecker performs static shape validation for tensor operations
type ShapeChecker struct {
	errors []error
}

// NewShapeChecker creates a new shape checker
func NewShapeChecker() *ShapeChecker {
	return &ShapeChecker{}
}

// CheckMatmulShape validates matrix multiplication: [M, K] x [K, N] -> [M, N]
func (sc *ShapeChecker) CheckMatmulShape(a, b TensorShape) (TensorShape, error) {
	if a.Ndims() < 2 {
		return TensorShape{}, fmt.Errorf("matmul: left operand must be at least 2D, got shape %s", a.String())
	}
	if b.Ndims() < 2 {
		return TensorShape{}, fmt.Errorf("matmul: right operand must be at least 2D, got shape %s", b.String())
	}

	kLeft := a.Dims[len(a.Dims)-1]
	kRight := b.Dims[len(b.Dims)-2]

	if kLeft != kRight {
		return TensorShape{}, fmt.Errorf(
			"matmul: inner dimensions mismatch: left has K=%d, right has K=%d (shapes %s x %s)",
			kLeft, kRight, a.String(), b.String())
	}

	// Result shape: [M, N] for 2D, or broadcast batch dims
	resultDims := make([]int, 0, len(a.Dims)+len(b.Dims)-2)
	resultDims = append(resultDims, a.Dims[:len(a.Dims)-1]...)
	resultDims = append(resultDims, b.Dims[len(b.Dims)-1:]...)
	return TensorShape{Dims: resultDims}, nil
}

// CheckReluShape validates relu: shape is preserved
func (sc *ShapeChecker) CheckReluShape(input TensorShape) TensorShape {
	return input
}

// CheckSoftmaxShape validates softmax: shape is preserved, axis must be valid
func (sc *ShapeChecker) CheckSoftmaxShape(input TensorShape, axis int) (TensorShape, error) {
	if input.Ndims() == 0 {
		return TensorShape{}, fmt.Errorf("softmax: input tensor cannot be scalar")
	}
	if axis < 0 || axis >= input.Ndims() {
		return TensorShape{}, fmt.Errorf(
			"softmax: axis %d out of range for tensor with %d dimensions", axis, input.Ndims())
	}
	return input, nil
}

// CheckConv2DShape validates 2D convolution:
// input: [N, C_in, H_in, W_in], kernel: [C_out, C_in, K_h, K_w]
// output: [N, C_out, H_out, W_out] where H_out = H_in - K_h + 1, W_out = W_in - K_w + 1
func (sc *ShapeChecker) CheckConv2DShape(input, kernel TensorShape) (TensorShape, error) {
	if input.Ndims() != 4 {
		return TensorShape{}, fmt.Errorf("conv2d: input must be 4D [N, C, H, W], got shape %s", input.String())
	}
	if kernel.Ndims() != 4 {
		return TensorShape{}, fmt.Errorf("conv2d: kernel must be 4D [C_out, C_in, K_h, K_w], got shape %s", kernel.String())
	}

	cIn := input.Dims[1]
	cOut := kernel.Dims[0]
	cInKernel := kernel.Dims[1]
	hIn := input.Dims[2]
	wIn := input.Dims[3]
	kH := kernel.Dims[2]
	kW := kernel.Dims[3]

	if cIn != cInKernel {
		return TensorShape{}, fmt.Errorf(
			"conv2d: input channels C_in=%d does not match kernel channels C_in=%d",
			cIn, cInKernel)
	}

	hOut := hIn - kH + 1
	wOut := wIn - kW + 1
	if hOut <= 0 || wOut <= 0 {
		return TensorShape{}, fmt.Errorf(
			"conv2d: output dimensions would be non-positive (H_out=%d, W_out=%d)", hOut, wOut)
	}

	return TensorShape{Dims: []int{input.Dims[0], cOut, hOut, wOut}}, nil
}

// CheckTransposeShape validates transpose: dims are reordered
func (sc *ShapeChecker) CheckTransposeShape(input TensorShape, dim0, dim1 int) (TensorShape, error) {
	if dim0 < 0 || dim0 >= input.Ndims() {
		return TensorShape{}, fmt.Errorf(
			"transpose: dim0=%d out of range for tensor with %d dimensions", dim0, input.Ndims())
	}
	if dim1 < 0 || dim1 >= input.Ndims() {
		return TensorShape{}, fmt.Errorf(
			"transpose: dim1=%d out of range for tensor with %d dimensions", dim1, input.Ndims())
	}

	resultDims := make([]int, len(input.Dims))
	copy(resultDims, input.Dims)
	resultDims[dim0], resultDims[dim1] = resultDims[dim1], resultDims[dim0]
	return TensorShape{Dims: resultDims}, nil
}

// ============================================================
// Reverse-Mode Autodiff — generates backward gradient nodes
// ============================================================

// AutodiffEngine builds forward and backward (gradient) DAGs
type AutodiffEngine struct {
	graph     *ADGraph
	checker   *ShapeChecker
	errors    []error
}

// NewAutodiffEngine creates a new autodiff engine
func NewAutodiffEngine() *AutodiffEngine {
	return &AutodiffEngine{
		graph:   NewADGraph(),
		checker: NewShapeChecker(),
	}
}

// GetGraph returns the underlying computation graph
func (e *AutodiffEngine) GetGraph() *ADGraph {
	return e.graph
}

// GetShapeChecker returns the shape checker
func (e *AutodiffEngine) GetShapeChecker() *ShapeChecker {
	return e.checker
}

// GetErrors returns accumulated errors
func (e *AutodiffEngine) GetErrors() []error {
	return e.errors
}

// TrackInput registers an input tensor in the graph
func (e *AutodiffEngine) TrackInput(name string, shape TensorShape) *ADNode {
	return e.graph.NewInputNode(name, shape)
}

// TrackMatmul records a matmul operation and returns its output node
func (e *AutodiffEngine) TrackMatmul(a, b *ADNode) (*ADNode, error) {
	resultShape, err := e.checker.CheckMatmulShape(a.Shape, b.Shape)
	if err != nil {
		e.errors = append(e.errors, err)
		return nil, err
	}
	return e.graph.NewOpNode("matmul", []*ADNode{a, b}, resultShape), nil
}

// TrackRelu records a relu operation
func (e *AutodiffEngine) TrackRelu(input *ADNode) *ADNode {
	resultShape := e.checker.CheckReluShape(input.Shape)
	return e.graph.NewOpNode("relu", []*ADNode{input}, resultShape)
}

// TrackSoftmax records a softmax operation
func (e *AutodiffEngine) TrackSoftmax(input *ADNode, axis int) (*ADNode, error) {
	resultShape, err := e.checker.CheckSoftmaxShape(input.Shape, axis)
	if err != nil {
		e.errors = append(e.errors, err)
		return nil, err
	}
	node := e.graph.NewOpNode("softmax", []*ADNode{input}, resultShape)
	node.Metadata["axis"] = axis
	return node, nil
}

// TrackConv2D records a conv2d operation
func (e *AutodiffEngine) TrackConv2D(input, kernel *ADNode) (*ADNode, error) {
	resultShape, err := e.checker.CheckConv2DShape(input.Shape, kernel.Shape)
	if err != nil {
		e.errors = append(e.errors, err)
		return nil, err
	}
	return e.graph.NewOpNode("conv2d", []*ADNode{input, kernel}, resultShape), nil
}

// TrackTranspose records a transpose operation
func (e *AutodiffEngine) TrackTranspose(input *ADNode, dim0, dim1 int) (*ADNode, error) {
	resultShape, err := e.checker.CheckTransposeShape(input.Shape, dim0, dim1)
	if err != nil {
		e.errors = append(e.errors, err)
		return nil, err
	}
	node := e.graph.NewOpNode("transpose", []*ADNode{input}, resultShape)
	node.Metadata["dim0"] = dim0
	node.Metadata["dim1"] = dim1
	return node, nil
}

// ============================================================
// Backward Pass — generates gradient nodes via reverse-mode AD
// ============================================================

// GradOp represents a backward (gradient) operation
type GradOp struct {
	ForwardNode *ADNode // The forward node this gradient is for
	GradNode    *ADNode // The gradient output node
}

// BuildBackwardPass generates gradient nodes for all operations in the graph
// traversing from output back to inputs (reverse-mode autodiff).
// upstreamGrad is the gradient flowing from the loss (dL/dOut).
func (e *AutodiffEngine) BuildBackwardPass(outputs []*ADNode, upstreamGrad *ADNode) []*GradOp {
	grads := make([]*GradOp, 0)
	gradMap := make(map[int]*ADNode) // nodeID -> gradient node
	gradMap[upstreamGrad.ID] = upstreamGrad

	// Map output nodes to the upstream gradient (dL/dOut flows into each output)
	for _, output := range outputs {
		gradMap[output.ID] = upstreamGrad
	}

	// Process nodes in reverse topological order
	for i := len(e.graph.Nodes) - 1; i >= 0; i-- {
		node := e.graph.Nodes[i]

		// Skip input nodes and the upstream grad itself
		if node.Op == "input" || node.ID == upstreamGrad.ID {
			continue
		}

		// Get the gradient of this node's output
		dOut, ok := gradMap[node.ID]
		if !ok {
			continue
		}

		// Generate backward derivative for this operation
		gradOps := e.generateGrad(node, dOut)
		for _, go2 := range gradOps {
			grads = append(grads, go2)
			// Propagate gradient to input nodes
			for _, input := range node.Inputs {
				if _, exists := gradMap[input.ID]; !exists {
					gradMap[input.ID] = go2.GradNode
				}
			}
		}
	}

	return grads
}

// generateGrad produces the backward gradient nodes for a specific operation
func (e *AutodiffEngine) generateGrad(node *ADNode, dOut *ADNode) []*GradOp {
	switch node.Op {
	case "matmul":
		return e.gradMatmul(node, dOut)
	case "relu":
		return e.gradRelu(node, dOut)
	case "softmax":
		return e.gradSoftmax(node, dOut)
	case "conv2d":
		return e.gradConv2D(node, dOut)
	case "transpose":
		return e.gradTranspose(node, dOut)
	default:
		return nil
	}
}

// gradMatmul generates gradients for matmul: dA = dOut @ B^T, dB = A^T @ dOut
func (e *AutodiffEngine) gradMatmul(node *ADNode, dOut *ADNode) []*GradOp {
	if len(node.Inputs) < 2 {
		return nil
	}

	a := node.Inputs[0]
	b := node.Inputs[1]

	// dA: [M, K] from [M, N] x [N, K] — shape matches A
	dAShape := TensorShape{Dims: a.Shape.Dims}
	dANode := e.graph.NewOpNode("matmul_grad_a", []*ADNode{dOut, b}, dAShape)

	// dB: [K, N] from [K, M] x [M, N] — shape matches B
	dBShape := TensorShape{Dims: b.Shape.Dims}
	dBNode := e.graph.NewOpNode("matmul_grad_b", []*ADNode{a, dOut}, dBShape)

	return []*GradOp{
		{ForwardNode: a, GradNode: dANode},
		{ForwardNode: b, GradNode: dBNode},
	}
}

// gradRelu generates gradient for relu: dInput = dOut * (input > 0)
func (e *AutodiffEngine) gradRelu(node *ADNode, dOut *ADNode) []*GradOp {
	if len(node.Inputs) < 1 {
		return nil
	}

	input := node.Inputs[0]
	gradShape := TensorShape{Dims: input.Shape.Dims}
	gradNode := e.graph.NewOpNode("relu_grad", []*ADNode{dOut, input}, gradShape)

	return []*GradOp{
		{ForwardNode: input, GradNode: gradNode},
	}
}

// gradSoftmax generates gradient for softmax (simplified)
func (e *AutodiffEngine) gradSoftmax(node *ADNode, dOut *ADNode) []*GradOp {
	if len(node.Inputs) < 1 {
		return nil
	}

	input := node.Inputs[0]
	gradShape := TensorShape{Dims: input.Shape.Dims}
	gradNode := e.graph.NewOpNode("softmax_grad", []*ADNode{dOut, input}, gradShape)

	return []*GradOp{
		{ForwardNode: input, GradNode: gradNode},
	}
}

// gradConv2D generates gradients for conv2d
func (e *AutodiffEngine) gradConv2D(node *ADNode, dOut *ADNode) []*GradOp {
	if len(node.Inputs) < 2 {
		return nil
	}

	input := node.Inputs[0]
	kernel := node.Inputs[1]

	// Gradient w.r.t. input: conv2d_transpose(dOut, kernel)
	dInputShape := TensorShape{Dims: input.Shape.Dims}
	dInputNode := e.graph.NewOpNode("conv2d_grad_input", []*ADNode{dOut, kernel}, dInputShape)

	// Gradient w.r.t. kernel
	dKernelShape := TensorShape{Dims: kernel.Shape.Dims}
	dKernelNode := e.graph.NewOpNode("conv2d_grad_kernel", []*ADNode{dOut, input}, dKernelShape)

	return []*GradOp{
		{ForwardNode: input, GradNode: dInputNode},
		{ForwardNode: kernel, GradNode: dKernelNode},
	}
}

// gradTranspose generates gradient for transpose: transpose back
func (e *AutodiffEngine) gradTranspose(node *ADNode, dOut *ADNode) []*GradOp {
	if len(node.Inputs) < 1 {
		return nil
	}

	input := node.Inputs[0]
	dim0 := node.Metadata["dim0"]
	dim1 := node.Metadata["dim1"]

	// Gradient of transpose is transpose back with same dims swapped
	gradShape := TensorShape{Dims: input.Shape.Dims}
	gradNode := e.graph.NewOpNode("transpose_grad", []*ADNode{dOut}, gradShape)
	gradNode.Metadata["dim0"] = dim0
	gradNode.Metadata["dim1"] = dim1

	return []*GradOp{
		{ForwardNode: input, GradNode: gradNode},
	}
}

// ============================================================
// Build from AST — walk a TensorStmt and construct the graph
// ============================================================

// AnalyzeTensorBlock builds an autodiff graph from a TensorStmt AST node
func (e *AutodiffEngine) AnalyzeTensorBlock(tensorStmt *parser.TensorStmt) error {
	e.errors = nil

	// Register input parameters
	inputNodes := make(map[string]*ADNode)
	for _, param := range tensorStmt.Params {
		shape := tensorShapeFromAST(param.Type)
		node := e.TrackInput(param.Name, shape)
		inputNodes[param.Name] = node
	}

	// Walk body statements
	var lastOutput *ADNode
	for _, stmt := range tensorStmt.Body {
		node, err := e.analyzeStmt(stmt, inputNodes)
		if err != nil {
			return err
		}
		if node != nil {
			lastOutput = node
		}
	}

	if lastOutput != nil {
		e.graph.Outputs = append(e.graph.Outputs, lastOutput)
	}

	return nil
}

// analyzeStmt processes a single statement in the tensor block
func (e *AutodiffEngine) analyzeStmt(stmt parser.Node, vars map[string]*ADNode) (*ADNode, error) {
	switch n := stmt.(type) {
	case *parser.VarDeclStmt:
		// e.g., let mat = ops.matmul(x, w);
		compExpr, ok := n.Value.(*parser.TensorOpExpr)
		if !ok {
			// Try CallExpr for backward compat
			return nil, nil
		}
		node, err := e.analyzeTensorOp(compExpr, vars)
		if err != nil {
			return nil, err
		}
		vars[n.Name] = node
		return node, nil

	case *parser.ExprStmt:
		compExpr, ok := n.Expression.(*parser.TensorOpExpr)
		if !ok {
			return nil, nil
		}
		return e.analyzeTensorOp(compExpr, vars)

	case *parser.TensorReturnStmt:
		// Resolve the return value
		return e.resolveNode(n.Value, vars)

	default:
		return nil, nil
	}
}

// analyzeTensorOp processes a tensor operation expression
func (e *AutodiffEngine) analyzeTensorOp(op *parser.TensorOpExpr, vars map[string]*ADNode) (*ADNode, error) {
	// Resolve arguments
	argNodes := make([]*ADNode, len(op.Args))
	for i, arg := range op.Args {
		node, err := e.resolveNode(arg, vars)
		if err != nil {
			return nil, err
		}
		argNodes[i] = node
	}

	switch op.Op {
	case "matmul":
		if len(argNodes) != 2 {
			return nil, fmt.Errorf("matmul requires exactly 2 arguments, got %d", len(argNodes))
		}
		return e.TrackMatmul(argNodes[0], argNodes[1])

	case "relu":
		if len(argNodes) != 1 {
			return nil, fmt.Errorf("relu requires exactly 1 argument, got %d", len(argNodes))
		}
		return e.TrackRelu(argNodes[0]), nil

	case "softmax":
		if len(argNodes) != 1 {
			return nil, fmt.Errorf("softmax requires exactly 1 argument, got %d", len(argNodes))
		}
		axis := 0
		if v, ok := op.Attrs["dim"]; ok {
			if intLit, ok := v.(*parser.IntLiteral); ok {
				fmt.Sscanf(intLit.Value, "%d", &axis)
			}
		}
		return e.TrackSoftmax(argNodes[0], axis)

	case "conv2d":
		if len(argNodes) != 2 {
			return nil, fmt.Errorf("conv2d requires exactly 2 arguments, got %d", len(argNodes))
		}
		return e.TrackConv2D(argNodes[0], argNodes[1])

	case "transpose":
		if len(argNodes) != 1 {
			return nil, fmt.Errorf("transpose requires exactly 1 argument, got %d", len(argNodes))
		}
		dim0, dim1 := 0, 1
		if v, ok := op.Attrs["dim0"]; ok {
			if intLit, ok := v.(*parser.IntLiteral); ok {
				fmt.Sscanf(intLit.Value, "%d", &dim0)
			}
		}
		if v, ok := op.Attrs["dim1"]; ok {
			if intLit, ok := v.(*parser.IntLiteral); ok {
				fmt.Sscanf(intLit.Value, "%d", &dim1)
			}
		}
		return e.TrackTranspose(argNodes[0], dim0, dim1)

	default:
		return nil, fmt.Errorf("unknown tensor operation: %s", op.Op)
	}
}

// resolveNode resolves an AST expression to a graph node
func (e *AutodiffEngine) resolveNode(node parser.Node, vars map[string]*ADNode) (*ADNode, error) {
	switch n := node.(type) {
	case *parser.Identifier:
		if adNode, ok := vars[n.Name]; ok {
			return adNode, nil
		}
		return nil, fmt.Errorf("undefined tensor variable: %s", n.Name)
	case *parser.TensorOpExpr:
		return e.analyzeTensorOp(n, vars)
	default:
		return nil, fmt.Errorf("unsupported expression type in tensor block: %T", node)
	}
}

// tensorShapeFromAST converts a TensorType AST node to a TensorShape
func tensorShapeFromAST(tt *parser.TensorType) TensorShape {
	dims := make([]int, len(tt.Shape))
	copy(dims, tt.Shape)
	return TensorShape{Dims: dims}
}
