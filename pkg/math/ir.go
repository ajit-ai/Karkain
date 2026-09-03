// Package math implements Karkain's Math IR — a Karkain-owned intermediate
// representation for mathematical computation. Math IR sits between the AST
// and Tensor IR, representing mathematical semantics independently from hardware.
//
// Math IR is NOT NumPy, NOT MLIR, NOT SymPy, NOT a vendor format.
// It is Karkain's own representation of mathematical computation.
package math

import (
	"fmt"
	"strings"
)

// ============================================================
// Node Interface
// ============================================================

// Node is the base interface for all Math IR nodes.
type Node interface {
	// Type returns the math type of this node's result.
	Type() MathType
	// Op returns the operation name (e.g., "Add", "Multiply", "Constant").
	Op() string
	// Operands returns the child nodes.
	Operands() []Node
	// NumOperands returns the number of child nodes.
	NumOperands() int
}

// ============================================================
// Constant Nodes
// ============================================================

// IntConst represents an integer constant.
type IntConst struct {
	Value int64
	Typ   MathType
}

func (n *IntConst) Type() MathType     { return n.Typ }
func (n *IntConst) Op() string         { return "IntConst" }
func (n *IntConst) Operands() []Node   { return nil }
func (n *IntConst) NumOperands() int   { return 0 }

// FloatConst represents a floating-point constant.
type FloatConst struct {
	Value float64
	Typ   MathType
}

func (n *FloatConst) Type() MathType     { return n.Typ }
func (n *FloatConst) Op() string         { return "FloatConst" }
func (n *FloatConst) Operands() []Node   { return nil }
func (n *FloatConst) NumOperands() int   { return 0 }

// BoolConst represents a boolean constant.
type BoolConst struct {
	Value bool
}

func (n *BoolConst) Type() MathType     { return Bool }
func (n *BoolConst) Op() string         { return "BoolConst" }
func (n *BoolConst) Operands() []Node   { return nil }
func (n *BoolConst) NumOperands() int   { return 0 }

// StringConst represents a string constant.
type StringConst struct {
	Value string
}

func (n *StringConst) Type() MathType     { 	return StringType }
func (n *StringConst) Op() string         { return "StringConst" }
func (n *StringConst) Operands() []Node   { return nil }
func (n *StringConst) NumOperands() int   { return 0 }

// ============================================================
// Variable Nodes
// ============================================================

// Variable represents a named variable with a known type.
type Variable struct {
	Name string
	Typ  MathType
}

func (n *Variable) Type() MathType     { return n.Typ }
func (n *Variable) Op() string         { return "Variable" }
func (n *Variable) Operands() []Node   { return nil }
func (n *Variable) NumOperands() int   { return 0 }

// Param represents a function parameter.
type Param struct {
	Name string
	Typ  MathType
}

func (n *Param) Type() MathType     { return n.Typ }
func (n *Param) Op() string         { return "Param" }
func (n *Param) Operands() []Node   { return nil }
func (n *Param) NumOperands() int   { return 0 }

// ============================================================
// Arithmetic Nodes
// ============================================================

// BinaryOp represents a binary operation (add, subtract, multiply, divide, etc.).
type BinaryOp struct {
	OpName string // "+", "-", "*", "/", "%", "^", "==", "!=", "<", ">", "<=", ">=", "&&", "||"
	Left   Node
	Right  Node
	Typ    MathType
}

func (n *BinaryOp) Type() MathType   { return n.Typ }
func (n *BinaryOp) Op() string       { return "BinaryOp:" + n.OpName }
func (n *BinaryOp) Operands() []Node { return []Node{n.Left, n.Right} }
func (n *BinaryOp) NumOperands() int { return 2 }

// UnaryOp represents a unary operation (negate, logical not, etc.).
type UnaryOp struct {
	OpName  string // "-", "!", "abs"
	Operand Node
	Typ     MathType
}

func (n *UnaryOp) Type() MathType   { return n.Typ }
func (n *UnaryOp) Op() string       { return "UnaryOp:" + n.OpName }
func (n *UnaryOp) Operands() []Node { return []Node{n.Operand} }
func (n *UnaryOp) NumOperands() int { return 1 }

// ============================================================
// Function Call Node
// ============================================================

// FuncCall represents a call to a mathematical function.
// The function registry is extensible — new functions can be added
// without redesigning the IR.
type FuncCall struct {
	Name string // "sqrt", "exp", "log", "sin", "cos", "tan", etc.
	Args []Node
	Typ  MathType
}

func (n *FuncCall) Type() MathType   { return n.Typ }
func (n *FuncCall) Op() string       { return "FuncCall:" + n.Name }
func (n *FuncCall) Operands() []Node { return n.Args }
func (n *FuncCall) NumOperands() int { return len(n.Args) }

