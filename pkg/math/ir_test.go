package math

import (
	"testing"
)

func TestIntConst(t *testing.T) {
	n := Int(42)
	if n.Value != 42 {
		t.Errorf("expected 42, got %d", n.Value)
	}
	if n.Type() != Int64 {
		t.Errorf("expected Int64, got %s", n.Type())
	}
	if n.Op() != "IntConst" {
		t.Errorf("expected IntConst, got %s", n.Op())
	}
	if n.NumOperands() != 0 {
		t.Errorf("expected 0 operands, got %d", n.NumOperands())
	}
}

func TestFloatConst(t *testing.T) {
	n := Float64v(3.14)
	if n.Value != 3.14 {
		t.Errorf("expected 3.14, got %f", n.Value)
	}
	if n.Type() != Float64 {
		t.Errorf("expected Float64, got %s", n.Type())
	}
}

func TestBoolConst(t *testing.T) {
	n := Boolv(true)
	if !n.Value {
		t.Error("expected true")
	}
	if n.Type() != Bool {
		t.Errorf("expected Bool, got %s", n.Type())
	}
}

func TestVariable(t *testing.T) {
	v := Var("x", Float64)
	if v.Name != "x" {
		t.Errorf("expected x, got %s", v.Name)
	}
	if v.Type() != Float64 {
		t.Errorf("expected Float64, got %s", v.Type())
	}
}

func TestAdd(t *testing.T) {
	a := Int(2)
	b := Int(3)
	node := Add(a, b)
	if node.Type() != Int64 {
		t.Errorf("expected Int64, got %s", node.Type())
	}
	if node.OpName != "+" {
		t.Errorf("expected +, got %s", node.OpName)
	}
}

func TestAddFloatInt(t *testing.T) {
	a := Float64v(1.5)
	b := Int(3)
	node := Add(a, b)
	if node.Type() != Float64 {
		t.Errorf("expected Float64, got %s", node.Type())
	}
}

func TestMul(t *testing.T) {
	a := Int(4)
	b := Int(5)
	node := Mul(a, b)
	if node.Type() != Int64 {
		t.Errorf("expected Int64, got %s", node.Type())
	}
}

func TestComparison(t *testing.T) {
	a := Int(3)
	b := Int(5)
	node := Lt(a, b)
	if node.Type() != Bool {
		t.Errorf("expected Bool, got %s", node.Type())
	}
	if node.OpName != "<" {
		t.Errorf("expected <, got %s", node.OpName)
	}
}

func TestLogicalNot(t *testing.T) {
	b := Boolv(true)
	node := Not(b)
	if node.Type() != Bool {
		t.Errorf("expected Bool, got %s", node.Type())
	}
}

func TestFuncCall(t *testing.T) {
	arg := Float64v(4.0)
	node := Call("sqrt", arg)
	if node.Type() != Float64 {
		t.Errorf("expected Float64, got %s", node.Type())
	}
	if node.Name != "sqrt" {
		t.Errorf("expected sqrt, got %s", node.Name)
	}
}

func TestFuncCallInvalid(t *testing.T) {
	arg := Int(4)
	node := Call("sqrt", arg)
	// Should still build, type inference from arg
	if node.Name != "sqrt" {
		t.Errorf("expected sqrt, got %s", node.Name)
	}
}

func TestLet(t *testing.T) {
	expr := Int(10)
	body := Add(Var("x", Int64), Int(5))
	node := Let_("x", expr, body)
	if node.Name != "x" {
		t.Errorf("expected x, got %s", node.Name)
	}
}

func TestPrintFlat(t *testing.T) {
	node := Add(Int(2), Mul(Int(3), Int(4)))
	got := PrintFlat(node)
	want := "(2 + (3 * 4))"
	if got != want {
		t.Errorf("expected %s, got %s", want, got)
	}
}

func TestPrintC(t *testing.T) {
	node := Add(Int(2), Mul(Int(3), Int(4)))
	got := PrintC(node)
	want := "(2 + (3 * 4))"
	if got != want {
		t.Errorf("expected %s, got %s", want, got)
	}
}

func TestPrintCMathFunc(t *testing.T) {
	arg := Float64v(9.0)
	node := Call("sqrt", arg)
	got := PrintC(node)
	want := "sqrt(9)"
	if got != want {
		t.Errorf("expected %s, got %s", want, got)
	}
}

func TestOptimizeConstFold(t *testing.T) {
	node := Add(Int(2), Int(3))
	opt := Optimize(node)
	ic, ok := opt.(*IntConst)
	if !ok {
		t.Fatalf("expected IntConst, got %T", opt)
	}
	if ic.Value != 5 {
		t.Errorf("expected 5, got %d", ic.Value)
	}
}

