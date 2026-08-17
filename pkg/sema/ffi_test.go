package sema

import (
	"testing"

	"karkain/pkg/jit"
	"karkain/pkg/parser"
)

func TestFFI_ParseCImportBlock(t *testing.T) {
	checker := NewFFIChecker()
	block, err := checker.ParseCImportBlock(`
		fn printf(fmt: *i8, args: ...) -> i32;
		fn malloc(size: u64) -> *void;
		fn free(ptr: *void) -> void;
		fn abs(x: i32) -> i32;
		fn pow(base: f64, exp: f64) -> f64;
	`)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	if len(block.Functions) != 5 {
		t.Fatalf("expected 5 functions, got %d", len(block.Functions))
	}

	// Check printf
	if block.Functions[0].Name != "printf" {
		t.Errorf("expected 'printf', got '%s'", block.Functions[0].Name)
	}
	if block.Functions[0].RetType != "i32" {
		t.Errorf("expected return type 'i32', got '%s'", block.Functions[0].RetType)
	}
	if len(block.Functions[0].Params) != 1 {
		t.Errorf("printf should have 1 param (varargs separate), got %d", len(block.Functions[0].Params))
	}
	if !block.Functions[0].IsVarArg {
		t.Error("printf should be varargs")
	}

	// Check malloc
	if block.Functions[1].Name != "malloc" {
		t.Errorf("expected 'malloc', got '%s'", block.Functions[1].Name)
	}
	if block.Functions[1].RetType != "*void" {
		t.Errorf("expected return type '*void', got '%s'", block.Functions[1].RetType)
	}

	// Check free
	if block.Functions[2].Name != "free" {
		t.Errorf("expected 'free', got '%s'", block.Functions[2].Name)
	}
	if block.Functions[2].RetType != "void" {
		t.Errorf("expected return type 'void', got '%s'", block.Functions[2].RetType)
	}

	// Check abs
	if block.Functions[3].Name != "abs" {
		t.Errorf("expected 'abs', got '%s'", block.Functions[3].Name)
	}

	// Check pow
	if block.Functions[4].Name != "pow" {
		t.Errorf("expected 'pow', got '%s'", block.Functions[4].Name)
	}
}

func TestFFI_ParseEmptyBlock(t *testing.T) {
	checker := NewFFIChecker()
	block, err := checker.ParseCImportBlock("")
	if err != nil {
		t.Fatalf("parse empty block failed: %v", err)
	}
	if len(block.Functions) != 0 {
		t.Errorf("expected 0 functions, got %d", len(block.Functions))
	}
}

func TestFFI_ParseCommentsOnly(t *testing.T) {
	checker := NewFFIChecker()
	block, err := checker.ParseCImportBlock("// this is a comment\n// another comment")
	if err != nil {
		t.Fatalf("parse comment block failed: %v", err)
	}
	if len(block.Functions) != 0 {
		t.Errorf("expected 0 functions, got %d", len(block.Functions))
	}
}

func TestFFI_ProcessProgram(t *testing.T) {
	checker := NewFFIChecker()
	prog := &parser.Program{
		Statements: []parser.Node{
			&parser.CImportBlock{
				Content: `fn strlen(s: *i8) -> u64;
fn memset(ptr: *void, val: i32, n: u64) -> *void;`,
			},
			&parser.VarDeclStmt{Name: "x", Value: &parser.IntLiteral{Value: "42"}, Type: "int"},
		},
	}

	err := checker.ProcessProgram(prog)
	if err != nil {
		t.Fatalf("process program failed: %v", err)
	}

	if len(checker.Blocks) != 1 {
		t.Fatalf("expected 1 block, got %d", len(checker.Blocks))
	}
	if len(checker.Registered) != 2 {
		t.Errorf("expected 2 registered functions, got %d", len(checker.Registered))
	}
}

func TestFFI_ValidateTypes(t *testing.T) {
	checker := NewFFIChecker()
	checker.Blocks = []*FFIBlock{
		{
			Functions: []FFIFuncDecl{
				{Name: "good_func", RetType: "i32", Params: []FFIParam{
					{Name: "x", Type: "f64"},
				}},
				{Name: "bad_func", RetType: "badtype", Params: []FFIParam{
					{Name: "x", Type: "alsonotvalid"},
				}},
			},
		},
	}

	err := checker.ValidateTypes()
	if err == nil {
		t.Fatal("expected validation error for bad types")
	}

	if len(checker.Errors) != 2 {
		t.Errorf("expected 2 errors, got %d", len(checker.Errors))
	}
}

func TestFFI_ValidateTypesAllValid(t *testing.T) {
	checker := NewFFIChecker()
	checker.Blocks = []*FFIBlock{
		{
			Functions: []FFIFuncDecl{
				{Name: "abs", RetType: "i32", Params: []FFIParam{
					{Name: "x", Type: "i32"},
				}},
				{Name: "malloc", RetType: "*void", Params: []FFIParam{
					{Name: "size", Type: "u64"},
				}},
			},
		},
	}

	err := checker.ValidateTypes()
	if err != nil {
		t.Errorf("expected no errors, got: %v", err)
	}
}

