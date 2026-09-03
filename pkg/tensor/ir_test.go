package tensor

import (
	"testing"
)

func TestElemType(t *testing.T) {
	if ElemF32.String() != "f32" {
		t.Errorf("expected f32, got %s", ElemF32.String())
	}
	if ElemI64.ByteSize() != 8 {
		t.Errorf("expected 8, got %d", ElemI64.ByteSize())
	}
	if !ElemF64.IsFloat() {
		t.Error("expected F64 to be float")
	}
	if !ElemI32.IsInteger() {
		t.Error("expected I32 to be integer")
	}
}

func TestShapeStatic(t *testing.T) {
	s := NewShape(3, 4)
	if s.Rank() != 2 {
		t.Errorf("expected rank 2, got %d", s.Rank())
	}
	if s.NumElements() != 12 {
		t.Errorf("expected 12, got %d", s.NumElements())
	}
	if !s.IsMatrix() {
		t.Error("expected matrix")
	}
	if !s.IsStatic() {
		t.Error("expected static")
	}
}

func TestShapeScalar(t *testing.T) {
	s := NewShape()
	if !s.IsScalar() {
		t.Error("expected scalar")
	}
	if s.Rank() != 0 {
		t.Errorf("expected rank 0, got %d", s.Rank())
	}
	if s.NumElements() != 1 {
		t.Errorf("expected 1, got %d", s.NumElements())
	}
}

func TestShapeDynamic(t *testing.T) {
	s := Shape{StaticDim(3), DynamicDim()}
	if s.IsStatic() {
		t.Error("expected dynamic")
	}
	if s.NumElements() != -1 {
		t.Errorf("expected -1, got %d", s.NumElements())
	}
}

func TestShapeSymbolic(t *testing.T) {
	s := Shape{SymbolicDim("N"), StaticDim(768)}
	if s.Rank() != 2 {
		t.Errorf("expected rank 2, got %d", s.Rank())
	}
	if s[0].Name != "N" {
		t.Errorf("expected N, got %s", s[0].Name)
	}
}

func TestShapeEquals(t *testing.T) {
	a := NewShape(3, 4)
	b := NewShape(3, 4)
	c := NewShape(4, 3)
	if !a.Equals(b) {
		t.Error("expected equal")
	}
	if a.Equals(c) {
		t.Error("expected not equal")
	}
}

func TestShapeString(t *testing.T) {
	s := NewShape(3, 4, 5)
	got := s.String()
	want := "[3, 4, 5]"
	if got != want {
		t.Errorf("expected %s, got %s", want, got)
	}
}

func TestTensorType(t *testing.T) {
	tt := NewTensorType(ElemF32, NewShape(3, 4))
	if tt.Rank() != 2 {
		t.Errorf("expected rank 2, got %d", tt.Rank())
	}
	if tt.NumElements() != 12 {
		t.Errorf("expected 12, got %d", tt.NumElements())
	}
	got := tt.String()
	want := "f32[3, 4]"
	if got != want {
		t.Errorf("expected %s, got %s", want, got)
	}
}

func TestBroadcastShapes(t *testing.T) {
	tests := []struct {
		name    string
		a, b    Shape
		want    Shape
		wantErr bool
	}{
		{"same", NewShape(3, 4), NewShape(3, 4), NewShape(3, 4), false},
		{"broadcast1", NewShape(3, 4), NewShape(1, 4), NewShape(3, 4), false},
		{"broadcast2", NewShape(3, 1), NewShape(1, 4), NewShape(3, 4), false},
		{"broadcast3", NewShape(3, 1), NewShape(4), NewShape(3, 4), false},
		{"broadcast4", NewShape(1), NewShape(3, 4), NewShape(3, 4), false},
		{"error", NewShape(3, 4), NewShape(3, 5), nil, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := broadcastShapes(tt.a, tt.b)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !got.Equals(tt.want) {
				t.Errorf("expected %s, got %s", tt.want, got)
			}
		})
	}
}

