package diagnostics

import "encoding/json"

// Diagnostic is a structured, machine-readable compiler diagnostic. It is the
// wire format for `karkain check --format=json` and feeds IDE integrations
// (VS Code, LiteIDE). The JSON field names are part of the public toolchain
// contract.
type Diagnostic struct {
	// File is the path of the file the diagnostic refers to. For whole-program
	// checks the path is the root file; line/column are relative to the
	// concatenated module source (the same coordinates the human report uses).
	File string `json:"file"`

	// Line is the 1-based source line containing the finding.
	Line int `json:"line"`

	// Column is the 1-based token-start column of the finding. Parser errors
	// carry the real token column; some deeper analyses are line-anchored and
	// report column 1 (documented limitation, not a guarantee). Together with
	// EndColumn this forms the optional source span.
	Column int `json:"column"`

	// EndColumn is the optional 1-based column just past the offending token
	// (a zero/absent endColumn means the range is unknown). The pair
	// (Column, EndColumn) is the diagnostic's source span.
	EndColumn int `json:"endColumn,omitempty"`

	// Excerpt is the optional trimmed source line of the finding, with long
	// lines truncated.
	Excerpt string `json:"excerpt,omitempty"`

	// Severity is one of "error", "warning", "note", "help".
	Severity Severity `json:"severity"`

	// Code is the stable machine-readable error code (E-K-*, W-K-*). Empty
	// when the front end produced a message without a code.
	Code string `json:"code"`

	// Message is the human-readable diagnostic text.
	Message string `json:"message"`

	// Help is optional remediation text shown below the source excerpt.
	Help string `json:"help,omitempty"`

	// Related holds optional secondary diagnostics that give context or point
	// at the origin of the finding (notes). Never part of a "list of
	// diagnostics" contract; they ride along with their parent.
	Related []Diagnostic `json:"related,omitempty"`
}

// ErrorDiagnostic builds an error-severity Diagnostic with a code.
func ErrorDiagnostic(file string, line, col int, code Code, msg string) Diagnostic {
	return Diagnostic{File: file, Line: line, Column: col, Severity: SeverityError, Code: string(code), Message: msg}
}

// WarningDiagnostic builds a warning-severity Diagnostic with a code.
// Warnings never terminate compilation; they are collected and reported
// alongside errors so authors can keep building.
func WarningDiagnostic(file string, line, col int, code Code, msg string) Diagnostic {
	return Diagnostic{File: file, Line: line, Column: col, Severity: SeverityWarning, Code: string(code), Message: msg}
}

// NoteDiagnostic builds a note-severity Diagnostic. Notes carry context about
// a primary finding (they print as "= note: ..." fringe lines) and typically
// have no code of their own.
func NoteDiagnostic(file string, line, col int, msg string) Diagnostic {
	return Diagnostic{File: file, Line: line, Column: col, Severity: SeverityNote, Message: msg}
}

// WithHelp returns a copy of d with remediation text attached. The help text
// is rendered as a "= help: ..." fringe line under the source excerpt.
func (d Diagnostic) WithHelp(help string) Diagnostic {
	d.Help = help
	return d
}

// WithRelated returns a copy of d carrying supplemental diagnostics (notes,
// e.g. the declaration site of a name being used here).
func (d Diagnostic) WithRelated(related ...Diagnostic) Diagnostic {
	d.Related = append([]Diagnostic(nil), related...)
	return d
}

// IsError reports whether the diagnostic has error severity.
func (d Diagnostic) IsError() bool {
	return d.Severity == SeverityError
}

// IsWarning reports whether the diagnostic has warning severity.
func (d Diagnostic) IsWarning() bool {
	return d.Severity == SeverityWarning
}

// MarshalJSON renders the diagnostic as its JSON wire form.
func (d Diagnostic) MarshalJSON() ([]byte, error) {
	type alias Diagnostic
	return json.Marshal(alias(d))
}

// MarshalDiagnostics renders a diagnostic set as a compact JSON array.
func MarshalDiagnostics(diags []Diagnostic) ([]byte, error) {
	if diags == nil {
		diags = []Diagnostic{}
	}
	return json.Marshal(diags)
}

// ToNumericCode converts E-K-* codes to their K* numeric equivalents for human-friendly display
func (d Diagnostic) ToNumericCode() string {
	switch d.Code {
	case string(CodeSyntax):
		return string(CodeK001)
	case string(CodeResolve):
		return string(CodeK002)
	case string(CodeBorrow):
		return string(CodeK003)
	case string(CodeSema):
		return string(CodeK004)
	case string(CodeType):
		return string(CodeK005)
	case string(CodeCodegen):
		return string(CodeK006)
	case string(CodePackage):
		return string(CodeK007)
	case string(CodeEnv):
		return string(CodeK008)
	case string(CodeWarnUnused):
		return string(CodeK100)
	default:
		return d.Code
	}
}

// Render returns a human-readable diagnostic with source context using the enhanced format
func (d Diagnostic) Render(sourceCode string) string {
	reporter := NewReporter(sourceCode, d.File)
	return reporter.ReportWithCode(d.Severity, d.Line, d.Column, d.ToNumericCode(), d.Message, d.Help)
}
