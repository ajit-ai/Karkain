package codegen

import (
	"fmt"
	"strings"
	"testing"

	"karkain/pkg/diagnostics"
	"karkain/pkg/parser"
)

func TestCodegenDiagnostics_AddError(t *testing.T) {
	cd := NewCodegenDiagnostics("test.kark")
	cd.AddError(10, 5, diagnostics.CodeCodegen, "codegen failed")

	diags := cd.GetDiagnostics()
	if len(diags) != 1 {
		t.Fatalf("expected 1 diagnostic, got %d", len(diags))
	}
	d := diags[0]
	if !d.IsError() {
		t.Error("expected error severity")
	}
	if d.File != "test.kark" {
		t.Errorf("expected file 'test.kark', got '%s'", d.File)
	}
	if d.Line != 10 || d.Column != 5 {
		t.Errorf("expected line 10 col 5, got line %d col %d", d.Line, d.Column)
	}
}

func TestCodegenDiagnostics_AddWarning(t *testing.T) {
	cd := NewCodegenDiagnostics("test.kark")
	cd.AddWarning(5, 1, diagnostics.CodeWarnUnused, "debug warning")

	diags := cd.GetDiagnostics()
	if len(diags) != 1 {
		t.Fatalf("expected 1 diagnostic, got %d", len(diags))
	}
	if !diags[0].IsWarning() {
		t.Error("expected warning severity")
	}
}

func TestCodegenDiagnostics_HasErrorsWarnings(t *testing.T) {
	cd := NewCodegenDiagnostics("test.kark")
	cd.AddError(1, 1, diagnostics.CodeCodegen, "err")
	cd.AddWarning(2, 1, diagnostics.CodeWarnUnused, "warn")

	if !cd.HasErrors() || !cd.HasWarnings() {
		t.Error("expected both HasErrors and HasWarnings true")
	}
	if cd.ErrorCount() != 1 || cd.WarningCount() != 1 {
		t.Errorf("counts wrong: errors=%d warnings=%d", cd.ErrorCount(), cd.WarningCount())
	}
}

func TestCodegenDiagnostics_Clear(t *testing.T) {
	cd := NewCodegenDiagnostics("test.kark")
	cd.AddError(1, 1, diagnostics.CodeCodegen, "err")
	cd.Clear()
	if cd.ErrorCount() != 0 {
		t.Error("Clear should remove all diagnostics")
	}
}

func TestCodegenDiagnostics_AddErrorWithHelp(t *testing.T) {
	cd := NewCodegenDiagnostics("test.kark")
	cd.AddErrorWithHelp(1, 1, diagnostics.CodeCodegen, "linker failed", "check symbols")

	diags := cd.GetDiagnostics()
	if len(diags) != 1 {
		t.Fatalf("expected 1 diagnostic, got %d", len(diags))
	}
	if diags[0].Help == "" {
		t.Error("expected non-empty help text")
	}
	if !strings.Contains(diags[0].Help, "check symbols") {
		t.Errorf("help text wrong: %s", diags[0].Help)
	}
}

func TestReportSymbolError(t *testing.T) {
	cd := NewCodegenDiagnostics("test.kark")
	cd.ReportSymbolError(3, 5, "karkain_user_foo", "undefined")

	diags := cd.GetDiagnostics()
	if len(diags) != 1 {
		t.Fatalf("expected 1 diagnostic, got %d", len(diags))
	}
	if !strings.Contains(diags[0].Message, "karkain_user_foo") {
		t.Errorf("message should mention symbol: %s", diags[0].Message)
	}
}

func TestReportRelocationError(t *testing.T) {
	cd := NewCodegenDiagnostics("test.kark")
	cd.ReportRelocationError(1, 1, "ghost", "ADDR32")

	diags := cd.GetDiagnostics()
	if len(diags) != 1 {
		t.Fatalf("expected 1 diagnostic, got %d", len(diags))
	}
	if !strings.Contains(diags[0].Message, "ghost") || !strings.Contains(diags[0].Message, "ADDR32") {
		t.Errorf("message should mention symbol and reloc type: %s", diags[0].Message)
	}
	if diags[0].Help == "" {
		t.Error("expected help text on relocation error")
	}
}

