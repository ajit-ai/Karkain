package native

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
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
	return compileNativeOS(t, OSLinux, src)
}

// compileNativeOS lowers src for the named native OS and structurally
// validates the linked image with the matching parser. Runs everywhere;
// only execution is host-gated.
func compileNativeOS(t *testing.T, osName, src string) []byte {
	t.Helper()
	img, err := CompileProgramForOS(parseNative(t, src), osName)
	if err != nil {
		t.Fatalf("compile (%s): %v", osName, err)
	}
	switch osName {
	case OSWindows:
		if _, _, err := ParsePE(img); err != nil {
			t.Fatalf("structural parse of PE image: %v", err)
		}
	case OSMacOS:
		if _, _, err := ParseMachO(img); err != nil {
			t.Fatalf("structural parse of Mach-O image: %v", err)
		}
	default:
		if _, _, err := Parse(img); err != nil {
			t.Fatalf("structural parse of linked image: %v", err)
		}
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

// runNativeWindows executes a PE image where it can run (windows/amd64)
// and skips elsewhere. Phase 149: the dev host itself is a Windows
// runner, so these execute locally as well as on Windows CI.
func runNativeWindows(t *testing.T, img []byte) (string, int) {
	t.Helper()
	if runtime.GOOS != "windows" || runtime.GOARCH != "amd64" {
		t.Skip("PE execution needs windows/amd64")
	}
	path := filepath.Join(t.TempDir(), "prog.exe")
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

// runNativeMacOS would execute a Mach-O image on darwin/amd64; no such
// runner exists (GitHub macOS legs are arm64), so it honestly skips.
// The structural pins in TestNativeMachO are the v1 proof.
func runNativeMacOS(t *testing.T, img []byte) (string, int) {
	t.Helper()
	if runtime.GOOS != "darwin" || runtime.GOARCH != "amd64" {
		t.Skip("Mach-O execution needs darwin/amd64 (no runner: structural pins only)")
	}
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
		{`func main() { print(nope(1)) }`, "undefined function"},
		{`func main() { let s = "x" print(s + 1) }`, "in int position"},
		{`func main() { break }`, "break outside"},
		{`func main() { continue }`, "continue outside"},
		{`func main() { if (1) { print(1) } }`, "must be a comparison"},
		{`func main() { let y = 1 for x in y { print(x) } }`, "for-in iterates an array"},
		{`func main() { let a = [1, 2] let b = a print(b[0]) }`, "array value must be a literal"},
		{`func main() { let a = ["x"] }`, "element 0 must be int"},
		{`func main() { let a = [1, "x"] }`, "element 1 must be int"},
		{`func main() { let n = 1 print(n[0]) }`, "index target must be an array"},
		{`func main() { print([1, 2][0]) }`, "index target must be a variable"},
		{`func main() { let a = [1, 2] print(len("x")) }`, "len() requires an array"},
		{`func main() { let a = [1, 2] print(len()) }`, "len() takes exactly 1 argument"},
		{`func main() { let a = [1, 2] print(len(a, a)) }`, "len() takes exactly 1 argument"},
		{`func main() { let a = [1, 2] let b = push(a, 3) print(b[0]) }`, "push() is not supported"},
		{`func f(p) { print(p[0]) } func main() { f([1, 2]) }`, "index target must be an array"},
		{`func main() { for k, v in [1, 2] { print(k) } }`, "for-in over maps is not supported"},
		{`func main() { let a = [1, 2] a = [3] }`, "cannot reassign array"},
		{`func main() { let a = [1, 2] a[0] = 5 }`, "assignment target must be a variable"},
		// Phase 150B string view/concat boundaries.
		{`func main() { let s = "ab" if (s < "ac") { print(1) } }`, "string ordering"},
		{`func main() { let s = "ab" if (1 == s) { print(1) } }`, "int operand in string comparison"},
		{`func main() { let s = "ab" if (s == 1) { print(1) } }`, "int operand in string comparison"},
		{`func main() { let n = 1 print(n[0:1]) }`, "slice target must be a string"},
		{`func main() { let a = [1, 2] print(a[0:1]) }`, "slice target must be a string"},
		{`func main() { let s = "ab" print(s - "c") }`, "unsupported string operator"},
		{`func main() { let s = "ab" print(s * "c") }`, "unsupported string operator"},
		{`func main() { let s = "ab" let t = s + 1 }`, "in int position"},
		{`func main() { print("a" + 1) }`, "unsupported expression"},
		{`func main() { let s = "ab" let t = "a" + "b" + 1 }`, "unsupported expression"},
		{`func main(x) { print(x) }`, "takes no arguments"},
		{`func main() { return "x" }`, "must return int"},
		{`func f(s string) { print(s) } func main() { f(1) }`, "int argument for string parameter"},
		{`func f(a int) { print(a) } func main() { f("x") }`, "string argument for int parameter"},
		// Phase 150A: floats lower (bits in RAX), so every path that
		// would treat those bits as an integer refuses loudly instead.
		// The cases below pin the *remaining* float boundaries: mixed
		// int+float operands, float-as-int positions, % on floats,
		// and float main returns. Positive float paths (print,
		// arithmetic, unary minus, comparison, float calls) execute
		// and are pinned by TestNativeFloatExec, not rejected here.
		{`func f(x float) { print(1) } func main() { let s = "x" f(s) }`, "string argument for float parameter"},
		{`func f(x float) { print(1) } func main() { let n = 1 f(n) }`, "argument for float parameter"},
		{`func f(x float) { return x } func main() { print(f(1)) }`, "argument for float parameter"},
		{`func main() { let x = 1.5 let y = x + 1 }`, "mixed float and int in arithmetic"},
		{`func main() { print(1.5 % 2.0) }`, "unsupported float operator"},
		{`func main() { let x = 1.5 if (x < 2) { print(1) } }`, "mixed float and int in comparison"},
		{`func main() { return 1.5 }`, "floating-point return values"},
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

// TestNativeFloatBits pins the Phase 150A float representation:
// float64 -> IEEE-754 bits -> RAX. Structural: each case asserts the
// linked image carries exactly `mov rax, imm64` with the expected
// pattern, absent from an int-only control. Execution goldens live
// in TestNativeFloatExec (compileNative + runNativeCode/Windows),
// which runs where the image can execute and still proves
// parse+lowering+structural validity elsewhere.
func TestNativeFloatBits(t *testing.T) {
	cases := []struct {
		name string
		lit  string
		bits uint64
	}{
		{"zero", "0.0", 0x0000000000000000},
		{"one", "1.0", 0x3FF0000000000000},
		{"pi", "3.141592653589793", 0x400921FB54442D18},
		{"half", "0.5", 0x3FE0000000000000},
		{"eighth", "0.125", 0x3FC0000000000000},
		{"large", "1000.25", 0x408F420000000000},
		{"big", "1234567.5", 0x4132D68780000000},
		{"small", "0.0009765625", 0x3F50000000000000}, // 2^-10
	}
	control := compileNative(t, "func main() {\n    let x = 7\n}\n")
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			// Negative literals are NOT float literals in Karkain: the
			// lexer always emits TokenMinus, so `-1.0`, `-0.125` and
			// `-0.0` parse as UnaryExpr("-", literal) and are covered by
			// the unary-minus K145 row until that later 150A step.
			img := compileNative(t, "func main() {\n    let x = "+c.lit+"\n}\n")
			want := hexOf(t, func(e *Emitter) { e.MovRegImm64(RAX, c.bits) })
			if !bytes.Contains(img, want) {
				t.Errorf("%s: image lacks mov rax, %#016x", c.lit, c.bits)
			}
			if bytes.Contains(control, want) {
				t.Errorf("%s: %#016x also appears in the int-only control image", c.lit, c.bits)
			}
		})
	}
}

// TestNativeFloatIdentifier pins the identifier half of emitFloat: a float
// variable resolves through the slot map (kind-checked) and its stored 64-bit
// pattern is reloaded into RAX unchanged, with no conversion in between. main
// takes no parameters, so the first `let` owns slot 0 (see layout).
func TestNativeFloatIdentifier(t *testing.T) {
	img := compileNative(t, "func main() {\n    let x = 1000.25\n    let y = x\n}\n")
	if !bytes.Contains(img, hexOf(t, func(e *Emitter) { e.MovRegImm64(RAX, 0x408F420000000000) })) {
		t.Error("image lacks the 1000.25 bit pattern from the literal")
	}
	if !bytes.Contains(img, hexOf(t, func(e *Emitter) { e.LoadStack(RAX, 0) })) {
		t.Error("image lacks the float identifier load (mov rax, [rsp+0])")
	}
}
// TestNativeFloatExec pins the Phase 150A float execution surface:
// print, arithmetic, unary minus, comparison, float params/returns and
// calls, plus the zero-divisor rule. Each case compiles through the
// shared lowering (compileNative/compileNativeOS) and executes where
// the image can run (Linux ELF via runNativeCode, Windows PE via
// runNativeWindows); elsewhere compilation + structural validation
// still prove parse + lowering. Expectations mirror the C backend
// (pkg/codegen binary_op: div-by-zero yields 0, %g-style printing
// with up to 6 fractional digits and trimmed trailing zeros).
func TestNativeFloatExec(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want string
	}{
		{"print_lit", "func main() {\n    print(1.5)\n}\n", "1.5\n"},
		{"print_intvalued", "func main() {\n    print(2.0)\n}\n", "2\n"},
		{"print_neg", "func main() {\n    print(-0.125)\n}\n", "-0.125\n"},
		{"arith", "func main() {\n    print(1.5 + 2.25)\n    print(5.0 - 2.5)\n    print(1.5 * 2.0)\n    print(7.0 / 2.0)\n}\n", "3.75\n2.5\n3\n3.5\n"},
		{"div_zero", "func main() {\n    print(1.0 / 0.0)\n}\n", "0\n"},
		{"unary_var", "func main() {\n    let x = 1.5\n    print(-x)\n    print(-(-x))\n}\n", "-1.5\n1.5\n"},
		{"cond", "func main() {\n    let x = 1.5\n    if (x < 2.0) {\n        print(1)\n    } else {\n        print(0)\n    }\n    if (x == 1.5) {\n        print(1)\n    } else {\n        print(0)\n    }\n    if (x != 1.5) {\n        print(0)\n    } else {\n        print(1)\n    }\n}\n", "1\n1\n1\n"},
		{"call_ret", "func half(x float) {\n    return x / 2.0\n}\nfunc main() {\n    print(half(3.0))\n    let y = half(1.0) + half(1.0)\n    print(y)\n}\n", "1.5\n1\n"},
		{"nested", "func main() {\n    print(1.5 + 2.5 * 0.5)\n    print((1.0 + 3.0) / 2.0)\n}\n", "2.75\n2\n"},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			img := compileNative(t, c.src)
			out, code := runNativeCode(t, img)
			if out != "" {
				if out != c.want {
					t.Errorf("%s: output %q, want %q", c.name, out, c.want)
				}
				if code != 0 {
					t.Errorf("%s: exit %d, want 0 (out=%q)", c.name, code, out)
				}
			}
		})
	}
}

