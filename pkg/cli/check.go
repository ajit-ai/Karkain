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

	sourceText, srcMap, err := resolveSourcesCheck(targetFile)
	if err != nil {
		return sourceLoadResult(err)
	}

	src := sourceText
	diags, stmts := AnalyzeSource(targetFile, src, srcMap)
	if len(diags) > 0 {
		stage := checkStageFor(diags)
		if format == CheckFormatJSON {
			emitJSONDiagnostics(diags)
			return CommandResult{ExitCode: ExitCompile, Message: ""}
		}
		renderDiagnostics(src, targetFile, diags)
		return CommandResult{ExitCode: ExitCompile, Message: checkStageMessage(stage, len(diags))}
	}

	if verbose {
		fmt.Printf("Check passed: %s (%d statements)\n", targetFile, stmts)
	}
	if format == CheckFormatJSON {
		return CommandResult{ExitCode: ExitSuccess, Message: ""}
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

// renderDiagnostics prints diagnostics in the existing human form.
func renderDiagnostics(src, targetFile string, diags []diagnostics.Diagnostic) {
	reporter := diagnostics.NewReporter(src, targetFile)
	for _, d := range diags {
		fmt.Fprint(os.Stderr, reporter.Report(diagnostics.SeverityError, d.Line, d.Column, d.Message))
	}
}
