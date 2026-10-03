package lsp

import (
	"testing"

	"karkain/pkg/diagnostics"
	"karkain/pkg/source"
)

// -----------------------------------------------------------------------------
// Phase 5.4.3 - LSP diagnostic range / location fidelity.
// The compiler reports 1-based BYTE columns; LSP positions are 0-based UTF-16
// code units. These tests pin that conversion and the resulting Range values.
// -----------------------------------------------------------------------------

// TestLineIndexReadsDocumentLines confirms the adapter is driven by the shared
// pkg/source LineIndex (which owns line splitting and is unit-tested there),
// including its CRLF handling.
func TestLineIndexReadsDocumentLines(t *testing.T) {
	li := source.NewLineIndex("one\ntwo\r\nthree\n")
	if got := li.LineCount(); got != 4 {
		t.Fatalf("got %d lines, want 4 (trailing newline yields an empty final line)", got)
	}
	for i, want := range []string{"one", "two", "three", ""} {
		if got, ok := sourceLine(li, i+1); !ok || got != want {
			t.Errorf("line %d = %q (ok=%v), want %q", i+1, got, ok, want)
		}
	}
}

// TestUTF16OffsetASCII proves the conversion is the identity on ASCII lines, so
// every pre-existing behaviour is preserved exactly.
func TestUTF16OffsetASCII(t *testing.T) {
	line := "let abc = 1"
	li := source.NewLineIndex(line)
	for b := 0; b <= len(line); b++ {
		got, ok := utf16Col(li, 1, b)
		if !ok {
			t.Fatalf("byte %d reported unknown line", b)
		}
		if got != b {
			t.Errorf("ASCII byte %d -> utf16 %d, want identity", b, got)
		}
	}
}

// TestUTF16OffsetNonASCII is the core fidelity check: a byte column from the
// compiler must land on the correct UTF-16 character.
func TestUTF16OffsetNonASCII(t *testing.T) {
	line := "héllo" // 'é' occupies 2 bytes
	li := source.NewLineIndex(line)
	if len(line) != 6 {
		t.Fatalf("fixture assumption broken: line has %d bytes", len(line))
	}
	if utf16Len(line) != 5 {
		t.Fatalf("utf16Len = %d, want 5", utf16Len(line))
	}
	for _, c := range []struct{ byteOff, want int }{
		{0, 0}, // start
		{1, 1}, // after 'h', before the multi-byte rune
		{3, 2}, // after 'é' (2 bytes) -> 2 characters
		{6, 5}, // end of line
	} {
		got, ok := utf16Col(li, 1, c.byteOff)
		if !ok {
			t.Fatalf("byte %d: unknown line", c.byteOff)
		}
		if got != c.want {
			t.Errorf("byte %d -> utf16 %d, want %d", c.byteOff, got, c.want)
		}
	}
}

// TestUTF16OffsetAstral covers characters outside the BMP (2 UTF-16 units).
func TestUTF16OffsetAstral(t *testing.T) {
	line := "a😀b" // 6 bytes, 4 UTF-16 units
	li := source.NewLineIndex(line)
	if got := utf16Len(line); got != 4 {
		t.Fatalf("utf16Len = %d, want 4", got)
	}
	got, ok := utf16Col(li, 1, 5) // after the emoji
	if !ok || got != 3 {
		t.Errorf("byte 5 -> utf16 %d (ok=%v), want 3", got, ok)
	}
}

// TestUTF16OffsetBoundaries covers clamping and unknown-line handling.
func TestUTF16OffsetBoundaries(t *testing.T) {
	li := source.NewLineIndex("ab\ncd\n")
	if got, ok := utf16Col(li, 1, -5); !ok || got != 0 {
		t.Errorf("negative offset -> %d (ok=%v), want 0", got, ok)
	}
	if got, ok := utf16Col(li, 1, 999); !ok || got != 2 {
		t.Errorf("offset past EOL -> %d (ok=%v), want 2 (line length)", got, ok)
	}
	if _, ok := utf16Col(li, 99, 1); ok {
		t.Error("out-of-range line must report unknown, not invent a position")
	}
	if _, ok := utf16Col(li, 0, 1); ok {
		t.Error("line 0 must report unknown")
	}
}

