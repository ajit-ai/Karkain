package lsp

import (
	"testing"

	"karkain/pkg/diagnostics"
)

// TestLSPDiagSeverityMapping pins every severity the compiler defines to the
// LSP protocol constant it must produce.
func TestLSPDiagSeverityMapping(t *testing.T) {
	cases := []struct {
		name string
		in   diagnostics.Severity
		want int
	}{
		{"error", diagnostics.SeverityError, DiagError},
		{"warning", diagnostics.SeverityWarning, DiagWarning},
		{"info", diagnostics.SeverityInfo, DiagInfo},
		{"note", diagnostics.SeverityNote, DiagInfo},
		{"help", diagnostics.SeverityHelp, DiagHint},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := lspDiagSeverity(tc.in); got != tc.want {
				t.Errorf("lspDiagSeverity(%q) = %d, want %d", tc.in, got, tc.want)
			}
		})
	}
	// Protocol values are asserted against the constants above; guard the
	// standard LSP DiagnosticSeverity numbers so a local edit cannot silently
	// change the wire contract.
	if DiagError != 1 || DiagWarning != 2 || DiagInfo != 3 || DiagHint != 4 {
		t.Errorf("LSP severity constants drifted: error=%d warning=%d info=%d hint=%d",
			DiagError, DiagWarning, DiagInfo, DiagHint)
	}
}

// TestLSPDiagSeverityUnknownFallsBackToInfo keeps the pre-existing safe
// behaviour for anything outside the documented vocabulary.
func TestLSPDiagSeverityUnknownFallsBackToInfo(t *testing.T) {
	for _, in := range []diagnostics.Severity{"", "bogus", "ERROR", "fatal"} {
		if got := lspDiagSeverity(in); got != DiagInfo {
			t.Errorf("lspDiagSeverity(%q) = %d, want fallback %d", in, got, DiagInfo)
		}
	}
}

// TestConvertDiagnosticsPreservesEachSeverity is the regression guard for the
// adapter itself: a slice is no longer published under one hardcoded severity.
func TestConvertDiagnosticsPreservesEachSeverity(t *testing.T) {
	in := []diagnostics.Diagnostic{
		{Line: 1, Column: 1, Severity: diagnostics.SeverityError, Message: "e"},
		{Line: 2, Column: 1, Severity: diagnostics.SeverityNote, Message: "n"},
		{Line: 3, Column: 1, Severity: diagnostics.SeverityWarning, Message: "w"},
		{Line: 4, Column: 1, Severity: diagnostics.SeverityHelp, Message: "h"},
		{Line: 5, Column: 1, Severity: diagnostics.SeverityInfo, Message: "i"},
	}
	want := []int{DiagError, DiagInfo, DiagWarning, DiagHint, DiagInfo}

	got := convertDiagnostics(in)
	if len(got) != len(want) {
		t.Fatalf("got %d diagnostics, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i].Severity != want[i] {
			t.Errorf("diagnostic %d (%s): severity = %d, want %d",
				i, got[i].Message, got[i].Severity, want[i])
		}
	}
}

// TestConvertDiagnosticsNoteInWarningSlice is the specific regression: a note
// travelling in the warning slice must NOT be published as a warning.
func TestConvertDiagnosticsNoteInWarningSlice(t *testing.T) {
	warnDiags := []diagnostics.Diagnostic{
		diagnostics.WarningDiagnostic("a.kark", 3, 5, "W1", "unused variable"),
		diagnostics.NoteDiagnostic("a.kark", 3, 9, "first declared here"),
	}
	got := convertDiagnostics(warnDiags)
	if len(got) != 2 {
		t.Fatalf("got %d diagnostics, want 2", len(got))
	}
	if got[0].Severity != DiagWarning {
		t.Errorf("real warning: severity = %d, want %d", got[0].Severity, DiagWarning)
	}
	if got[1].Severity == DiagWarning {
		t.Errorf("note in warning slice was published as a warning (%d); want its own severity %d",
			got[1].Severity, DiagInfo)
	}
	if got[1].Severity != DiagInfo {
		t.Errorf("note in warning slice: severity = %d, want %d", got[1].Severity, DiagInfo)
	}
}

// TestConvertDiagnosticsPreservesRangeAndSource guards the metadata the
// severity change must not disturb.
func TestConvertDiagnosticsPreservesRangeAndSource(t *testing.T) {
	in := []diagnostics.Diagnostic{
		{Line: 10, Column: 7, EndColumn: 12, Severity: diagnostics.SeverityError, Message: "m", Code: "K002"},
		{Line: 4, Column: 1, Severity: diagnostics.SeverityWarning, Message: "single"},
	}
	got := convertDiagnostics(in)

	if got[0].Range.Start.Line != 9 || got[0].Range.Start.Character != 6 {
		t.Errorf("start range = %+v, want line 9 char 6", got[0].Range.Start)
	}
	if got[0].Range.End.Line != 9 || got[0].Range.End.Character != 11 {
		t.Errorf("end range = %+v, want line 9 char 11 (EndColumn honoured)", got[0].Range.End)
	}
	if got[1].Range.End.Character != 1 {
		t.Errorf("no EndColumn: end char = %d, want 1 (single position)", got[1].Range.End.Character)
	}
	for i := range got {
		if got[i].Source != "karkain" {
			t.Errorf("diagnostic %d source = %q, want karkain", i, got[i].Source)
		}
	}
	if got[0].Message != "m" {
		t.Errorf("message = %q, want m", got[0].Message)
	}
}

func TestConvertDiagnosticsEmpty(t *testing.T) {
	if got := convertDiagnostics(nil); len(got) != 0 {
		t.Errorf("nil input produced %d diagnostics, want 0", len(got))
	}
}