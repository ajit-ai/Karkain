package bootstrap

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// oneCompiler1 builds the self-hosted stage2 compiler (karkain-compiler1) once
// and returns its path. It uses real process arguments in every test below.
type compiler1Handle struct {
	mu        sync.Mutex
	path      string
	project   string
	haveBuild bool
}

var compiler1Build compiler1Handle

func (h *compiler1Handle) ensure(t *testing.T) string {
	t.Helper()
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.haveBuild {
		return h.path
	}
	root := findProjectRoot(t)
	s1, err := RunStage1(root)
	if err != nil {
		t.Fatalf("Stage 1 (build compiler1) failed: %v", err)
	}
	h.path = s1.Binary
	h.project = root
	h.haveBuild = true
	return h.path
}

// runCompiler1 execs the real compiler1 binary with the given argv (args[0] is
// the program name, injected automatically by os/exec). Returns combined output
// and the exit code.
func runCompiler1(t *testing.T, args ...string) (string, int) {
	t.Helper()
	bin := compiler1Build.ensure(t)
	cmd := exec.Command(bin, args...)
	cmd.Dir = compiler1Build.project
	out, err := cmd.CombinedOutput()
	exitCode := 0
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			exitCode = ee.ExitCode()
		} else {
			t.Fatalf("failed to run compiler1: %v", err)
		}
	}
	return string(out), exitCode
}

// writeFixture writes a minimal .kark program to a temp dir and returns its path.
func writeFixture(t *testing.T, name, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("writing fixture: %v", err)
	}
	return path
}

const helloKar = `func main() {
    print("hello")
}
`

// Argument Test 1: No arguments -> usage text, exit 0.
func TestArgs_NoArguments(t *testing.T) {
	out, code := runCompiler1(t)
	if !strings.Contains(out, "Usage: karkain") {
		t.Fatalf("expected usage text, got: %q", out)
	}
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
}

// Argument Test 2: One argument (a command with nothing else) -> usage.
func TestArgs_OneArgument(t *testing.T) {
	out, code := runCompiler1(t, "build")
	if !strings.Contains(out, "Usage: karkain build") {
		t.Fatalf("expected build usage text, got: %q", out)
	}
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
}

// Argument Test 3: Multiple arguments -> --version recognized.
func TestArgs_Version(t *testing.T) {
	out, code := runCompiler1(t, "--version")
	if !strings.Contains(out, "Karkain Compiler v0.117.0") {
		t.Fatalf("expected version text, got: %q", out)
	}
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
}

// Argument Test 4: Arguments containing spaces -> build a path with spaces.
func TestArgs_PathWithSpaces(t *testing.T) {
	spaced := filepath.Join(t.TempDir(), "my dir with spaces")
	if err := os.MkdirAll(spaced, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	path := filepath.Join(spaced, "hello.kark")
	if err := os.WriteFile(path, []byte(helloKar), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	out, code := runCompiler1(t, "build", path, "--target", "c23")
	if code != 0 {
		t.Fatalf("build with spaces path failed (%d): %s", code, out)
	}
	cPath := strings.TrimSuffix(path, ".kark") + ".c23"
	if _, err := os.Stat(cPath); err != nil {
		t.Fatalf("expected generated %s, got error: %v (out=%s)", cPath, err, out)
	}
}

// Argument Test 5: compiler build file.kark -> real output produced at real path.
func TestArgs_BuildFile(t *testing.T) {
	path := writeFixture(t, "hello.kark", helloKar)

	out, code := runCompiler1(t, "build", path, "--target", "c23")
	if code != 0 {
		t.Fatalf("build failed (%d): %s", code, out)
	}
	cPath := strings.TrimSuffix(path, ".kark") + ".c23"
	data, err := os.ReadFile(cPath)
	if err != nil {
		t.Fatalf("expected generated %s: %v (out=%s)", cPath, err, out)
	}
	if !strings.Contains(string(data), "#include <stdio.h>") {
		t.Fatalf("generated C missing preamble in %s", cPath)
	}
}

// Argument Test 6: Invalid command handling.
func TestArgs_InvalidCommand(t *testing.T) {
	out, code := runCompiler1(t, "notacommand")
	if !strings.Contains(out, "Unknown command: notacommand") {
		t.Fatalf("expected unknown-command message, got: %q", out)
	}
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
}
