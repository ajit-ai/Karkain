package tensor

import (
	"fmt"
	"strings"
)

// ============================================================
// TensorNode
// ============================================================

// TensorNode represents a single tensor operation in the IR.
type TensorNode struct {
	ID    string        // unique identifier
	Op    Op            // operation
	Shape Shape         // output shape
	DType ElemType      // element type
	Args  []interface{} // operation-specific arguments
}

// NewNode creates a new tensor node.
func NewNode(id string, op Op, shape Shape, dtype ElemType, args ...interface{}) *TensorNode {
	return &TensorNode{
		ID:    id,
		Op:    op,
		Shape: shape,
		DType: dtype,
		Args:  args,
	}
}

// String returns a human-readable representation.
func (n *TensorNode) String() string {
	return fmt.Sprintf("%s = %s : %s%s", n.ID, n.Op, n.DType, n.Shape)
}

// ============================================================
// TensorGraph
// ============================================================

// TensorGraph represents a computation graph of tensor operations.
type TensorGraph struct {
	Nodes   []*TensorNode
	Inputs  []*TensorNode // input nodes
	Outputs []*TensorNode // output nodes
	index   map[string]*TensorNode
	counter int
}

// NewGraph creates an empty tensor graph.
func NewGraph() *TensorGraph {
	return &TensorGraph{
		Nodes:  make([]*TensorNode, 0),
		Inputs: make([]*TensorNode, 0),
		Outputs: make([]*TensorNode, 0),
		index:  make(map[string]*TensorNode),
	}
}

// NewID generates a unique node ID.
func (g *TensorGraph) NewID(prefix string) string {
	g.counter++
	return fmt.Sprintf("%s_%d", prefix, g.counter)
}

// AddNode adds a node to the graph.
func (g *TensorGraph) AddNode(node *TensorNode) {
	g.Nodes = append(g.Nodes, node)
	g.index[node.ID] = node
}

// GetNode retrieves a node by ID.
func (g *TensorGraph) GetNode(id string) (*TensorNode, bool) {
	n, ok := g.index[id]
	return n, ok
}

// AddInput marks a node as an input to the graph.
func (g *TensorGraph) AddInput(node *TensorNode) {
	g.Inputs = append(g.Inputs, node)
}

// AddOutput marks a node as an output of the graph.
func (g *TensorGraph) AddOutput(node *TensorNode) {
	g.Outputs = append(g.Outputs, node)
}

// NumNodes returns the number of nodes in the graph.
func (g *TensorGraph) NumNodes() int {
	return len(g.Nodes)
}

// ============================================================
// Builder
// ============================================================

// Builder provides a fluent API for constructing tensor graphs.
type Builder struct {
	graph *TensorGraph
}

// NewBuilder creates a new tensor graph builder.
func NewBuilder() *Builder {
	return &Builder{graph: NewGraph()}
}

// Input adds an input node.
func (b *Builder) Input(id string, dtype ElemType, shape Shape) *Builder {
	node := NewNode(id, OpCreate, shape, dtype)
	b.graph.AddNode(node)
	b.graph.AddInput(node)
	return b
}

// Create adds a tensor creation node.
func (b *Builder) Create(id string, dtype ElemType, shape Shape) *Builder {
	node := NewNode(id, OpCreate, shape, dtype)
	b.graph.AddNode(node)
	return b
}

// Add adds an element-wise add node.
func (b *Builder) Add(id string, left, right *TensorNode) *Builder {
	shape, _ := broadcastShapes(left.Shape, right.Shape)
	dtype := promoteElemType(left.DType, right.DType)
	node := NewNode(id, OpAdd, shape, dtype, left.ID, right.ID)
	b.graph.AddNode(node)
	return b
}

// Sub adds an element-wise subtract node.
func (b *Builder) Sub(id string, left, right *TensorNode) *Builder {
	shape, _ := broadcastShapes(left.Shape, right.Shape)
	dtype := promoteElemType(left.DType, right.DType)
	node := NewNode(id, OpSub, shape, dtype, left.ID, right.ID)
	b.graph.AddNode(node)
	return b
}

