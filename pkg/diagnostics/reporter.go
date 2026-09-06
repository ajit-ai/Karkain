package diagnostics

import (
	"fmt"
	"strings"
)

// Severity represents the severity level of a diagnostic message. The string
// value is the JSON wire value used by the karkain-diagnostics-v1 contract.
type Severity string

const (
	SeverityError   Severity = "error"
	SeverityWarning Severity = "warning"
	SeverityInfo    Severity = "info"
	SeverityNote    Severity = "note"
	SeverityHelp    Severity = "help"
)

// String returns the wire-string form of the severity.
func (s Severity) String() string {
	return string(s)
}

// DiagnosticReporter formats and renders diagnostic messages with source context
type DiagnosticReporter struct {
	sourceCode string
	filePath   string
	lines      []string
}

// NewReporter creates a new diagnostic reporter from source code
func NewReporter(sourceCode string, filePath string) *DiagnosticReporter {
	return &DiagnosticReporter{
		sourceCode: sourceCode,
		filePath:   filePath,
		lines:      strings.Split(sourceCode, "\n"),
	}
}

// Report formats and returns a diagnostic message with source context
func (r *DiagnosticReporter) Report(severity Severity, line, col int, message string) string {
	return r.ReportWithCode(severity, line, col, "", message, "")
}

// ReportWithCode formats and returns a diagnostic message with source context, code, and help text
func (r *DiagnosticReporter) ReportWithCode(severity Severity, line, col int, code string, message string, help string) string {
	var sb strings.Builder

	// Header with severity, code (if provided), and location
	severityStr := severityLabel(severity)
	if code != "" {
		sb.WriteString(fmt.Sprintf("%s[%s]: %s\n", severityStr, code, message))
	} else {
		sb.WriteString(fmt.Sprintf("%s: %s\n", severityStr, message))
	}
	sb.WriteString(fmt.Sprintf("  --> %s:%d:%d\n", r.filePath, line, col))

	// Source snippet context (show the error line and surrounding context)
	// line is 1-based; convert to 0-based index
	startIdx := line - 2
	if startIdx < 0 {
		startIdx = 0
	}
	endIdx := line
	if endIdx >= len(r.lines) {
		endIdx = len(r.lines) - 1
	}

	// Use a fixed width for line number gutter
	lineNumWidth := len(fmt.Sprintf("%d", endIdx+1))
	if lineNumWidth < 3 {
		lineNumWidth = 3
	}

	for i := startIdx; i <= endIdx; i++ {
		if i < 0 || i >= len(r.lines) {
			continue
		}
		lineNum := fmt.Sprintf("%*d", lineNumWidth, i+1)
		sb.WriteString(fmt.Sprintf("%s | %s\n", lineNum, r.lines[i]))

		// Caret marker on the error line
		if i == line-1 {
			prefix := strings.Repeat(" ", lineNumWidth+3)
			caretPos := strings.Repeat(" ", col-1)
			sb.WriteString(fmt.Sprintf("%s%s%s\n", prefix, caretPos, "^"))
		}
	}

	// Help text if provided
	if help != "" {
		sb.WriteString(fmt.Sprintf("   = help: %s\n", help))
	}

	return sb.String()
}

func severityLabel(s Severity) string {
	return s.String()
}
