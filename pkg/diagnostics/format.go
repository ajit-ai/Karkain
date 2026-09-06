package diagnostics

import (
	"fmt"
	"strings"
)

// Format renders a single diagnostic as the human-readable compiler report:
//
//	error[E-K-RES]:
//	undefined identifier `total`
//
//	--> main.kark:18:14
//	|
//	18 | print(total)
//	|       ^^^^^
//	|
//	= help: declare `total` before using it
//
// src is the source text of the file the diagnostic refers to; it is used to
// draw the excerpt line and the caret underline. If src is empty (or the line
// falls outside it), d.Excerpt — a captured snippet — is used; if neither
// exists the location frame is still printed, just without a source line.
func Format(d Diagnostic, src string) string {
	var b strings.Builder

	// Header: severity[code]: — the bracket is omitted when no code is known.
	// Use numeric codes (K001, K002, etc.) for human-friendly display
	displayCode := d.ToNumericCode()
	if displayCode != "" {
		fmt.Fprintf(&b, "%s[%s]:\n", d.Severity, displayCode)
	} else {
		fmt.Fprintf(&b, "%s:\n", d.Severity)
	}
	b.WriteString(d.Message)
	if !strings.HasSuffix(d.Message, "\n") {
		b.WriteByte('\n')
	}

	// Location frame.
	locFile := d.File
	if locFile == "" {
		locFile = "<input>"
	}
	fmt.Fprintf(&b, "\n--> %s:%d:%d\n", locFile, d.Line, d.Column)

	// Draw the gutter, the source line and the caret underline. Column and
	// EndColumn are 1-based; the caret starts at the token start and covers the
	// token width (Column..EndColumn), at least one caret.
	width := len(fmt.Sprintf("%d", d.Line))
	gutterPipe := strings.Repeat(" ", width+1) + "|"
	b.WriteString(gutterPipe)
	b.WriteByte('\n')

	lineText := d.Excerpt
	if src != "" {
		lines := strings.Split(src, "\n")
		if d.Line >= 1 && d.Line <= len(lines) {
			lineText = strings.TrimRight(lines[d.Line-1], "\r")
		}
	}
	if lineText != "" {
		fmt.Fprintf(&b, "%d | %s\n", d.Line, lineText)
		caretLen := d.EndColumn - d.Column
		if caretLen < 1 {
			caretLen = 1
		}
		// The source text starts at column width+3 ("%d | "); underline the
		// token at offset (Column-1) within it.
		b.WriteString(strings.Repeat(" ", width+3))
		b.WriteString(strings.Repeat(" ", d.Column-1))
		b.WriteString(strings.Repeat("^", caretLen))
		b.WriteByte('\n')
	}
	b.WriteString(gutterPipe)
	b.WriteByte('\n')

	if d.Help != "" {
		fmt.Fprintf(&b, "\n= help: %s\n", d.Help)
	}
	for _, rn := range d.Related {
		fmt.Fprintf(&b, "\n= %s: %s\n", rn.Severity, rn.Message)
	}

	return b.String()
}

// FormatDiagnostics renders a diagnostic set as consecutive reports, separated
// by a blank line. Errors and warnings use the same infrastructure; the caller
// decides ordering (check renders errors first, then warnings).
func FormatDiagnostics(diags []Diagnostic, src string) string {
	var b strings.Builder
	for i, d := range diags {
		if i > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(Format(d, src))
	}
	return b.String()
}