func TestConvertToDiagnostics(t *testing.T) {
	errors := []string{"error 1", "error 2"}
	diags := ConvertToDiagnostics("src.kark", errors)

	if len(diags) != 2 {
		t.Fatalf("expected 2 diagnostics, got %d", len(diags))
	}
	for i, d := range diags {
		if !d.IsError() {
			t.Errorf("diag %d should be error", i)
		}
		if d.File != "src.kark" {
			t.Errorf("diag %d file wrong: %s", i, d.File)
		}
	}
	if diags[0].Message != "error 1" {
		t.Errorf("diag 0 message wrong: %s", diags[0].Message)
	}
}

func TestReportNativeCodegenError(t *testing.T) {
	diag := ReportNativeCodegenError("src.kark", fmt.Errorf("emit failed"))
	if !diag.IsError() {
		t.Error("expected error severity")
	}
	if !strings.Contains(diag.Message, "emit failed") {
		t.Errorf("message should contain error: %s", diag.Message)
	}
	if diag.Help == "" {
		t.Error("expected help text")
	}
}

func TestReportLinkerDiagnostic(t *testing.T) {
	diag := ReportLinkerDiagnostic("src.kark", "symbol resolution", fmt.Errorf("undefined sym"))
	if !diag.IsError() {
		t.Error("expected error severity")
	}
	if !strings.Contains(diag.Message, "symbol resolution") {
		t.Errorf("message should mention stage: %s", diag.Message)
	}
}

func TestReportObjectDiagnostic(t *testing.T) {
	diag := ReportObjectDiagnostic("src.kark", ".text", fmt.Errorf("overflow"))
	if !diag.IsError() {
		t.Error("expected error severity")
	}
	if !strings.Contains(diag.Message, ".text") {
		t.Errorf("message should mention section: %s", diag.Message)
	}
}

func TestReportDebugDiagnostic(t *testing.T) {
	diag := ReportDebugDiagnostic("src.kark", "line table", fmt.Errorf("corrupt"))
	if !diag.IsWarning() {
		t.Error("expected warning severity (debug info failures are non-fatal)")
	}
	if !strings.Contains(diag.Help, "incomplete") {
		t.Errorf("help should mention incomplete: %s", diag.Help)
	}
}

func TestGenerateObject_ASTToObject(t *testing.T) {
	prog := &parser.Program{
		Statements: []parser.Node{
			&parser.FuncDecl{
				Name:   "main",
				Params: []string{},
				Body:   []parser.Node{},
			},
			&parser.StructDeclStmt{
				Name:   "Point",
				Fields: []parser.StructField{{Name: "x", Type: "int"}},
			},
		},
	}

	gen := NewNativeGenerator()
	obj, diags, err := gen.GenerateObject(prog, "test.kark")
	if err != nil {
		t.Fatalf("GenerateObject failed: %v", err)
	}
	// Diagnostics from symbol collection should be clean
	if len(diags) > 0 {
		for _, d := range diags {
			if d.IsError() {
				t.Errorf("unexpected error diagnostic: %s", d.Message)
			}
		}
	}

	// Sections
	if len(obj.Sections) != 3 {
		t.Fatalf("expected 3 sections, got %d", len(obj.Sections))
	}
	for _, name := range []string{".text", ".rodata", ".data"} {
		if obj.FindSection(name) == nil {
			t.Errorf("missing section '%s'", name)
		}
	}

	// Symbols (main + Point)
	names := make(map[string]bool)
	for _, sym := range obj.Symbols {
		names[sym.Name] = true
	}
	if !names["main"] {
		t.Error("expected 'main' symbol")
	}
	if !names["Point"] {
		t.Error("expected 'Point' symbol")
	}

	// Debug info
	if obj.DebugInfo == nil {
		t.Fatal("expected non-nil DebugInfo")
	}
	if len(obj.DebugInfo.FunctionInfo) == 0 {
		t.Error("expected at least one function in debug info")
	}
}

func TestGenerateObject_DuplicateMainDiagnostic(t *testing.T) {
	prog := &parser.Program{
		Statements: []parser.Node{
			&parser.FuncDecl{Name: "main", Params: []string{}, Body: []parser.Node{}},
			&parser.FuncDecl{Name: "main", Params: []string{}, Body: []parser.Node{}},
		},
	}

	gen := NewNativeGenerator()
	_, diags, err := gen.GenerateObject(prog, "test.kark")
	if err == nil {
		t.Fatal("expected error for duplicate main")
	}
	found := false
	for _, d := range diags {
		if d.IsError() {
			found = true
		}
	}
	if !found {
		t.Error("expected at least one error diagnostic for duplicate main")
	}
}
