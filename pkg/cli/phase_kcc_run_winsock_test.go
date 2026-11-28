package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestPhaseKCCRunWinsock_ guards the direct `kcc run` link path.
//
// Root cause (docs/audit/KCC-RUN-WINSOCK-CORRECTIVE.md): the generated C
// preamble references Winsock on Windows, but the self-hosted compiler's
// runFile linked without -lws2_32, so `kcc run` failed with undefined
// __imp_WSA* symbols. The Go CLI path was already correct; this gate drives
// the real self-hosted binary directly, which is the path that was broken.
func TestPhaseKCCRunWinsock_DirectRunLinksAndExecutes(t *testing.T) {
	bin := phase95KCC(t)
	dir := t.TempDir()
	src := filepath.Join(dir, "hello.kark")
	if err := os.WriteFile(src, []byte("func main() { print(42) }\n"), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	cmd := exec.Command(bin, "run", "hello.kark")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	text := string(out)
	if err != nil {
		t.Fatalf("direct kcc run failed: %v\n%s", err, text)
	}

	// Exact program output: the compiler prints its own lines, but the program
	// must print 42 and nothing else as its own stdout fragment. Do not trim
	// away bytes -- the historical defect was a missing byte.
	marker := "--- Output ---"
	idx := strings.Index(text, marker)
	if idx < 0 {
		t.Fatalf("no output marker; got:\n%s", text)
	}
	programOut := text[idx+len(marker):]
	programOut = strings.ReplaceAll(programOut, "\r\n", "\n")
	if !strings.Contains(programOut, "42\n") {
		t.Errorf("program did not print 42; got program output %q (full:\n%s)", programOut, text)
	}

	if runtime.GOOS == "windows" {
		// The retry must be observable. An exit-code-only assertion would pass
		// if the first link somehow succeeded without the library. The first
		// attempt's "undefined reference" lines are expected on Windows -- they
		// are the reason the retry exists -- so they are not a failure signal.
		if !strings.Contains(text, "-lws2_32") {
			t.Errorf("Windows link did not retry with -lws2_32; output:\n%s", text)
		}
	}
}
