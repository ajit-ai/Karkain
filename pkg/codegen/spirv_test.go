package codegen

import (
	"karkain/pkg/parser"
	"testing"
)

func TestGenerateSPIRV_HeaderAndOpcodes(t *testing.T) {
	kernel := &parser.KernelDeclStmt{
		Name: "vector_add",
		Params: []parser.Parameter{
			{Name: "a", Type: "float"},
			{Name: "b", Type: "float"},
			{Name: "c", Type: "float"},
		},
		WorkGroupX: 64,
		WorkGroupY: 1,
		WorkGroupZ: 1,
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

	gen := NewSPIRVGenerator()
	result, err := gen.GenerateSPIRV(kernel)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result) == 0 {
		t.Fatal("expected non-empty SPIR-V output")
	}

	// Verify header magic number
	if result[0] != SPIRV_MAGIC {
		t.Errorf("expected magic number 0x%08X, got 0x%08X", SPIRV_MAGIC, result[0])
	}

	// Verify version
	if result[1] != SPIRV_VERSION {
		t.Errorf("expected version 0x%08X, got 0x%08X", SPIRV_VERSION, result[1])
	}

	// Verify schema
	if result[4] != SPIRV_SCHEMA {
		t.Errorf("expected schema 0, got %d", result[4])
	}

	// Verify we have at least the header + some instructions
	if len(result) < 10 {
		t.Errorf("expected at least 10 words, got %d", len(result))
	}

	// Verify binary output is non-empty
	binary := gen.GetBinary(result)
	if len(binary) == 0 {
		t.Error("expected non-empty binary output")
	}
	if len(binary)%4 != 0 {
		t.Errorf("expected binary length to be multiple of 4, got %d", len(binary))
	}
}

func TestGenerateSPIRV_SimpleKernel(t *testing.T) {
	kernel := &parser.KernelDeclStmt{
		Name: "compute",
		Params: []parser.Parameter{
			{Name: "input", Type: "int"},
			{Name: "output", Type: "int"},
		},
		WorkGroupX: 32,
		WorkGroupY: 1,
		WorkGroupZ: 1,
		Body:       []parser.Node{},
	}

	gen := NewSPIRVGenerator()
	result, err := gen.GenerateSPIRV(kernel)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify magic number
	if result[0] != SPIRV_MAGIC {
		t.Errorf("expected magic number 0x%08X, got 0x%08X", SPIRV_MAGIC, result[0])
	}

	// Verify bound is reasonable (should have allocated several IDs)
	if result[3] < 5 {
		t.Errorf("expected bound >= 5, got %d", result[3])
	}
}

func TestGenerateSPIRV_EmptyKernel(t *testing.T) {
	kernel := &parser.KernelDeclStmt{
		Name:       "empty",
		Params:     []parser.Parameter{},
		Body:       []parser.Node{},
		WorkGroupX: 1,
		WorkGroupY: 1,
		WorkGroupZ: 1,
	}

	gen := NewSPIRVGenerator()
	result, err := gen.GenerateSPIRV(kernel)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result) < 5 {
		t.Errorf("expected at least 5 words for header, got %d", len(result))
	}
}
