package sema

import (
	"karkain/pkg/parser"
	"strings"
	"testing"
)

func TestTensor_ShapeValidation(t *testing.T) {
	checker := NewShapeChecker()

	// --- Valid matmul: [32, 128] x [128, 64] -> [32, 64] ---
	a := NewTensorShape(32, 128)
	b := NewTensorShape(128, 64)
	result, err := checker.CheckMatmulShape(a, b)
	if err != nil {
		t.Fatalf("valid matmul failed: %v", err)
	}
	if !result.Equal(NewTensorShape(32, 64)) {
		t.Errorf("expected [32, 64], got %s", result.String())
	}

	// --- Invalid matmul: [32, 128] x [64, 128] -> error (inner dims mismatch) ---
	a = NewTensorShape(32, 128)
	b = NewTensorShape(64, 128)
	_, err = checker.CheckMatmulShape(a, b)
	if err == nil {
		t.Error("expected error for mismatched inner dimensions, got nil")
	}
	if err != nil && !strings.Contains(err.Error(), "inner dimensions mismatch") {
		t.Errorf("expected 'inner dimensions mismatch', got: %v", err)
	}

	// --- Invalid matmul: [32] x [64, 128] -> error (1D operand) ---
	a = NewTensorShape(32)
	b = NewTensorShape(64, 128)
	_, err = checker.CheckMatmulShape(a, b)
	if err == nil {
		t.Error("expected error for 1D operand, got nil")
	}

	// --- Valid batched matmul: [4, 32, 128] x [4, 128, 64] -> [4, 32, 64] ---
	a = NewTensorShape(4, 32, 128)
	b = NewTensorShape(4, 128, 64)
	result, err = checker.CheckMatmulShape(a, b)
	if err != nil {
		t.Fatalf("valid batched matmul failed: %v", err)
	}
	expected := NewTensorShape(4, 32, 64)
	if !result.Equal(expected) {
		t.Errorf("expected %s, got %s", expected.String(), result.String())
	}

	// --- Valid relu: shape preserved ---
	input := NewTensorShape(32, 64)
	reluResult := checker.CheckReluShape(input)
	if !reluResult.Equal(input) {
		t.Errorf("relu should preserve shape: expected %s, got %s", input.String(), reluResult.String())
	}

	// --- Valid softmax: [32, 64] axis=1 ---
	input = NewTensorShape(32, 64)
	result, err = checker.CheckSoftmaxShape(input, 1)
	if err != nil {
		t.Fatalf("valid softmax failed: %v", err)
	}
	if !result.Equal(input) {
		t.Errorf("softmax should preserve shape")
	}

	// --- Invalid softmax: axis out of range ---
	_, err = checker.CheckSoftmaxShape(input, 5)
	if err == nil {
		t.Error("expected error for out-of-range axis, got nil")
	}

	// --- Valid conv2d: [1, 3, 32, 32] x [16, 3, 3, 3] -> [1, 16, 30, 30] ---
	input = NewTensorShape(1, 3, 32, 32)
	kernel := NewTensorShape(16, 3, 3, 3)
	result, err = checker.CheckConv2DShape(input, kernel)
	if err != nil {
		t.Fatalf("valid conv2d failed: %v", err)
	}
	expected = NewTensorShape(1, 16, 30, 30)
	if !result.Equal(expected) {
		t.Errorf("expected %s, got %s", expected.String(), result.String())
	}

	// --- Invalid conv2d: channel mismatch ---
	input = NewTensorShape(1, 3, 32, 32)
	kernel = NewTensorShape(16, 5, 3, 3) // C_in=5 != C_in=3
	_, err = checker.CheckConv2DShape(input, kernel)
	if err == nil {
		t.Error("expected error for channel mismatch, got nil")
	}

	// --- Invalid conv2d: non-4D input ---
	input = NewTensorShape(32, 32)
	kernel = NewTensorShape(16, 3, 3, 3)
	_, err = checker.CheckConv2DShape(input, kernel)
	if err == nil {
		t.Error("expected error for non-4D input, got nil")
	}

	// --- Valid transpose: [32, 64] dim0=0, dim1=1 -> [64, 32] ---
	input = NewTensorShape(32, 64)
	result, err = checker.CheckTransposeShape(input, 0, 1)
	if err != nil {
		t.Fatalf("valid transpose failed: %v", err)
	}
	expected = NewTensorShape(64, 32)
	if !result.Equal(expected) {
		t.Errorf("expected %s, got %s", expected.String(), result.String())
	}

	// --- Invalid transpose: out of range dim ---
	_, err = checker.CheckTransposeShape(input, 0, 5)
	if err == nil {
		t.Error("expected error for out-of-range dim, got nil")
	}

	// --- NumElements ---
	s := NewTensorShape(2, 3, 4)
	if s.NumElements() != 24 {
		t.Errorf("expected 24 elements, got %d", s.NumElements())
	}
	if s.Ndims() != 3 {
		t.Errorf("expected 3 dims, got %d", s.Ndims())
	}
}

