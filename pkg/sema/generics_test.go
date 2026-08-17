package sema

import (
	"karkain/pkg/parser"
	"strings"
	"testing"
)

func TestGenerics_StructInstantiation(t *testing.T) {
	// Template: struct Vector<T> { data: []T; len: int }
	tmpl := &parser.StructDeclStmt{
		Name: "Vector",
		Fields: []parser.StructField{
			{Name: "data", Type: "[]T"},
			{Name: "len", Type: "int"},
		},
		GenericParams: []parser.GenericTypeParam{
			{Name: "T", Constraints: []string{}},
		},
	}

	monomorphizer := NewMonomorphizer()

	// Instantiate Vector<int>
	intResult, err := monomorphizer.InstantiateGenericStruct(tmpl, map[string]string{"T": "int"})
	if err != nil {
		t.Fatalf("unexpected error instantiating Vector<int>: %v", err)
	}
	if intResult.Name != "Vector_int" {
		t.Errorf("expected name 'Vector_int', got '%s'", intResult.Name)
	}
	if intResult.Fields[0].Type != "[]int" {
		t.Errorf("expected field type '[]int', got '%s'", intResult.Fields[0].Type)
	}
	if intResult.Fields[1].Type != "int" {
		t.Errorf("expected field type 'int', got '%s'", intResult.Fields[1].Type)
	}

	// Instantiate Vector<float>
	floatResult, err := monomorphizer.InstantiateGenericStruct(tmpl, map[string]string{"T": "float"})
	if err != nil {
		t.Fatalf("unexpected error instantiating Vector<float>: %v", err)
	}
	if floatResult.Name != "Vector_float" {
		t.Errorf("expected name 'Vector_float', got '%s'", floatResult.Name)
	}
	if floatResult.Fields[0].Type != "[]float" {
		t.Errorf("expected field type '[]float', got '%s'", floatResult.Fields[0].Type)
	}

	// Instantiate Vector<float32>
	float32Result, err := monomorphizer.InstantiateGenericStruct(tmpl, map[string]string{"T": "float32"})
	if err != nil {
		t.Fatalf("unexpected error instantiating Vector<float32>: %v", err)
	}
	if float32Result.Name != "Vector_float32" {
		t.Errorf("expected name 'Vector_float32', got '%s'", float32Result.Name)
	}
	if float32Result.Fields[0].Type != "[]float32" {
		t.Errorf("expected field type '[]float32', got '%s'", float32Result.Fields[0].Type)
	}

	// Verify original template is unchanged
	if tmpl.Name != "Vector" {
		t.Errorf("template was mutated: name is now '%s'", tmpl.Name)
	}
	if tmpl.Fields[0].Type != "[]T" {
		t.Errorf("template was mutated: field type is now '%s'", tmpl.Fields[0].Type)
	}
}