func TestMatMulShape(t *testing.T) {
	a := NewShape(2, 3)
	b := NewShape(3, 4)
	rule := matmulShapeRule
	got, err := rule([]Shape{a, b})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := NewShape(2, 4)
	if !got.Equals(want) {
		t.Errorf("expected %s, got %s", want, got)
	}
}

func TestMatMulShapeError(t *testing.T) {
	a := NewShape(2, 3)
	b := NewShape(4, 5) // 3 != 4
	rule := matmulShapeRule
	_, err := rule([]Shape{a, b})
	if err == nil {
		t.Error("expected error for mismatched inner dimensions")
	}
}

func TestMatMulBatched(t *testing.T) {
	a := NewShape(8, 2, 3)
	b := NewShape(8, 3, 4)
	rule := matmulShapeRule
	got, err := rule([]Shape{a, b})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := NewShape(8, 2, 4)
	if !got.Equals(want) {
		t.Errorf("expected %s, got %s", want, got)
	}
}

func TestTransposeShape(t *testing.T) {
	a := NewShape(3, 4)
	rule := transposeShapeRule
	got, err := rule([]Shape{a})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := NewShape(4, 3)
	if !got.Equals(want) {
		t.Errorf("expected %s, got %s", want, got)
	}
}

func TestReduceShape(t *testing.T) {
	a := NewShape(3, 4)
	rule := reduceShapeRule
	got, err := rule([]Shape{a})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := NewShape(3)
	if !got.Equals(want) {
		t.Errorf("expected %s, got %s", want, got)
	}
}

func TestGraph(t *testing.T) {
	g := NewGraph()
	if g.NumNodes() != 0 {
		t.Errorf("expected 0, got %d", g.NumNodes())
	}

	n := NewNode("x", OpCreate, NewShape(3, 4), ElemF32)
	g.AddNode(n)
	if g.NumNodes() != 1 {
		t.Errorf("expected 1, got %d", g.NumNodes())
	}

	got, ok := g.GetNode("x")
	if !ok {
		t.Fatal("expected to find node")
	}
	if got.ID != "x" {
		t.Errorf("expected x, got %s", got.ID)
	}
}

func TestBuilder(t *testing.T) {
	input := NewNode("input", OpCreate, NewShape(3, 4), ElemF32)
	weights := NewNode("weights", OpCreate, NewShape(4, 2), ElemF32)

	g := NewBuilder().
		Input("input", ElemF32, NewShape(3, 4)).
		Create("weights", ElemF32, NewShape(4, 2)).
		Add("result", input, weights).
		Build()

	if g.NumNodes() != 3 {
		t.Errorf("expected 3 nodes, got %d", g.NumNodes())
	}
	if len(g.Inputs) != 1 {
		t.Errorf("expected 1 input, got %d", len(g.Inputs))
	}

	// Test matmul separately with proper nodes
	g2 := NewBuilder().
		MatMul("matmul", input, weights).
		Build()
	if g2.NumNodes() != 1 {
		t.Errorf("expected 1 node, got %d", g2.NumNodes())
	}
	if g2.Nodes[0].Op != OpMatMul {
		t.Errorf("expected MatMul, got %s", g2.Nodes[0].Op)
	}
}

func TestBuilderAdd(t *testing.T) {
	a := NewNode("a", OpCreate, NewShape(3, 4), ElemF32)
	b := NewNode("b", OpCreate, NewShape(3, 4), ElemF32)

	g := NewBuilder().
		Add("sum", a, b).
		Build()

	if g.NumNodes() != 1 {
		t.Errorf("expected 1 node, got %d", g.NumNodes())
	}
	node := g.Nodes[0]
	if node.Op != OpAdd {
		t.Errorf("expected Add, got %s", node.Op)
	}
	if !node.Shape.Equals(NewShape(3, 4)) {
		t.Errorf("expected [3,4], got %s", node.Shape)
	}
}

