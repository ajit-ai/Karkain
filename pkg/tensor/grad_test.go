package tensor

import (
	"testing"
)

// ============================================================
// Gradient shape tests
// ============================================================

func TestGradientScalarSquare(t *testing.T) {
	// y = x * x, dy/dx = 2x
	// Forward: x = [3.0], y = x*x
	x := NewNode("x", OpCreate, NewShape(1), ElemF32)
	y := NewNode("y", OpMul, NewShape(1), ElemF32, "x", "x")

	g := NewGraph()
	g.AddNode(x)
	g.AddNode(y)
	g.AddInput(x)
	g.AddOutput(y)

	result, err := BuildGradient(g)
	if err != nil {
		t.Fatalf("gradient build failed: %v", err)
	}
	if result.BackwardGraph == nil {
		t.Fatal("expected backward graph")
	}
	// grad of seed -> y, then y's mul gradient flows to x
	dxID, ok := result.InputGrads["x"]
	if !ok {
		t.Fatalf("expected gradient for input x. grads: %v", result.InputGrads)
	}
	if dxID == "" {
		t.Error("expected non-empty gradient id")
	}
}

func TestGradientMatMulShapes(t *testing.T) {
	// y = A @ B
	// A: [2,3], B: [3,4], y: [2,4]
	a := NewNode("A", OpCreate, NewShape(2, 3), ElemF32)
	b := NewNode("B", OpCreate, NewShape(3, 4), ElemF32)
	y := NewNode("y", OpMatMul, NewShape(2, 4), ElemF32, "A", "B")

	g := NewGraph()
	g.AddNode(a)
	g.AddNode(b)
	g.AddNode(y)
	g.AddInput(a)
	g.AddInput(b)
	g.AddOutput(y)

	result, err := BuildGradient(g)
	if err != nil {
		t.Fatalf("gradient build failed: %v", err)
	}

	// dA should be [2,3]
	dAID, ok := result.InputGrads["A"]
	if !ok {
		t.Fatalf("expected gradient for A. grads: %v", result.InputGrads)
	}
	dA, _ := result.BackwardGraph.GetNode(dAID)
	if dA == nil {
		t.Fatalf("gradient node %s not found", dAID)
	}
	if !dA.Shape.Equals(NewShape(2, 3)) {
		t.Errorf("expected dA shape [2,3], got %s", dA.Shape)
	}

	// dB should be [3,4]
	dBID, ok := result.InputGrads["B"]
	if !ok {
		t.Fatalf("expected gradient for B. grads: %v", result.InputGrads)
	}
	dB, _ := result.BackwardGraph.GetNode(dBID)
	if dB == nil {
		t.Fatalf("gradient node %s not found", dBID)
	}
	if !dB.Shape.Equals(NewShape(3, 4)) {
		t.Errorf("expected dB shape [3,4], got %s", dB.Shape)
	}
}

func TestGradientLinearChain(t *testing.T) {
	// z = relu(A @ B)
	// A: [3,2], B: [2,4] -> z: [3,4]
	a := NewNode("A", OpCreate, NewShape(3, 2), ElemF32)
	b := NewNode("B", OpCreate, NewShape(2, 4), ElemF32)
	mat := NewNode("mat", OpMatMul, NewShape(3, 4), ElemF32, "A", "B")
	z := NewNode("z", OpRelu, NewShape(3, 4), ElemF32, "mat")

	g := NewGraph()
	g.AddNode(a)
	g.AddNode(b)
	g.AddNode(mat)
	g.AddNode(z)
	g.AddInput(a)
	g.AddInput(b)
	g.AddOutput(z)

	result, err := BuildGradient(g)
	if err != nil {
		t.Fatalf("gradient build failed: %v", err)
	}

	for _, input := range []string{"A", "B"} {
		gid, ok := result.InputGrads[input]
		if !ok {
			t.Fatalf("expected gradient for %s", input)
		}
		if gid == "" {
			t.Errorf("expected non-empty gradient for %s", input)
		}
	}
}

func TestGradientAddElementWise(t *testing.T) {
	// z = a + b, dz/da = 1
	a := NewNode("a", OpCreate, NewShape(3), ElemF32)
	b := NewNode("b", OpCreate, NewShape(3), ElemF32)
	z := NewNode("z", OpAdd, NewShape(3), ElemF32, "a", "b")

	g := NewGraph()
	g.AddNode(a)
	g.AddNode(b)
	g.AddNode(z)
	g.AddInput(a)
	g.AddInput(b)
	g.AddOutput(z)

	result, err := BuildGradient(g)
	if err != nil {
		t.Fatalf("gradient build failed: %v", err)
	}
	if _, ok := result.InputGrads["a"]; !ok {
		t.Fatalf("expected gradient for a. grads: %v", result.InputGrads)
	}
	if _, ok := result.InputGrads["b"]; !ok {
		t.Fatalf("expected gradient for b. grads: %v", result.InputGrads)
	}
}

func TestGradientElementWiseMul(t *testing.T) {
	// z = a * b, dz/da = b, dz/db = a
	a := NewNode("a", OpCreate, NewShape(3), ElemF32)
	b := NewNode("b", OpCreate, NewShape(3), ElemF32)
	z := NewNode("z", OpMul, NewShape(3), ElemF32, "a", "b")

	g := NewGraph()
	g.AddNode(a)
	g.AddNode(b)
	g.AddNode(z)
	g.AddInput(a)
	g.AddInput(b)
	g.AddOutput(z)

	result, err := BuildGradient(g)
	if err != nil {
		t.Fatalf("gradient build failed: %v", err)
	}
	dA, ok := result.InputGrads["a"]
	if !ok {
		t.Fatalf("expected gradient for a")
	}
	dANode, _ := result.BackwardGraph.GetNode(dA)
	if dANode == nil || dANode.Op != OpMul {
		t.Errorf("expected dA to be a Mul op, got %v", dANode)
	}
}

func TestGradientTopoOrder(t *testing.T) {
	// Verify the topo sort respects dependencies
	a := NewNode("a", OpCreate, NewShape(2), ElemF32)
	b := NewNode("b", OpCreate, NewShape(2), ElemF32)
	c := NewNode("c", OpAdd, NewShape(2), ElemF32, "a", "b")
	d := NewNode("d", OpMul, NewShape(2), ElemF32, "c", "a")

	g := NewGraph()
	g.AddNode(a)
	g.AddNode(b)
	g.AddNode(c)
	g.AddNode(d)
	g.AddInput(a)
	g.AddInput(b)
	g.AddOutput(d)

	var order []*TensorNode
	visited := make(map[string]bool)
	topoSort(g, &order, visited, g.Outputs)

	if len(order) != 4 {
		t.Fatalf("expected 4 nodes in topo order, got %d: %v", len(order), order)
	}
	// d (output) must come last
	if order[len(order)-1].ID != "d" {
		t.Errorf("expected d last, got %s", order[len(order)-1].ID)
	}
	// a and b must come before c
	aIdx, cIdx := -1, -1
	for i, n := range order {
		if n.ID == "a" {
			aIdx = i
		}
		if n.ID == "c" {
			cIdx = i
		}
	}
	if aIdx > cIdx {
		t.Error("expected a before c in topo order")
	}
}

func TestReverseShape(t *testing.T) {
	s := NewShape(2, 3, 4)
	got := reverseShape(s)
	want := NewShape(4, 3, 2)
	if !got.Equals(want) {
		t.Errorf("expected %s, got %s", want, got)
	}
}