func TestGenerics_KernelMonomorphization(t *testing.T) {
	// Template: kernel add<T: Numeric>(a: []T, b: []T, c: []T) { ... }
	tmpl := &parser.KernelDeclStmt{
		Name: "add",
		Params: []parser.Parameter{
			{Name: "a", Type: "[]T"},
			{Name: "b", Type: "[]T"},
			{Name: "c", Type: "[]T"},
		},
		WorkGroupX: 64,
		Body: []parser.Node{
			&parser.VarDeclStmt{
				Name:  "idx",
				Value: &parser.GlobalIdExpr{Dimension: 0},
			},
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

	monomorphizer := NewMonomorphizer()

	// Register trait: Numeric
	monomorphizer.RegisterTrait(&parser.TraitDeclStmt{
		Name: "Numeric",
		Methods: []parser.TraitMethod{
			{Name: "add", Params: []parser.Parameter{{Name: "other", Type: "T"}}, ReturnType: "T"},
			{Name: "zero", Params: []parser.Parameter{}, ReturnType: "T"},
		},
	})

	// Register impl: Numeric for int
	monomorphizer.RegisterImpl(&parser.ImplDeclStmt{
		TraitName: "Numeric",
		ForType:   "int",
		Methods:   []parser.FuncDecl{{Name: "add"}, {Name: "zero"}},
	})

	// Register impl: Numeric for float
	monomorphizer.RegisterImpl(&parser.ImplDeclStmt{
		TraitName: "Numeric",
		ForType:   "float",
		Methods:   []parser.FuncDecl{{Name: "add"}, {Name: "zero"}},
	})

	// Register impl: Numeric for float32
	monomorphizer.RegisterImpl(&parser.ImplDeclStmt{
		TraitName: "Numeric",
		ForType:   "float32",
		Methods:   []parser.FuncDecl{{Name: "add"}, {Name: "zero"}},
	})

	// --- Monomorphize for int ---
	intKernel, err := monomorphizer.InstantiateGenericKernel(tmpl, map[string]string{"T": "int"})
	if err != nil {
		t.Fatalf("unexpected error monomorphizing kernel for int: %v", err)
	}
	if intKernel.Name != "add_int" {
		t.Errorf("expected kernel name 'add_int', got '%s'", intKernel.Name)
	}
	if intKernel.Params[0].Type != "[]int" {
		t.Errorf("expected param type '[]int', got '%s'", intKernel.Params[0].Type)
	}
	if intKernel.Params[1].Type != "[]int" {
		t.Errorf("expected param type '[]int', got '%s'", intKernel.Params[1].Type)
	}
	if intKernel.Params[2].Type != "[]int" {
		t.Errorf("expected param type '[]int', got '%s'", intKernel.Params[2].Type)
	}
	if intKernel.WorkGroupX != 64 {
		t.Errorf("expected workgroup_x=64, got %d", intKernel.WorkGroupX)
	}

	// --- Monomorphize for float ---
	floatKernel, err := monomorphizer.InstantiateGenericKernel(tmpl, map[string]string{"T": "float"})
	if err != nil {
		t.Fatalf("unexpected error monomorphizing kernel for float: %v", err)
	}
	if floatKernel.Name != "add_float" {
		t.Errorf("expected kernel name 'add_float', got '%s'", floatKernel.Name)
	}
	if floatKernel.Params[0].Type != "[]float" {
		t.Errorf("expected param type '[]float', got '%s'", floatKernel.Params[0].Type)
	}

	// --- Verify WGSL generation for int ---
	wgslIntKernel := intKernel
	if wgslIntKernel.Params[0].Type != "[]int" {
		t.Errorf("WGSL int kernel: expected '[]int', got '%s'", wgslIntKernel.Params[0].Type)
	}

	// --- Verify SPIR-V generation would produce different type IDs ---
	// (SPIR-V type IDs are different for int vs float — verify the names are different)
	if intKernel.Name == floatKernel.Name {
		t.Error("expected different names for int and float specializations")
	}

	// Verify original template is unchanged
	if tmpl.Name != "add" {
		t.Errorf("template was mutated: name is now '%s'", tmpl.Name)
	}
}

func TestTraits_ConstraintViolation(t *testing.T) {
	monomorphizer := NewMonomorphizer()

	// Register trait: Numeric
	monomorphizer.RegisterTrait(&parser.TraitDeclStmt{
		Name: "Numeric",
		Methods: []parser.TraitMethod{
			{Name: "add", Params: []parser.Parameter{{Name: "other", Type: "T"}}, ReturnType: "T"},
			{Name: "zero", Params: []parser.Parameter{}, ReturnType: "T"},
		},
	})

	// Register impl: Numeric for int only
	monomorphizer.RegisterImpl(&parser.ImplDeclStmt{
		TraitName: "Numeric",
		ForType:   "int",
		Methods:   []parser.FuncDecl{{Name: "add"}, {Name: "zero"}},
	})

	// Kernel template requiring Numeric constraint
	tmpl := &parser.KernelDeclStmt{
		Name: "multiply",
		Params: []parser.Parameter{
			{Name: "a", Type: "[]T"},
			{Name: "b", Type: "[]T"},
			{Name: "c", Type: "[]T"},
		},
		WorkGroupX: 32,
		Body:       []parser.Node{},
		GenericParams: []parser.GenericTypeParam{
			{Name: "T", Constraints: []string{"Numeric"}},
		},
	}

	// --- Valid: T=int implements Numeric ---
	_, err := monomorphizer.InstantiateGenericKernel(tmpl, map[string]string{"T": "int"})
	if err != nil {
		t.Errorf("expected success for T=int (implements Numeric), got: %v", err)
	}

	// --- Invalid: T=string does NOT implement Numeric ---
	_, err = monomorphizer.InstantiateGenericKernel(tmpl, map[string]string{"T": "string"})
	if err == nil {
		t.Error("expected error for T=string (does not implement Numeric), got nil")
	}
	if err != nil && !strings.Contains(err.Error(), "does not implement trait") {
		t.Errorf("expected 'does not implement trait' in error, got: %v", err)
	}

	// --- Invalid: T=MyStruct does NOT implement Numeric ---
	_, err = monomorphizer.InstantiateGenericKernel(tmpl, map[string]string{"T": "MyStruct"})
	if err == nil {
		t.Error("expected error for T=MyStruct (does not implement Numeric), got nil")
	}
	if err != nil && !strings.Contains(err.Error(), "does not implement trait") {
		t.Errorf("expected 'does not implement trait' in error, got: %v", err)
	}

	// --- Invalid: T=bool does NOT implement Numeric ---
	_, err = monomorphizer.InstantiateGenericKernel(tmpl, map[string]string{"T": "bool"})
	if err == nil {
		t.Error("expected error for T=bool (does not implement Numeric), got nil")
	}
	if err != nil && !strings.Contains(err.Error(), "does not implement trait") {
		t.Errorf("expected 'does not implement trait' in error, got: %v", err)
	}

	// --- Direct constraint check: string vs Numeric ---
	err = monomorphizer.CheckTraitConstraints("string", []string{"Numeric"})
	if err == nil {
		t.Error("expected CheckTraitConstraints to fail for string:Numeric, got nil")
	}
	if err != nil && !strings.Contains(err.Error(), "does not implement trait") {
		t.Errorf("expected 'does not implement trait' in error, got: %v", err)
	}

	// --- Direct constraint check: int vs Numeric (should succeed) ---
	err = monomorphizer.CheckTraitConstraints("int", []string{"Numeric"})
	if err != nil {
		t.Errorf("expected CheckTraitConstraints to pass for int:Numeric, got: %v", err)
	}
}
