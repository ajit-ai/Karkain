package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Phase 112 gate: `karkain debug` — compile and run with opt-in function-level
// execution tracing on the Go engine. Every function enter/leave emits a
// deterministic karkain:<file>:enter/leave <func> trace line on stderr while
// the program's stdout passes through unmodified. kcc is an explicit
// "unsupported/deferred" boundary (no silent fallback), and plain `karkain run`
// never instruments the program.

const phase112DebugSrc = `
func square(x) {
	return x * x
}

func main() {
	print("d = " + str(square(square(3))))
}
`

func writePhase112DebugFixture(t *testing.T, src string) string {
	t.Helper()
	f := filepath.Join(t.TempDir(), "main.kark")
	if err := os.WriteFile(f, []byte(src), 0644); err != nil {
		t.Fatalf("write test file: %v", err)
	}
	return f
}

func TestPhase112_DebugCommand_Trace(t *testing.T) {
	skipIfNoCompiler(t)
	f := writePhase112DebugFixture(t, phase112DebugSrc)
	stdout := captureStdout(t, func() {
		errOut := captureStderr(t, func() {
			res := DebugCommand(f, "go", false)
			if res.ExitCode != ExitSuccess {
				t.Fatalf("DebugCommand failed: [%d] %s", res.ExitCode, res.Message)
			}
		})
		for _, want := range []string{
			"karkain:main.kark:enter main",
			"karkain:main.kark:enter square",
			"karkain:main.kark:leave square",
			"karkain:main.kark:leave main",
		} {
			if !strings.Contains(strings.ReplaceAll(errOut, "\r\n", "\n"), want) {
				t.Errorf("trace output missing %q\n%s", want, errOut)
			}
		}
	})
	if !strings.Contains(strings.ReplaceAll(stdout, "\r\n", "\n"), "d = 81") {
		t.Errorf("program stdout did not pass through\n%s", stdout)
	}
}

func TestPhase112_DebugCommand_KCCBoundary(t *testing.T) {
	f := writePhase112DebugFixture(t, phase112DebugSrc)
	res := DebugCommand(f, "kcc", false)
	if res.ExitCode != ExitFailure {
		t.Fatalf("expected exit failure for kcc, got [%d] %s", res.ExitCode, res.Message)
	}
	if !strings.Contains(res.Message, "supports the Go engine only") {
		t.Errorf("expected Go-engine-only boundary message, got %q", res.Message)
	}
}

func TestPhase112_DebugCommand_Usage(t *testing.T) {
	res := DebugCommand("", "go", false)
	if res.ExitCode != ExitUsage {
		t.Fatalf("expected usage error for missing file, got [%d] %s", res.ExitCode, res.Message)
	}
}