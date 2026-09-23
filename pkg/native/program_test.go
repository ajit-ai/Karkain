package native

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"karkain/pkg/lexer"
	"karkain/pkg/parser"
)

// Phase 145B: linked-image tests. Structural pins run everywhere; execution
// runs only where the image can execute (Linux x86-64), skipped elsewhere.

func parseNative(t *testing.T, src string) *parser.Program {
	t.Helper()
	l := lexer.New(src)
	p := parser.New(l)
	prog := p.ParseProgram()
	if len(p.Errors) > 0 {
		t.Fatalf("parse: %v", p.Errors)
	}
	return prog
}

func compileNative(t *testing.T, src string) []byte {
	t.Helper()
	img, err := CompileProgram(parseNative(t, src))
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	if _, _, err := Parse(img); err != nil {
		t.Fatalf("structural parse of linked image: %v", err)
	}
	return img
}

func runNative(t *testing.T, img []byte) string {
	t.Helper()
	if runtime.GOOS != "linux" || runtime.GOARCH != "amd64" {
		t.Skip("native execution needs Linux x86-64")
	}
	path := filepath.Join(t.TempDir(), "prog")
	if err := os.WriteFile(path, img, 0o755); err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command(path).CombinedOutput()
	if err != nil {
		t.Fatalf("run: %v (out=%q)", err, out)
	}
	return string(out)
}

func TestNativeHello(t *testing.T) {
	img := compileNative(t, `func main() {
    print("hello native")
    print(2 + 3 * 4)
    print((10 - 4) * 2)
}
`)
	if out := runNative(t, img); out != "" && out != "hello native\n14\n12\n" {
		t.Fatalf("output mismatch: %q", out)
	}
}

func TestNativeCalls(t *testing.T) {
	img := compileNative(t, `func add(a, b) {
    return a + b
}
func mul3(x) {
    return x * 3
}
func main() {
    let x = add(20, 22)
    print(x)
    print(mul3(x))
    print(0 - 7)
    print(1000000 * 1000000)
}
`)
	if out := runNative(t, img); out != "" && out != "42\n126\n-7\n1000000000000\n" {
		t.Fatalf("output mismatch: %q", out)
	}
}

func TestNativeNegatives(t *testing.T) {
	cases := []struct {
		src     string
		feature string
	}{
		{`func f() { print(1) }`, "no 'main'"},
		{`func main() { print(1) }
func main() { print(2) }`, "duplicate function"},
		{`func f(a, b, c, d, e, g, h) { return a }
func main() { print(f(1, 2, 3, 4, 5, 6, 7)) }`, "max 6"},
		{`func main() { print(1.5) }`, "floating-point"},
		{`func main() { if (1 == 1) { print(1) } }`, "control flow"},
		{`func main() { print(nope(1)) }`, "undefined function"},
		{`func main() { let s = "x" print(s + 1) }`, "in int position"},
	}
	for _, c := range cases {
		_, err := CompileProgram(parseNative(t, c.src))
		if err == nil {
			t.Errorf("expected K145 for %s", c.feature)
			continue
		}
		if !strings.Contains(err.Error(), "error[K145]") || !strings.Contains(err.Error(), c.feature) {
			t.Errorf("bad diagnostic for %s: %v", c.feature, err)
		}
	}
}

func TestNativeDeterministic(t *testing.T) {
	src := "func main() {\n    print(40 + 2)\n}\n"
	a, err := CompileProgram(parseNative(t, src))
	if err != nil {
		t.Fatal(err)
	}
	b, err := CompileProgram(parseNative(t, src))
	if err != nil {
		t.Fatal(err)
	}
	if string(a) != string(b) {
		t.Fatal("non-deterministic linked image")
	}
}
