package math

// Builder provides a convenient API for constructing Math IR nodes.
// All nodes are allocated on the heap and returned as interfaces.

// ============================================================
// Constants
// ============================================================

// Int creates an integer constant.
func Int(val int64) *IntConst {
	return &IntConst{Value: val, Typ: Int64}
}

// Int32v creates a 32-bit integer constant.
func Int32v(val int32) *IntConst {
	return &IntConst{Value: int64(val), Typ: Int32}
}

// Float creates a float64 constant.
func Float64v(val float64) *FloatConst {
	return &FloatConst{Value: val, Typ: Float64}
}

// Float32v creates a float32 constant.
func Float32v(val float32) *FloatConst {
	return &FloatConst{Value: float64(val), Typ: Float32}
}

// Bool creates a boolean constant.
func Boolv(val bool) *BoolConst {
	return &BoolConst{Value: val}
}

// Stringv creates a string constant.
func Stringv(val string) *StringConst {
	return &StringConst{Value: val}
}

// ============================================================
// Variables
// ============================================================

// Var creates a variable with a given name and type.
func Var(name string, typ MathType) *Variable {
	return &Variable{Name: name, Typ: typ}
}

// Param_ creates a function parameter with a given name and type.
// Named Param_ to avoid collision with the Param struct field.
func Param_(name string, typ MathType) *Param {
	return &Param{Name: name, Typ: typ}
}

// ============================================================
// Arithmetic
// ============================================================

// Add creates an addition node.
func Add(left, right Node) *BinaryOp {
	return &BinaryOp{OpName: "+", Left: left, Right: right, Typ: PromoteType(left.Type(), right.Type())}
}

// Sub creates a subtraction node.
func Sub(left, right Node) *BinaryOp {
	return &BinaryOp{OpName: "-", Left: left, Right: right, Typ: PromoteType(left.Type(), right.Type())}
}

// Mul creates a multiplication node.
func Mul(left, right Node) *BinaryOp {
	return &BinaryOp{OpName: "*", Left: left, Right: right, Typ: PromoteType(left.Type(), right.Type())}
}

// Div creates a division node.
func Div(left, right Node) *BinaryOp {
	return &BinaryOp{OpName: "/", Left: left, Right: right, Typ: PromoteType(left.Type(), right.Type())}
}

// Mod creates a modulo node.
func Mod(left, right Node) *BinaryOp {
	return &BinaryOp{OpName: "%", Left: left, Right: right, Typ: PromoteType(left.Type(), right.Type())}
}

// Pow creates an exponentiation node.
func Pow(base, exp Node) *BinaryOp {
	return &BinaryOp{OpName: "^", Left: base, Right: exp, Typ: PromoteType(base.Type(), exp.Type())}
}

// Neg creates a negation node.
func Neg(operand Node) *UnaryOp {
	return &UnaryOp{OpName: "-", Operand: operand, Typ: operand.Type()}
}

// Not creates a logical NOT node.
func Not(operand Node) *UnaryOp {
	return &UnaryOp{OpName: "!", Operand: operand, Typ: Bool}
}

// ============================================================
// Comparisons (return Bool)
// ============================================================

// Eq creates an equality comparison node.
func Eq(left, right Node) *BinaryOp {
	return &BinaryOp{OpName: "==", Left: left, Right: right, Typ: Bool}
}

// Neq creates an inequality comparison node.
func Neq(left, right Node) *BinaryOp {
	return &BinaryOp{OpName: "!=", Left: left, Right: right, Typ: Bool}
}

// Lt creates a less-than comparison node.
func Lt(left, right Node) *BinaryOp {
	return &BinaryOp{OpName: "<", Left: left, Right: right, Typ: Bool}
}

// Gt creates a greater-than comparison node.
func Gt(left, right Node) *BinaryOp {
	return &BinaryOp{OpName: ">", Left: left, Right: right, Typ: Bool}
}

// Lte creates a less-or-equal comparison node.
func Lte(left, right Node) *BinaryOp {
	return &BinaryOp{OpName: "<=", Left: left, Right: right, Typ: Bool}
}

// Gte creates a greater-or-equal comparison node.
func Gte(left, right Node) *BinaryOp {
	return &BinaryOp{OpName: ">=", Left: left, Right: right, Typ: Bool}
}

// ============================================================
// Logical
// ============================================================

// And creates a logical AND node.
func And(left, right Node) *BinaryOp {
	return &BinaryOp{OpName: "&&", Left: left, Right: right, Typ: Bool}
}

// Or creates a logical OR node.
func Or(left, right Node) *BinaryOp {
	return &BinaryOp{OpName: "||", Left: left, Right: right, Typ: Bool}
}

// ============================================================
// Function Calls
// ============================================================

// Call creates a function call node.
func Call(name string, args ...Node) *FuncCall {
	typ := resolveFuncType(name, args)
	return &FuncCall{Name: name, Args: args, Typ: typ}
}

// resolveFuncType determines the return type of a function call.
func resolveFuncType(name string, args []Node) MathType {
	sig, ok := LookupFunc(name)
	if !ok {
		if len(args) > 0 {
			return args[0].Type()
		}
		return Unknown
	}
	if sig.RetType != Unknown {
		return sig.RetType
	}
	// Infer from first numeric arg
	for _, arg := range args {
		if arg.Type().IsNumeric() {
			return arg.Type()
		}
	}
	return Float64
}

// ============================================================
// Let Binding
// ============================================================

// Let creates a let binding.
func Let_(name string, expr, body Node) *Let {
	return &Let{Name: name, Expr: expr, Body: body, Typ: body.Type()}
}

// ============================================================
// Graph Builder
// ============================================================

// GraphBuilder provides a fluent API for building math graphs.
type GraphBuilder struct {
	graph *Graph
}

// NewGraphBuilder creates a new graph builder.
func NewGraphBuilder() *GraphBuilder {
	return &GraphBuilder{graph: NewGraph()}
}

// Input adds an input variable to the graph.
func (gb *GraphBuilder) Input(name string, typ MathType) *GraphBuilder {
	v := Var(name, typ)
	gb.graph.AddInput(v)
	return gb
}

// Node adds a named node to the graph.
func (gb *GraphBuilder) Node(name string, node Node) *GraphBuilder {
	gb.graph.AddNode(name, node)
	return gb
}

// Output marks a node as an output of the graph.
func (gb *GraphBuilder) Output(node Node) *GraphBuilder {
	gb.graph.AddOutput(node)
	return gb
}

// Build returns the constructed graph.
func (gb *GraphBuilder) Build() *Graph {
	return gb.graph
}