// TestNativeFloatExecPE mirrors the float execution surface on the
// Windows container (PE + Win64 boundary + PEB bootstrap): the same
// cases execute live on windows/amd64 and structurally validate
// elsewhere, proving the shared float lowering is OS-neutral.
func TestNativeFloatExecPE(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want string
	}{
		{"print_lit", "func main() {\n    print(1.5)\n}\n", "1.5\n"},
		{"print_neg", "func main() {\n    print(-0.125)\n}\n", "-0.125\n"},
		{"arith", "func main() {\n    print(1.5 + 2.25)\n    print(7.0 / 2.0)\n}\n", "3.75\n3.5\n"},
		{"cond_call", "func half(x float) {\n    return x / 2.0\n}\nfunc main() {\n    if (half(3.0) == 1.5) {\n        print(1)\n    } else {\n        print(0)\n    }\n}\n", "1\n"},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			img := compileNativeOS(t, OSWindows, c.src)
			out, code := runNativeWindows(t, img)
			if out != "" {
				if out != c.want {
					t.Errorf("%s: output %q, want %q", c.name, out, c.want)
				}
				if code != 0 {
					t.Errorf("%s: exit %d, want 0 (out=%q)", c.name, code, out)
				}
			}
		})
	}
}



// nativeArrayCases is the shared Phase 150A array surface, executed on both
// containers: array literal + header, indexing, len(), for-in over int
// arrays, and the control-flow interactions (break/continue/nesting) that
// share the loop-label stack with while/C-for.
var nativeArrayCases = []struct {
	name string
	src  string
	want string
}{
	{"index", "func main() {\n    let a = [10, 20, 30]\n    print(a[0])\n    print(a[2])\n}\n", "10\n30\n"},
	{"index_expr", "func main() {\n    let a = [5, 6, 7]\n    print(a[1 + 1])\n}\n", "7\n"},
	{"len", "func main() {\n    let a = [10, 20, 30]\n    print(len(a))\n}\n", "3\n"},
	{"forin", "func main() {\n    let a = [10, 20, 30]\n    for x in a {\n        print(x)\n    }\n}\n", "10\n20\n30\n"},
	{"forin_sum", "func main() {\n    let a = [1, 2, 3, 4]\n    let s = 0\n    for x in a {\n        s = s + x\n    }\n    print(s)\n}\n", "10\n"},
	{"forin_empty", "func main() {\n    let a = []\n    for x in a {\n        print(x)\n    }\n    print(len(a))\n}\n", "0\n"},
	// The loop guard is `len - i` compared with jle, and each loop owns its
	// own hidden index slot. This case fails if the guard is inverted (the
	// body never runs) or if the index slot is shared (the outer counter is
	// clobbered by the inner loop), so it is the regression pin for both.
	{"forin_nested", "func main() {\n    let a = [1, 2]\n    let b = [10, 20]\n    for x in a {\n        for y in b {\n            print(x * 10 + y)\n        }\n    }\n}\n", "10\n20\n11\n21\n"},
	{"forin_break", "func main() {\n    let a = [1, 2, 3, 4]\n    for x in a {\n        if (x == 3) {\n            break\n        }\n        print(x)\n    }\n}\n", "1\n2\n"},
	{"forin_continue", "func main() {\n    let a = [1, 2, 3, 4]\n    for x in a {\n        if (x == 2) {\n            continue\n        }\n        print(x)\n    }\n}\n", "1\n3\n4\n"},
	{"forin_in_while", "func main() {\n    let a = [1, 2]\n    let i = 0\n    while (i < 2) {\n        for x in a {\n            print(x + i)\n        }\n        i = i + 1\n    }\n}\n", "1\n2\n2\n3\n"},
	{"two_arrays", "func main() {\n    let a = [1, 2]\n    let b = [3, 4]\n    print(a[1] + b[0])\n    print(len(a) + len(b))\n}\n", "5\n4\n"},
}

