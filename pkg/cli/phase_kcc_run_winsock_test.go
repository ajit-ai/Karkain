package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestPhaseKCCRunWinsock_ guards the direct `kcc run` link path and its
// stdout contract.
//
// Root cause (docs/audit/KCC-RUN-WINSOCK-CORRECTIVE.md): the generated C
// preamble references Winsock on Windows, but the self-hosted compiler's
// runFile linked without -lws2_32, so `kcc run` failed with undefined
// __imp_WSA* symbols. The Go CLI path was already correct; this gate drives
// the real self-hosted binary directly, which is the path that was broken.
//
// The user-visible contract is now: kcc run prints ONLY the program's stdout.
// No "Compiling:" line, no "--- Output ---" separator, and no failed-link
// diagnostics on Windows (the Winsock flag is on the first link).
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

	// kcc run must emit ONLY the program's output. No compiler chatter, no
	// separator, no failed-link diagnostics. Normalise line endings only;
	// do not trim away bytes -- the historical defect was a missing byte.
	programOut := strings.ReplaceAll(text, "\r\n", "\n")
	if programOut != "42\n" {
		t.Errorf("kcc run output = %q, want exactly %q (full raw:\n%s)", programOut, "42\n", text)
	}
	for _, banned := range []string{"Compiling:", "--- Output ---", "undefined reference"} {
		if strings.Contains(text, banned) {
			t.Errorf("kcc run emitted %q; stdout must contain only program output:\n%s", banned, text)
		}
	}
}
