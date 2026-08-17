package codegen

import (
	"karkain/pkg/parser"
	"strings"
	"testing"
)

func TestGenerateWGSL_VectorAdd(t *testing.T) {
	kernel := &parser.KernelDeclStmt{
		Name: "vec_add",
		Params: []parser.Parameter{
			{Name: "a", Type: "float"},
			{Name: "b", Type: "float"},
			{Name: "c", Type: "float"},
		},
		WorkGroupX: 64,
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

	gen := NewWGSLGenerator()
	result, err := gen.GenerateWGSL(kernel)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(result, "@group(0) @binding(0) var<storage, read_write> a: array<f32>") {
		t.Errorf("expected storage binding for 'a', got:\n%s", result)
	}
	if !strings.Contains(result, "@group(0) @binding(1) var<storage, read_write> b: array<f32>") {
		t.Errorf("expected storage binding for 'b', got:\n%s", result)
	}
	if !strings.Contains(result, "@group(0) @binding(2) var<storage, read_write> c: array<f32>") {
		t.Errorf("expected storage binding for 'c', got:\n%s", result)
	}
	if !strings.Contains(result, "@compute @workgroup_size(64, 1, 1)") {
		t.Errorf("expected @compute @workgroup_size(64, 1, 1), got:\n%s", result)
	}
	if !strings.Contains(result, "global_id.x") {
		t.Errorf("expected global_id.x, got:\n%s", result)
	}
}

func TestGenerateWGSL_Multidimensional(t *testing.T) {
	kernel := &parser.KernelDeclStmt{
		Name: "mat_kernel",
		Params: []parser.Parameter{
			{Name: "input", Type: "float"},
		},
		WorkGroupX: 32,
		WorkGroupY: 8,
		WorkGroupZ: 4,
		Body: []parser.Node{
			&parser.VarDeclStmt{
				Name:  "x",
				Value: &parser.GlobalIdExpr{Dimension: 0},
			},
			&parser.VarDeclStmt{
				Name:  "y",
				Value: &parser.GlobalIdExpr{Dimension: 1},
			},
			&parser.VarDeclStmt{
				Name:  "z",
				Value: &parser.GlobalIdExpr{Dimension: 2},
			},
		},
	}

	gen := NewWGSLGenerator()
	result, err := gen.GenerateWGSL(kernel)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(result, "global_id.x") {
		t.Errorf("expected global_id.x for dimension 0, got:\n%s", result)
	}
	if !strings.Contains(result, "global_id.y") {
		t.Errorf("expected global_id.y for dimension 1, got:\n%s", result)
	}
	if !strings.Contains(result, "global_id.z") {
		t.Errorf("expected global_id.z for dimension 2, got:\n%s", result)
	}
	if !strings.Contains(result, "@compute @workgroup_size(32, 8, 4)") {
		t.Errorf("expected @compute @workgroup_size(32, 8, 4), got:\n%s", result)
	}
}

func TestWGSLGenerator_UnsupportedNode(t *testing.T) {
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

	gen := NewWGSLGenerator()
	_, err := gen.GenerateWGSL(kernel)
	if err == nil {
		t.Fatal("expected error for unsupported node type, got nil")
	}
	if !strings.Contains(err.Error(), "unsupported") {
		t.Errorf("expected 'unsupported' in error, got: %v", err)
	}
}