// TestNativeArrayExec pins the Phase 150A array surface on the Linux ELF
// container. Execution runs on linux/amd64; elsewhere compileNative still
// proves parse + lowering + structural validity.
func TestNativeArrayExec(t *testing.T) {
	for _, c := range nativeArrayCases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			img := compileNative(t, c.src)
			out, code := runNativeCode(t, img)
			if out != "" {
				if out != c.want {
					t.Errorf("%s: output %q, want %q", c.name, out, c.want)
				}
				if code != 0 {
					t.Errorf("%s: exit %d, want 0 (out=%q)", c.name, code, out)
				}
			}
		})
	}
}

// TestNativeArrayExecPE runs the same array surface through the PE + Win64
// boundary + PEB bootstrap container, proving the array lowering is
// OS-neutral: frame layout, header stores, scaled element addressing and
// the for-in guard are emitted by one shared lowering per OS.
func TestNativeArrayExecPE(t *testing.T) {
	for _, c := range nativeArrayCases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			img := compileNativeOS(t, OSWindows, c.src)
			out, code := runNativeWindows(t, img)
			if out != "" {
				if out != c.want {
					t.Errorf("%s: output %q, want %q", c.name, out, c.want)
				}
				if code != 0 {
					t.Errorf("%s: exit %d, want 0 (out=%q)", c.name, code, out)
				}
			}
		})
	}
}

