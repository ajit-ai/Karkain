package wasm

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"karkain/pkg/lexer"
	"karkain/pkg/parser"
)

// parseProgram parses a source string with the standard pipeline.
func parseProgram(src, file string) (*parser.Program, error) {
	l := lexer.New(src)
	p := parser.New(l)
	prog := p.ParseProgram()
	if len(p.Errors) > 0 {
		return nil, fmt.Errorf("parse errors: %v", p.Errors)
	}
	return prog, nil
}

// findWasmtime locates a runnable wasmtime binary.
func findWasmtime() string {
	if p := os.Getenv("KARKAIN_WASM_RUNTIME"); p != "" {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	if p, err := exec.LookPath("wasmtime"); err == nil {
		return p
	}
	home, err := os.UserHomeDir()
	if err == nil {
		p := filepath.Join(home, "bin", "wasmtime.exe")
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

func compile(t *testing.T, src, file string) []byte {
	t.Helper()
	prog, err := parseProgram(src, file)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	bin, err := CompileProgram(prog, file)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	return bin
}

func runWasm(t *testing.T, bin []byte) (string, error) {
	t.Helper()
	wt := findWasmtime()
	if wt == "" {
		t.Skip("wasmtime not found")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "prog.wasm")
	if err := os.WriteFile(path, bin, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	var out bytes.Buffer
	cmd := exec.Command(wt, "run", path)
	cmd.Stdout = &out
	cmd.Stderr = &out
	err := cmd.Run()
	return out.String(), err
}

func TestHello(t *testing.T) {
	src := `func main() {
    print("Hello from Karkain WASM")
}`
	bin := compile(t, src, "main.kark")
	if !MagicOK(bin) {
		t.Fatal("bad magic")
	}
	out, err := runWasm(t, bin)
	if err != nil {
		t.Fatalf("run: %v (out=%q)", err, out)
	}
	if strings.TrimSpace(out) != "Hello from Karkain WASM" {
		t.Fatalf("output mismatch: %q", out)
	}
}

func TestArithmeticAndCalls(t *testing.T) {
	src := `func add(a, b) {
    return a + b
}

func multiply(a, b) {
    return a * b
}

func main() {
    print(add(10, 20))
    print(multiply(4, 5))
    print(2 + 3 * 4)
    print(10 / 3)
    print(10 % 3)
}
`
	bin := compile(t, src, "main.kark")
	out, err := runWasm(t, bin)
	if err != nil {
		t.Fatalf("run: %v (out=%q)", err, out)
	}
	want := "30\n20\n14\n3\n1\n"
	if out != want {
		t.Fatalf("output mismatch: got %q want %q", out, want)
	}
}

func TestControlFlowAndRecursion(t *testing.T) {
	src := `func factorial(n) {
    if n <= 1 {
        return 1
    }
    return n * factorial(n - 1)
}

func fib(n) {
    if n <= 1 {
        return n
    }
    return fib(n - 1) + fib(n - 2)
}

func main() {
    print(factorial(5))
    print(fib(10))
    let i = 0
    while (i < 5) {
        print(i)
        i = i + 2
    }
    for (let j = 0; j < 3; j = j + 1) {
        print(j)
    }
}
`
	bin := compile(t, src, "main.kark")
	out, err := runWasm(t, bin)
	if err != nil {
		t.Fatalf("run: %v (out=%q)", err, out)
	}
	want := "120\n55\n0\n2\n4\n0\n1\n2\n"
	if out != want {
		t.Fatalf("output mismatch: got %q want %q", out, want)
	}
}

func TestArraysAndStrings(t *testing.T) {
	src := `func main() {
    let xs = [10, 20, 30, 40]
    print(xs[0])
    print(xs[3])
    print(len(xs))
    xs[1] = 99
    print(xs[1])
    let s = "wasm"
    print(len(s))
    print(s[1])
    print("a" == "a")
    print("a" == "b")
    print(1 < 2)
    print(5 >= 5)
    print(true)
    print(false)
}
`
	bin := compile(t, src, "main.kark")
	out, err := runWasm(t, bin)
	if err != nil {
		t.Fatalf("run: %v (out=%q)", err, out)
	}
	want := "10\n40\n4\n99\n4\n97\n1\n0\n1\n1\n1\n0\n"
	if out != want {
		t.Fatalf("output mismatch: got %q want %q", out, want)
	}
}

func TestRuntimeErrorDivByZero(t *testing.T) {
	src := `func main() {
    let a = 5
    print(a / 0)
}
`
	bin := compile(t, src, "main.kark")
	out, err := runWasm(t, bin)
	if err == nil {
		t.Fatalf("expected runtime error, got success (out=%q)", out)
	}
	if !strings.Contains(out, "integer division by zero") {
		t.Fatalf("missing division-by-zero diagnostics: %q", out)
	}
	if !strings.Contains(out, "main.kark:3") {
		t.Fatalf("missing source location: %q", out)
	}
}

func TestDeterministicBuild(t *testing.T) {
	src := `func main() {
    print("determinism")
}`
	a := compile(t, src, "main.kark")
	b := compile(t, src, "main.kark")
	if !bytes.Equal(a, b) {
		t.Fatal("non-deterministic module output")
	}
}

func TestUnsupportedFeatures(t *testing.T) {
	cases := []struct {
		src     string
		feature string
	}{
		{`func main() {
	let x = 1.5
	print(x)
}`, "floats"},
		{`func main() {
	let m = {"a": 1}
	let k = m["a"]
	print(k)
}`, "maps"},
		{`func main() {
	let xs = [1,2,3]
	print(xs[0:2])
}`, "slices"},
		{`func main() {
	C.puts("hi")
}`, "C-interop"},
		{`func main() {
	spawn(f)
}
func f() {}`, "concurrency"},
		{`func main() { print(sqrt(4.0)) }`, "math builtins"},
	}
	for _, c := range cases {
		prog, err := parseProgram(c.src, "main.kark")
		if err != nil {
			t.Fatalf("parse %s: %v", c.feature, err)
		}
		_, err = CompileProgram(prog, "main.kark")
		if err == nil {
			t.Fatalf("expected K108 for %s", c.feature)
		}
		if !strings.Contains(err.Error(), "error K108") || !strings.Contains(err.Error(), c.feature) {
			t.Fatalf("bad diagnostic for %s: %v", c.feature, err)
		}
	}
}

func TestBooleanLogic(t *testing.T) {
	src := `func main() {
    print(true && false)
    print(true && true)
    print(false || true)
    print(false || false)
    print(1 < 2 && 2 < 3)
    if (1 == 1 && 2 == 2) {
        print(7)
    } else {
        print(8)
    }
}
`
	bin := compile(t, src, "main.kark")
	out, err := runWasm(t, bin)
	if err != nil {
		t.Fatalf("run: %v (out=%q)", err, out)
	}
	want := "0\n1\n1\n0\n1\n7\n"
	if out != want {
		t.Fatalf("output mismatch: got %q want %q", out, want)
	}
}

func TestForIn(t *testing.T) {
	src := `func main() {
    let xs = [5, 6, 7]
    for x in xs {
        print(x)
    }
}
`
	bin := compile(t, src, "main.kark")
	out, err := runWasm(t, bin)
	if err != nil {
		t.Fatalf("run: %v (out=%q)", err, out)
	}
	want := "5\n6\n7\n"
	if out != want {
		t.Fatalf("output mismatch: got %q want %q", out, want)
	}
}

func TestStructs(t *testing.T) {
	// Phase 139: struct values lower to boxed cells; literal order is free
	// (placement is by declared index) and fields are mutable.
	src := `type Point struct { x int; y int }
func main() {
    let p = Point { y: 20, x: 10 }
    print(p.x)
    print(p.y)
    p.y = 30
    print(p.x + p.y)
}
`
	bin := compile(t, src, "main.kark")
	out, err := runWasm(t, bin)
	if err != nil {
		t.Fatalf("run: %v (out=%q)", err, out)
	}
	want := "10\n20\n40\n"
	if out != want {
		t.Fatalf("output mismatch: got %q want %q", out, want)
	}
}

func TestStructNegatives(t *testing.T) {
	cases := []struct {
		src     string
		feature string
	}{
		{`type Point struct { x int }
func main() {
    let p = Blob { x: 1 }
    print(p.x)
}`, "unknown struct 'Blob'"},
		{`type Point struct { x int; y int }
func main() {
    let p = Point { x: 1 }
    print(p.x)
}`, "struct 'Point' field mismatch"},
		{`type Point struct { x int }
func main() {
    let p = Point { x: 1 }
    print(p.zzz)
}`, "unknown field 'zzz'"},
		{`type A struct { v int; w int }
type B struct { w int; v int }
func main() {
    let a = A { v: 1, w: 2 }
    print(a.v)
}`, "ambiguous field 'v'"},
	}
	for _, c := range cases {
		prog, err := parseProgram(c.src, "main.kark")
		if err != nil {
			t.Fatalf("parse %s: %v", c.feature, err)
		}
		_, err = CompileProgram(prog, "main.kark")
		if err == nil {
			t.Fatalf("expected K108 for %s", c.feature)
		}
		if !strings.Contains(err.Error(), "error K108") || !strings.Contains(err.Error(), c.feature) {
			t.Fatalf("bad diagnostic for %s: %v", c.feature, err)
		}
	}
}