// sourceLine is a small test shim over pkg/source's 1-based line accessor.
func sourceLine(li *source.LineIndex, line1 int) (string, bool) {
	if li == nil || line1 < 1 || line1 > li.LineCount() {
		return "", false
	}
	return li.LineText(line1), true
}

// TestDiagnosticRangeSingleLine covers a span within one line, with and without
// an EndColumn.
func TestDiagnosticRangeSingleLine(t *testing.T) {
	li := source.NewLineIndex("func main() {\n  let value = 1\n}\n")
	// `value` starts at 1-based byte column 7; EndColumn is just past the token.
	got := convertDiagnostics([]diagnostics.Diagnostic{
		{Line: 2, Column: 7, EndColumn: 12, Severity: diagnostics.SeverityError, Message: "bad"},
		{Line: 2, Column: 3, Severity: diagnostics.SeverityWarning, Message: "point"},
	}, li)

	// Compiler line 2 (1-based) -> index 1 (0-based).
	if got[0].Range.Start.Line != 1 || got[1].Range.Start.Line != 1 {
		t.Errorf("line conversion wrong: %+v / %+v", got[0].Range, got[1].Range)
	}
	// Column 7 -> 0-based 6; EndColumn 12 is exclusive -> 11.
	if got[0].Range.Start.Character != 6 || got[0].Range.End.Character != 11 {
		t.Errorf("multi-char span = %d..%d, want 6..11 (exclusive end)",
			got[0].Range.Start.Character, got[0].Range.End.Character)
	}
	if got[0].Range.End.Line != got[0].Range.Start.Line {
		t.Error("single-line span must not change line")
	}
	// No EndColumn -> single-character range at the start column.
	if got[1].Range.Start.Character != 2 || got[1].Range.End.Character != 3 {
		t.Errorf("point span = %d..%d, want 2..3",
			got[1].Range.Start.Character, got[1].Range.End.Character)
	}
}

// TestDiagnosticRangeLineBoundaries covers the first and last line of a file.
func TestDiagnosticRangeLineBoundaries(t *testing.T) {
	li := source.NewLineIndex("bad()\nx\nlast()")
	got := convertDiagnostics([]diagnostics.Diagnostic{
		{Line: 1, Column: 1, EndColumn: 5, Severity: diagnostics.SeverityError, Message: "first"},
		{Line: 3, Column: 1, EndColumn: 7, Severity: diagnostics.SeverityError, Message: "last"},
	}, li)

	if got[0].Range.Start.Line != 0 || got[0].Range.Start.Character != 0 {
		t.Errorf("line 1 col 1 -> %+v, want line 0 char 0", got[0].Range)
	}
	if got[0].Range.End.Character != 4 {
		t.Errorf("first line end = %d, want 4", got[0].Range.End.Character)
	}
	if got[1].Range.Start.Line != 2 || got[1].Range.End.Line != 2 {
		t.Errorf("last line index = %+v, want line 2", got[1].Range)
	}
	if got[1].Range.End.Character != 6 {
		t.Errorf("last line end = %d, want 6", got[1].Range.End.Character)
	}
}

// TestDiagnosticRangeNonASCIILine proves the published character offsets match
// the UTF-16 view of the line, not the compiler's raw byte offsets.
func TestDiagnosticRangeNonASCIILine(t *testing.T) {
	const prefix = "  let x = 1  // "
	line := prefix + "é"
	li := source.NewLineIndex(line)
	byteCol := len(prefix) + 1 // 1-based byte column of 'é'
	got := convertDiagnostics([]diagnostics.Diagnostic{
		{Line: 1, Column: byteCol, EndColumn: byteCol + 2,
			Severity: diagnostics.SeverityError, Message: "non-ascii"},
	}, li)

	wantStart := len([]rune(prefix)) // 0-based UTF-16 index of 'é'
	if got[0].Range.Start.Character != wantStart {
		t.Errorf("start = %d, want %d (UTF-16 index, not byte %d)",
			got[0].Range.Start.Character, wantStart, byteCol-1)
	}
	if got[0].Range.End.Character <= got[0].Range.Start.Character {
		t.Errorf("end %d must be past start %d",
			got[0].Range.End.Character, got[0].Range.Start.Character)
	}
}

