package math

import "math"

// Optimize applies safe algebraic optimizations to a Math IR node.
// Optimizations are semantics-preserving and deterministic.
func Optimize(node Node) Node {
	if node == nil {
		return nil
	}
	return Transform(node, func(n Node) Node {
		if n == nil {
			return nil
		}
		if bin, ok := n.(*BinaryOp); ok {
			return optimizeBinary(bin)
		}
		if un, ok := n.(*UnaryOp); ok {
			return optimizeUnary(un)
		}
		if fc, ok := n.(*FuncCall); ok {
			return optimizeFuncCall(fc)
		}
		return n
	})
}

func optimizeBinary(n *BinaryOp) Node {
	// Constant folding
	if IsConstant(n.Left) && IsConstant(n.Right) {
		if folded := foldBinary(n); folded != nil {
			return folded
		}
	}

	switch n.OpName {
	case "+":
		// x + 0 = x
		if isZero(n.Right) {
			return n.Left
		}
		// 0 + x = x
		if isZero(n.Left) {
			return n.Right
		}

	case "*":
		// x * 0 = 0
		if isZero(n.Right) {
			return zeroForType(n.Typ)
		}
		// 0 * x = 0
		if isZero(n.Left) {
			return zeroForType(n.Typ)
		}
		// x * 1 = x
		if isOne(n.Right) {
			return n.Left
		}
		// 1 * x = x
		if isOne(n.Left) {
			return n.Right
		}

	case "-":
		// x - 0 = x
		if isZero(n.Right) {
			return n.Left
		}

	case "/":
		// x / 1 = x
		if isOne(n.Right) {
			return n.Left
		}

	case "^":
		// x ^ 0 = 1
		if isZero(n.Right) {
			return oneForType(n.Typ)
		}
		// x ^ 1 = x
		if isOne(n.Right) {
			return n.Left
		}
		// 0 ^ x = 0 (for x > 0)
		if isZero(n.Left) {
			return zeroForType(n.Typ)
		}
		// 1 ^ x = 1
		if isOne(n.Left) {
			return oneForType(n.Typ)
		}
	}

	return n
}

func optimizeUnary(n *UnaryOp) Node {
	// Double negation
	if n.OpName == "-" {
		if inner, ok := n.Operand.(*UnaryOp); ok && inner.OpName == "-" {
			return inner.Operand
		}
	}
	// Double logical NOT
	if n.OpName == "!" {
		if inner, ok := n.Operand.(*UnaryOp); ok && inner.OpName == "!" {
			return inner.Operand
		}
	}
	return n
}

func optimizeFuncCall(n *FuncCall) Node {
	// Constant fold single-argument functions
	if len(n.Args) == 1 && IsConstant(n.Args[0]) {
		if folded := foldFuncCall(n); folded != nil {
			return folded
		}
	}
	return n
}

// ============================================================
// Constant folding helpers
// ============================================================

func foldBinary(n *BinaryOp) Node {
	left := n.Left
	right := n.Right

	ic, lok := left.(*IntConst)
	jc, rok := right.(*IntConst)
	if lok && rok {
		return foldIntBinary(n.OpName, ic, jc)
	}

	fc, lok := left.(*FloatConst)
	gc, rok := right.(*FloatConst)
	if lok && rok {
		return foldFloatBinary(n.OpName, fc, gc)
	}

	return nil
}

func foldIntBinary(op string, a, b *IntConst) *IntConst {
	var result int64
	switch op {
	case "+":
		result = a.Value + b.Value
	case "-":
		result = a.Value - b.Value
	case "*":
		result = a.Value * b.Value
	case "/":
		if b.Value == 0 {
			return nil
		}
		result = a.Value / b.Value
	case "%":
		if b.Value == 0 {
			return nil
		}
		result = a.Value % b.Value
	case "^":
		if b.Value < 0 {
			return nil
		}
		result = intPow(a.Value, b.Value)
	default:
		return nil
	}
	return &IntConst{Value: result, Typ: PromoteType(a.Typ, b.Typ)}
}

func foldFloatBinary(op string, a, b *FloatConst) *FloatConst {
	var result float64
	switch op {
	case "+":
		result = a.Value + b.Value
	case "-":
		result = a.Value - b.Value
	case "*":
		result = a.Value * b.Value
	case "/":
		if b.Value == 0.0 {
			return nil
		}
		result = a.Value / b.Value
	case "^":
		result = math.Pow(a.Value, b.Value)
	default:
		return nil
	}
	return &FloatConst{Value: result, Typ: PromoteType(a.Typ, b.Typ)}
}

func foldFuncCall(n *FuncCall) Node {
	arg := n.Args[0]
	fc, ok := arg.(*FloatConst)
	if !ok {
		return nil
	}
	var result float64
	switch n.Name {
	case "sqrt":
		if fc.Value < 0 {
			return nil
		}
		result = math.Sqrt(fc.Value)
	case "exp":
		result = math.Exp(fc.Value)
	case "log":
		if fc.Value <= 0 {
			return nil
		}
		result = math.Log(fc.Value)
	case "log2":
		if fc.Value <= 0 {
			return nil
		}
		result = math.Log2(fc.Value)
	case "log10":
		if fc.Value <= 0 {
			return nil
		}
		result = math.Log10(fc.Value)
	case "sin":
		result = math.Sin(fc.Value)
	case "cos":
		result = math.Cos(fc.Value)
	case "tan":
		result = math.Tan(fc.Value)
	case "asin":
		if fc.Value < -1 || fc.Value > 1 {
			return nil
		}
		result = math.Asin(fc.Value)
	case "acos":
		if fc.Value < -1 || fc.Value > 1 {
			return nil
		}
		result = math.Acos(fc.Value)
	case "atan":
		result = math.Atan(fc.Value)
	case "floor":
		result = math.Floor(fc.Value)
	case "ceil":
		result = math.Ceil(fc.Value)
	case "round":
		result = math.Round(fc.Value)
	default:
		return nil
	}
	return &FloatConst{Value: result, Typ: fc.Typ}
}

// ============================================================
// Zero/One helpers
// ============================================================

func isZero(n Node) bool {
	switch v := n.(type) {
	case *IntConst:
		return v.Value == 0
	case *FloatConst:
		return v.Value == 0.0
	}
	return false
}

func isOne(n Node) bool {
	switch v := n.(type) {
	case *IntConst:
		return v.Value == 1
	case *FloatConst:
		return v.Value == 1.0
	}
	return false
}

func zeroForType(t MathType) Node {
	switch t {
	case Int32:
		return &IntConst{Value: 0, Typ: Int32}
	case Int64:
		return &IntConst{Value: 0, Typ: Int64}
	case Float32:
		return &FloatConst{Value: 0.0, Typ: Float32}
	case Float64:
		return &FloatConst{Value: 0.0, Typ: Float64}
	default:
		return &IntConst{Value: 0, Typ: Int64}
	}
}

func oneForType(t MathType) Node {
	switch t {
	case Int32:
		return &IntConst{Value: 1, Typ: Int32}
	case Int64:
		return &IntConst{Value: 1, Typ: Int64}
	case Float32:
		return &FloatConst{Value: 1.0, Typ: Float32}
	case Float64:
		return &FloatConst{Value: 1.0, Typ: Float64}
	default:
		return &IntConst{Value: 1, Typ: Int64}
	}
}

func intPow(base, exp int64) int64 {
	result := int64(1)
	for exp > 0 {
		if exp%2 == 1 {
			result *= base
		}
		base *= base
		exp /= 2
	}
	return result
}