func TestOptimizeAddZero(t *testing.T) {
	x := Var("x", Int64)
	node := Add(x, Int(0))
	opt := Optimize(node)
	v, ok := opt.(*Variable)
	if !ok {
		t.Fatalf("expected Variable, got %T", opt)
	}
	if v.Name != "x" {
		t.Errorf("expected x, got %s", v.Name)
	}
}

func TestOptimizeMulOne(t *testing.T) {
	x := Var("x", Float64)
	node := Mul(x, Float64v(1.0))
	opt := Optimize(node)
	v, ok := opt.(*Variable)
	if !ok {
		t.Fatalf("expected Variable, got %T", opt)
	}
	if v.Name != "x" {
		t.Errorf("expected x, got %s", v.Name)
	}
}

func TestOptimizeMulZero(t *testing.T) {
	x := Var("x", Int64)
	node := Mul(x, Int(0))
	opt := Optimize(node)
	ic, ok := opt.(*IntConst)
	if !ok {
		t.Fatalf("expected IntConst, got %T", opt)
	}
	if ic.Value != 0 {
		t.Errorf("expected 0, got %d", ic.Value)
	}
}

func TestOptimizeDoubleNeg(t *testing.T) {
	x := Var("x", Float64)
	node := Neg(Neg(x))
	opt := Optimize(node)
	v, ok := opt.(*Variable)
	if !ok {
		t.Fatalf("expected Variable, got %T", opt)
	}
	if v.Name != "x" {
		t.Errorf("expected x, got %s", v.Name)
	}
}

func TestOptimizePowZero(t *testing.T) {
	x := Var("x", Float64)
	node := Pow(x, Float64v(0.0))
	opt := Optimize(node)
	fc, ok := opt.(*FloatConst)
	if !ok {
		t.Fatalf("expected FloatConst, got %T", opt)
	}
	if fc.Value != 1.0 {
		t.Errorf("expected 1.0, got %f", fc.Value)
	}
}

func TestOptimizePowOne(t *testing.T) {
	x := Var("x", Float64)
	node := Pow(x, Float64v(1.0))
	opt := Optimize(node)
	v, ok := opt.(*Variable)
	if !ok {
		t.Fatalf("expected Variable, got %T", opt)
	}
	if v.Name != "x" {
		t.Errorf("expected x, got %s", v.Name)
	}
}

func TestOptimizeNestedConstFold(t *testing.T) {
	// (2 + 3) * (4 + 1) = 5 * 5 = 25
	node := Mul(Add(Int(2), Int(3)), Add(Int(4), Int(1)))
	opt := Optimize(node)
	ic, ok := opt.(*IntConst)
	if !ok {
		t.Fatalf("expected IntConst, got %T", opt)
	}
	if ic.Value != 25 {
		t.Errorf("expected 25, got %d", ic.Value)
	}
}

func TestOptimizeFuncFold(t *testing.T) {
	node := Call("sqrt", Float64v(4.0))
	opt := Optimize(node)
	fc, ok := opt.(*FloatConst)
	if !ok {
		t.Fatalf("expected FloatConst, got %T", opt)
	}
	if fc.Value != 2.0 {
		t.Errorf("expected 2.0, got %f", fc.Value)
	}
}

func TestOptimizeFuncFoldTrig(t *testing.T) {
	node := Call("sin", Float64v(0.0))
	opt := Optimize(node)
	fc, ok := opt.(*FloatConst)
	if !ok {
		t.Fatalf("expected FloatConst, got %T", opt)
	}
	if fc.Value != 0.0 {
		t.Errorf("expected 0.0, got %f", fc.Value)
	}
}

func TestOptimizeNoChange(t *testing.T) {
	x := Var("x", Int64)
	y := Var("y", Int64)
	node := Add(x, y)
	opt := Optimize(node)
	bin, ok := opt.(*BinaryOp)
	if !ok {
		t.Fatalf("expected BinaryOp, got %T", opt)
	}
	if bin.OpName != "+" {
		t.Errorf("expected +, got %s", bin.OpName)
	}
}

// ============================================================
// Eval Tests
// ============================================================

func TestEvalIntAdd(t *testing.T) {
	node := Add(Int(2), Int(3))
	v, ok := Eval(node, nil)
	if !ok {
		t.Fatal("eval failed")
	}
	if v.Int != 5 {
		t.Errorf("expected 5, got %d", v.Int)
	}
}

func TestEvalFloatMul(t *testing.T) {
	node := Mul(Float64v(3.0), Float64v(4.0))
	v, ok := Eval(node, nil)
	if !ok {
		t.Fatal("eval failed")
	}
	if v.Float != 12.0 {
		t.Errorf("expected 12.0, got %f", v.Float)
	}
}

