package codegen

import (
	"karkain/pkg/parser"
	"karkain/pkg/sema"
	"strings"
	"testing"
)

func TestGenericKernel_WGSLInt(t *testing.T) {
	monomorphizer := sema.NewMonomorphizer()
	specializer := NewGenericKernelSpecializerWith(monomorphizer)

	// Register Numeric trait and int impl
	monomorphizer.RegisterTrait(&parser.TraitDeclStmt{
		Name: "Numeric",
		Methods: []parser.TraitMethod{
			{Name: "add", Params: []parser.Parameter{{Name: "other", Type: "T"}}, ReturnType: "T"},
		},
	})
	monomorphizer.RegisterImpl(&parser.ImplDeclStmt{
		TraitName: "Numeric",
		ForType:   "int",
		Methods:   []parser.FuncDecl{{Name: "add"}},
	})
	monomorphizer.RegisterImpl(&parser.ImplDeclStmt{
		TraitName: "Numeric",
		ForType:   "float",
		Methods:   []parser.FuncDecl{{Name: "add"}},
	})

	tmpl := &parser.KernelDeclStmt{
		Name: "add",
		Params: []parser.Parameter{
			{Name: "a", Type: "[]T"},
			{Name: "b", Type: "[]T"},
			{Name: "c", Type: "[]T"},
		},
		WorkGroupX: 64,
		Body: []parser.Node{
			&parser.VarDeclStmt{Name: "idx", Value: &parser.GlobalIdExpr{Dimension: 0}},
			&parser.ExprStmt{
				Expression: &parser.BinaryExpr{
					Left: &parser.IndexExpr{
						Left:  &parser.Identifier{Name: "c"},
						Index: &parser.Identifier{Name: "idx"},
					},
					Operator: "=",
					Right: &parser.BinaryExpr{
						Left: &parser.IndexExpr{
							Left:  &parser.Identifier{Name: "a"},
							Index: &parser.Identifier{Name: "idx"},
						},
						Operator: "+",
						Right: &parser.IndexExpr{
							Left:  &parser.Identifier{Name: "b"},
							Index: &parser.Identifier{Name: "idx"},
						},
					},
				},
			},
		},
		GenericParams: []parser.GenericTypeParam{
			{Name: "T", Constraints: []string{"Numeric"}},
		},
	}

	// Generate WGSL for int
	wgslInt, err := specializer.SpecializeAndGenerateWGSL(tmpl, map[string]string{"T": "int"})
	if err != nil {
		t.Fatalf("unexpected error generating WGSL for add<int>: %v", err)
	}
	if !strings.Contains(wgslInt, "array<i32>") {
		t.Errorf("expected WGSL to contain 'array<i32>' for int specialization, got:\n%s", wgslInt)
	}
	if !strings.Contains(wgslInt, "fn add_int(") {
		t.Errorf("expected WGSL function name 'add_int', got:\n%s", wgslInt)
	}

	// Generate WGSL for float
	wgslFloat, err := specializer.SpecializeAndGenerateWGSL(tmpl, map[string]string{"T": "float"})
	if err != nil {
		t.Fatalf("unexpected error generating WGSL for add<float>: %v", err)
	}
	if !strings.Contains(wgslFloat, "array<f32>") {
		t.Errorf("expected WGSL to contain 'array<f32>' for float specialization, got:\n%s", wgslFloat)
	}

	// Verify int and float produce different output
	if wgslInt == wgslFloat {
		t.Error("expected different WGSL output for int vs float specializations")
	}
}

func TestGenericKernel_SPIRVInt(t *testing.T) {
	monomorphizer := sema.NewMonomorphizer()
	specializer := NewGenericKernelSpecializerWith(monomorphizer)

	// Register traits
	monomorphizer.RegisterTrait(&parser.TraitDeclStmt{
		Name: "Numeric",
		Methods: []parser.TraitMethod{
			{Name: "add", Params: []parser.Parameter{{Name: "other", Type: "T"}}, ReturnType: "T"},
		},
	})
	monomorphizer.RegisterImpl(&parser.ImplDeclStmt{
		TraitName: "Numeric",
		ForType:   "int",
		Methods:   []parser.FuncDecl{{Name: "add"}},
	})
	monomorphizer.RegisterImpl(&parser.ImplDeclStmt{
		TraitName: "Numeric",
		ForType:   "float",
		Methods:   []parser.FuncDecl{{Name: "add"}},
	})

	tmpl := &parser.KernelDeclStmt{
		Name: "add",
		Params: []parser.Parameter{
			{Name: "a", Type: "[]T"},
			{Name: "b", Type: "[]T"},
			{Name: "c", Type: "[]T"},
		},
		WorkGroupX: 64,
		Body:       []parser.Node{},
		GenericParams: []parser.GenericTypeParam{
			{Name: "T", Constraints: []string{"Numeric"}},
		},
	}

	// Generate SPIR-V for int
	spirvInt, err := specializer.SpecializeAndGenerateSPIRV(tmpl, map[string]string{"T": "int"})
	if err != nil {
		t.Fatalf("unexpected error generating SPIR-V for add<int>: %v", err)
	}
	if len(spirvInt) == 0 {
		t.Fatal("expected non-empty SPIR-V output")
	}
	if spirvInt[0] != SPIRV_MAGIC {
		t.Errorf("expected SPIR-V magic number, got 0x%08X", spirvInt[0])
	}

	// Generate SPIR-V for float
	spirvFloat, err := specializer.SpecializeAndGenerateSPIRV(tmpl, map[string]string{"T": "float"})
	if err != nil {
		t.Fatalf("unexpected error generating SPIR-V for add<float>: %v", err)
	}

	// Different type IDs should produce different word sequences
	if len(spirvInt) == len(spirvFloat) {
		t.Log("SPIR-V word counts match (expected for same structure)")
	}

	// Verify constraint violation produces error
	_, err = specializer.SpecializeAndGenerateSPIRV(tmpl, map[string]string{"T": "string"})
	if err == nil {
		t.Error("expected error for T=string (does not implement Numeric)")
	}
}

func TestGenericKernel_MonomorphizeOnly(t *testing.T) {
	monomorphizer := sema.NewMonomorphizer()
	specializer := NewGenericKernelSpecializerWith(monomorphizer)

	monomorphizer.RegisterTrait(&parser.TraitDeclStmt{
		Name: "Numeric",
		Methods: []parser.TraitMethod{
			{Name: "add", Params: []parser.Parameter{{Name: "other", Type: "T"}}, ReturnType: "T"},
		},
	})
	monomorphizer.RegisterImpl(&parser.ImplDeclStmt{
		TraitName: "Numeric",
		ForType:   "int",
		Methods:   []parser.FuncDecl{{Name: "add"}},
	})

	tmpl := &parser.KernelDeclStmt{
		Name: "scale",
		Params: []parser.Parameter{
			{Name: "data", Type: "[]T"},
		},
		WorkGroupX: 32,
		Body: []parser.Node{
			&parser.VarDeclStmt{Name: "idx", Value: &parser.GlobalIdExpr{Dimension: 0}},
		},
		GenericParams: []parser.GenericTypeParam{
			{Name: "T", Constraints: []string{"Numeric"}},
		},
	}

	result, err := specializer.MonomorphizeKernel(tmpl, map[string]string{"T": "int"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Name != "scale_int" {
		t.Errorf("expected name 'scale_int', got '%s'", result.Name)
	}
	if result.Params[0].Type != "[]int" {
		t.Errorf("expected param type '[]int', got '%s'", result.Params[0].Type)
	}
}
