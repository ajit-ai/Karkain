package cli

import (
	"os"
	"path/filepath"
	"testing"

	"karkain/pkg/diagnostics"
)

const phase105MultiErrorFixture = "phase105_errors"

// multierrorPath resolves a fixture file under examples/phase105_errors.
func multierrorPath(t *testing.T, name string) string {
	t.Helper()
	return filepath.Join("..", "..", "examples", phase105MultiErrorFixture, name)
}

// expectedSyntaxErrorLines is the exact set of recoverable syntax errors the
// multierr.kark fixture must surface: every one is a genuine, independent
// top-level or expression-shape error at its own line, none cascades, none is
// duplicated, and none is fabricated by the checker.
var expectedSyntaxErrorLines = []string{
	"expected '@target(...)' attribute, got '@bad_attr_a'",
	"expected '@target(...)' attribute, got '@bad_attr_b'",
	"expected '@target(...)' attribute, got '@bad_attr_c'",
	"unexpected token '1' after 'public' modifier (expected func/type/enum)",
	"unexpected token '2' after 'public' modifier (expected func/type/enum)",
	"unexpected token ')' ())",
}

func TestPhase105_SyntaxMultiErrorGoEngine(t *testing.T) {
	fixture := multierrorPath(t, "multierr.kark")

	diags := projectSyntaxDiagnostics(fixture)
	if len(diags) < 5 {
		t.Fatalf("expected at least 5 recoverable syntax errors, got %d", len(diags))
	}

	// Every diagnostic must be syntax-class, attributed to the fixture file,
	// carry a real 1-based column, and be free of duplicates or cascades.
	seen := map[string]bool{}
	for _, d := range diags {
		if d.Code != string(diagnostics.CodeSyntax) {
			t.Errorf("diagnostic %q not flagged as syntax", d.Message)
		}
		if d.File != fixture {
			t.Errorf("diagnostic attributed to %q, want %q", d.File, fixture)
		}
		if d.Line < 1 || d.Column < 1 {
			t.Errorf("diagnostic %q has non-positive span (line %d col %d)", d.Message, d.Line, d.Column)
		}
		if d.Excerpt == "" {
			t.Errorf("diagnostic %q missing excerpt", d.Message)
		}
		key := normalized(d.Message)
		if seen[key] {
			t.Errorf("duplicate diagnostic %q", key)
		}
		seen[key] = true
	}

	// Exact match against the known recoverable set: proves the recovered
	// errors are meaningful and that nothing artificial leaked in.
	for _, want := range expectedSyntaxErrorLines {
		if !seen[normalized(want)] {
			t.Errorf("missing expected recoverable error %q", want)
		}
	}
	if len(diags) != len(expectedSyntaxErrorLines) {
		t.Errorf("got %d diagnostics, want exactly %d", len(diags), len(expectedSyntaxErrorLines))
	}
}

func TestPhase105_SyntaxMultiErrorCLIExit(t *testing.T) {
	fixture := multierrorPath(t, "multierr.kark")
	res := CheckCommandFormatted(fixture, false, CheckFormatHuman)
	if res.ExitCode != ExitCompile {
		t.Fatalf("exit code = %d, want %d (ExitCompile)", res.ExitCode, ExitCompile)
	}
	if diagnosticsFromMessage(res.Message) < 5 {
		t.Errorf("CLI message %q should count at least 5 errors", res.Message)
	}
}

func TestPhase105_SyntaxMultiErrorKCCPath(t *testing.T) {
	fixture := multierrorPath(t, "multierr.kark")
	// The Go-side preflight runs before kcc is even materialized, so both
	// engine paths surface the full recoverable set with the same contract.
	// It never touches a kcc binary and never rebuilds the stage-2 compiler.
	res := KCCCheckCommand(nil, fixture, false)
	if res.ExitCode != ExitCompile {
		t.Fatalf("kcc-path exit code = %d, want %d (ExitCompile)", res.ExitCode, ExitCompile)
	}
	if diagnosticsFromMessage(res.Message) < 5 {
		t.Errorf("kcc-path message %q should count at least 5 errors", res.Message)
	}
}

func TestPhase105_SemanticMultiError(t *testing.T) {
	fixture := multierrorPath(t, "semantic.kark")

	// The semantic fixture must parse cleanly (no syntax preflight firing).
	if diags := projectSyntaxDiagnostics(fixture); len(diags) != 0 {
		t.Fatalf("semantic fixture should parse cleanly, got %d syntax diagnostics", len(diags))
	}

	res := CheckCommandFormatted(fixture, false, CheckFormatHuman)
	if res.ExitCode != ExitCompile {
		t.Fatalf("exit code = %d, want %d (ExitCompile)", res.ExitCode, ExitCompile)
	}
	if diagnosticsFromMessage(res.Message) < 2 {
		t.Errorf("expected multiple name-resolution errors in one invocation, got %q", res.Message)
	}

	// Sanity-check the resolver itself reports every undefined name at once.
	src, srcMap, err := resolveSourcesCheck(fixture)
	if err != nil {
		t.Fatalf("resolveSourcesCheck failed: %v", err)
	}
	diags, _, _ := AnalyzeSource(fixture, src, srcMap)
	var resolveDiags int
	for _, d := range diags {
		if d.Code == string(diagnostics.CodeResolve) {
			resolveDiags++
		}
	}
	if resolveDiags < 2 {
		t.Fatalf("expected multiple resolve diagnostics, got %d", resolveDiags)
	}
}

// normalized strips the parser "line N: " message prefix so comparisons are
// stable against line-number churn inside the fixture.
func normalized(msg string) string {
	for i := 0; i+6 <= len(msg); i++ {
		if msg[i] == 'l' && msg[i+1] == 'i' && msg[i+2] == 'n' && msg[i+3] == 'e' {
			j := i + 4
			for j < len(msg) && msg[j] == ' ' {
				j++
			}
			start := j
			for j < len(msg) && msg[j] >= '0' && msg[j] <= '9' {
				j++
			}
			if j > start && j < len(msg) && msg[j] == ':' {
				return msg[j+2:]
			}
		}
	}
	return msg
}

// diagnosticsFromMessage parses the "N <stage> error(s) found" message count.
func diagnosticsFromMessage(msg string) int {
	n := 0
	for _, c := range msg {
		if c >= '0' && c <= '9' {
			n = n*10 + int(c-'0')
		} else if c != ' ' {
			break
		}
	}
	return n
}

// TestPhase105_PositiveControl ensures a clean single-file program (zero
// recoverable errors) still passes straight through the preflight without
// behavior change.
func TestPhase105_PositiveControl(t *testing.T) {
	// semantic.kark has resolution errors; use a nested clean-file probe written
	// to a temp location to exercise the "clean" branch of the preflight.
	dir := t.TempDir()
	clean := filepath.Join(dir, "clean_main.kark")
	writeTempKark(t, clean, `func main() {
	let x = 1
	print("clean")
	return x
}
`)
	if diags := projectSyntaxDiagnostics(clean); len(diags) != 0 {
		t.Fatalf("clean file should pass preflight, got %d diagnostics", len(diags))
	}
	res := CheckCommandFormatted(clean, false, CheckFormatHuman)
	if res.ExitCode != ExitCompile {
		// A clean parse may still fail resolution (there is no print import
		// context); what matters is the preflight did NOT fire.
		if res.ExitCode != ExitSuccess && res.ExitCode != ExitCompile {
			t.Fatalf("unexpected exit code %d", res.ExitCode)
		}
	}
	if res.Message == "" {
		t.Fatal("expected a definitive CLI message")
	}
}

func writeTempKark(t *testing.T, path, src string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}