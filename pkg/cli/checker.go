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
// columns) into structured diagnostics. When a SourceMap is provided, the line
// is attributed to its owning file so multi-module units report per-file spans;
// otherwise every diagnostic is stamped with targetFile.
func collectSyntaxDiagnostics(targetFile string, p *parser.Parser, src string, srcMap sema.SourceMap) []diagnostics.Diagnostic {
	var diags []diagnostics.Diagnostic
	for i, parseErr := range p.Errors {
		line, _ := extractLineCol(parseErr)
		col := 1
		if i < len(p.ErrorCols) && p.ErrorCols[i] > 0 {
			col = p.ErrorCols[i]
		}
		file := targetFile
		if srcMap != nil {
			if owned := srcMap[line]; owned != "" {
				file = owned
			}
		}
		d := diagnostics.ErrorDiagnostic(file, line, col, diagnostics.CodeSyntax, parseErr)
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
		return collectSyntaxDiagnostics(file, p, src, srcMap), nil, 0
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

	// NPU target attribute stage (Phase 98): validate @target(...) placement
	// and target names. Placement is enforced by the parser; this stage
	// rejects unknown/unsupported execution targets.
	if npuDiags := npuTargetDiagnostics(file, src); len(npuDiags) > 0 {
		return npuDiags, nil, len(prog.Statements)
	}

	// Warning pass: only runs for programs that survived resolution, so a
	// missing name can never cascade into bogus unused-variable findings.
	return nil, collectUnusedWarnings(file, src, sema.UnusedVars(prog)), len(prog.Statements)
}

// npuTargetDiagnostics validates every @target(...) attribute in src with the
// Go front-end analyzer and returns one diagnostic per unknown/unsupported
// execution target. It is deliberately parse-only (no name resolution, no type
// checking) so it can be run over sources whose deeper validation is performed
// by another front end — specifically, the self-hosted engine's check/build/run
// paths, which preserve @target as inert C comments and therefore cannot reject
// an invalid target themselves. Syntax errors are not reported here: the engine
// that owns the parse reports them with its own spans.
func npuTargetDiagnostics(file, src string) []diagnostics.Diagnostic {
	l := lexer.New(src)
	p := parser.New(l)
	prog := p.ParseProgram()
	if len(p.Errors) > 0 {
		return nil
	}
	prog = parser.ApplyMacroExpansion(prog)
	npuAnalyzer := sema.NewNPUAnalyzer()
	var npuDiags []diagnostics.Diagnostic
	for _, ne := range npuAnalyzer.Analyze(prog) {
		col := 1
		if ne.Col > 0 {
			col = ne.Col + 1
		}
		d := diagnostics.ErrorDiagnostic(file, ne.Line, col, diagnostics.CodeSema, ne.Msg)
		d.Excerpt = source.Excerpt(src, ne.Line, 80)
		npuDiags = append(npuDiags, d)
	}
	return npuDiags
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

// runSemanticPreflight is the build/run analog of AnalyzeSource's resolve+sema
// stages: whole-program name resolution plus NPU @target validation over a
// program that has already been lexed, parsed and macro-expanded by the caller
// (parseSource). Errors short-circuit exactly like the check path (resolve
// before NPU), producing the same structured diagnostics and exit contract so
// `karkain build`/`karkain run` reject semantically invalid programs with a
// clean `error[K00x]` report and ExitCompile — never raw compiler noise from
// codegen of an un-resolvable program. Returns nil when the program resolves.
func runSemanticPreflight(file, src string, srcMap sema.SourceMap, prog *parser.Program) []diagnostics.Diagnostic {
	resolver := sema.NewResolver(prog, srcMap)
	if resolveErrs := resolver.Resolve(); len(resolveErrs) > 0 {
		return collectResolveDiagnostics(file, src, resolveErrs)
	}
	if npuDiags := npuTargetDiagnostics(file, src); len(npuDiags) > 0 {
		return npuDiags
	}
	return nil
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