// TestNativeStrConcatExec executes Phase-150B string concatenation. These
// cases are the first consumers of the heap arena, so they are what proves
// the whole path: the writable data segment (ELF second PT_LOAD, PE inside
// the R/W .idata), the bump allocator, its link-time arena addresses, and
// the byte-wise copy. Run live on both containers where each can execute.
func TestNativeStrConcatExec(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want string
	}{
		{"literal_pair", "func main() {\n    print(\"ab\" + \"cd\")\n}\n", "abcd\n"},
		{"empty_operands", "func main() {\n    print(\"\" + \"x\")\n}\n", "x\n"},
		{"both_empty", "func main() {\n    print(\"\" + \"\")\n}\n", "\n"},
		{"chained", "func main() {\n    print(\"a\" + \"b\" + \"c\")\n}\n", "abc\n"},
		{"chained_grouped", "func main() {\n    print(\"a\" + (\"b\" + \"c\"))\n}\n", "abc\n"},
		{"assigned", "func main() {\n    let s = \"hello\" + \" \" + \"world\"\n    print(s)\n}\n", "hello world\n"},
		{"variable_operand", "func main() {\n    let a = \"foo\"\n    print(a + \"bar\")\n}\n", "foobar\n"},
		{"two_variables", "func main() {\n    let a = \"x\"\n    let b = \"y\"\n    let c = a + b\n    print(c)\n}\n", "xy\n"},
		{"in_branch", "func main() {\n    let i = 0\n    if (i == 0) {\n        print(\"a\" + \"0\")\n    } else {\n        print(\"b\" + \"1\")\n    }\n}\n", "a0\n"},
		{"call_arg", "func show(t string) {\n    print(t)\n}\nfunc main() {\n    show(\"na\" + \"me\")\n}\n", "name\n"},
		{"returned", "func join() {\n    return \"re\" + \"turn\"\n}\nfunc main() {\n    print(join())\n}\n", "return\n"},
		{"many_sites", "func main() {\n    print(\"1\" + \"2\")\n    print(\"3\" + \"4\")\n    print(\"5\" + \"6\")\n    print(\"7\" + \"8\")\n}\n", "12\n34\n56\n78\n"},
		{"long_operand", "func main() {\n    let a = \"abcdefghijklmnopqrstuvwxyz0123456789\"\n    print(a + a)\n}\n", "abcdefghijklmnopqrstuvwxyz0123456789abcdefghijklmnopqrstuvwxyz0123456789\n"},
		{"in_loop", "func main() {\n    let i = 0\n    while (i < 3) {\n        print(\"it\" + \"er\")\n        i = i + 1\n    }\n}\n", "iter\niter\niter\n"},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			img := compileNative(t, c.src)
			out, code := runNativeCode(t, img)
			if out != "" {
				if out != c.want {
					t.Errorf("elf %s: output %q, want %q", c.name, out, c.want)
				}
				if code != 0 {
					t.Errorf("elf %s: exit %d, want 0 (out=%q)", c.name, code, out)
				}
			}
		})
	}
}

