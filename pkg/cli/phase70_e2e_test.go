package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"karkain/pkg/codegen"
	"karkain/pkg/lexer"
	"karkain/pkg/parser"
)

// buildRunCapture compiles a .kark source to an executable and runs it,
// returning the program's stdout. Transient gcc failures (Windows toolchain
// file locks under parallel load) are retried.
func buildRunCapture(t *testing.T, dir, name, src string) string {
	t.Helper()
	if _, err := exec.LookPath("gcc"); err != nil {
		t.Skip("gcc not available")
	}
	file := filepath.Join(dir, name)
	if err := os.WriteFile(file, []byte(src), 0644); err != nil {
		t.Fatalf("write .kark: %v", err)
	}
	defer os.Remove(filepath.Join(dir, name+".c"))

	const attempts = 4
	var lastErr error
	for attempt := 1; attempt <= attempts; attempt++ {
		exe := filepath.Join(dir, fmt.Sprintf("prog_%d", attempt))
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
			lastErr = err
			if strings.Contains(err.Error(), "C compilation failed") && attempt < attempts {
				time.Sleep(500 * time.Millisecond)
				continue
			}
			t.Fatalf("compile failed: %v", err)
		}

		run := exe
		if runtime.GOOS == "windows" {
			run += ".exe"
		}
		out, err := exec.Command(run).CombinedOutput()
		if err != nil {
			if attempt < attempts {
				lastErr = err
				time.Sleep(500 * time.Millisecond)
				continue
			}
			t.Fatalf("run failed: %v output=%s", err, string(out))
		}
		defer os.Remove(run)
		return string(out)
	}
	t.Fatalf("transient gcc failures in buildRunCapture: %v", lastErr)
	return ""
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
