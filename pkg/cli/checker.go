package cli

import (
	"fmt"

	"karkain/pkg/diagnostics"
	"karkain/pkg/lexer"
	"karkain/pkg/parser"
	"karkain/pkg/sema"
	"karkain/pkg/source"
)

// collectSyntaxDiagnostics converts parser errors (strings + parallel token
// columns) into structured diagnostics.
func collectSyntaxDiagnostics(targetFile string, p *parser.Parser, src string) []diagnostics.Diagnostic {
	var diags []diagnostics.Diagnostic
	for i, parseErr := range p.Errors {
		line, _ := extractLineCol(parseErr)
		col := 1
		if i < len(p.ErrorCols) && p.ErrorCols[i] > 0 {
			col = p.ErrorCols[i]
		}
		d := diagnostics.ErrorDiagnostic(targetFile, line, col, diagnostics.CodeSyntax, parseErr)
		d.Excerpt = source.Excerpt(src, line, 80)
		diags = append(diags, d)
	}
	return diags
}

// collectResolveDiagnostics converts name-resolution errors into structured
// diagnostics. Resolver errors are span-aware: column and endColumn come from
// the parser-recorded token columns (0-based in the resolver, 1-based here).
func collectResolveDiagnostics(targetFile string, src string, resolveErrs []sema.ResolveError) []diagnostics.Diagnostic {
	var diags []diagnostics.Diagnostic
	for _, re := range resolveErrs {
		col := 1
		if re.Col > 0 {
			col = re.Col + 1
		}
		d := diagnostics.ErrorDiagnostic(targetFile, re.Line, col, diagnostics.CodeResolve, re.Msg)
		if re.EndCol > re.Col {
			d.EndColumn = re.EndCol + 1
		}
		d.Excerpt = source.Excerpt(src, re.Line, 80)
		diags = append(diags, d)
	}
	return diags
}

// AnalyzeSource is the canonical front-end diagnostic pass shared by the CLI
// (`karkain check`) and the LSP. It runs lex+parse, macro expansion, whole-
// program name resolution, the kernel analyzer and the Phase 83 warning
// pass over an already-assembled source unit, returning the first failing
// stage's structured error diagnostics, the collected warnings, and the
// statement count.
//
// Errors short-circuit (identical stage ordering to the CLI); warnings are
// computed only once the program resolves, never terminate compilation, and
// use the same Diagnostic model so the human report, the JSON wire format and
// IDE severity mapping all share one shape.
//
// It performs no file I/O and never prints. “file“ is stamped into each
// diagnostic for the CLI wire format; the LSP passes "" because it attaches
// the document URI separately at publish time. Passing a nil SourceMap keeps
// cross-file visibility enforcement off (single-document units).
func AnalyzeSource(file string, src string, srcMap sema.SourceMap) (diags []diagnostics.Diagnostic, warns []diagnostics.Diagnostic, stmts int) {
	l := lexer.New(src)
	p := parser.New(l)
	prog := p.ParseProgram()

	// Stage 1: syntax.
	if len(p.Errors) > 0 {
		return collectSyntaxDiagnostics(file, p, src), nil, 0
	}

	// Whole-program name resolution (Phase 81 fix: macro expansion runs
	// before resolution and preserves node spans).
	prog = parser.ApplyMacroExpansion(prog)
	resolver := sema.NewResolver(prog, srcMap)
	if resolveErrs := resolver.Resolve(); len(resolveErrs) > 0 {
		return collectResolveDiagnostics(file, src, resolveErrs), nil, 0
	}

	// Kernel analyzer stage.
	analyzer := sema.NewKernelAnalyzer()
	var kernelDiags []diagnostics.Diagnostic
	for _, stmt := range prog.Statements {
		if kernel, ok := stmt.(*parser.KernelDeclStmt); ok {
			for _, ke := range analyzer.AnalyzeKernel(kernel) {
				kernelDiags = append(kernelDiags, diagnostics.ErrorDiagnostic(file, 1, 1, diagnostics.CodeSema, ke.Error()))
			}
		}
	}
	if len(kernelDiags) > 0 {
		return kernelDiags, nil, len(prog.Statements)
	}

	// Warning pass: only runs for programs that survived resolution, so a
	// missing name can never cascade into bogus unused-variable findings.
	return nil, collectUnusedWarnings(file, src, sema.UnusedVars(prog)), len(prog.Statements)
}

// collectUnusedWarnings converts sema name-warnings into structured
// diagnostics with true 1-based columns and spans.
func collectUnusedWarnings(file string, src string, warns []sema.Warning) []diagnostics.Diagnostic {
	diags := make([]diagnostics.Diagnostic, 0, len(warns))
	for _, w := range warns {
		col := 1
		if w.Col > 0 {
			col = w.Col + 1
		}
		d := diagnostics.WarningDiagnostic(file, w.Line, col, diagnostics.CodeWarnUnused, w.Msg)
		if w.EndCol > w.Col {
			d.EndColumn = w.EndCol + 1
		}
		d.Excerpt = source.Excerpt(src, w.Line, 80)
		diags = append(diags, d)
	}
	return diags
}

// checkStageFor derives the failing stage from a diagnostic set produced by
// AnalyzeSource (stages short-circuit, so all diagnostics share one code).
func checkStageFor(diags []diagnostics.Diagnostic) checkStage {
	for _, d := range diags {
		switch d.Code {
		case string(diagnostics.CodeSyntax):
			return checkStageSyntax
		case string(diagnostics.CodeResolve):
			return checkStageResolve
		case string(diagnostics.CodeSema):
			return checkStageSema
		}
	}
	return checkStageNone
}

// checkStageMessage formats a stage heading for the human report.
func checkStageMessage(stage checkStage, n int) string {
	switch stage {
	case checkStageSyntax:
		return fmt.Sprintf("%d parse error(s) found", n)
	case checkStageResolve:
		return fmt.Sprintf("%d name-resolution error(s) found", n)
	case checkStageSema:
		return fmt.Sprintf("%d semantic error(s) found", n)
	}
	return fmt.Sprintf("%d diagnostic(s) found", n)
}