// TestNativeStrConcatExecPE runs the concat surface on the PE container, where
// it executes for real on windows/amd64. The arena lives inside the R/W
// .idata there, so this is what proves the PE-specific placement, the
// bootstrap's IAT publication coexisting with arena addresses, and the
// DIR64 fixups for those addresses.
func TestNativeStrConcatExecPE(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want string
	}{
		{"literal_pair", "func main() {\n    print(\"ab\" + \"cd\")\n}\n", "abcd\n"},
		{"chained", "func main() {\n    print(\"a\" + \"b\" + \"c\")\n}\n", "abc\n"},
		{"assigned", "func main() {\n    let s = \"hello\" + \" \" + \"world\"\n    print(s)\n}\n", "hello world\n"},
		{"variable_operand", "func main() {\n    let a = \"foo\"\n    print(a + \"bar\")\n}\n", "foobar\n"},
		{"call_arg", "func show(t string) {\n    print(t)\n}\nfunc main() {\n    show(\"na\" + \"me\")\n}\n", "name\n"},
		{"many_sites", "func main() {\n    print(\"1\" + \"2\")\n    print(\"3\" + \"4\")\n}\n", "12\n34\n"},
		{"long_operand", "func main() {\n    let a = \"abcdefghijklmnopqrstuvwxyz0123456789\"\n    print(a + a)\n}\n", "abcdefghijklmnopqrstuvwxyz0123456789abcdefghijklmnopqrstuvwxyz0123456789\n"},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			img := compileNativeOS(t, OSWindows, c.src)
			out, code := runNativeWindows(t, img)
			if out != "" {
				if out != c.want {
					t.Errorf("pe %s: output %q, want %q", c.name, out, c.want)
				}
				if code != 0 {
					t.Errorf("pe %s: exit %d, want 0 (out=%q)", c.name, code, out)
				}
			}
		})
	}
}