func TestBuilderRelu(t *testing.T) {
	input := NewNode("input", OpCreate, NewShape(3, 4), ElemF32)

	g := NewBuilder().
		Relu("relu_out", input).
		Build()

	node := g.Nodes[0]
	if node.Op != OpRelu {
		t.Errorf("expected Relu, got %s", node.Op)
	}
	if !node.Shape.Equals(NewShape(3, 4)) {
		t.Errorf("expected [3,4], got %s", node.Shape)
	}
}

func TestLowerToC(t *testing.T) {
	a := NewNode("a", OpCreate, NewShape(3, 4), ElemF32)
	b := NewNode("b", OpCreate, NewShape(3, 4), ElemF32)
	sum := NewNode("sum", OpAdd, NewShape(3, 4), ElemF32, "a", "b")

	g := NewGraph()
	g.AddNode(a)
	g.AddNode(b)
	g.AddNode(sum)

	ops := LowerToC(g)
	if len(ops) != 3 {
		t.Fatalf("expected 3 ops, got %d", len(ops))
	}
	if ops[2].FuncName != "tensor_add" {
		t.Errorf("expected tensor_add, got %s", ops[2].FuncName)
	}
	if ops[2].Target != "sum" {
		t.Errorf("expected sum, got %s", ops[2].Target)
	}
}

func TestEmitC(t *testing.T) {
	op := LoweredOp{
		FuncName: "tensor_add",
		Args:     []string{"a", "b"},
		Target:   "sum",
	}
	got := EmitC(op)
	want := "Tensor* sum = tensor_add(a, b);"
	if got != want {
		t.Errorf("expected %s, got %s", want, got)
	}
}

func TestEmitRuntimeHeader(t *testing.T) {
	header := EmitRuntimeHeader()
	if header == "" {
		t.Error("expected non-empty header")
	}
}

func TestPrintGraph(t *testing.T) {
	g := NewBuilder().
		Input("x", ElemF32, NewShape(3, 4)).
		Relu("y", NewNode("x", OpCreate, NewShape(3, 4), ElemF32)).
		Output(NewNode("y", OpRelu, NewShape(3, 4), ElemF32)).
		Build()

	out := PrintGraph(g)
	if out == "" {
		t.Error("expected non-empty output")
	}
}

func TestPromoteElemType(t *testing.T) {
	tests := []struct {
		a, b ElemType
		want ElemType
	}{
		{ElemF32, ElemF32, ElemF32},
		{ElemF32, ElemF64, ElemF64},
		{ElemI32, ElemI64, ElemI64},
		{ElemI32, ElemF32, ElemF32},
		{ElemI64, ElemF32, ElemF64},
	}
	for _, tt := range tests {
		got := promoteElemType(tt.a, tt.b)
		if got != tt.want {
			t.Errorf("promote(%s, %s) = %s, want %s", tt.a, tt.b, got, tt.want)
		}
	}
}

func TestOpString(t *testing.T) {
	tests := []struct {
		op   Op
		want string
	}{
		{OpAdd, "add"},
		{OpMatMul, "matmul"},
		{OpRelu, "relu"},
		{OpSoftmax, "softmax"},
	}
	for _, tt := range tests {
		got := tt.op.String()
		if got != tt.want {
			t.Errorf("Op(%d).String() = %s, want %s", tt.op, got, tt.want)
		}
	}
}

func TestOpNumInputs(t *testing.T) {
	if OpAdd.NumInputs() != 2 {
		t.Errorf("expected 2, got %d", OpAdd.NumInputs())
	}
	if OpRelu.NumInputs() != 1 {
		t.Errorf("expected 1, got %d", OpRelu.NumInputs())
	}
	if OpConcat.NumInputs() != -1 {
		t.Errorf("expected -1 (variadic), got %d", OpConcat.NumInputs())
	}
}

func TestOpIsElementWise(t *testing.T) {
	if !OpAdd.IsElementWise() {
		t.Error("expected Add to be element-wise")
	}
	if !OpRelu.IsElementWise() {
		t.Error("expected Relu to be element-wise")
	}
	if OpMatMul.IsElementWise() {
		t.Error("expected MatMul to NOT be element-wise")
	}
}
