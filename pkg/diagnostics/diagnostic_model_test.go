package diagnostics

import (
	"strings"
	"testing"
)

// Phase 83 — Diagnostic & Warning Foundation. Focused tests for the model,
// constructor helpers, severity mapping, spans and the human report format.

func TestModel_ErrorDiagnosticCreation(t *testing.T) {
	d := ErrorDiagnostic("main.kark", 18, 14, CodeResolve, "undefined identifier `total`")
	if d.File != "main.kark" || d.Line != 18 || d.Column != 14 {
		t.Errorf("location not captured: %+v", d)
	}
	if d.Severity != SeverityError {
		t.Errorf("expected error severity, got %q", d.Severity)
	}
	if !d.IsError() || d.IsWarning() {
		t.Error("IsError/IsWarning mismatch for an error diagnostic")
	}
	if d.Message != "undefined identifier `total`" {
		t.Errorf("message not captured: %q", d.Message)
	}
}

func TestModel_WarningDiagnosticCreation(t *testing.T) {
	d := WarningDiagnostic("main.kark", 7, 5, CodeWarnUnused, "unused variable `count`")
	if d.Severity != SeverityWarning {
		t.Errorf("expected warning severity, got %q", d.Severity)
	}
	if !d.IsWarning() || d.IsError() {
		t.Error("IsWarning/IsError mismatch for a warning diagnostic")
	}
	if d.Code != string(CodeWarnUnused) {
		t.Errorf("expected W-K-UNUSED, got %q", d.Code)
	}
}

func TestModel_SeverityLevels(t *testing.T) {
	cases := map[Severity]string{
		SeverityError:   "error",
		SeverityWarning: "warning",
		SeverityInfo:    "info",
		SeverityNote:    "note",
		SeverityHelp:    "help",
	}
	for s, want := range cases {
		if string(s) != want {
			t.Errorf("severity %s: want string %q", s, want)
		}
	}
}

func TestModel_DiagnosticCode(t *testing.T) {
	d := ErrorDiagnostic("f.kark", 1, 1, CodeSyntax, "unexpected token")
	if d.Code != string(CodeSyntax) {
		t.Errorf("syntax diagnostic must carry E-K-SYN, got %q", d.Code)
	}
	// New (Phase 83) warning code is registered for explain/tooling.
	if !Known("W-K-UNUSED") || !Known("E-K-RES") {
		t.Error("Known() must accept W-K-UNUSED and the compiler classes")
	}
}

func TestModel_SourceLocation(t *testing.T) {
	d := ErrorDiagnostic("src/other.kark", 42, 9, CodeType, "type mismatch")
	if d.Line != 42 || d.Column != 9 || d.File != "src/other.kark" {
		t.Errorf("source location not captured: %+v", d)
	}
}

func TestModel_SourceSpanAndHelp(t *testing.T) {
	d := ErrorDiagnostic("main.kark", 2, 8, CodeResolve, "undefined identifier `total`")
	d.EndColumn = 13
	d = d.WithHelp("declare `total` before using it")
	if d.EndColumn != 13 {
		t.Errorf("endColumn not captured: %d", d.EndColumn)
	}
	if d.Help != "declare `total` before using it" {
		t.Errorf("help text not captured: %q", d.Help)
	}
	// Related diagnostics ride along without entering the parent list.
	note := NoteDiagnostic("main.kark", 1, 5, "a value must be declared before use")
	d2 := d.WithRelated(note)
	if len(d2.Related) != 1 || d2.Related[0].Message != note.Message {
		t.Errorf("related note not attached: %+v", d2.Related)
	}
}

func TestFormat_ErrorOutput(t *testing.T) {
	src := "func main() {\n  print(total)\n}\n"
	d := ErrorDiagnostic("main.kark", 2, 8, CodeResolve, "undefined identifier `total`")
	d.EndColumn = 13
	d = d.WithHelp("declare `total` before using it")

	out := Format(d, src)

	want := []string{
		"error[K002]:\n",
		"undefined identifier `total`\n",
		"--> main.kark:2:8\n",
		"2 |   print(total)\n",
		"^^^^^", // span 8..13 → five carets under `total`
		"= help: declare `total` before using it\n",
	}
	for _, w := range want {
		if !strings.Contains(out, w) {
			t.Errorf("formatted error missing %q:\n%s", w, out)
		}
	}
}

func TestFormat_WarningOutput(t *testing.T) {
	src := "let count = 10\n"
	d := WarningDiagnostic("main.kark", 1, 5, CodeWarnUnused, "unused variable `count`")
	d.EndColumn = 10

	out := Format(d, src)

	want := []string{
		"warning[K100]:\n",
		"unused variable `count`\n",
		"--> main.kark:1:5\n",
		"1 | let count = 10\n",
		"^^^^^", // span 5..10 → five carets under `count`
	}
	for _, w := range want {
		if !strings.Contains(out, w) {
			t.Errorf("formatted warning missing %q:\n%s", w, out)
		}
	}
	if strings.Contains(out, "error[") {
		t.Errorf("warning report must not use error header:\n%s", out)
	}
}