// TestNativeStrViewExec covers the Phase-150B string operations that only
// read bytes and therefore need no heap: slicing (a view, not a copy) and
// equality/inequality by content. Equality runs its own length check first,
// so the cases below pin both the length-mismatch path and the byte path.
var nativeStrViewCases = []struct {
	name string
	src  string
	want string
}{
	// Slicing.
	{"slice_mid", "func main() {\n    print(\"hello world\"[6:11])\n}\n", "world\n"},
	{"slice_prefix", "func main() {\n    print(\"abcdef\"[0:3])\n}\n", "abc\n"},
	{"slice_empty", "func main() {\n    print(\"abcdef\"[2:2])\n}\n", "\n"},
	{"slice_var_bounds", "func main() {\n    let s = \"abcdefgh\"\n    let a = 2\n    let b = 5\n    print(s[a:b])\n}\n", "cde\n"},
	{"slice_of_concat", "func main() {\n    let s = \"ab\" + \"cdef\"\n    print(s[0:2])\n    print(s[2:6])\n}\n", "ab\ncdef\n"},
	{"slice_whole", "func main() {\n    print(\"xyz\"[0:3])\n}\n", "xyz\n"},
	// Equality.
	{"eq_same_literal", "func main() {\n    if (\"ab\" == \"ab\") {\n        print(1)\n    } else {\n        print(0)\n    }\n}\n", "1\n"},
	{"eq_diff_literal", "func main() {\n    if (\"ab\" == \"ac\") {\n        print(1)\n    } else {\n        print(0)\n    }\n}\n", "0\n"},
	{"eq_len_mismatch", "func main() {\n    if (\"ab\" == \"abc\") {\n        print(1)\n    } else {\n        print(0)\n    }\n}\n", "0\n"},
	{"ne_diff", "func main() {\n    if (\"ab\" != \"cd\") {\n        print(1)\n    } else {\n        print(0)\n    }\n}\n", "1\n"},
	{"ne_same", "func main() {\n    if (\"ab\" != \"ab\") {\n        print(1)\n    } else {\n        print(0)\n    }\n}\n", "0\n"},
	{"eq_both_empty", "func main() {\n    if (\"\" == \"\") {\n        print(1)\n    } else {\n        print(0)\n    }\n}\n", "1\n"},
	// Combined: a slice compared against a literal decides a branch.
	{"slice_eq_gates", "func main() {\n    let s = \"prefix-body-suffix\"\n    if (s[7:11] == \"body\") {\n        print(1)\n    } else {\n        print(0)\n    }\n}\n", "1\n"},
	{"slice_in_while", "func main() {\n    let s = \"abcdef\"\n    let i = 0\n    while (i < 3) {\n        print(s[i:2])\n        i = i + 1\n    }\n}\n", "ab\nbc\ncd\n"},
	{"eq_in_while", "func main() {\n    let s = \"ab\"\n    let i = 0\n    while (i < 2) {\n        if (s == \"ab\") {\n            print(7)\n        }\n        i = i + 1\n    }\n}\n", "7\n7\n"},
}

// TestNativeStrViewExecPE runs the string-view surface on the PE container,
// where slicing and comparison execute for real on windows/amd64.
func TestNativeStrViewExecPE(t *testing.T) {
	for _, c := range nativeStrViewCases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			img := compileNativeOS(t, OSWindows, c.src)
			out, code := runNativeWindows(t, img)
			if out != "" {
				if out != c.want {
					t.Errorf("%s: output %q, want %q", c.name, out, c.want)
				}
				if code != 0 {
					t.Errorf("%s: exit %d, want 0 (out=%q)", c.name, code, out)
				}
			}
		})
	}
}

// TestNativeStrViewExec mirrors the same surface on the Linux ELF container.
func TestNativeStrViewExec(t *testing.T) {
	for _, c := range nativeStrViewCases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			img := compileNative(t, c.src)
			out, code := runNativeCode(t, img)
			if out != "" {
				if out != c.want {
					t.Errorf("%s: output %q, want %q", c.name, out, c.want)
				}
				if code != 0 {
					t.Errorf("%s: exit %d, want 0 (out=%q)", c.name, code, out)
				}
			}
		})
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

