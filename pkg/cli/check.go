package cli

import (
	"fmt"
	"karkain/pkg/diagnostics"
	"karkain/pkg/lexer"
	"karkain/pkg/parser"
	"karkain/pkg/sema"
	"os"
)

// CheckFormatHuman is the default human-readable diagnostic output.
const CheckFormatHuman = ""

// CheckFormatJSON selects machine-readable structured diagnostics.
const CheckFormatJSON = "json"

// checkStage distinguishes which front-end phase produced a diagnostic set, so
// the human renderer can keep its existing short-circuit behavior.
type checkStage int

const (
	checkStageNone checkStage = iota
	checkStageSyntax
	checkStageResolve
	checkStageSema
)

// CheckCommand validates a .kark file without producing output binaries.
func CheckCommand(targetFile string, verbose bool) CommandResult {
	return CheckCommandFormatted(targetFile, verbose, CheckFormatHuman)
}

// CheckCommandFormatted is CheckCommand with a structured-output mode. format
// "" renders the diagnostic report to stderr as before; format "json" writes a
// []diagnostics.Diagnostic array to stdout (nothing else touches stdout). Exit
// codes and short-circuit ordering are identical across formats.
func CheckCommandFormatted(targetFile string, verbose bool, format string) CommandResult {
	if err := ValidateKarFile(targetFile); err != nil {
		return CommandResult{ExitCode: ExitUsage, Message: err.Error()}
	}

	sourceText, srcMap, err := resolveSourcesCheck(targetFile)
	if err != nil {
		return sourceLoadResult(err)
	}

	src := sourceText
	l := lexer.New(src)
	p := parser.New(l)
	prog := p.ParseProgram()

	// Check for parse errors
	if len(p.Errors) > 0 {
		diags := collectSyntaxDiagnostics(targetFile, p)
		if format == CheckFormatJSON {
			emitJSONDiagnostics(diags)
			return CommandResult{ExitCode: ExitCompile, Message: ""}
		}
		renderDiagnostics(src, targetFile, diags)
		return CommandResult{ExitCode: ExitCompile, Message: fmt.Sprintf("%d parse error(s) found", len(p.Errors))}
	}

	// Apply macro expansion
	prog = parser.ApplyMacroExpansion(prog)

	// Whole-program name-resolution diagnostics: duplicate top-level
	// definitions and undefined bare function references at the Karkain level.
	resolver := sema.NewResolver(prog, srcMap)
	if resolveErrs := resolver.Resolve(); len(resolveErrs) > 0 {
		diags := collectResolveDiagnostics(targetFile, resolveErrs)
		if format == CheckFormatJSON {
			emitJSONDiagnostics(diags)
			return CommandResult{ExitCode: ExitCompile, Message: ""}
		}
		renderDiagnostics(src, targetFile, diags)
		return CommandResult{ExitCode: ExitCompile, Message: fmt.Sprintf("%d name-resolution error(s) found", len(resolveErrs))}
	}

	// Run kernel analyzer for semantic checks
	analyzer := sema.NewKernelAnalyzer()
	errorCount := 0
	var semaDiags []diagnostics.Diagnostic
	for _, stmt := range prog.Statements {
		if kernel, ok := stmt.(*parser.KernelDeclStmt); ok {
			kernelErrors := analyzer.AnalyzeKernel(kernel)
			if len(kernelErrors) > 0 {
				for _, ke := range kernelErrors {
					semaDiags = append(semaDiags, diagnostics.ErrorDiagnostic(targetFile, 1, 1, diagnostics.CodeSema, ke.Error()))
				}
				errorCount += len(kernelErrors)
			}
		}
	}

	if errorCount > 0 {
		if format == CheckFormatJSON {
			emitJSONDiagnostics(semaDiags)
			return CommandResult{ExitCode: ExitCompile, Message: ""}
		}
		renderDiagnostics(src, targetFile, semaDiags)
		return CommandResult{ExitCode: ExitCompile, Message: fmt.Sprintf("%d semantic error(s) found", errorCount)}
	}

	if verbose {
		fmt.Printf("Check passed: %s (%d statements)\n", targetFile, len(prog.Statements))
	}
	if format == CheckFormatJSON {
		return CommandResult{ExitCode: ExitSuccess, Message: ""}
	}
	return CommandResult{ExitCode: ExitSuccess, Message: "Check passed."}
}

// collectSyntaxDiagnostics converts parser errors (strings + parallel token
// columns) into structured diagnostics.
func collectSyntaxDiagnostics(targetFile string, p *parser.Parser) []diagnostics.Diagnostic {
	var diags []diagnostics.Diagnostic
	for i, parseErr := range p.Errors {
		line, _ := extractLineCol(parseErr)
		col := 1
		if i < len(p.ErrorCols) && p.ErrorCols[i] > 0 {
			col = p.ErrorCols[i]
		}
		diags = append(diags, diagnostics.ErrorDiagnostic(targetFile, line, col, diagnostics.CodeSyntax, parseErr))
	}
	return diags
}

// collectResolveDiagnostics converts name-resolution errors into structured
// diagnostics. Resolver errors are line-anchored (column best-effort = 1).
func collectResolveDiagnostics(targetFile string, resolveErrs []sema.ResolveError) []diagnostics.Diagnostic {
	var diags []diagnostics.Diagnostic
	for _, re := range resolveErrs {
		diags = append(diags, diagnostics.ErrorDiagnostic(targetFile, re.Line, 1, diagnostics.CodeResolve, re.Msg))
	}
	return diags
}

// emitJSONDiagnostics writes the structured diagnostic set as JSON to stdout.
func emitJSONDiagnostics(diags []diagnostics.Diagnostic) {
	out, err := diagnostics.MarshalDiagnostics(diags)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error serializing diagnostics: %v\n", err)
		return
	}
	fmt.Println(string(out))
}

// renderDiagnostics prints diagnostics in the existing human form.
func renderDiagnostics(src, targetFile string, diags []diagnostics.Diagnostic) {
	reporter := diagnostics.NewReporter(src, targetFile)
	for _, d := range diags {
		fmt.Fprint(os.Stderr, reporter.Report(diagnostics.SeverityError, d.Line, d.Column, d.Message))
	}
}