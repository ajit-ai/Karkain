package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestPhase108WasmE2E(t *testing.T) {
	wt := findWasmtime()
	if wt == "" {
		t.Skip("wasmtime not found; install from https://wasmtime.dev")
	}

	srcDir := filepath.Join("..", "..", "examples", "wasm")
	srcFile := filepath.Join(srcDir, "hello.kark")

	if _, err := os.Stat(srcFile); err != nil {
		t.Skipf("examples/wasm/hello.kark missing: %v", err)
	}

	tmpDir := t.TempDir()
	outWasm := filepath.Join(tmpDir, "hello.wasm")

	cmd := exec.Command("go", "run", "../../cmd/karkain", "build", "--target", "wasm32-wasi", "-o", outWasm, srcFile)
	out, err := cmd.CombinedOutput()
	t.Logf("build stdout: %s", out)
	if err != nil {
		t.Fatalf("build failed: %v\n%s", err, out)
	}

	if _, err := os.Stat(outWasm); err != nil {
		t.Fatalf("expected .wasm output at %s", outWasm)
	}

	bin, err := os.ReadFile(outWasm)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("wasm binary: %d bytes", len(bin))
	if len(bin) < 100 {
		t.Fatal("wasm binary too small")
	}

	run := exec.Command(wt, "run", outWasm)
	run.Dir = tmpDir
	runOut, runErr := run.CombinedOutput()
	t.Logf("run output: %q", string(runOut))
	if runErr != nil {
		t.Fatalf("wasmtime run failed: %v\n%s", runErr, runOut)
	}

	want := "hello wasmtime\n42\ndone\n"
	if string(runOut) != want {
		t.Fatalf("output mismatch: got %q want %q", string(runOut), want)
	}
}

func TestPhase108WasmDeterminism(t *testing.T) {
	wt := findWasmtime()
	if wt == "" {
		t.Skip("wasmtime not found")
	}

	srcDir := filepath.Join("..", "..", "examples", "wasm")
	srcFile := filepath.Join(srcDir, "hello.kark")
	if _, err := os.Stat(srcFile); err != nil {
		t.Skip("examples/wasm/hello.kark missing")
	}

	tmpDir := t.TempDir()
	out1 := filepath.Join(tmpDir, "a.wasm")
	out2 := filepath.Join(tmpDir, "b.wasm")

	build := func(out string) []byte {
		cmd := exec.Command("go", "run", "../../cmd/karkain", "build", "--target", "wasm32-wasi", "-o", out, srcFile)
		outBytes, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("build failed: %v\n%s", err, outBytes)
		}
		b, err := os.ReadFile(out)
		if err != nil {
			t.Fatal(err)
		}
		return b
	}

	a := build(out1)
	b := build(out2)

	if !strings.EqualFold(string(a), string(b)) {
		t.Fatalf("non-deterministic wasm output: len=%d vs %d", len(a), len(b))
	}
	t.Logf("deterministic wasm: %d bytes", len(a))
}