// nativeLegacyCorpus is the pre-150A program corpus: the Phase-145
// straight-line/strings programs, the Phase-147 bisect cases, the Phase-148
// control-flow and calls-ABI cases, and the Phase-149 shared programs. None
// of them use a 150A value kind (float/array), so the Phase-150A value
// model must not move a single byte of their ELF images.
var nativeLegacyCorpus = []struct {
	name string
	src  string
}{
	{"p145_hello", "func main() {\n    print(\"hello native\")\n    print(2 + 3 * 4)\n    print((10 - 4) * 2)\n}\n"},
	{"p145_int42", "func main() {\n    print(42)\n}\n"},
	{"p145_str", "func main() {\n    print(\"hi\")\n}\n"},
	{"p145_arith", "func main() {\n    print(2 + 3 * 4)\n}\n"},
	{"p145_call", "func add(a, b) {\n    return a + b\n}\nfunc main() {\n    print(add(20, 22))\n}\n"},
	{"p145_mult", "func main() {\n    print(1000000 * 1000000)\n}\n"},
	{"p147_empty", "func main() {\n}\n"},
	{"p147_retcode", "func main() {\n    return 7\n}\n"},
	{"p148_if_else", "func classify(n) {\n    if (n < 0) {\n        return 0 - 1\n    }\n    if (n == 0) {\n        return 0\n    }\n    return 1\n}\nfunc main() {\n    print(classify(0 - 5))\n    print(classify(0))\n    print(classify(9))\n}\n"},
	{"p148_while_sum", "func main() {\n    let s = 0\n    let i = 1\n    while (i <= 10) {\n        s = s + i\n        i = i + 1\n    }\n    print(s)\n}\n"},
	{"p148_break_continue", "func main() {\n    let s = 0\n    let i = 0\n    for (; i < 10; i = i + 1) {\n        if (i == 3) {\n            continue\n        }\n        if (i == 7) {\n            break\n        }\n        s = s + i\n    }\n    print(s)\n}\n"},
	{"p148_nested", "func main() {\n    let t = 0\n    let i = 0\n    while (i < 3) {\n        let j = 0\n        while (j < 3) {\n            if (i == j) {\n                t = t + 1\n            }\n            j = j + 1\n        }\n        i = i + 1\n    }\n    print(t)\n}\n"},
	{"p148_string_arg", "func greet(name string) {\n    print(name)\n}\nfunc main() {\n    greet(\"hi\")\n}\n"},
	{"p148_string_var_arg", "func greet(name string) {\n    print(name)\n}\nfunc main() {\n    let w = \"yo\"\n    greet(w)\n}\n"},
	{"p148_string_return", "func word() {\n    return \"abc\"\n}\nfunc main() {\n    print(word())\n}\n"},
	{"p148_mixed_args", "func show(a int, s string, b int) {\n    print(a)\n    print(s)\n    print(b)\n}\nfunc main() {\n    show(1, \"two\", 3)\n}\n"},
	{"p148_seven_params", "func sum7(a, b, c, d, e, f, g) {\n    return a + b + c + d + e + f + g\n}\nfunc main() {\n    print(sum7(1, 2, 3, 4, 5, 6, 7))\n}\n"},
	{"p148_straddle", "func mix(a, b, c, d, e, s string) {\n    print(a)\n    print(s)\n}\nfunc main() {\n    mix(1, 2, 3, 4, 5, \"six\")\n}\n"},
	{"p148_nested_strcall", "func id(s string) {\n    return s\n}\nfunc main() {\n    print(id(id(\"ok\")))\n}\n"},
}

// nativeLegacyELF pins the SHA-256 of each pre-150A ELF image. The Phase-150A
// value model added kinds and lowerings for values these programs never use,
// so any byte here is drift: it means the int/string path changed shape
// (frame layout, helper emission, or the entry tail). Increment 150C
// (register allocation) validates against this same table, which is what
// makes "zero golden drift" a measurement instead of an assertion.
//
// The values are the increment-149 (951ee10) images, measured — not
// regenerated after the fact. That is what proves the differential: an
// earlier 150A state emitted print_float unconditionally and grew every one
// of these images (hello 503 -> 1226 bytes) until the helper was gated on
// scanFloatUsage.
var nativeLegacyELF = map[string]string{
	"p145_hello":          "c73a6e38d6571dc7a19420fb8dae60294cebda9e74b41a90a0b9cc22e5769fb3",
	"p145_int42":          "865c5846912a192aa90dd364d00a02b7d1c510e5368d0d4a3c978aa20733e23e",
	"p145_str":            "6ae64efc3edcb56a5a7b6c11e6283f1a8eea7bf2603a951f63d8fb2d53ea0f75",
	"p145_arith":          "2c314d9b7af8b5d47742f6947e60327eb475671d0c9f14cf7da5740ea3dc1b06",
	"p145_call":           "2f925e86ea1c2d3877304587549ce2c28cfd912f19ed451e4d02b30cf86813ff",
	"p145_mult":           "f2035ee4a5da40b4b6e5025340a64a495f2b1455fd5e41215f4903e0c3895c34",
	"p147_empty":          "30263ddf22f56b65a499e725b47f7e9ce641eb219fea156e32de4cf8dddf4303",
	"p147_retcode":        "853afe5070f705d4c4a9f1358fa124504bef39235cb753a87378dd0dc2c45aa6",
	"p148_if_else":        "2020a957bf7c05117f93c40ee78510c736a2e576578bad5653a937c075010215",
	"p148_while_sum":      "a8e6cbbc3bebbc22ca41351d802f28fb928b41fb30155b0c499b427d8879d001",
	"p148_break_continue": "bb138f6e3309d39e74a04a2d2158b5b071048c330b10041daa3dae0cd3ecac98",
	"p148_nested":         "6e49dc9ced20348912da1a991223f7b59905d716fec5b098b56951475b1a0043",
	"p148_string_arg":     "41dd9df614fe60dfee01b0235b4a74b0ce0c573e741e75429f5fa20c3aa09ec0",
	"p148_string_var_arg": "2425a5f7c600621ec8188cc4ac945b62d8d37f47140d2eb62d4d3d7a3cd68c20",
	"p148_string_return":  "72a22569763cacef040b242d4b68753d1d20586b3be9742f7654598de57e8112",
	"p148_mixed_args":     "31588945b8168a988042e979f65362a297ed2f8fd1cdc3a5876e68e86901478e",
	"p148_seven_params":   "ff2c76ae18bb7703e054e6e88079d8caaebdc2726b98cceac2c1131d625de9dc",
	"p148_straddle":       "149b2e1148acae6db1895aebd43830d74ee0d4f8a9bf742eff94d610e7a14f82",
	"p148_nested_strcall": "51c9f9ae3e1963daa739147deb1e892407ea6e0dc0fe7b031a099413fdc169f9",
}

