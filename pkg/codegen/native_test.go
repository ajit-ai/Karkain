package codegen

import (
	"karkain/pkg/parser"
	"strings"
	"testing"
)

func TestNativeGen_BasicFunction(t *testing.T) {
	prog := &parser.Program{
		Statements: []parser.Node{
			&parser.FuncDecl{
				Name:   "add",
				Params: []string{"a", "b"},
				Body: []parser.Node{
					&parser.ReturnStmt{
						Value: &parser.BinaryExpr{
							Left:     &parser.Identifier{Name: "a"},
							Operator: "+",
							Right:    &parser.Identifier{Name: "b"},
						},
					},
				},
			},
		},
	}

	gen := NewNativeGenerator()
	result, err := gen.GenerateModule(prog)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(result, "Value* add(Value* a, Value* b)") {
		t.Errorf("expected function signature 'Value* add(Value* a, Value* b)', got:\n%s", result)
	}
	if !strings.Contains(result, "return (a + b);") {
		t.Errorf("expected return statement with addition, got:\n%s", result)
	}
}

func TestNativeGen_StructLayout(t *testing.T) {
	prog := &parser.Program{
		Statements: []parser.Node{
			&parser.StructDeclStmt{
				Name: "Vector3",
				Fields: []parser.StructField{
					{Name: "x", Type: "float64"},
					{Name: "y", Type: "float64"},
					{Name: "z", Type: "float64"},
				},
			},
		},
	}

	gen := NewNativeGenerator()
	result, err := gen.GenerateModule(prog)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(result, "typedef struct __attribute__((aligned(16)))") {
		t.Errorf("expected aligned struct attribute, got:\n%s", result)
	}
	if !strings.Contains(result, "double x;") {
		t.Errorf("expected field 'double x;', got:\n%s", result)
	}
	if !strings.Contains(result, "double y;") {
		t.Errorf("expected field 'double y;', got:\n%s", result)
	}
	if !strings.Contains(result, "double z;") {
		t.Errorf("expected field 'double z;', got:\n%s", result)
	}
	if !strings.Contains(result, "} Vector3;") {
		t.Errorf("expected struct name 'Vector3', got:\n%s", result)
	}
}

func TestNativeGen_ControlFlow(t *testing.T) {
	prog := &parser.Program{
		Statements: []parser.Node{
			&parser.FuncDecl{
				Name:   "control_test",
				Params: []string{"n"},
				Body: []parser.Node{
					&parser.IfStmt{
						Condition: &parser.BinaryExpr{
							Left:     &parser.Identifier{Name: "n"},
							Operator: ">",
							Right:    &parser.IntLiteral{Value: "0"},
						},
						Consequence: []parser.Node{
							&parser.ReturnStmt{
								Value: &parser.IntLiteral{Value: "1"},
							},
						},
						Alternative: []parser.Node{
							&parser.ReturnStmt{
								Value: &parser.IntLiteral{Value: "0"},
							},
						},
					},
					&parser.WhileStmt{
						Condition: &parser.BinaryExpr{
							Left:     &parser.Identifier{Name: "n"},
							Operator: ">",
							Right:    &parser.IntLiteral{Value: "0"},
						},
						Body: []parser.Node{
							&parser.ExprStmt{
								Expression: &parser.BinaryExpr{
									Left:     &parser.Identifier{Name: "n"},
									Operator: "=",
									Right: &parser.BinaryExpr{
										Left:     &parser.Identifier{Name: "n"},
										Operator: "-",
										Right:    &parser.IntLiteral{Value: "1"},
									},
								},
							},
						},
					},
					&parser.ForStmt{
						Init: &parser.VarDeclStmt{
							Name:  "i",
							Value: &parser.IntLiteral{Value: "0"},
							Type:  "int",
						},
						Condition: &parser.BinaryExpr{
							Left:     &parser.Identifier{Name: "i"},
							Operator: "<",
							Right:    &parser.IntLiteral{Value: "10"},
						},
						Post: &parser.BinaryExpr{
							Left:     &parser.Identifier{Name: "i"},
							Operator: "=",
							Right: &parser.BinaryExpr{
								Left:     &parser.Identifier{Name: "i"},
								Operator: "+",
								Right:    &parser.IntLiteral{Value: "1"},
							},
						},
						Body: []parser.Node{
							&parser.PrintStmt{
								Value: &parser.Identifier{Name: "i"},
							},
						},
					},
				},
			},
		},
	}

	gen := NewNativeGenerator()
	result, err := gen.GenerateModule(prog)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check if statement
	if !strings.Contains(result, "if ((n > 0))") {
		t.Errorf("expected if statement with condition, got:\n%s", result)
	}

	// Check while loop
	if !strings.Contains(result, "while ((n > 0))") {
		t.Errorf("expected while loop, got:\n%s", result)
	}

	// Check for loop
	if !strings.Contains(result, "for (") {
		t.Errorf("expected for loop, got:\n%s", result)
	}

	// Check return statement
	if !strings.Contains(result, "return 1;") {
		t.Errorf("expected return 1, got:\n%s", result)
	}
}

func TestNativeGen_ExternDeclarations(t *testing.T) {
	prog := &parser.Program{
		Statements: []parser.Node{},
	}

	gen := NewNativeGenerator()
	result, err := gen.GenerateModule(prog)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(result, "extern void* karkain_gpu_sync") {
		t.Errorf("expected extern karkain_gpu_sync declaration, got:\n%s", result)
	}
	if !strings.Contains(result, "extern int karkain_actor_spawn") {
		t.Errorf("expected extern karkain_actor_spawn declaration, got:\n%s", result)
	}
}

func TestNativeGen_LLVMIR(t *testing.T) {
	prog := &parser.Program{
		Statements: []parser.Node{
			&parser.FuncDecl{
				Name:   "main",
				Params: []string{},
				Body: []parser.Node{
					&parser.ReturnStmt{
						Value: &parser.IntLiteral{Value: "0"},
					},
				},
			},
		},
	}

	gen := NewNativeGenerator()
	result, err := gen.EmitLLVMIR(prog)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(result, "define i32 @main()") {
		t.Errorf("expected LLVM IR function definition, got:\n%s", result)
	}
	if !strings.Contains(result, "ret i32 0") {
		t.Errorf("expected LLVM IR return, got:\n%s", result)
	}
}
