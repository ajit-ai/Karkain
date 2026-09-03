package math

import "math"

// Value represents a runtime value during evaluation.
type Value struct {
	Int    int64
	Float  float64
	Bool   bool
	String string
	Typ    MathType
}

// Eval evaluates a Math IR node with the given variable bindings.
// This is a reference evaluator for testing correctness, not for production.
func Eval(node Node, env map[string]Value) (Value, bool) {
	if node == nil {
		return Value{}, false
	}

	switch n := node.(type) {
	case *IntConst:
		return Value{Int: n.Value, Typ: n.Typ}, true
	case *FloatConst:
		return Value{Float: n.Value, Typ: n.Typ}, true
	case *BoolConst:
		return Value{Bool: n.Value, Typ: Bool}, true
	case *StringConst:
		return Value{String: n.Value, Typ: StringType}, true
	case *Variable:
		v, ok := env[n.Name]
		return v, ok
	case *Param:
		v, ok := env[n.Name]
		return v, ok
	case *BinaryOp:
		return evalBinary(n, env)
	case *UnaryOp:
		return evalUnary(n, env)
	case *FuncCall:
		return evalFuncCall(n, env)
	case *Let:
		expr, ok := Eval(n.Expr, env)
		if !ok {
			return Value{}, false
		}
		newEnv := copyEnv(env)
		newEnv[n.Name] = expr
		return Eval(n.Body, newEnv)
	}
	return Value{}, false
}

func evalBinary(n *BinaryOp, env map[string]Value) (Value, bool) {
	left, ok := Eval(n.Left, env)
	if !ok {
		return Value{}, false
	}
	right, ok := Eval(n.Right, env)
	if !ok {
		return Value{}, false
	}

	// Bool-to-bool operations
	if n.Typ == Bool && left.Typ == Bool && right.Typ == Bool {
		return evalBoolBinary(n.OpName, left, right)
	}

	// Float operations
	if left.Typ.IsFloat() || right.Typ.IsFloat() {
		lf := toFloat(left)
		rf := toFloat(right)
		return evalFloatBinary(n.OpName, lf, rf, n.Typ)
	}

	// Int operations (includes comparisons returning Bool)
	return evalIntBinary(n.OpName, left, right, n.Typ)
}

func evalBoolBinary(op string, left, right Value) (Value, bool) {
	switch op {
	case "==":
		return Value{Bool: left.Bool == right.Bool, Typ: Bool}, true
	case "!=":
		return Value{Bool: left.Bool != right.Bool, Typ: Bool}, true
	case "&&":
		return Value{Bool: left.Bool && right.Bool, Typ: Bool}, true
	case "||":
		return Value{Bool: left.Bool || right.Bool, Typ: Bool}, true
	}
	return Value{}, false
}

func evalIntBinary(op string, left, right Value, typ MathType) (Value, bool) {
	a, b := left.Int, right.Int
	var result int64
	switch op {
	case "+":
		result = a + b
	case "-":
		result = a - b
	case "*":
		result = a * b
	case "/":
		if b == 0 {
			return Value{}, false
		}
		result = a / b
	case "%":
		if b == 0 {
			return Value{}, false
		}
		result = a % b
	case "==":
		return Value{Bool: a == b, Typ: Bool}, true
	case "!=":
		return Value{Bool: a != b, Typ: Bool}, true
	case "<":
		return Value{Bool: a < b, Typ: Bool}, true
	case ">":
		return Value{Bool: a > b, Typ: Bool}, true
	case "<=":
		return Value{Bool: a <= b, Typ: Bool}, true
	case ">=":
		return Value{Bool: a >= b, Typ: Bool}, true
	default:
		return Value{}, false
	}
	return Value{Int: result, Typ: typ}, true
}

