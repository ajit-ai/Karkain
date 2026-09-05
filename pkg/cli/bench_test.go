package cli

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"karkain/pkg/lexer"
	"karkain/pkg/parser"
)

func TestDiscoverBenchmarks_SortsByName(t *testing.T) {
	src := `
func bench_z() { print(1) }
func ordinary() { return 1 }
func bench_a() { print(2) }
`
	l := lexer.New(src)
	p := parser.New(l)
	prog := p.ParseProgram()
	fns := discoverBenchmarks(prog)
	if len(fns) != 2 {
		t.Fatalf("expected 2 benchmarks, got %d", len(fns))
	}
	if fns[0].Name != "bench_a" || fns[1].Name != "bench_z" {
		t.Errorf("unexpected order: %s, %s", fns[0].Name, fns[1].Name)
	}
}

func TestFindBenchFiles_AcceptsFileAndDir(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "x_bench.kark"), "func bench_x() {}\n")
	writeFile(t, filepath.Join(dir, "y_test.kark"), "func test_y() {}\n")
	writeFile(t, filepath.Join(dir, "notes.txt"), "ignore")
	writeFile(t, filepath.Join(dir, "plain.kark"), "func main() {}")

	files, err := findBenchFiles(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 2 {
		t.Fatalf("expected 2 files, got %d: %v", len(files), files)
	}

	f, err := findBenchFiles(filepath.Join(dir, "x_bench.kark"))
	if err != nil || len(f) != 1 {
		t.Fatalf("single-file bench should be accepted: %v %v", f, err)
	}
	if _, err := findBenchFiles(filepath.Join(dir, "plain.kark")); err == nil {
		t.Error("plain .kark file should be rejected by bench")
	}
}

func TestBenchCommand_E2E(t *testing.T) {
	if _, err := exec.LookPath("gcc"); err != nil {
		t.Skip("gcc not available")
	}
	bin := buildKarkain(t)
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "bm_bench.kark"), `
func bench_add() {
  print(1 + 2 + 3 + 4 + 5 + 6 + 7 + 8 + 9 + 10)
}
func bench_mul() {
  print(1 * 2 * 3 * 4 * 5 * 6 * 7 * 8 * 9 * 10)
}
`)

	out, err := runBin(t, bin, root, "bench")
	if err != nil {
		t.Fatalf("bench failed: %v\n%s", err, out)
	}
	for _, want := range []string{"BENCHMARK", "bench_add", "bench_mul", "DURATION", "2 benchmark(s) completed"} {
		if !strings.Contains(out, want) {
			t.Errorf("bench output missing %q:\n%s", want, out)
		}
	}
}

func TestCLI_Bench_EmptyDirExitsZero(t *testing.T) {
	bin := buildKarkain(t)
	root := t.TempDir()
	out, err := runBin(t, bin, root, "bench")
	if err != nil {
		t.Fatalf("bench on empty dir should succeed: %v\n%s", err, out)
	}
	if !strings.Contains(out, "No benchmark files found") {
		t.Errorf("unexpected empty-dir output:\n%s", out)
	}
}

func TestCLI_Bench_RejectsPlainFile(t *testing.T) {
	bin := buildKarkain(t)
	root := t.TempDir()
	plain := filepath.Join(root, "plain.kark")
	if err := os.WriteFile(plain, []byte("func main() {}\n"), 0644); err != nil {
		t.Fatal(err)
	}
	_, err := runBin(t, bin, root, "bench", plain)
	if err == nil {
		t.Fatal("bench on a plain .kark file should fail")
	}
	var ee *exec.ExitError
	if !errors.As(err, &ee) || ee.ExitCode() == 0 {
		t.Errorf("bench on plain file should exit non-zero")
	}
}