func TestFormat_RelatedNotes(t *testing.T) {
	d := ErrorDiagnostic("main.kark", 3, 2, CodeResolve, "undefined identifier `sum`").
		WithRelated(NoteDiagnostic("sum.kark", 1, 1, "declared here"))
	out := Format(d, "  print(sum)\n")
	if !strings.Contains(out, "= note: declared here\n") {
		t.Errorf("related note not rendered:\n%s", out)
	}
}

func TestFormat_MultipleDiagnostics(t *testing.T) {
	src := "let count = 10\nprint(total)\n"
	d1 := WarningDiagnostic("main.kark", 1, 5, CodeWarnUnused, "unused variable `count`")
	d2 := ErrorDiagnostic("main.kark", 2, 7, CodeResolve, "undefined identifier `total`")
	out := FormatDiagnostics([]Diagnostic{d1, d2}, src)

	if strings.Count(out, "warning[K100]:") != 1 || strings.Count(out, "error[K002]:") != 1 {
		t.Errorf("expected exactly one warning and one error header:\n%s", out)
	}
	if !strings.Contains(out, "--> main.kark:1:5") || !strings.Contains(out, "--> main.kark:2:7") {
		t.Errorf("both locations must be rendered:\n%s", out)
	}
}

func TestFormat_ExcerptFallback(t *testing.T) {
	// When src is unavailable, a captured Excerpt still yields a caret line.
	d := WarningDiagnostic("main.kark", 7, 5, CodeWarnUnused, "unused variable `count`")
	d.EndColumn = 10
	d.Excerpt = "    let count = 10"
	out := Format(d, "")
	if !strings.Contains(out, "7 |     let count = 10\n") {
		t.Errorf("excerpt fallback not rendered:\n%s", out)
	}
	if !strings.Contains(out, "^^^^^") {
		t.Errorf("caret underline missing with excerpt fallback:\n%s", out)
	}
}

func TestModel_NumericCodeConversion(t *testing.T) {
	tests := []struct {
		input    Code
		expected string
	}{
		{CodeSyntax, "K001"},
		{CodeResolve, "K002"},
		{CodeBorrow, "K003"},
		{CodeSema, "K004"},
		{CodeType, "K005"},
		{CodeCodegen, "K006"},
		{CodePackage, "K007"},
		{CodeEnv, "K008"},
		{CodeWarnUnused, "K100"},
	}

	for _, tt := range tests {
		d := ErrorDiagnostic("test.kark", 1, 1, tt.input, "test")
		if d.ToNumericCode() != tt.expected {
			t.Errorf("ToNumericCode(%s) = %s, want %s", tt.input, d.ToNumericCode(), tt.expected)
		}
	}

	// Unknown codes should pass through
	d := ErrorDiagnostic("test.kark", 1, 1, "UNKNOWN", "test")
	if d.ToNumericCode() != "UNKNOWN" {
		t.Errorf("Unknown code should pass through, got %s", d.ToNumericCode())
	}
}

func TestReporter_EnhancedFormatWithCode(t *testing.T) {
	src := "func main() {\n  print(total)\n}\n"
	reporter := NewReporter(src, "main.kark")

	out := reporter.ReportWithCode(SeverityError, 2, 8, "K002", "undefined identifier `total`", "declare `total` before using it")

	want := []string{
		"error[K002]:",
		"undefined identifier `total`",
		"--> main.kark:2:8",
		"2 |   print(total)",
		"^",
		"= help: declare `total` before using it",
	}
	for _, w := range want {
		if !strings.Contains(out, w) {
			t.Errorf("enhanced format missing %q:\n%s", w, out)
		}
	}
}

func TestDiagnostic_Render(t *testing.T) {
	src := "let count = 10\n"
	d := WarningDiagnostic("main.kark", 1, 5, CodeWarnUnused, "unused variable `count`")
	d = d.WithHelp("remove the unused variable or use it")

	out := d.Render(src)

	// Should use numeric code K100
	if !strings.Contains(out, "warning[K100]:") {
		t.Errorf("Render should use numeric code K100, got:\n%s", out)
	}
	if !strings.Contains(out, "unused variable `count`") {
		t.Errorf("Render missing message:\n%s", out)
	}
	if !strings.Contains(out, "= help: remove the unused variable or use it") {
		t.Errorf("Render missing help text:\n%s", out)
	}
}

func TestReporter_WithoutCode(t *testing.T) {
	src := "let x = 5\n"
	reporter := NewReporter(src, "test.kark")

	out := reporter.Report(SeverityError, 1, 5, "type mismatch")

	// Should not have brackets when no code is provided
	if strings.Contains(out, "[") && strings.Contains(out, "]") {
		t.Errorf("Report without code should not use brackets:\n%s", out)
	}
	if !strings.Contains(out, "error:") {
		t.Errorf("Report without code should still have severity:\n%s", out)
	}
}
