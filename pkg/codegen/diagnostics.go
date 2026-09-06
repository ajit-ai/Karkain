package codegen

import (
	"fmt"
	"karkain/pkg/diagnostics"
)

// CodegenDiagnostics provides diagnostic reporting for code generation
type CodegenDiagnostics struct {
	diagnostics []diagnostics.Diagnostic
	sourceFile  string
}

// NewCodegenDiagnostics creates a new codegen diagnostics instance
func NewCodegenDiagnostics(sourceFile string) *CodegenDiagnostics {
	return &CodegenDiagnostics{
		diagnostics: make([]diagnostics.Diagnostic, 0),
		sourceFile:  sourceFile,
	}
}

// AddError adds an error diagnostic
func (cd *CodegenDiagnostics) AddError(line, col int, code diagnostics.Code, message string) {
	diag := diagnostics.ErrorDiagnostic(cd.sourceFile, line, col, code, message)
	cd.diagnostics = append(cd.diagnostics, diag)
}

// AddWarning adds a warning diagnostic
func (cd *CodegenDiagnostics) AddWarning(line, col int, code diagnostics.Code, message string) {
	diag := diagnostics.WarningDiagnostic(cd.sourceFile, line, col, code, message)
	cd.diagnostics = append(cd.diagnostics, diag)
}

// AddErrorWithHelp adds an error diagnostic with help text
func (cd *CodegenDiagnostics) AddErrorWithHelp(line, col int, code diagnostics.Code, message, help string) {
	diag := diagnostics.ErrorDiagnostic(cd.sourceFile, line, col, code, message)
	diag = diag.WithHelp(help)
	cd.diagnostics = append(cd.diagnostics, diag)
}

// AddWarningWithHelp adds a warning diagnostic with help text
func (cd *CodegenDiagnostics) AddWarningWithHelp(line, col int, code diagnostics.Code, message, help string) {
	diag := diagnostics.WarningDiagnostic(cd.sourceFile, line, col, code, message)
	diag = diag.WithHelp(help)
	cd.diagnostics = append(cd.diagnostics, diag)
}

// GetDiagnostics returns all diagnostics
func (cd *CodegenDiagnostics) GetDiagnostics() []diagnostics.Diagnostic {
	return cd.diagnostics
}

// HasErrors returns true if there are any error diagnostics
func (cd *CodegenDiagnostics) HasErrors() bool {
	for _, diag := range cd.diagnostics {
		if diag.IsError() {
			return true
		}
	}
	return false
}

// HasWarnings returns true if there are any warning diagnostics
func (cd *CodegenDiagnostics) HasWarnings() bool {
	for _, diag := range cd.diagnostics {
		if diag.IsWarning() {
			return true
		}
	}
	return false
}

// ErrorCount returns the number of error diagnostics
func (cd *CodegenDiagnostics) ErrorCount() int {
	count := 0
	for _, diag := range cd.diagnostics {
		if diag.IsError() {
			count++
		}
	}
	return count
}

// WarningCount returns the number of warning diagnostics
func (cd *CodegenDiagnostics) WarningCount() int {
	count := 0
	for _, diag := range cd.diagnostics {
		if diag.IsWarning() {
			count++
		}
	}
	return count
}

// Clear clears all diagnostics
func (cd *CodegenDiagnostics) Clear() {
	cd.diagnostics = make([]diagnostics.Diagnostic, 0)
}

// ReportSymbolError reports a symbol-related error
func (cd *CodegenDiagnostics) ReportSymbolError(line, col int, symbolName, reason string) {
	message := fmt.Sprintf("symbol error: '%s' - %s", symbolName, reason)
	cd.AddError(line, col, diagnostics.CodeCodegen, message)
}

// ReportRelocationError reports a relocation-related error
func (cd *CodegenDiagnostics) ReportRelocationError(line, col int, symbolName, relocType string) {
	message := fmt.Sprintf("relocation error: cannot resolve '%s' for %s", symbolName, relocType)
	help := "ensure the symbol is defined and accessible"
	cd.AddErrorWithHelp(line, col, diagnostics.CodeCodegen, message, help)
}

// ReportLinkerError reports a linker-related error
func (cd *CodegenDiagnostics) ReportLinkerError(line, col int, stage, reason string) {
	message := fmt.Sprintf("linker error: %s - %s", stage, reason)
	cd.AddError(line, col, diagnostics.CodeCodegen, message)
}

// ReportObjectError reports an object file generation error
func (cd *CodegenDiagnostics) ReportObjectError(line, col int, section, reason string) {
	message := fmt.Sprintf("object generation error: section '%s' - %s", section, reason)
	cd.AddError(line, col, diagnostics.CodeCodegen, message)
}

