package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestPhaseFoundation_KCCSourceCompiles is a regression gate for a P0 defect
// found during the foundation audit: src/compiler/native_value.kark called an
// undefined accessor (indexRight). The Go front end transpiled the compiler
// source to C23 without a semantic error, so the defect surfaced only at the
// gcc stage as "implicit declaration of function 'indexRight'" -- which broke
// EVERY gate that needs the self-hosted compiler.
//
// The gate therefore builds kcc from the CURRENT source (never a cached or
// KARKAIN_KCC-supplied binary) and then proves the resulting binary is usable
// by checking a trivial program with it. A source-only assertion would not
// have caught this: the source parsed; only the generated C failed.
func TestPhaseFoundation_KCCSourceCompiles(t *testing.T) {
	// Force phase95KCC to build from the working tree instead of reusing a
	// prebuilt binary from the environment.
	t.Setenv("KARKAIN_KCC", "")

	kcc := phase95KCC(t)
	if _, err := os.Stat(kcc); err != nil {
		t.Fatalf("freshly built kcc binary is missing: %v", err)
	}

	dir := t.TempDir()
	prog := filepath.Join(dir, "main.kark")
	if err := os.WriteFile(prog, []byte("func main() { print(42) }\n"), 0o644); err != nil {
		t.Fatalf("write probe program: %v", err)
	}

	cmd := exec.Command(kcc, "check", prog)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("freshly built kcc cannot check a program (this means the compiler source did not build correctly): %v\n%s", err, string(out))
	}
	if !strings.Contains(string(out), "[ok]") {
		t.Fatalf("freshly built kcc check did not report [ok]: %s", string(out))
	}
}