func TestEvalVariable(t *testing.T) {
	node := Add(Var("x", Int64), Int(10))
	env := map[string]Value{"x": {Int: 5, Typ: Int64}}
	v, ok := Eval(node, env)
	if !ok {
		t.Fatal("eval failed")
	}
	if v.Int != 15 {
		t.Errorf("expected 15, got %d", v.Int)
	}
}

func TestEvalComparison(t *testing.T) {
	node := Lt(Int(3), Int(5))
	v, ok := Eval(node, nil)
	if !ok {
		t.Fatal("eval failed")
	}
	if !v.Bool {
		t.Error("expected true")
	}
}

func TestEvalNested(t *testing.T) {
	// 2 + 3 * 4 = 14
	node := Add(Int(2), Mul(Int(3), Int(4)))
	v, ok := Eval(node, nil)
	if !ok {
		t.Fatal("eval failed")
	}
	if v.Int != 14 {
		t.Errorf("expected 14, got %d", v.Int)
	}
}

func TestEvalFuncSqrt(t *testing.T) {
	node := Call("sqrt", Float64v(9.0))
	v, ok := Eval(node, nil)
	if !ok {
		t.Fatal("eval failed")
	}
	if v.Float != 3.0 {
		t.Errorf("expected 3.0, got %f", v.Float)
	}
}

func TestEvalFuncExp(t *testing.T) {
	node := Call("exp", Float64v(0.0))
	v, ok := Eval(node, nil)
	if !ok {
		t.Fatal("eval failed")
	}
	if v.Float != 1.0 {
		t.Errorf("expected 1.0, got %f", v.Float)
	}
}

func TestEvalFuncLog(t *testing.T) {
	node := Call("log", Float64v(1.0))
	v, ok := Eval(node, nil)
	if !ok {
		t.Fatal("eval failed")
	}
	if v.Float != 0.0 {
		t.Errorf("expected 0.0, got %f", v.Float)
	}
}

func TestEvalFuncPow(t *testing.T) {
	node := Call("pow", Float64v(2.0), Float64v(3.0))
	v, ok := Eval(node, nil)
	if !ok {
		t.Fatal("eval failed")
	}
	if v.Float != 8.0 {
		t.Errorf("expected 8.0, got %f", v.Float)
	}
}

func TestEvalFuncMin(t *testing.T) {
	node := Call("min", Float64v(3.0), Float64v(7.0))
	v, ok := Eval(node, nil)
	if !ok {
		t.Fatal("eval failed")
	}
	if v.Float != 3.0 {
		t.Errorf("expected 3.0, got %f", v.Float)
	}
}

func TestEvalFuncMax(t *testing.T) {
	node := Call("max", Float64v(3.0), Float64v(7.0))
	v, ok := Eval(node, nil)
	if !ok {
		t.Fatal("eval failed")
	}
	if v.Float != 7.0 {
		t.Errorf("expected 7.0, got %f", v.Float)
	}
}

func TestEvalLet(t *testing.T) {
	expr := Int(42)
	body := Add(Var("x", Int64), Int(8))
	node := Let_("x", expr, body)
	v, ok := Eval(node, nil)
	if !ok {
		t.Fatal("eval failed")
	}
	if v.Int != 50 {
		t.Errorf("expected 50, got %d", v.Int)
	}
}

func TestEvalDivByZero(t *testing.T) {
	node := Div(Int(1), Int(0))
	_, ok := Eval(node, nil)
	if ok {
		t.Error("expected eval to fail for division by zero")
	}
}

func TestEvalPowZero(t *testing.T) {
	node := Pow(Float64v(5.0), Float64v(0.0))
	v, ok := Eval(node, nil)
	if !ok {
		t.Fatal("eval failed")
	}
	if v.Float != 1.0 {
		t.Errorf("expected 1.0, got %f", v.Float)
	}
}

func TestEvalUnaryNeg(t *testing.T) {
	node := Neg(Int(7))
	v, ok := Eval(node, nil)
	if !ok {
		t.Fatal("eval failed")
	}
	if v.Int != -7 {
		t.Errorf("expected -7, got %d", v.Int)
	}
}

func TestEvalLogicalNot(t *testing.T) {
	node := Not(Boolv(false))
	v, ok := Eval(node, nil)
	if !ok {
		t.Fatal("eval failed")
	}
	if !v.Bool {
		t.Error("expected true")
	}
}

// ============================================================
// Type Promotion Tests
// ============================================================

func TestPromoteTypeIntInt(t *testing.T) {
	r := PromoteType(Int32, Int64)
	if r != Int64 {
		t.Errorf("expected Int64, got %s", r)
	}
}

