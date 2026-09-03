package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"karkain/pkg/codegen"
	"karkain/pkg/lexer"
	"karkain/pkg/parser"
)

// buildRunCapture compiles a .kark source to an executable and runs it,
// returning the program's stdout.
func buildRunCapture(t *testing.T, dir, name, src string) string {
	t.Helper()
	if _, err := exec.LookPath("gcc"); err != nil {
		t.Skip("gcc not available")
	}
	file := filepath.Join(dir, name)
	if err := os.WriteFile(file, []byte(src), 0644); err != nil {
		t.Fatalf("write .kark: %v", err)
	}
	exe := filepath.Join(dir, "prog")
	cfg := codegen.NewConfig()
	cfg.CompileOnly = false
	cfg.RunAfter = false
	cfg.OutputPath = exe

	l := lexer.New(src)
	p := parser.New(l)
	prog := p.ParseProgram()
	if prog == nil {
		t.Fatal("ParseProgram returned nil")
	}
	g := codegen.New(cfg)
	if err := g.GenerateAndCompile(prog, file); err != nil {
		t.Fatalf("compile failed: %v", err)
	}

	if runtime.GOOS == "windows" {
		exe += ".exe"
	}
	out, err := exec.Command(exe).CombinedOutput()
	if err != nil {
		t.Fatalf("run failed: %v output=%s", err, string(out))
	}
	defer os.Remove(filepath.Join(dir, name+".c"))
	defer os.Remove(exe)
	return string(out)
}

// TestPhase70_NormalProgramGCC verifies the Phase 70 header additions
// (stdatomic.h + immintrin.h) do not break normal compile-and-run via gcc.
func TestPhase70_NormalProgramGCC(t *testing.T) {
	out := buildRunCapture(t, t.TempDir(), "simple.kark", `
func add(a, b) {
    return a + b
}
func main() {
    print(add(2, 3))
}
`)
	if !strings.Contains(out, "5") {
		t.Errorf("expected print to contain 5, got %q", out)
	}
}
