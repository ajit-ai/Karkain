package cli

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"karkain/pkg/diagnostics"
)

// Phase 83 — warning foundation through the real check pipeline: warnings are
// collected, rendered (human + JSON) and never terminate compilation.

func writePhase83File(t *testing.T, body string) string {
	t.Helper()
	f := filepath.Join(t.TempDir(), "phase83.kark")
	if err := os.WriteFile(f, []byte(body), 0644); err != nil {
		t.Fatalf("write test file: %v", err)
	}
	return f
}

func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stderr = w
	defer func() { os.Stderr = old }()

	var buf bytes.Buffer
	done := make(chan struct{})
	go func() {
		io.Copy(&buf, r)
		close(done)
	}()

	fn()

	w.Close()
	<-done
	os.Stderr = old
	return buf.String()
}

func TestCheck_WarningsDoNotTerminate(t *testing.T) {
	f := writePhase83File(t, "func main() {\n  let count = 10\n  println(\"hello\")\n}\n")
	res := CheckCommand(f, false)
	if res.ExitCode != ExitSuccess {
		t.Fatalf("warnings must not terminate compilation: exit %d (%s)", res.ExitCode, res.Message)
	}
	if !strings.Contains(res.Message, "warning") {
		t.Errorf("success message should mention warnings, got %q", res.Message)
	}
}

func TestCheck_HumanWarningRendered(t *testing.T) {
	f := writePhase83File(t, "func main() {\n  let count = 10\n  println(\"hello\")\n}\n")
	out := captureStderr(t, func() {
		if res := CheckCommand(f, false); res.ExitCode != ExitSuccess {
			t.Fatalf("warnings must not terminate: %d", res.ExitCode)
		}
	})
	if !strings.Contains(out, "warning[W-K-UNUSED]:") {
		t.Errorf("stderr missing warning header:\n%s", out)
	}
	if !strings.Contains(out, "unused variable `count`") {
		t.Errorf("stderr missing warning message:\n%s", out)
	}
	if !strings.Contains(out, f+":2:7") {
		t.Errorf("stderr missing true source location:\n%s", out)
	}
	if strings.Contains(out, "error[") {
		t.Errorf("warning report must not use error header:\n%s", out)
	}
}

func TestCheck_JSONWarnings(t *testing.T) {
	f := writePhase83File(t, "func main() {\n  let count = 10\n  println(\"hello\")\n}\n")
	out := captureStdout(t, func() {
		res := CheckCommandFormatted(f, false, CheckFormatJSON)
		if res.ExitCode != ExitSuccess {
			t.Fatalf("JSON check with warnings: exit %d want 0", res.ExitCode)
		}
		if res.Message != "" {
			t.Errorf("JSON check must not print summary on stdout, got %q", res.Message)
		}
	})

	var diags []diagnostics.Diagnostic
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &diags); err != nil {
		t.Fatalf("stdout is not valid JSON: %v\nstdout=%q", err, out)
	}
	if len(diags) != 1 {
		t.Fatalf("expected 1 warning diagnostic, got %d (stdout=%q)", len(diags), out)
	}
	d := diags[0]
	if d.Severity != "warning" {
		t.Errorf("expected severity warning, got %q", d.Severity)
	}
	if d.Code != string(diagnostics.CodeWarnUnused) {
		t.Errorf("expected code W-K-UNUSED, got %q", d.Code)
	}
	if d.Line != 2 || d.Column != 7 || d.EndColumn != 12 {
		t.Errorf("warning span wrong: line=%d col=%d end=%d", d.Line, d.Column, d.EndColumn)
	}
	if d.Excerpt == "" {
		t.Error("warning should carry an excerpt")
	}
}

func TestCheck_ErrorsStillShortCircuitWarnings(t *testing.T) {
	// A resolve error means the warning pass never runs (no cascade).
	f := writePhase83File(t, "func main() {\n  let count = 10\n  println(ghost)\n}\n")
	res := CheckCommand(f, false)
	if res.ExitCode != ExitCompile {
		t.Fatalf("errors must still fail check: exit %d", res.ExitCode)
	}
	out := captureStderr(t, func() {
		CheckCommand(f, false)
	})
	if strings.Contains(out, "W-K-UNUSED") {
		t.Errorf("warning pass must be skipped when resolution fails:\n%s", out)
	}
	if !strings.Contains(out, "error[E-K-RES]:") {
		t.Errorf("error should still render with structured header:\n%s", out)
	}
}

func TestAnalyzeSource_DisjointErrorWarningResults(t *testing.T) {
	clean := "func main() {\n  let count = 10\n  println(\"hi\")\n}\n"
	_ = clean
	errDiags, warnDiags, n := AnalyzeSource("x.kark", clean, nil)
	if len(errDiags) != 0 {
		t.Fatalf("clean program must have no errors, got %+v", errDiags)
	}
	if len(warnDiags) != 1 {
		t.Fatalf("expected 1 warning, got %d", len(warnDiags))
	}
	// n is the statement count for the success message, never the diag count.
	_ = n
}