// ReportDebugInfoError reports a debug information error
func (cd *CodegenDiagnostics) ReportDebugInfoError(line, col int, debugItem, reason string) {
	message := fmt.Sprintf("debug info error: %s - %s", debugItem, reason)
	cd.AddWarning(line, col, diagnostics.CodeCodegen, message)
}

// ReportCodegenError reports a general code generation error
func (cd *CodegenDiagnostics) ReportCodegenError(line, col int, stage, reason string) {
	message := fmt.Sprintf("code generation error: %s - %s", stage, reason)
	cd.AddError(line, col, diagnostics.CodeCodegen, message)
}

// ConvertToDiagnostics converts codegen errors to Phase-83 diagnostics
func ConvertToDiagnostics(sourceFile string, errors []string) []diagnostics.Diagnostic {
	var diags []diagnostics.Diagnostic
	
	for i, err := range errors {
		line := i + 1 // Approximate line number
		diag := diagnostics.ErrorDiagnostic(sourceFile, line, 1, diagnostics.CodeCodegen, err)
		diags = append(diags, diag)
	}
	
	return diags
}

// ReportNativeCodegenError reports native code generation errors using Phase-83 diagnostics
func ReportNativeCodegenError(sourceFile string, err error) diagnostics.Diagnostic {
	line := 1 // Default line number for codegen errors
	message := fmt.Sprintf("native code generation failed: %v", err)
	help := "check the Karkain source for syntax or semantic errors"
	
	diag := diagnostics.ErrorDiagnostic(sourceFile, line, 1, diagnostics.CodeCodegen, message)
	diag = diag.WithHelp(help)
	return diag
}

// ReportLinkerDiagnostic reports linker diagnostics using Phase-83 framework
func ReportLinkerDiagnostic(sourceFile string, stage string, err error) diagnostics.Diagnostic {
	line := 1 // Default line number for linker errors
	message := fmt.Sprintf("linker %s failed: %v", stage, err)
	help := "ensure all referenced symbols are defined and object files are valid"
	
	diag := diagnostics.ErrorDiagnostic(sourceFile, line, 1, diagnostics.CodeCodegen, message)
	diag = diag.WithHelp(help)
	return diag
}

// ReportObjectDiagnostic reports object file diagnostics using Phase-83 framework
func ReportObjectDiagnostic(sourceFile string, section string, err error) diagnostics.Diagnostic {
	line := 1 // Default line number for object errors
	message := fmt.Sprintf("object generation failed for section '%s': %v", section, err)
	help := "check the code generation output for syntax errors"
	
	diag := diagnostics.ErrorDiagnostic(sourceFile, line, 1, diagnostics.CodeCodegen, message)
	diag = diag.WithHelp(help)
	return diag
}

// ReportSymbolDiagnostic reports symbol-related diagnostics using Phase-83 framework
func ReportSymbolDiagnostic(sourceFile string, symbolName string, err error) diagnostics.Diagnostic {
	line := 1 // Default line number for symbol errors
	message := fmt.Sprintf("symbol error for '%s': %v", symbolName, err)
	help := "ensure the symbol is properly defined and not conflicting with runtime symbols"
	
	diag := diagnostics.ErrorDiagnostic(sourceFile, line, 1, diagnostics.CodeCodegen, message)
	diag = diag.WithHelp(help)
	return diag
}

// ReportRelocationDiagnostic reports relocation diagnostics using Phase-83 framework
func ReportRelocationDiagnostic(sourceFile string, symbolName string, relocType string, err error) diagnostics.Diagnostic {
	line := 1 // Default line number for relocation errors
	message := fmt.Sprintf("relocation error for '%s' (%s): %v", symbolName, relocType, err)
	help := "ensure the symbol is defined and the relocation type is supported"
	
	diag := diagnostics.ErrorDiagnostic(sourceFile, line, 1, diagnostics.CodeCodegen, message)
	diag = diag.WithHelp(help)
	return diag
}

// ReportDebugDiagnostic reports debug information diagnostics using Phase-83 framework
func ReportDebugDiagnostic(sourceFile string, debugItem string, err error) diagnostics.Diagnostic {
	line := 1 // Default line number for debug errors
	message := fmt.Sprintf("debug info error for '%s': %v", debugItem, err)
	help := "debug information generation may be incomplete, but compilation will continue"
	
	diag := diagnostics.WarningDiagnostic(sourceFile, line, 1, diagnostics.CodeCodegen, message)
	diag = diag.WithHelp(help)
	return diag
}