// TestDiagnosticRangeNonASCIIIsNotByteIdentical is the regression guard: with a
// naive byte-only conversion the start character would be shifted right. The
// diagnostic must sit AFTER a multi-byte character, since offsets before it are
// identical in both encodings.
func TestDiagnosticRangeNonASCIIIsNotByteIdentical(t *testing.T) {
	const prefix = "// é = 1 // "
	li := source.NewLineIndex(prefix + "target")
	byteCol := len(prefix) + 1 // 1-based byte column of 't'
	got := convertDiagnostics([]diagnostics.Diagnostic{
		{Line: 1, Column: byteCol, Severity: diagnostics.SeverityError, Message: "x"},
	}, li)

	wantChars := len([]rune(prefix))
	if got[0].Range.Start.Character == byteCol-1 {
		t.Errorf("character offset %d equals the raw byte offset; UTF-16 conversion is not applied",
			got[0].Range.Start.Character)
	}
	if got[0].Range.Start.Character != wantChars {
		t.Errorf("start = %d, want %d (byte %d, character %d)",
			got[0].Range.Start.Character, wantChars, byteCol-1, wantChars)
	}
}

// TestDiagnosticRangeDistinctLocations proves each diagnostic keeps its own range.
func TestDiagnosticRangeDistinctLocations(t *testing.T) {
	li := source.NewLineIndex("aaa\nbbb\nccc\nddd\n")
	got := convertDiagnostics([]diagnostics.Diagnostic{
		{Line: 1, Column: 1, Severity: diagnostics.SeverityError, Message: "a"},
		{Line: 2, Column: 2, EndColumn: 4, Severity: diagnostics.SeverityWarning, Message: "b"},
		{Line: 4, Column: 3, EndColumn: 4, Severity: diagnostics.SeverityInfo, Message: "c"},
	}, li)
	for i, c := range []struct{ line, start, end int }{
		{0, 0, 1}, {1, 1, 3}, {3, 2, 3},
	} {
		if got[i].Range.Start.Line != c.line {
			t.Errorf("diag %d line = %d, want %d", i, got[i].Range.Start.Line, c.line)
		}
		if got[i].Range.Start.Character != c.start || got[i].Range.End.Character != c.end {
			t.Errorf("diag %d range = %d..%d, want %d..%d", i,
				got[i].Range.Start.Character, got[i].Range.End.Character, c.start, c.end)
		}
	}
}

// TestDiagnosticRangeNeverInvertedOrOutOfBounds covers invalid span data.
func TestDiagnosticRangeNeverInvertedOrOutOfBounds(t *testing.T) {
	li := source.NewLineIndex("short\nline two here\n")
	got := convertDiagnostics([]diagnostics.Diagnostic{
		// EndColumn behind Column.
		{Line: 1, Column: 4, EndColumn: 2, Severity: diagnostics.SeverityError, Message: "inverted"},
		// Columns far past the end of the line.
		{Line: 1, Column: 900, EndColumn: 999, Severity: diagnostics.SeverityError, Message: "past EOL"},
		// Non-positive positions.
		{Line: 0, Column: 0, Severity: diagnostics.SeverityError, Message: "zero"},
		{Line: -3, Column: -5, Severity: diagnostics.SeverityError, Message: "negative"},
	}, li)

	for i, d := range got {
		if d.Range.End.Character < d.Range.Start.Character {
			t.Errorf("diag %d inverted: %d..%d", i, d.Range.Start.Character, d.Range.End.Character)
		}
		if d.Range.Start.Line < 0 {
			t.Errorf("diag %d negative line %d", i, d.Range.Start.Line)
		}
		if l, ok := sourceLine(li, d.Range.Start.Line + 1); ok {
			if d.Range.End.Character > utf16Len(l) {
				t.Errorf("diag %d end %d exceeds line length %d",
					i, d.Range.End.Character, utf16Len(l))
			}
		}
	}
}