func TestFFI_ValidateCall(t *testing.T) {
	checker := NewFFIChecker()
	checker.Registered["printf"] = &FFIFuncDecl{
		Name:     "printf",
		RetType:  "i32",
		IsVarArg: true,
		Params:   []FFIParam{{Name: "fmt", Type: "*i8"}},
	}
	checker.Registered["abs"] = &FFIFuncDecl{
		Name:     "abs",
		RetType:  "i32",
		IsVarArg: false,
		Params:   []FFIParam{{Name: "x", Type: "i32"}},
	}

	// Valid: printf with varargs
	err := checker.ValidateCall("printf", 3)
	if err != nil {
		t.Errorf("printf(3 args) should be valid: %v", err)
	}

	// Valid: abs with 1 arg
	err = checker.ValidateCall("abs", 1)
	if err != nil {
		t.Errorf("abs(1 arg) should be valid: %v", err)
	}

	// Invalid: abs with 2 args
	err = checker.ValidateCall("abs", 2)
	if err == nil {
		t.Error("abs(2 args) should be invalid")
	}

	// Invalid: undeclared function
	err = checker.ValidateCall("undeclared", 0)
	if err == nil {
		t.Error("undeclared function should fail")
	}
}

func TestFFI_ValidateCallTypes(t *testing.T) {
	checker := NewFFIChecker()
	checker.Registered["memcpy"] = &FFIFuncDecl{
		Name:    "memcpy",
		RetType: "*void",
		Params: []FFIParam{
			{Name: "dst", Type: "*void"},
			{Name: "src", Type: "*void"},
			{Name: "n", Type: "u64"},
		},
	}

	// Valid types
	err := checker.ValidateCallTypes("memcpy", []string{"*i8", "*u8", "u32"})
	if err != nil {
		t.Errorf("pointer-to-pointer with compatible int should be valid: %v", err)
	}

	// Invalid: wrong number of args
	err = checker.ValidateCallTypes("memcpy", []string{"*void"})
	if err == nil {
		t.Error("too few args should fail")
	}

	// Invalid: undeclared
	err = checker.ValidateCallTypes("unknown", []string{})
	if err == nil {
		t.Error("undeclared function should fail")
	}
}

func TestFFI_GetReturnType(t *testing.T) {
	checker := NewFFIChecker()
	checker.Registered["abs"] = &FFIFuncDecl{Name: "abs", RetType: "i32"}
	checker.Registered["malloc"] = &FFIFuncDecl{Name: "malloc", RetType: "*void"}

	if rt := checker.GetReturnType("abs"); rt != "i32" {
		t.Errorf("expected 'i32', got '%s'", rt)
	}
	if rt := checker.GetReturnType("malloc"); rt != "*void" {
		t.Errorf("expected '*void', got '%s'", rt)
	}
	if rt := checker.GetReturnType("unknown"); rt != "" {
		t.Errorf("expected '' for unknown, got '%s'", rt)
	}
}

func TestFFI_GetParamTypes(t *testing.T) {
	checker := NewFFIChecker()
	checker.Registered["memcpy"] = &FFIFuncDecl{
		Name: "memcpy",
		Params: []FFIParam{
			{Name: "dst", Type: "*void"},
			{Name: "src", Type: "*void"},
			{Name: "n", Type: "u64"},
		},
	}

	types := checker.GetParamTypes("memcpy")
	if len(types) != 3 {
		t.Fatalf("expected 3 types, got %d", len(types))
	}
	if types[0] != "*void" || types[1] != "*void" || types[2] != "u64" {
		t.Errorf("unexpected types: %v", types)
	}

	types = checker.GetParamTypes("unknown")
	if types != nil {
		t.Errorf("expected nil for unknown, got %v", types)
	}
}

func TestFFI_HasDeclaration(t *testing.T) {
	checker := NewFFIChecker()
	checker.Registered["printf"] = &FFIFuncDecl{Name: "printf"}

	if !checker.HasDeclaration("printf") {
		t.Error("should have printf declaration")
	}
	if checker.HasDeclaration("abs") {
		t.Error("should not have abs declaration")
	}
}

func TestFFI_GenerateBridge(t *testing.T) {
	checker := NewFFIChecker()
	fn := &FFIFuncDecl{
		Name:     "my_func",
		RetType:  "i32",
		IsVarArg: true,
		Params: []FFIParam{
			{Name: "fmt", Type: "*i8"},
			{Name: "count", Type: "u32"},
		},
	}

	bridge := checker.GenerateFFIBridge(fn)
	if bridge == "" {
		t.Error("bridge should not be empty")
	}
	if !contains(bridge, "my_func") {
		t.Error("bridge should contain function name")
	}
}

func TestFFI_SimulateAbs(t *testing.T) {
	// Test abs simulation via FFI bridge
	checker := NewFFIChecker()
	checker.Registered["abs"] = &FFIFuncDecl{
		Name:    "abs",
		RetType: "i32",
		Params:  []FFIParam{{Name: "x", Type: "i32"}},
	}

	if !checker.HasDeclaration("abs") {
		t.Error("should have abs")
	}
	if checker.GetReturnType("abs") != "i32" {
		t.Errorf("expected i32 return, got %s", checker.GetReturnType("abs"))
	}
}