// TestNativeELFByteIdentity is the Phase-150A ELF byte-identity differential
// against the 147/148/149 corpus. It also re-compiles each program twice to
// keep the determinism property pinned alongside the hash.
func TestNativeELFByteIdentity(t *testing.T) {
	for _, c := range nativeLegacyCorpus {
		c := c
		t.Run(c.name, func(t *testing.T) {
			img, err := CompileProgram(parseNative(t, c.src))
			if err != nil {
				t.Fatalf("compile: %v", err)
			}
			again, err := CompileProgram(parseNative(t, c.src))
			if err != nil {
				t.Fatalf("compile: %v", err)
			}
			if string(img) != string(again) {
				t.Fatal("non-deterministic ELF image")
			}
			sum := sha256.Sum256(img)
			got := hex.EncodeToString(sum[:])
			want, ok := nativeLegacyELF[c.name]
			if !ok {
				t.Fatalf("unpinned corpus program %q: sha256 %s (%d bytes)", c.name, got, len(img))
			}
			if got != want {
				t.Errorf("%s: ELF sha256 %s, want %s (%d bytes) — byte drift in the pre-150A corpus", c.name, got, want, len(img))
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

// TestNativePE pins the Phase-149 Windows target: the same goldens as
// the Linux suite, executed where they can run (windows/amd64 — the dev
// host proves them locally as well as Windows CI) and structurally
// validated elsewhere. kernel32 imports + Win64 boundary sequences are
// what these exercise beyond the shared lowering.
func TestNativePE(t *testing.T) {
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
		{"add", "func add(a, b) {\n    return a + b\n}\nfunc main() {\n    print(add(20, 22))\n}\n", "42\n", 0},
		{"while_sum", "func main() {\n    let s = 0\n    let i = 1\n    while (i <= 10) {\n        s = s + i\n        i = i + 1\n    }\n    print(s)\n}\n", "55\n", 0},
		{"string_arg", "func greet(name string) {\n    print(name)\n}\nfunc main() {\n    greet(\"yo\")\n}\n", "yo\n", 0},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			img := compileNativeOS(t, OSWindows, c.src)
			out, code := runNativeWindows(t, img)
			if out != "" && out != c.want {
				t.Errorf("%s: output %q, want %q", c.name, out, c.want)
			}
			if code != c.wantCode {
				t.Errorf("%s: exit %d, want %d (out=%q)", c.name, code, c.wantCode, out)
			}
		})
	}
}

// TestNativeMachO pins the Phase-149 macOS target structurally: magic,
// cputype, EXECUTE type, four load commands with LC_MAIN inside the
// file. No Intel-mac runner exists, so execution honestly skips
// (runNativeMacOS); the first run on a real Mac validates the
// dyld-info shape beyond these pins.
func TestNativeMachO(t *testing.T) {
	srcs := []string{
		"func main() {\n}\n",
		"func main() {\n    print(40 + 2)\n}\n",
		"func add(a, b) {\n    return a + b\n}\nfunc main() {\n    print(add(20, 22))\n}\n",
	}
	for i, src := range srcs {
		img := compileNativeOS(t, OSMacOS, src)
		entry, textOff, err := ParseMachO(img)
		if err != nil {
			t.Fatalf("case %d: %v", i, err)
		}
		if entry < MachoBase+uint64(textOff) || entry >= MachoBase+uint64(len(img)) {
			t.Fatalf("case %d: entry %#x outside image", i, entry)
		}
		a, err := CompileProgramForOS(parseNative(t, src), OSMacOS)
		if err != nil {
			t.Fatal(err)
		}
		if string(a) != string(img) {
			t.Fatalf("case %d: non-deterministic mach-o image", i)
		}
	}
}