func TestAutodiff_DAGGeneration(t *testing.T) {
	engine := NewAutodiffEngine()

	// Create input nodes: x[32, 128], w[128, 64]
	x := engine.TrackInput("x", NewTensorShape(32, 128))
	w := engine.TrackInput("w", NewTensorShape(128, 64))

	// Forward: matmul(x, w) -> [32, 64]
	matmulOut, err := engine.TrackMatmul(x, w)
	if err != nil {
		t.Fatalf("matmul failed: %v", err)
	}
	if !matmulOut.Shape.Equal(NewTensorShape(32, 64)) {
		t.Errorf("expected matmul output shape [32, 64], got %s", matmulOut.Shape.String())
	}

	// Forward: relu(matmulOut) -> [32, 64]
	reluOut := engine.TrackRelu(matmulOut)
	if !reluOut.Shape.Equal(NewTensorShape(32, 64)) {
		t.Errorf("expected relu output shape [32, 64], got %s", reluOut.Shape.String())
	}

	// Build backward pass
	upstreamGrad := engine.TrackInput("dL_dOut", NewTensorShape(32, 64))
	gradOps := engine.BuildBackwardPass([]*ADNode{reluOut}, upstreamGrad)

	if len(gradOps) == 0 {
		t.Fatal("expected gradient operations, got none")
	}

	// Verify gradient nodes exist
	gradNames := make([]string, 0)
	for _, go2 := range gradOps {
		gradNames = append(gradNames, go2.GradNode.Name)
	}

	// Should have gradients for relu and matmul inputs
	hasReluGrad := false
	hasMatmulGradA := false
	hasMatmulGradB := false
	for _, name := range gradNames {
		if strings.Contains(name, "relu_grad") {
			hasReluGrad = true
		}
		if strings.Contains(name, "matmul_grad_a") {
			hasMatmulGradA = true
		}
		if strings.Contains(name, "matmul_grad_b") {
			hasMatmulGradB = true
		}
	}

	if !hasReluGrad {
		t.Error("expected relu_grad node in backward pass")
	}
	if !hasMatmulGradA {
		t.Error("expected matmul_grad_a node in backward pass")
	}
	if !hasMatmulGradB {
		t.Error("expected matmul_grad_b node in backward pass")
	}

	// Verify graph structure
	graph := engine.GetGraph()
	if len(graph.Nodes) < 5 { // x, w, matmul, relu, dL_dOut
		t.Errorf("expected at least 5 nodes in graph, got %d", len(graph.Nodes))
	}
	if len(graph.Inputs) != 3 { // x, w, dL_dOut
		t.Errorf("expected 3 input nodes, got %d", len(graph.Inputs))
	}

	// Test with softmax
	engine2 := NewAutodiffEngine()
	input2 := engine2.TrackInput("logits", NewTensorShape(32, 10))
	softmaxOut, err := engine2.TrackSoftmax(input2, 1)
	if err != nil {
		t.Fatalf("softmax failed: %v", err)
	}
	gradOp2 := engine2.BuildBackwardPass([]*ADNode{softmaxOut}, engine2.TrackInput("grad", NewTensorShape(32, 10)))
	if len(gradOp2) == 0 {
		t.Error("expected gradient ops for softmax")
	}

	// Test with conv2d
	engine3 := NewAutodiffEngine()
	img := engine3.TrackInput("img", NewTensorShape(1, 3, 32, 32))
	filt := engine3.TrackInput("filt", NewTensorShape(16, 3, 3, 3))
	convOut, err := engine3.TrackConv2D(img, filt)
	if err != nil {
		t.Fatalf("conv2d failed: %v", err)
	}
	if !convOut.Shape.Equal(NewTensorShape(1, 16, 30, 30)) {
		t.Errorf("expected conv2d output [1, 16, 30, 30], got %s", convOut.Shape.String())
	}

	// Test with transpose
	engine4 := NewAutodiffEngine()
	mat := engine4.TrackInput("mat", NewTensorShape(32, 64))
	transOut, err := engine4.TrackTranspose(mat, 0, 1)
	if err != nil {
		t.Fatalf("transpose failed: %v", err)
	}
	if !transOut.Shape.Equal(NewTensorShape(64, 32)) {
		t.Errorf("expected transpose output [64, 32], got %s", transOut.Shape.String())
	}
}