// ============================================================
// Let Binding (for intermediate values)
// ============================================================

// Let binds a name to a value expression.
type Let struct {
	Name string
	Expr Node
	Body Node
	Typ  MathType
}

func (n *Let) Type() MathType   { return n.Typ }
func (n *Let) Op() string       { return "Let" }
func (n *Let) Operands() []Node { return []Node{n.Expr, n.Body} }
func (n *Let) NumOperands() int { return 2 }

// ============================================================
// Graph (top-level container)
// ============================================================

// Graph is a collection of named nodes forming a mathematical computation.
type Graph struct {
	Nodes   map[string]Node
	Inputs  []*Variable
	Outputs []Node
}

// NewGraph creates an empty graph.
func NewGraph() *Graph {
	return &Graph{
		Nodes:   make(map[string]Node),
		Inputs:  make([]*Variable, 0),
		Outputs: make([]Node, 0),
	}
}

// AddNode adds a named node to the graph.
func (g *Graph) AddNode(name string, node Node) {
	g.Nodes[name] = node
}

// AddInput adds an input variable to the graph.
func (g *Graph) AddInput(v *Variable) {
	g.Inputs = append(g.Inputs, v)
}

// AddOutput adds an output node to the graph.
func (g *Graph) AddOutput(n Node) {
	g.Outputs = append(g.Outputs, n)
}

// GetNode retrieves a node by name.
func (g *Graph) GetNode(name string) (Node, bool) {
	n, ok := g.Nodes[name]
	return n, ok
}

// NumNodes returns the number of nodes in the graph.
func (g *Graph) NumNodes() int {
	return len(g.Nodes)
}

// ============================================================
// Node visitor
// ============================================================

// Walk visits all nodes in the tree, calling fn for each.
func Walk(node Node, fn func(Node)) {
	if node == nil {
		return
	}
	fn(node)
	for _, child := range node.Operands() {
		Walk(child, fn)
	}
}

// Transform applies a transformation function to each node bottom-up.
// If fn returns a non-nil node, it replaces the original.
func Transform(node Node, fn func(Node) Node) Node {
	if node == nil {
		return nil
	}
	// Transform children first
	switch n := node.(type) {
	case *BinaryOp:
		left := Transform(n.Left, fn)
		right := Transform(n.Right, fn)
		if left != n.Left || right != n.Right {
			node = &BinaryOp{OpName: n.OpName, Left: left, Right: right, Typ: n.Typ}
		}
	case *UnaryOp:
		operand := Transform(n.Operand, fn)
		if operand != n.Operand {
			node = &UnaryOp{OpName: n.OpName, Operand: operand, Typ: n.Typ}
		}
	case *FuncCall:
		args := make([]Node, len(n.Args))
		changed := false
		for i, arg := range n.Args {
			args[i] = Transform(arg, fn)
			if args[i] != arg {
				changed = true
			}
		}
		if changed {
			node = &FuncCall{Name: n.Name, Args: args, Typ: n.Typ}
		}
	case *Let:
		expr := Transform(n.Expr, fn)
		body := Transform(n.Body, fn)
		if expr != n.Expr || body != n.Body {
			node = &Let{Name: n.Name, Expr: expr, Body: body, Typ: n.Typ}
		}
	}
	return fn(node)
}

// ============================================================
// Utilities
// ============================================================

// IsConstant returns true if the node is a compile-time constant.
func IsConstant(node Node) bool {
	switch node.(type) {
	case *IntConst, *FloatConst, *BoolConst, *StringConst:
		return true
	}
	return false
}

// IsPure returns true if the node has no side effects.
func IsPure(node Node) bool {
	switch n := node.(type) {
	case *IntConst, *FloatConst, *BoolConst, *StringConst:
		return true
	case *Variable, *Param:
		return true
	case *BinaryOp:
		return IsPure(n.Left) && IsPure(n.Right)
	case *UnaryOp:
		return IsPure(n.Operand)
	case *FuncCall:
		for _, arg := range n.Args {
			if !IsPure(arg) {
				return false
			}
		}
		return true
	case *Let:
		return IsPure(n.Expr) && IsPure(n.Body)
	}
	return false
}

// NodeType returns a human-readable name for the node type.
func NodeType(node Node) string {
	if node == nil {
		return "<nil>"
	}
	return fmt.Sprintf("%T", node)
}

// Summary returns a summary of the node.
func Summary(node Node) string {
	if node == nil {
		return "<nil>"
	}
	parts := []string{node.Op(), "→", node.Type().String()}
	if node.NumOperands() > 0 {
		parts = append(parts, fmt.Sprintf("(%d operands)", node.NumOperands()))
	}
	return strings.Join(parts, " ")
}
