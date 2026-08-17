package codegen

import (
	"karkain/pkg/parser"
	"strings"
	"testing"
)

func TestGenerateOpenCL_VectorAdd(t *testing.T) {
	kernel := &parser.KernelDeclStmt{
		Name: "vec_add",
		Params: []parser.Parameter{
			{Name: "a", Type: "float"},
			{Name: "b", Type: "float"},
			{Name: "c", Type: "float"},
		},
		Body: []parser.Node{
			&parser.VarDeclStmt{
				Name:  "id",
				Value: &parser.GlobalIdExpr{Dimension: 0},
			},
			&parser.ExprStmt{
				Expression: &parser.BinaryExpr{
					Left: &parser.IndexExpr{
						Left:  &parser.Identifier{Name: "c"},
						Index: &parser.Identifier{Name: "id"},
					},
					Operator: "=",
					Right: &parser.BinaryExpr{
						Left: &parser.IndexExpr{
							Left:  &parser.Identifier{Name: "a"},
							Index: &parser.Identifier{Name: "id"},
						},
						Operator: "+",
						Right: &parser.IndexExpr{
							Left:  &parser.Identifier{Name: "b"},
							Index: &parser.Identifier{Name: "id"},
						},
					},
				},
			},
		},
	}

	gen := NewGPUGenerator()
	result, err := gen.GenerateOpenCL(kernel)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(result, "__kernel void vec_add") {
		t.Errorf("expected __kernel void vec_add, got:\n%s", result)
	}
	if !strings.Contains(result, "__global float* a") {
		t.Errorf("expected __global float* a, got:\n%s", result)
	}
	if !strings.Contains(result, "__global float* b") {
		t.Errorf("expected __global float* b, got:\n%s", result)
	}
	if !strings.Contains(result, "__global float* c") {
		t.Errorf("expected __global float* c, got:\n%s", result)
	}
	if !strings.Contains(result, "get_global_id(0)") {
		t.Errorf("expected get_global_id(0), got:\n%s", result)
	}
}

func TestGPUGenerator_UnsupportedNode(t *testing.T) {
	kernel := &parser.KernelDeclStmt{
		Name:   "bad_kernel",
		Params: []parser.Parameter{},
		Body: []parser.Node{
			&parser.ActorDeclStmt{
				Name:   "SpawnActor",
				Params: []string{},
				Body:   []parser.Node{},
			},
		},
	}

	gen := NewGPUGenerator()
	_, err := gen.GenerateOpenCL(kernel)
	if err == nil {
		t.Fatal("expected error for unsupported node type, got nil")
	}
	if !strings.Contains(err.Error(), "unsupported") {
		t.Errorf("expected 'unsupported' in error, got: %v", err)
	}
}

func TestCPUKernelFallback(t *testing.T) {
	kernel := &parser.KernelDeclStmt{
		Name: "test_kernel",
		Params: []parser.Parameter{
			{Name: "a", Type: "float"},
			{Name: "b", Type: "float"},
			{Name: "c", Type: "float"},
		},
		Body: []parser.Node{
			&parser.VarDeclStmt{
				Name:  "id",
				Value: &parser.GlobalIdExpr{Dimension: 0},
			},
			&parser.ExprStmt{
				Expression: &parser.BinaryExpr{
					Left: &parser.IndexExpr{
						Left:  &parser.Identifier{Name: "c"},
						Index: &parser.Identifier{Name: "id"},
					},
					Operator: "=",
					Right: &parser.BinaryExpr{
						Left: &parser.IndexExpr{
							Left:  &parser.Identifier{Name: "a"},
							Index: &parser.Identifier{Name: "id"},
						},
						Operator: "+",
						Right: &parser.IndexExpr{
							Left:  &parser.Identifier{Name: "b"},
							Index: &parser.Identifier{Name: "id"},
						},
					},
				},
			},
		},
	}

	args := map[string]interface{}{
		"a": []float32{1, 2, 3, 4, 5},
		"b": []float32{10, 20, 30, 40, 50},
		"c": []float32{0, 0, 0, 0, 0},
	}

	err := ExecuteCPUKernelFallback(kernel, args, 5)
	if err != nil {
		t.Fatalf("CPU fallback execution failed: %v", err)
	}
}

func TestGenerateHostLauncher(t *testing.T) {
	kernel := &parser.KernelDeclStmt{
		Name: "vec_add",
		Params: []parser.Parameter{
			{Name: "a", Type: "float"},
			{Name: "b", Type: "float"},
			{Name: "c", Type: "float"},
		},
		Body:       []parser.Node{},
		WorkGroupX: 256,
	}

	result := GenerateHostLauncher(kernel)
	if !strings.Contains(result, "launch_vec_add") {
		t.Errorf("expected launch_vec_add function, got:\n%s", result)
	}
	if !strings.Contains(result, "clSetKernelArg") {
		t.Errorf("expected clSetKernelArg calls, got:\n%s", result)
	}
	if !strings.Contains(result, "clEnqueueNDRangeKernel") {
		t.Errorf("expected clEnqueueNDRangeKernel call, got:\n%s", result)
	}
	if !strings.Contains(result, "vec_add_source") {
		t.Errorf("expected vec_add_source constant, got:\n%s", result)
	}
}