// TestDiagnosticRangeUnknownLineKeepsExistingBehaviour: when the compiler
// reports a line the document does not have, the adapter must not invent a
// position from unrelated text.
func TestDiagnosticRangeUnknownLineKeepsExistingBehaviour(t *testing.T) {
	li := source.NewLineIndex("only one line\n")
	got := convertDiagnostics([]diagnostics.Diagnostic{
		{Line: 42, Column: 5, Severity: diagnostics.SeverityError, Message: "beyond eof"},
	}, li)
	if got[0].Range.Start.Line != 41 {
		t.Errorf("line = %d, want 41 (unmapped, arithmetic preserved)", got[0].Range.Start.Line)
	}
	if got[0].Range.Start.Character != 4 {
		t.Errorf("char = %d, want 4 (existing arithmetic preserved)", got[0].Range.Start.Character)
	}
}

// TestDiagnosticRangeWithSeverityInteraction proves range fidelity and severity
// fidelity hold simultaneously (Phase 5.4.2 behaviour is preserved).
func TestDiagnosticRangeWithSeverityInteraction(t *testing.T) {
	// Lines are long enough for the requested columns, so no clamping occurs
	// and the assertions test conversion rather than bounds.
	li := source.NewLineIndex("alpha\nbravo\ndelta\necho\nfoxtrot\n")
	got := convertDiagnostics([]diagnostics.Diagnostic{
		{Line: 2, Column: 1, EndColumn: 3, Severity: diagnostics.SeverityError, Message: "e"},
		{Line: 3, Column: 2, EndColumn: 4, Severity: diagnostics.SeverityNote, Message: "n"},
		{Line: 4, Column: 1, EndColumn: 2, Severity: diagnostics.SeverityWarning, Message: "w"},
		{Line: 5, Column: 1, EndColumn: 3, Severity: diagnostics.SeverityHelp, Message: "h"},
	}, li)
	wantSev := []int{DiagError, DiagInfo, DiagWarning, DiagHint}
	wantStart := []int{0, 1, 0, 0}
	wantEnd := []int{2, 3, 1, 2}
	for i := range got {
		if got[i].Severity != wantSev[i] {
			t.Errorf("diag %d severity = %d, want %d", i, got[i].Severity, wantSev[i])
		}
		if got[i].Range.Start.Character != wantStart[i] || got[i].Range.End.Character != wantEnd[i] {
			t.Errorf("diag %d range = %d..%d, want %d..%d", i,
				got[i].Range.Start.Character, got[i].Range.End.Character,
				wantStart[i], wantEnd[i])
		}
	}
}

// TestRunDiagnosticsEndToEnd exercises the real adapter path used by
// publishDiagnostics, proving published ranges line up with the source text.
func TestRunDiagnosticsEndToEnd(t *testing.T) {
	h := &Handler{}
	li := source.NewLineIndex("func main() {\n  let = 42\n}\n")
	diags := h.runDiagnostics("file:///x.kark", "func main() {\n  let = 42\n}\n")
	if len(diags) == 0 {
		t.Fatal("expected at least one diagnostic from the real analysis path")
	}
	for _, d := range diags {
		if d.Source != "karkain" {
			t.Errorf("source = %q, want karkain", d.Source)
		}
		if d.Range.End.Character < d.Range.Start.Character {
			t.Errorf("inverted range %+v", d.Range)
		}
		if d.Range.Start.Line < 0 || d.Range.Start.Line >= li.LineCount() {
			t.Errorf("line %d outside the document (%d lines)", d.Range.Start.Line, li.LineCount())
		}
		line, _ := sourceLine(li, d.Range.Start.Line + 1)
		if d.Range.End.Character > utf16Len(line) {
			t.Errorf("end %d beyond line length %d: %q", d.Range.End.Character, utf16Len(line), line)
		}
	}
}