func TestFFI_SimulateStrlen(t *testing.T) {
	checker := NewFFIChecker()
	checker.Registered["strlen"] = &FFIFuncDecl{
		Name:    "strlen",
		RetType: "u64",
		Params:  []FFIParam{{Name: "s", Type: "*i8"}},
	}

	err := checker.ValidateCall("strlen", 1)
	if err != nil {
		t.Errorf("strlen(1 arg) should be valid: %v", err)
	}
}

func TestFFI_SimulatePow(t *testing.T) {
	checker := NewFFIChecker()
	checker.Registered["pow"] = &FFIFuncDecl{
		Name:    "pow",
		RetType: "f64",
		Params: []FFIParam{
			{Name: "base", Type: "f64"},
			{Name: "exp", Type: "f64"},
		},
	}

	err := checker.ValidateCall("pow", 2)
	if err != nil {
		t.Errorf("pow(2 args) should be valid: %v", err)
	}

	err = checker.ValidateCall("pow", 1)
	if err == nil {
		t.Error("pow(1 arg) should be invalid")
	}
}

func TestFFI_TypeCompatibility(t *testing.T) {
	tests := []struct {
		expected string
		actual   string
		want     bool
	}{
		{"i32", "i32", true},
		{"i32", "i64", true},     // int-to-int
		{"f64", "f32", true},     // float-to-float
		{"*void", "*i8", true},   // pointer-to-pointer
		{"i32", "f64", false},    // int-to-float
		{"*void", "i32", false},  // pointer-to-int
		{"void", "void", true},   // void
	}

	for _, tt := range tests {
		got := typesCompatible(tt.expected, tt.actual)
		if got != tt.want {
			t.Errorf("typesCompatible(%q, %q) = %v, want %v", tt.expected, tt.actual, got, tt.want)
		}
	}
}

func TestFFI_FFIRegistry(t *testing.T) {
	registry := jit.NewFFIRegistry()
	lib, err := registry.LoadLibrary("test_lib", "")
	if err != nil {
		t.Fatalf("load library failed: %v", err)
	}

	// Register a symbol
	lib.RegisterSymbol("test_func", 0, jit.CTypeI32, []jit.CType{jit.CTypeI32})

	if !lib.HasSymbol("test_func") {
		t.Error("should have test_func symbol")
	}
	if lib.HasSymbol("other_func") {
		t.Error("should not have other_func symbol")
	}

	// Get library
	got, err := registry.GetLibrary("test_lib")
	if err != nil {
		t.Fatalf("get library failed: %v", err)
	}
	if got.Name != "test_lib" {
		t.Errorf("expected 'test_lib', got '%s'", got.Name)
	}

	// Unload
	err = registry.UnloadLibrary("test_lib")
	if err != nil {
		t.Fatalf("unload failed: %v", err)
	}

	_, err = registry.GetLibrary("test_lib")
	if err == nil {
		t.Error("should fail to get unloaded library")
	}
}

func TestFFI_CTypeParsing(t *testing.T) {
	tests := []struct {
		input string
		want  jit.CType
	}{
		{"void", jit.CTypeVoid},
		{"i32", jit.CTypeI32},
		{"int", jit.CTypeI32},
		{"u64", jit.CTypeU64},
		{"f64", jit.CTypeF64},
		{"*void", jit.CTypePtr},
		{"*i8", jit.CTypeStr},
		{"bool", jit.CTypeBool},
	}

	for _, tt := range tests {
		got := jit.ParseCType(tt.input)
		if got != tt.want {
			t.Errorf("ParseCType(%q) = %d, want %d", tt.input, got, tt.want)
		}
	}
}

func TestFFI_CTypeString(t *testing.T) {
	tests := []struct {
		ct   jit.CType
		want string
	}{
		{jit.CTypeVoid, "void"},
		{jit.CTypeI32, "i32"},
		{jit.CTypeU64, "u64"},
		{jit.CTypeF64, "f64"},
		{jit.CTypePtr, "*void"},
		{jit.CTypeStr, "*i8"},
		{jit.CTypeBool, "bool"},
	}

	for _, tt := range tests {
		got := tt.ct.String()
		if got != tt.want {
			t.Errorf("CType(%d).String() = %q, want %q", tt.ct, got, tt.want)
		}
	}
}

func TestFFI_VaArgDeclaration(t *testing.T) {
	checker := NewFFIChecker()
	block, err := checker.ParseCImportBlock(`fn my_printf(fmt: *i8, ...) -> i32;`)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	if len(block.Functions) != 1 {
		t.Fatalf("expected 1 function, got %d", len(block.Functions))
	}

	fn := block.Functions[0]
	if !fn.IsVarArg {
		t.Error("expected varargs")
	}
	if fn.RetType != "i32" {
		t.Errorf("expected i32 return, got %s", fn.RetType)
	}
	if len(fn.Params) != 1 {
		t.Errorf("expected 1 param, got %d", len(fn.Params))
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && containsHelper(s, sub))
}

func containsHelper(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
