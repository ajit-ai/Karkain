package cli

import (
	"fmt"
	"karkain/pkg/diagnostics"
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

	// Phase 105: multi-file error recovery. Parse every file of the unit
	// independently first; if any file has recoverable syntax errors, report
	// them ALL (per-file path/line/col) in one invocation instead of letting
	// the first failing stage or a module-graph abort hide the rest. Clean
	// units fall through to the normal assembled analysis path untouched.
	if errDiags := projectSyntaxDiagnostics(targetFile); len(errDiags) > 0 {
		if format == CheckFormatJSON {
			emitJSONDiagnostics(errDiags)
			return CommandResult{ExitCode: ExitCompile, Message: ""}
		}
		renderDiagnostics("", errDiags)
		return CommandResult{ExitCode: ExitCompile, Message: checkStageMessage(checkStageSyntax, len(errDiags))}
	}

	sourceText, srcMap, err := resolveSourcesCheck(targetFile)
	if err != nil {
		return sourceLoadResult(err)
	}

	src := sourceText
	errDiags, warnDiags, stmts := AnalyzeSource(targetFile, src, srcMap)
	if len(errDiags) > 0 {
		stage := checkStageFor(errDiags)
		if format == CheckFormatJSON {
			emitJSONDiagnostics(errDiags)
			return CommandResult{ExitCode: ExitCompile, Message: ""}
		}
		renderDiagnostics(src, errDiags)
		return CommandResult{ExitCode: ExitCompile, Message: checkStageMessage(stage, len(errDiags))}
	}

	// Warnings never terminate compilation: render them (if any) and continue
	// to a successful result. JSON consumers receive the warning array.
	if len(warnDiags) > 0 {
		if format == CheckFormatJSON {
			emitJSONDiagnostics(warnDiags)
			return CommandResult{ExitCode: ExitSuccess, Message: ""}
		}
		renderDiagnostics(src, warnDiags)
	}

	if verbose {
		fmt.Printf("Check passed: %s (%d statements)\n", targetFile, stmts)
	}
	if format == CheckFormatJSON {
		return CommandResult{ExitCode: ExitSuccess, Message: ""}
	}
	if len(warnDiags) > 0 {
		return CommandResult{ExitCode: ExitSuccess, Message: fmt.Sprintf("Check passed with %d warning(s).", len(warnDiags))}
	}
	return CommandResult{ExitCode: ExitSuccess, Message: "Check passed."}
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

// renderDiagnostics prints diagnostics in the Phase 83 human report form
// (error[E-K-*]:/warning[W-K-*]: headers, source frame, caret underline, help
// and notes). One renderer serves errors and warnings alike.
func renderDiagnostics(src string, diags []diagnostics.Diagnostic) {
	fmt.Fprint(os.Stderr, diagnostics.FormatDiagnostics(diags, src))
}