// Mul adds an element-wise multiply node.
func (b *Builder) Mul(id string, left, right *TensorNode) *Builder {
	shape, _ := broadcastShapes(left.Shape, right.Shape)
	dtype := promoteElemType(left.DType, right.DType)
	node := NewNode(id, OpMul, shape, dtype, left.ID, right.ID)
	b.graph.AddNode(node)
	return b
}

// MatMul adds a matrix multiply node.
func (b *Builder) MatMul(id string, left, right *TensorNode) *Builder {
	rule := matmulShapeRule
	shape, _ := rule([]Shape{left.Shape, right.Shape})
	dtype := promoteElemType(left.DType, right.DType)
	node := NewNode(id, OpMatMul, shape, dtype, left.ID, right.ID)
	b.graph.AddNode(node)
	return b
}

// Relu adds a ReLU activation node.
func (b *Builder) Relu(id string, input *TensorNode) *Builder {
	node := NewNode(id, OpRelu, input.Shape, input.DType, input.ID)
	b.graph.AddNode(node)
	return b
}

// Softmax adds a softmax node.
func (b *Builder) Softmax(id string, input *TensorNode) *Builder {
	node := NewNode(id, OpSoftmax, input.Shape, input.DType, input.ID)
	b.graph.AddNode(node)
	return b
}

// Reshape adds a reshape node.
func (b *Builder) Reshape(id string, input *TensorNode, newShape Shape) *Builder {
	node := NewNode(id, OpReshape, newShape, input.DType, input.ID)
	b.graph.AddNode(node)
	return b
}

// Transpose adds a transpose node.
func (b *Builder) Transpose(id string, input *TensorNode) *Builder {
	rule := transposeShapeRule
	shape, _ := rule([]Shape{input.Shape})
	node := NewNode(id, OpTranspose, shape, input.DType, input.ID)
	b.graph.AddNode(node)
	return b
}

// Output marks a node as an output.
func (b *Builder) Output(node *TensorNode) *Builder {
	b.graph.AddOutput(node)
	return b
}

// Build returns the constructed graph.
func (b *Builder) Build() *TensorGraph {
	return b.graph
}

// ============================================================
// Utilities
// ============================================================

func promoteElemType(a, b ElemType) ElemType {
	if a == b {
		return a
	}
	if a.IsFloat() || b.IsFloat() {
		if a.ByteSize() >= 8 || b.ByteSize() >= 8 {
			return ElemF64
		}
		return ElemF32
	}
	if a.IsInteger() && b.IsInteger() {
		if a.ByteSize() >= 8 || b.ByteSize() >= 8 {
			return ElemI64
		}
		return ElemI32
	}
	return ElemF64
}

// PrintGraph returns a human-readable string of the graph.
func PrintGraph(g *TensorGraph) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("=== Tensor IR Graph ===\n"))
	sb.WriteString(fmt.Sprintf("Nodes: %d, Inputs: %d, Outputs: %d\n\n", g.NumNodes(), len(g.Inputs), len(g.Outputs)))

	for _, input := range g.Inputs {
		sb.WriteString(fmt.Sprintf("Input: %s\n", input))
	}
	sb.WriteString("\n")

	for _, node := range g.Nodes {
		inputs := ""
		if len(node.Args) > 0 {
			parts := make([]string, len(node.Args))
			for i, a := range node.Args {
				parts[i] = fmt.Sprintf("%v", a)
			}
			inputs = " (" + strings.Join(parts, ", ") + ")"
		}
		sb.WriteString(fmt.Sprintf("  %s%s\n", node, inputs))
	}

	if len(g.Outputs) > 0 {
		sb.WriteString("\nOutputs:\n")
		for _, out := range g.Outputs {
			sb.WriteString(fmt.Sprintf("  %s\n", out.ID))
		}
	}

	return sb.String()
}
