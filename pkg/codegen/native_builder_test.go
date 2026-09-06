package codegen

import (
	"karkain/pkg/parser"
	"testing"
)

func TestNativeBuilder_SimpleFunction(t *testing.T) {
	prog := &parser.Program{
		Statements: []parser.Node{
			&parser.FuncDecl{
				Name:   "main",
				Params: []string{},
				Line:   1,
				Body: []parser.Node{
					&parser.ReturnStmt{
						Value: &parser.IntLiteral{Value: "0"},
						Line:  3,
					},
				},
			},
		},
	}

	builder := NewNativeBuilder()
	result, err := builder.Build(prog, "test.kark")
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}
	if result.Object == nil {
		t.Fatal("expected non-nil Object")
	}
	if result.Executable == nil {
		t.Fatal("expected non-nil Executable")
	}
	if result.DebugInfo == nil {
		t.Fatal("expected non-nil DebugInfo")
	}
	if result.SourceMap == nil {
		t.Fatal("expected non-nil SourceMap")
	}

	// Verify object has sections
	if len(result.Object.Sections) != 3 {
		t.Errorf("expected 3 sections, got %d", len(result.Object.Sections))
	}

	// Verify executable has entry point
	if result.Executable.EntryPoint == 0 {
		t.Error("expected non-zero entry point")
	}

	// Verify debug info has function
	if len(result.DebugInfo.FunctionInfo) == 0 {
		t.Error("expected at least one function in debug info")
	}

	// Verify source map has mapping
	addr, ok := result.SourceMap.GetAddressForSource("test.kark", 1)
	if !ok || addr == 0 {
		t.Error("expected source-to-address mapping")
	}
}

func TestNativeBuilder_WithSymbols(t *testing.T) {
	prog := &parser.Program{
		Statements: []parser.Node{
			&parser.FuncDecl{
				Name:   "main",
				Params: []string{},
				Body:   []parser.Node{},
			},
			&parser.FuncDecl{
				Name:   "helper",
				Params: []string{"x"},
				Body:   []parser.Node{},
			},
			&parser.StructDeclStmt{
				Name:   "Point",
				Fields: []parser.StructField{{Name: "x", Type: "int"}, {Name: "y", Type: "int"}},
			},
		},
	}

	builder := NewNativeBuilder()
	result, err := builder.Build(prog, "test.kark")
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	// Verify symbols are collected
	names := make(map[string]bool)
	for _, sym := range result.Object.Symbols {
		names[sym.Name] = true
	}

	if !names["main"] {
		t.Error("expected 'main' symbol")
	}
	if !names["karkain_user_helper"] {
		t.Error("expected 'karkain_user_helper' symbol")
	}
	if !names["Point"] {
		t.Error("expected 'Point' symbol")
	}
}

func TestNativeBuilder_Diagnostics(t *testing.T) {
	// Program with duplicate main should produce diagnostics
	prog := &parser.Program{
		Statements: []parser.Node{
			&parser.FuncDecl{Name: "main", Params: []string{}, Body: []parser.Node{}},
			&parser.FuncDecl{Name: "main", Params: []string{}, Body: []parser.Node{}},
		},
	}

	builder := NewNativeBuilder()
	result, err := builder.Build(prog, "test.kark")
	if err == nil {
		t.Fatal("expected error for duplicate main")
	}
	if len(result.Diagnostics) == 0 {
		t.Error("expected diagnostics for duplicate main")
	}
}

func TestNativeBuilder_DebugInfoPreserved(t *testing.T) {
	prog := &parser.Program{
		Statements: []parser.Node{
			&parser.FuncDecl{
				Name:   "main",
				Params: []string{},
				Line:   1,
				Body: []parser.Node{
					&parser.VarDeclStmt{
						Name:  "x",
						Type:  "int",
						Value: &parser.IntLiteral{Value: "42"},
						Line:  2,
					},
					&parser.ReturnStmt{
						Value: &parser.IntLiteral{Value: "0"},
						Line:  3,
					},
				},
			},
		},
	}

	builder := NewNativeBuilder()
	result, err := builder.Build(prog, "test.kark")
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	// Verify debug info has function
	foundMain := false
	for _, fn := range result.DebugInfo.FunctionInfo {
		if fn.Name == "main" {
			foundMain = true
			if fn.StartLine == 0 {
				t.Error("expected non-zero start line for main")
			}
		}
	}
	if !foundMain {
		t.Error("expected 'main' in function debug info")
	}

	// Verify debug info has variable
	foundX := false
	for _, v := range result.DebugInfo.Variables {
		if v.Name == "x" {
			foundX = true
			if !v.IsLocal {
				t.Error("expected 'x' to be local")
			}
		}
	}
	if !foundX {
		t.Error("expected 'x' in variable debug info")
	}
}

func TestNativeBuilder_SourceAddressMap(t *testing.T) {
	prog := &parser.Program{
		Statements: []parser.Node{
			&parser.FuncDecl{
				Name:   "main",
				Params: []string{},
				Line:   1,
				Body: []parser.Node{
					&parser.ReturnStmt{
						Value: &parser.IntLiteral{Value: "0"},
						Line:  3,
					},
				},
			},
		},
	}

	builder := NewNativeBuilder()
	result, err := builder.Build(prog, "test.kark")
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	// Verify forward lookup
	addr, ok := result.SourceMap.GetAddressForSource("test.kark", 1)
	if !ok {
		t.Error("expected source-to-address mapping for line 1")
	}
	if addr == 0 {
		t.Error("expected non-zero address")
	}

	// Verify reverse lookup
	loc, ok := result.SourceMap.GetSourceForAddress(addr)
	if !ok {
		t.Error("expected address-to-source mapping")
	}
	if loc.File != "test.kark" {
		t.Errorf("expected file 'test.kark', got '%s'", loc.File)
	}
}

func TestNativeBuilder_MultiFunction(t *testing.T) {
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

	builder := NewNativeBuilder()
	result, err := builder.Build(prog, "test.kark")
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	// Verify both functions are in debug info
	functions := make(map[string]bool)
	for _, fn := range result.DebugInfo.FunctionInfo {
		functions[fn.Name] = true
	}
	if !functions["add"] {
		t.Error("expected 'add' in debug info")
	}
	if !functions["main"] {
		t.Error("expected 'main' in debug info")
	}

	// Verify executable has sections
	if len(result.Executable.Sections) == 0 {
		t.Error("expected sections in executable")
	}
}

func TestNativeBuilder_HasErrors(t *testing.T) {
	builder := NewNativeBuilder()

	// Initially no errors
	if builder.HasErrors() {
		t.Error("expected no errors initially")
	}
}

func TestNativeBuilder_GetDiagnostics(t *testing.T) {
	builder := NewNativeBuilder()

	diags := builder.GetDiagnostics()
	if len(diags) != 0 {
		t.Error("expected no diagnostics initially")
	}
}
