package codegen

import (
	"karkain/pkg/sema"
	"strings"
	"testing"
)

func TestTensor_WGSLGeneration(t *testing.T) {
	gen := NewTensorWGSLGenerator()

	// Generate matmul kernel for f32: A[32,128] x B[128,64] -> C[32,64]
	wgsl, err := gen.GenerateMatmulKernel("matmul_f32", 32, 128, 64, "f32")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify bindings
	if !strings.Contains(wgsl, "@group(0) @binding(0) var<storage, read> A: array<f32>") {
		t.Errorf("expected A binding, got:\n%s", wgsl)
	}
	if !strings.Contains(wgsl, "@group(0) @binding(1) var<storage, read> B: array<f32>") {
		t.Errorf("expected B binding, got:\n%s", wgsl)
	}
	if !strings.Contains(wgsl, "@group(0) @binding(2) var<storage, read_write> C: array<f32>") {
		t.Errorf("expected C binding, got:\n%s", wgsl)
	}

	// Verify compute shader
	if !strings.Contains(wgsl, "@compute @workgroup_size(16, 16, 1)") {
		t.Errorf("expected workgroup_size(16, 16, 1), got:\n%s", wgsl)
	}
	if !strings.Contains(wgsl, "fn matmul_f32(") {
		t.Errorf("expected function name matmul_f32, got:\n%s", wgsl)
	}

	// Verify tiling structure
	if !strings.Contains(wgsl, "var<workgroup> tileA") {
		t.Errorf("expected workgroup tileA, got:\n%s", wgsl)
	}
	if !strings.Contains(wgsl, "var<workgroup> tileB") {
		t.Errorf("expected workgroup tileB, got:\n%s", wgsl)
	}
	if !strings.Contains(wgsl, "workgroupBarrier()") {
		t.Errorf("expected workgroupBarrier, got:\n%s", wgsl)
	}

	// Verify params struct
	if !strings.Contains(wgsl, "struct Params { M: u32, K: u32, N: u32 }") {
		t.Errorf("expected Params struct, got:\n%s", wgsl)
	}
}

func TestTensor_WGSLReluKernel(t *testing.T) {
	gen := NewTensorWGSLGenerator()
	wgsl := gen.GenerateReluKernel("relu_fwd", "f32")

	if !strings.Contains(wgsl, "@compute @workgroup_size(64, 1, 1)") {
		t.Errorf("expected workgroup_size(64), got:\n%s", wgsl)
	}
	if !strings.Contains(wgsl, "fn relu_fwd(") {
		t.Errorf("expected function name relu_fwd, got:\n%s", wgsl)
	}
	if !strings.Contains(wgsl, "max(input[idx], 0.0)") {
		t.Errorf("expected max ReLU, got:\n%s", wgsl)
	}
}

func TestTensor_WGSLIntMatmul(t *testing.T) {
	gen := NewTensorWGSLGenerator()
	wgsl, err := gen.GenerateMatmulKernel("matmul_i32", 16, 32, 16, "i32")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(wgsl, "array<i32>") {
		t.Errorf("expected i32 types, got:\n%s", wgsl)
	}
}

func TestTensor_WGSLGraphGeneration(t *testing.T) {
	// Build a computation graph: x[3,4] @ w[4,5] -> relu -> output
	engine := sema.NewAutodiffEngine()

	x := engine.TrackInput("x", sema.NewTensorShape(3, 4))
	w := engine.TrackInput("w", sema.NewTensorShape(4, 5))
	matmulOut, err := engine.TrackMatmul(x, w)
	if err != nil {
		t.Fatalf("matmul failed: %v", err)
	}
	reluOut := engine.TrackRelu(matmulOut)

	graph := engine.GetGraph()
	graph.Outputs = []*sema.ADNode{reluOut}

	gen := NewTensorWGSLGenerator()
	wgsl, err := gen.GenerateTensorGraph(graph)
	if err != nil {
		t.Fatalf("graph generation failed: %v", err)
	}

	// Verify input declarations
	if !strings.Contains(wgsl, "var<storage, read> x: array<f32>") {
		t.Errorf("expected x input declaration, got:\n%s", wgsl)
	}
	if !strings.Contains(wgsl, "var<storage, read> w: array<f32>") {
		t.Errorf("expected w input declaration, got:\n%s", wgsl)
	}

	// Verify output declaration
	if !strings.Contains(wgsl, "var<storage, read_write> output") {
		t.Errorf("expected output declaration, got:\n%s", wgsl)
	}

	// Verify shape comments
	if !strings.Contains(wgsl, "[3, 4]") {
		t.Errorf("expected x shape comment [3, 4], got:\n%s", wgsl)
	}
	if !strings.Contains(wgsl, "[4, 5]") {
		t.Errorf("expected w shape comment [4, 5], got:\n%s", wgsl)
	}

	// Verify compute function
	if !strings.Contains(wgsl, "fn tensor_graph_main(") {
		t.Errorf("expected tensor_graph_main function, got:\n%s", wgsl)
	}
	if !strings.Contains(wgsl, "@compute @workgroup_size(64, 1, 1)") {
		t.Errorf("expected workgroup_size(64), got:\n%s", wgsl)
	}

	// Verify node operations are emitted
	if !strings.Contains(wgsl, "// matmul:") {
		t.Errorf("expected matmul comment, got:\n%s", wgsl)
	}
	if !strings.Contains(wgsl, "// relu:") {
		t.Errorf("expected relu comment, got:\n%s", wgsl)
	}
}

func TestTensorElemTypeMapping(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"f32", "f32"},
		{"float", "f32"},
		{"float32", "f32"},
		{"f16", "f32"}, // mapped to f32
		{"i32", "i32"},
		{"int", "i32"},
		{"u32", "u32"},
		{"uint", "u32"},
		{"unknown", "f32"},
	}

	for _, tt := range tests {
		result := mapTensorElemType(tt.input)
		if result != tt.expected {
			t.Errorf("mapTensorElemType(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}
