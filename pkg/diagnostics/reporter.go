package diagnostics

import (
	"fmt"
	"strings"
)

// Severity represents the severity level of a diagnostic message
type Severity int

const (
	SeverityError Severity = iota
	SeverityWarning
	SeverityInfo
)

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
	var sb strings.Builder

	// Header with severity and location
	severityStr := severityLabel(severity)
	sb.WriteString(fmt.Sprintf("%s: %s:%d:%d: %s\n", severityStr, r.filePath, line, col, message))

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

	return sb.String()
}

func severityLabel(s Severity) string {
	switch s {
	case SeverityError:
		return "error"
	case SeverityWarning:
		return "warning"
	case SeverityInfo:
		return "info"
	default:
		return "unknown"
	}
}