func TestAutodiff_ShapeErrors(t *testing.T) {
	engine := NewAutodiffEngine()

	x := engine.TrackInput("x", NewTensorShape(32, 128))
	w := engine.TrackInput("w", NewTensorShape(64, 128)) // Mismatched inner dim

	_, err := engine.TrackMatmul(x, w)
	if err == nil {
		t.Error("expected error for mismatched matmul dimensions")
	}

	// Verify error is accumulated
	if len(engine.GetErrors()) == 0 {
		t.Error("expected errors to be accumulated")
	}
}

func TestAutodiff_ASTBlockAnalysis(t *testing.T) {
	engine := NewAutodiffEngine()

	// Simulate: tensor ForwardPass(x: Tensor<f32, [32, 128]>, w: Tensor<f32, [128, 64]>) -> Tensor<f32, [32, 64]>
	// { let mat = ops.matmul(x, w); return ops.relu(mat); }
	tensorStmt := &parser.TensorStmt{
		Name: "ForwardPass",
		Params: []parser.TensorParam{
			{Name: "x", Type: &parser.TensorType{ElementType: "f32", Shape: []int{32, 128}}},
			{Name: "w", Type: &parser.TensorType{ElementType: "f32", Shape: []int{128, 64}}},
		},
		ReturnType: &parser.TensorType{ElementType: "f32", Shape: []int{32, 64}},
		Body: []parser.Node{
			&parser.VarDeclStmt{
				Name: "mat",
				Type: "",
				Value: &parser.TensorOpExpr{
					Op:   "matmul",
					Args: []parser.Node{&parser.Identifier{Name: "x"}, &parser.Identifier{Name: "w"}},
				},
			},
			&parser.TensorReturnStmt{
				Value: &parser.TensorOpExpr{
					Op:   "relu",
					Args: []parser.Node{&parser.Identifier{Name: "mat"}},
				},
			},
		},
	}

	err := engine.AnalyzeTensorBlock(tensorStmt)
	if err != nil {
		t.Fatalf("tensor block analysis failed: %v", err)
	}

	graph := engine.GetGraph()
	if len(graph.Outputs) != 1 {
		t.Errorf("expected 1 output node, got %d", len(graph.Outputs))
	}
	if !graph.Outputs[0].Shape.Equal(NewTensorShape(32, 64)) {
		t.Errorf("expected output shape [32, 64], got %s", graph.Outputs[0].Shape.String())
	}

	// Build backward pass
	upstream := engine.TrackInput("dL", NewTensorShape(32, 64))
	grads := engine.BuildBackwardPass(graph.Outputs, upstream)
	if len(grads) == 0 {
		t.Error("expected gradient ops from AST block analysis")
	}
}