func TestPromoteTypeIntFloat(t *testing.T) {
	r := PromoteType(Int64, Float32)
	if r != Float64 {
		t.Errorf("expected Float64, got %s", r)
	}
}

func TestPromoteTypeFloatFloat(t *testing.T) {
	r := PromoteType(Float32, Float64)
	if r != Float64 {
		t.Errorf("expected Float64, got %s", r)
	}
}

func TestPromoteTypeFloatComplex(t *testing.T) {
	r := PromoteType(Float64, Complex64)
	if r != Complex128 {
		t.Errorf("expected Complex128, got %s", r)
	}
}

func TestPromoteTypeSame(t *testing.T) {
	r := PromoteType(Int32, Int32)
	if r != Int32 {
		t.Errorf("expected Int32, got %s", r)
	}
}

// ============================================================
// Graph Tests
// ============================================================

func TestGraph(t *testing.T) {
	gb := NewGraphBuilder()
	g := gb.Input("x", Float64).
		Input("y", Float64).
		Node("sum", Add(Var("x", Float64), Var("y", Float64))).
		Output(Var("sum", Float64)).
		Build()

	if g.NumNodes() != 1 {
		t.Errorf("expected 1 node, got %d", g.NumNodes())
	}
	if len(g.Inputs) != 2 {
		t.Errorf("expected 2 inputs, got %d", len(g.Inputs))
	}
	if len(g.Outputs) != 1 {
		t.Errorf("expected 1 output, got %d", len(g.Outputs))
	}
}

func TestWalk(t *testing.T) {
	node := Add(Int(1), Mul(Int(2), Int(3)))
	count := 0
	Walk(node, func(n Node) {
		count++
	})
	if count != 5 { // Add, 1, Mul, 2, 3
		t.Errorf("expected 5 nodes visited, got %d", count)
	}
}

func TestIsConstant(t *testing.T) {
	if !IsConstant(Int(5)) {
		t.Error("expected Int to be constant")
	}
	if !IsConstant(Float64v(3.14)) {
		t.Error("expected Float to be constant")
	}
	if IsConstant(Var("x", Int64)) {
		t.Error("expected Variable to not be constant")
	}
}

func TestIsPure(t *testing.T) {
	if !IsPure(Add(Int(1), Int(2))) {
		t.Error("expected Add to be pure")
	}
	if !IsPure(Var("x", Int64)) {
		t.Error("expected Var to be pure")
	}
	if !IsPure(Call("sqrt", Float64v(4.0))) {
		t.Error("expected Call to be pure")
	}
}

// ============================================================
// Validate Tests
// ============================================================

func TestValidateValid(t *testing.T) {
	g := NewGraph()
	g.AddNode("x", Var("x", Float64))
	g.AddNode("sum", Add(Int(2), Int(3)))
	g.AddInput(Var("x", Float64))
	g.AddOutput(Var("sum", Float64))

	errs := Validate(g)
	if len(errs) != 0 {
		t.Errorf("expected no errors, got %d: %v", len(errs), errs)
	}
}

func TestValidateDivisionByZero(t *testing.T) {
	g := NewGraph()
	g.AddNode("div", Div(Int(1), Int(0)))

	errs := Validate(g)
	if len(errs) == 0 {
		t.Error("expected division by zero error")
	}
}

func TestValidateBadLogicalNot(t *testing.T) {
	g := NewGraph()
	g.AddNode("not", Not(Int(1)))

	errs := Validate(g)
	if len(errs) == 0 {
		t.Error("expected type error for NOT on int")
	}
}

func TestPrintGraph(t *testing.T) {
	gb := NewGraphBuilder()
	g := gb.Input("x", Float64).
		Node("result", Call("sqrt", Var("x", Float64))).
		Output(Var("result", Float64)).
		Build()

	out := PrintGraph(g)
	if out == "" {
		t.Error("expected non-empty output")
	}
}

// ============================================================
// Transform Tests
// ============================================================

func TestTransform(t *testing.T) {
	// Replace all Int(3) with Int(7)
	node := Add(Int(3), Mul(Int(3), Int(5)))
	result := Transform(node, func(n Node) Node {
		if ic, ok := n.(*IntConst); ok && ic.Value == 3 {
			return Int(7)
		}
		return n
	})
	got := PrintFlat(result)
	want := "(7 + (7 * 5))"
	if got != want {
		t.Errorf("expected %s, got %s", want, got)
	}
}

func TestTransformNil(t *testing.T) {
	result := Transform(nil, func(n Node) Node {
		return n
	})
	if result != nil {
		t.Error("expected nil")
	}
}

func TestWalkNil(t *testing.T) {
	count := 0
	Walk(nil, func(n Node) {
		count++
	})
	if count != 0 {
		t.Errorf("expected 0, got %d", count)
	}
}