func evalFloatBinary(op string, left, right float64, typ MathType) (Value, bool) {
	var result float64
	switch op {
	case "+":
		result = left + right
	case "-":
		result = left - right
	case "*":
		result = left * right
	case "/":
		if right == 0 {
			return Value{}, false
		}
		result = left / right
	case "^":
		result = math.Pow(left, right)
	case "==":
		return Value{Bool: left == right, Typ: Bool}, true
	case "!=":
		return Value{Bool: left != right, Typ: Bool}, true
	case "<":
		return Value{Bool: left < right, Typ: Bool}, true
	case ">":
		return Value{Bool: left > right, Typ: Bool}, true
	case "<=":
		return Value{Bool: left <= right, Typ: Bool}, true
	case ">=":
		return Value{Bool: left >= right, Typ: Bool}, true
	default:
		return Value{}, false
	}
	return Value{Float: result, Typ: typ}, true
}

func evalUnary(n *UnaryOp, env map[string]Value) (Value, bool) {
	operand, ok := Eval(n.Operand, env)
	if !ok {
		return Value{}, false
	}

	switch n.OpName {
	case "-":
		if operand.Typ.IsFloat() {
			return Value{Float: -toFloat(operand), Typ: operand.Typ}, true
		}
		return Value{Int: -operand.Int, Typ: operand.Typ}, true
	case "!":
		return Value{Bool: !operand.Bool, Typ: Bool}, true
	}
	return Value{}, false
}

func evalFuncCall(n *FuncCall, env map[string]Value) (Value, bool) {
	args := make([]Value, len(n.Args))
	for i, arg := range n.Args {
		v, ok := Eval(arg, env)
		if !ok {
			return Value{}, false
		}
		args[i] = v
	}

	if len(args) == 1 {
		return evalSingleArgFunc(n.Name, args[0])
	}
	if len(args) == 2 {
		return evalDualArgFunc(n.Name, args[0], args[1])
	}
	return Value{}, false
}

func evalSingleArgFunc(name string, arg Value) (Value, bool) {
	f := toFloat(arg)
	var result float64
	switch name {
	case "sqrt":
		if f < 0 {
			return Value{}, false
		}
		result = math.Sqrt(f)
	case "exp":
		result = math.Exp(f)
	case "log":
		if f <= 0 {
			return Value{}, false
		}
		result = math.Log(f)
	case "log2":
		if f <= 0 {
			return Value{}, false
		}
		result = math.Log2(f)
	case "log10":
		if f <= 0 {
			return Value{}, false
		}
		result = math.Log10(f)
	case "sin":
		result = math.Sin(f)
	case "cos":
		result = math.Cos(f)
	case "tan":
		result = math.Tan(f)
	case "asin":
		if f < -1 || f > 1 {
			return Value{}, false
		}
		result = math.Asin(f)
	case "acos":
		if f < -1 || f > 1 {
			return Value{}, false
		}
		result = math.Acos(f)
	case "atan":
		result = math.Atan(f)
	case "abs":
		result = math.Abs(f)
	case "floor":
		result = math.Floor(f)
	case "ceil":
		result = math.Ceil(f)
	case "round":
		result = math.Round(f)
	default:
		return Value{}, false
	}
	return Value{Float: result, Typ: arg.Typ}, true
}

func evalDualArgFunc(name string, a, b Value) (Value, bool) {
	af := toFloat(a)
	bf := toFloat(b)
	var result float64
	switch name {
	case "pow":
		result = math.Pow(af, bf)
	case "atan2":
		result = math.Atan2(af, bf)
	case "min":
		if af < bf {
			result = af
		} else {
			result = bf
		}
	case "max":
		if af > bf {
			result = af
		} else {
			result = bf
		}
	case "mod":
		if bf == 0 {
			return Value{}, false
		}
		result = math.Mod(af, bf)
	default:
		return Value{}, false
	}
	typ := PromoteType(a.Typ, b.Typ)
	return Value{Float: result, Typ: typ}, true
}

func toFloat(v Value) float64 {
	if v.Typ.IsFloat() {
		return v.Float
	}
	return float64(v.Int)
}

func copyEnv(env map[string]Value) map[string]Value {
	newEnv := make(map[string]Value, len(env))
	for k, v := range env {
		newEnv[k] = v
	}
	return newEnv
}
