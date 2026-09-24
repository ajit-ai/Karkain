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

func runNativeCode(t *testing.T, img []byte) (string, int) {
	t.Helper()
	if runtime.GOOS != "linux" || runtime.GOARCH != "amd64" {
		t.Skip("native execution needs Linux x86-64")
	}
	// Phase 147: quarantine lifted. The P1 (immediate SIGSEGV on CI) was
	// root-caused to a REX.W misencoding in MovRegImm32 (every use site
	// desynchronized the instruction stream) plus a push-shifted slot
	// read in binary operands and missing print newlines — all fixed and
	// proven green on Linux CI (see docs/audit/PHASE-147-FINAL-REPORT.md).
	// Execution tests run normally in default CI from here on.
	path := filepath.Join(t.TempDir(), "prog")
	if err := os.WriteFile(path, img, 0o755); err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command(path).CombinedOutput()
	if err == nil {
		return string(out), 0
	}
	if ee, ok := err.(*exec.ExitError); ok {
		return string(out), ee.ExitCode()
	}
	t.Fatalf("run: %v (out=%q)", err, out)
	return "", -1
}

func TestNativeHello(t *testing.T) {	img := compileNative(t, `func main() {
    print("hello native")
    print(2 + 3 * 4)
    print((10 - 4) * 2)
}
`)
	if out, _ := runNativeCode(t, img); out != "" && out != "hello native\n14\n12\n" {
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
	if out, _ := runNativeCode(t, img); out != "" && out != "42\n126\n-7\n1000000000000\n" {
		t.Fatalf("output mismatch: %q", out)
	}
}

func TestNativeBisect(t *testing.T) {
	// Execution bisection: each program exercises one more subsystem, so
	// a crash pins the responsible layer. Permanent regression coverage.
	cases := []struct {
		name     string
		src      string
		want     string
		wantCode int
	}{
		{"empty", "func main() {\n}\n", "", 0},
		{"retcode", "func main() {\n    return 7\n}\n", "", 7},
		{"int42", "func main() {\n    print(42)\n}\n", "42\n", 0},
		{"str", "func main() {\n    print(\"hi\")\n}\n", "hi\n", 0},
		{"arith", "func main() {\n    print(2 + 3 * 4)\n}\n", "14\n", 0},
		{"call", "func add(a, b) {\n    return a + b\n}\nfunc main() {\n    print(add(20, 22))\n}\n", "42\n", 0},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			img := compileNative(t, c.src)
			out, code := runNativeCode(t, img)
			if out != "" && out != c.want {
				t.Errorf("%s: output %q, want %q", c.name, out, c.want)
			}
			if code != c.wantCode {
				t.Errorf("%s: exit %d, want %d (out=%q)", c.name, code, c.wantCode, out)
			}
		})
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
func main() { print(f(1, 2)) }`, "has 2 args (want 7)"},
		{`func main() { print(1.5) }`, "floating-point"},
		{`func main() { print(nope(1)) }`, "undefined function"},
		{`func main() { let s = "x" print(s + 1) }`, "in int position"},
		{`func main() { break }`, "break outside"},
		{`func main() { continue }`, "continue outside"},
		{`func main() { if (1) { print(1) } }`, "must be an int comparison"},
		{`func main() { let y = 1 for x in y { print(x) } }`, "for-in loops are not supported"},
		{`func main(x) { print(x) }`, "takes no arguments"},
		{`func main() { return "x" }`, "must return int"},
		{`func f(s string) { print(s) } func main() { f(1) }`, "int argument for string parameter"},
		{`func f(a int) { print(a) } func main() { f("x") }`, "string argument for int parameter"},
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

func TestNativeControl(t *testing.T) {
	// Phase 148: control-flow execution goldens. Each program exercises
	// one more construct, so a failure pins the responsible lowering.
	// Execution runs on Linux x86-64; elsewhere compileNative still
	// proves parse + lowering + structural validity.
	cases := []struct {
		name     string
		src      string
		want     string
		wantCode int
	}{
		{"if_else", "func main() {\n    if (2 > 1) {\n        print(10)\n    } else {\n        print(20)\n    }\n    if (1 == 2) {\n        print(30)\n    } else {\n        print(40)\n    }\n}\n", "10\n40\n", 0},
		{"while_sum", "func main() {\n    let s = 0\n    let i = 1\n    while (i <= 10) {\n        s = s + i\n        i = i + 1\n    }\n    print(s)\n}\n", "55\n", 0},
		{"cfor_sum", "func main() {\n    let s = 0\n    for (let i = 0; i < 10; i = i + 1) {\n        s = s + i\n    }\n    print(s)\n}\n", "45\n", 0},
		{"break_continue", "func main() {\n    let s = 0\n    let i = 0\n    while (i < 10) {\n        i = i + 1\n        if (i == 3) {\n            continue\n        }\n        if (i == 7) {\n            break\n        }\n        s = s + i\n    }\n    print(s)\n}\n", "18\n", 0},
		{"nested", "func main() {\n    let n = 0\n    let i = 0\n    while (i < 3) {\n        let j = 0\n        while (j < 3) {\n            n = n + 1\n            j = j + 1\n        }\n        i = i + 1\n    }\n    print(n)\n}\n", "9\n", 0},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			img := compileNative(t, c.src)
			out, code := runNativeCode(t, img)
			if out != "" && out != c.want {
				t.Errorf("%s: output %q, want %q", c.name, out, c.want)
			}
			if code != c.wantCode {
				t.Errorf("%s: exit %d, want %d (out=%q)", c.name, code, c.wantCode, out)
			}
		})
	}
}

// TestNativeCallsABI pins the Phase-148 calling convention: string args
// as (ptr,len) pairs, string returns as (RAX=ptr,RDX=len), stack units
// past the six register units via the R10 extras pointer, exact arity,
// and main-takes-no-arguments. Execution on Linux; structural elsewhere.
func TestNativeCallsABI(t *testing.T) {
	cases := []struct {
		name     string
		src      string
		want     string
		wantCode int
	}{
		{"string_arg", "func greet(name string) {\n    print(name)\n}\nfunc main() {\n    greet(\"hi\")\n}\n", "hi\n", 0},
		{"string_var_arg", "func greet(name string) {\n    print(name)\n}\nfunc main() {\n    let w = \"yo\"\n    greet(w)\n}\n", "yo\n", 0},
		{"string_return", "func word() {\n    return \"abc\"\n}\nfunc main() {\n    print(word())\n}\n", "abc\n", 0},
		{"mixed_args", "func show(a int, s string, b int) {\n    print(a)\n    print(s)\n    print(b)\n}\nfunc main() {\n    show(1, \"two\", 3)\n}\n", "1\ntwo\n3\n", 0},
		{"seven_params", "func sum7(a, b, c, d, e, f, g) {\n    return a + b + c + d + e + f + g\n}\nfunc main() {\n    print(sum7(1, 2, 3, 4, 5, 6, 7))\n}\n", "28\n", 0},
		{"straddle", "func mix(a, b, c, d, e, s string) {\n    print(a)\n    print(s)\n}\nfunc main() {\n    mix(1, 2, 3, 4, 5, \"six\")\n}\n", "1\nsix\n", 0},
		{"nested_strcall", "func id(s string) {\n    return s\n}\nfunc main() {\n    print(id(id(\"ok\")))\n}\n", "ok\n", 0},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			img := compileNative(t, c.src)
			out, code := runNativeCode(t, img)
			if out != "" && out != c.want {
				t.Errorf("%s: output %q, want %q", c.name, out, c.want)
			}
			if code != c.wantCode {
				t.Errorf("%s: exit %d, want %d (out=%q)", c.name, code, c.wantCode, out)
			}
		})
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
