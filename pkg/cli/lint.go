package cli

import (
	"fmt"
	"karkain/pkg/diagnostics"
	"karkain/pkg/lexer"
	"karkain/pkg/parser"
	"karkain/pkg/sema"
	"os"
)

// lintIssue is one reported finding with its stable code class.
type lintIssue struct {
	Code string
	Line int
	Msg  string
}

// LintCommand runs every front-end analysis the toolchain provides without
// producing output binaries: lexical/parser checks, macro expansion,
// whole-program name resolution, kernel semantic analysis, and the borrow
// checker. It differs from `check` by also running the borrow checker and by
// tagging every finding with a stable error code (E-K-*).
func LintCommand(targetFile string, verbose bool) CommandResult {
	if err := ValidateKarFile(targetFile); err != nil {
		return CommandResult{ExitCode: ExitUsage, Message: err.Error()}
	}

	sourceText, srcMap, err := resolveSourcesWithMap(targetFile)
	if err != nil {
		return CommandResult{ExitCode: ExitFailure, Message: fmt.Sprintf("Error reading file: %v", err)}
	}

	src := sourceText
	l := lexer.New(src)
	p := parser.New(l)
	prog := p.ParseProgram()

	var issues []lintIssue

	// Syntax: lexer/parser errors. A malformed program short-circuits: later
	// analyses would only produce confusing cascading noise.
	if len(p.Errors) > 0 {
		reporter := diagnostics.NewReporter(src, targetFile)
		fmt.Printf("E-K-SYN\n")
		for _, pe := range p.Errors {
			line, col := extractLineCol(pe)
			issues = append(issues, lintIssue{Code: string(diagnostics.CodeSyntax), Line: line, Msg: pe})
			fmt.Fprint(os.Stderr, reporter.Report(diagnostics.SeverityError, line, col, pe))
		}
		return CommandResult{ExitCode: ExitCompile, Message: fmt.Sprintf("Lint found %d issue(s)", len(issues))}
	}

	prog = parser.ApplyMacroExpansion(prog)

	// Name resolution: undefined/duplicate/private-access violations.
	resolver := sema.NewResolver(prog, srcMap)
	if resolveErrs := resolver.Resolve(); len(resolveErrs) > 0 {
		reporter := diagnostics.NewReporter(src, targetFile)
		fmt.Printf("E-K-RES\n")
		for _, re := range resolveErrs {
			issues = append(issues, lintIssue{Code: string(diagnostics.CodeResolve), Line: re.Line, Msg: re.Msg})
			fmt.Fprint(os.Stderr, reporter.Report(diagnostics.SeverityError, re.Line, 1, re.Msg))
		}
	}

	// Semantic analysis: kernel declarations.
	analyzer := sema.NewKernelAnalyzer()
	reporter := diagnostics.NewReporter(src, targetFile)
	semaIssues := 0
	for _, stmt := range prog.Statements {
		if kernel, ok := stmt.(*parser.KernelDeclStmt); ok {
			for _, ke := range analyzer.AnalyzeKernel(kernel) {
				semaIssues++
				issues = append(issues, lintIssue{Code: string(diagnostics.CodeSema), Line: 1, Msg: ke.Error()})
				fmt.Fprint(os.Stderr, reporter.Report(diagnostics.SeverityError, 1, 1, ke.Error()))
			}
		}
	}
	if semaIssues > 0 {
		fmt.Printf("E-K-SEM\n")
	}

	// Semantic analysis: @target(...) function attributes (Phase 98).
	npuAnalyzer := sema.NewNPUAnalyzer()
	npuSemaIssues := 0
	for _, ne := range npuAnalyzer.Analyze(prog) {
		npuSemaIssues++
		col := 1
		if ne.Col > 0 {
			col = ne.Col + 1
		}
		issues = append(issues, lintIssue{Code: string(diagnostics.CodeSema), Line: ne.Line, Msg: ne.Msg})
		fmt.Fprint(os.Stderr, reporter.Report(diagnostics.SeverityError, ne.Line, col, ne.Msg))
	}
	if npuSemaIssues > 0 {
		fmt.Printf("E-K-SEM\n")
	}

	// Borrow checker: ownership/lifetime violations.
	if borrowErrs := runBorrowCheck(prog); len(borrowErrs) > 0 {
		fmt.Printf("E-K-BRW\n")
		for _, be := range borrowErrs {
			issues = append(issues, lintIssue{Code: string(diagnostics.CodeBorrow), Line: int(be.Line), Msg: be.Message})
			fmt.Fprintf(os.Stderr, "  error[%s]: %s\n", diagnostics.CodeBorrow, be.Message)
		}
	}

	if len(issues) > 0 {
		return CommandResult{ExitCode: ExitCompile, Message: fmt.Sprintf("Lint found %d issue(s)", len(issues))}
	}
	if verbose {
		fmt.Printf("Lint passed: %s (%d statements)\n", targetFile, len(prog.Statements))
	}
	return CommandResult{ExitCode: ExitSuccess, Message: "Lint passed."}
}