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
	// report column 1 (documented limitation, not a guarantee).
	Column int `json:"column"`

	// EndColumn is the optional 1-based column just past the offending token
	// (a zero/absent endColumn means the range is unknown). Added in Phase 83;
	// additive optional field, part of the karkain-diagnostics-v1 contract.
	EndColumn int `json:"endColumn,omitempty"`

	// Excerpt is the optional trimmed source line of the finding, with long
	// lines truncated. Added in Phase 83; additive optional field.
	Excerpt string `json:"excerpt,omitempty"`

	// Severity is one of "error", "warning", "info".
	Severity string `json:"severity"`

	// Code is the stable machine-readable error code (E-K-*). Empty when the
	// front end produced a message without a code.
	Code string `json:"code"`

	// Message is the human-readable diagnostic text.
	Message string `json:"message"`
}

// ErrorDiagnostic builds an error-severity Diagnostic with a code.
func ErrorDiagnostic(file string, line, col int, code Code, msg string) Diagnostic {
	return Diagnostic{File: file, Line: line, Column: col, Severity: "error", Code: string(code), Message: msg}
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
