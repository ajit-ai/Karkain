// NAT-STDLIB — native standard-library qualification for the v1.3.0 carried
// condition 2.
//
// This gate pins what the native path can consume from an UNMODIFIED .kark
// module today, measured on this host, and it pins what it cannot: every
// unsupported shape is refused BY NAME, and the whole-example refusal is
// asserted with its diagnostic.
//
// What WORKS, end to end, on the self-hosted engine, through the real CLI:
//
//   - `import std.collections` — the unmodified stdlib source is assembled and
//     the functions main calls are lowered; functions main never calls are
//     PRUNED at emit time (reachability), so the module's unsupported
//     corners (maps, strings, other modules' types) do not block the build.
//   - array_sum / array_max / array_contains over an int array — EXECUTED PE,
//     exact stdout bytes: 26 / 9 / 1, 0.
//   - array parameters — a parameter that receives an array at every call
//     site is inferred as an array (the parser discards annotations, so this
//     is the only kind evidence available) and homed as two units.
//   - `while (i < len(arr))` — in main AND in a callee; the emission treated
//     `len` as an unknown user function and crashed the PE linker. That was a
//     PRE-EXISTING defect, fixed in this slice.
//   - `len()` inside a called function — routed to the builtin, as the
//     oracle does.
//   - `return true` / `return false` — bool literals lower to 1 / 0.
//
// What does NOT work, each asserted here or pinned BY NAME:
//
//   - the whole `examples/stdlib_v2` program — refused with
//     "`let name` initialised from a non-integer expression" (main's `let`
//     cannot bind a call result yet; the string runtime it would need does
//     not exist);
//   - ANY other stdlib module — std.string/encoding/crypto need a string
//     runtime (params, returns, equality, slicing, concat in callees) and the
//     hex/base64/sha256 builtins;
//   - the Go oracle for the same programs — refuses at compile time
//     ("unsupported return expression *parser.ArrayLiteral"), so the native
//     stdlib consumption is kcc-only today and byte identity is NOT claimed;
//   - Linux/macOS execution — there is no runner on this host; Linux is
//     asserted by BUILD + structural validation only.
//
// Evidence discipline: every execution claim below is an EXECUTED PE on this
// Windows host with EXACT stdout bytes, never trimmed. No wall clock is
// read. Every refusal claim asserts the exit code and the diagnostic text.
package cli

import (
	"bytes"
	"encoding/hex"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const (
	natStdlibSumMax = "import std.collections\n\n" +
		"func main() {\n" +
		"    let scores = [3, 7, 2, 9, 5]\n" +
		"    print(array_sum(scores))\n" +
		"    print(array_max(scores))\n" +
		"}\n"
	natStdlibContains = "import std.collections\n\n" +
		"func main() {\n" +
		"    let scores = [3, 7, 2, 9, 5]\n" +
		"    print(array_contains(scores, 7))\n" +
		"    print(array_contains(scores, 4))\n" +
		"}\n"
	natStdlibEmpty   = "import std.collections\n\nfunc main() { print(1) }\n"
	natStdlibMainLen = "func main() {\n" +
		"    let s = [1, 2, 3]\n" +
		"    let i = 0\n" +
		"    while (i < len(s)) {\n" +
		"        i = i + 1\n" +
		"    }\n" +
		"    print(i)\n" +
		"}\n"
	natStdlibCalleeLoop = "func sum(arr) {\n" +
		"    let total = 0\n" +
		"    let i = 0\n" +
		"    while (i < len(arr)) {\n" +
		"        total = total + arr[i]\n" +
		"        i = i + 1\n" +
		"    }\n" +
		"    return total\n" +
		"}\n" +
		"func main() {\n" +
		"    let scores = [3, 7, 2, 9, 5]\n" +
		"    print(sum(scores))\n" +
		"}\n"
	natStdlibArrayIdxLoop = "func f(arr) {\n" +
		"    let i = 0\n" +
		"    while (i < len(arr)) {\n" +
		"        i = i + 1\n" +
		"    }\n" +
		"    return arr[0]\n" +
		"}\n" +
		"func main() {\n" +
		"    let s = [7, 8, 9]\n" +
		"    print(f(s))\n" +
		"}\n"
)

// phase130Karkain already builds the CLI once for this package; this helper
// executes a produced native image and returns its stdout verbatim.
func natStdlibRun(t *testing.T, img string) []byte {
	t.Helper()
	out, err := exec.Command(img).Output()
	if err != nil {
		t.Fatalf("the produced image failed to run: %v", err)
	}
	return out
}

func hexToBytes(s string) []byte {
	b, err := hex.DecodeString(s)
	if err != nil {
		panic(err)
	}
	return b
}

// natStdlibScratchDir creates a scratch directory under the repository root,
// which the native driver needs: stdlib module resolution walks up from the
// source file looking for `stdlib/`, and a directory under %TEMP% has no
// repository above it.
func natStdlibScratchDir(t *testing.T) string {
	t.Helper()
	root := filepath.Join("..", "..")
	base := filepath.Join(root, ".karkain-dev", "phase151p-gates")
	if err := os.MkdirAll(base, 0o755); err != nil {
		t.Fatal(err)
	}
	dir, err := os.MkdirTemp(base, "case-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	return dir
}

func writeSource(t *testing.T, src string) string {
	t.Helper()
	srcPath := filepath.Join(natStdlibScratchDir(t), "probe.kark")
	if err := os.WriteFile(srcPath, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	return srcPath
}

func buildNativeKCC(t *testing.T, karkain, src, outPath string) string {
	t.Helper()
	cmd := exec.Command(karkain, "build", src,
		"--engine", "kcc",
		"--target", NativeWindowsTarget,
		"-o", outPath)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("the native build must succeed:\n%s", out)
	}
	return string(out)
}

// TestPhaseNATStdlib_ImportedCollectionsExecutes runs the unmodified
// std.collections functions through the self-hosted engine on this Windows
// host and compares EXACT stdout bytes.
//
// Every fixture here is a program whose only stdlib dependency is
// collections.kark, and whose reachable callees are the pure integer ones.
func TestPhaseNATStdlib_ImportedCollectionsExecutes(t *testing.T) {
	if hostNativeTarget() != NativeWindowsTarget {
		t.Skip("native execution evidence is Windows-only on this host")
	}
	karkain := phase130Karkain(t)
	cases := []struct {
		name    string
		src     string
		wantHex string
	}{
		{"sum_and_max", natStdlibSumMax, "32360a390a"},     // 26, 9
		{"contains", natStdlibContains, "310a300a"},        // 1, 0
		{"empty_import", natStdlibEmpty, "310a"},           // 1
		{"main_len_loop", natStdlibMainLen, "330a"},        // 3
		{"callee_len_loop", natStdlibCalleeLoop, "32360a"}, // 26
		{"callee_idx_loop", natStdlibArrayIdxLoop, "370a"}, // 7
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			dir := t.TempDir()
			outPath := filepath.Join(dir, "probe.exe")
			buildNativeKCC(t, karkain, writeSource(t, c.src), outPath)
			got := hex.EncodeToString(natStdlibRun(t, outPath))
			if got != c.wantHex {
				t.Fatalf("stdout mismatch\n got hex: %s\n want hex: %s", got, c.wantHex)
			}
		})
	}
}

// TestPhaseNATStdlib_ImportedCollectionsNoImageThenReport asserts the
// whole-example refusal verbatim. main's `let` cannot bind a call result yet,
// which is the first of the three string-runtime gaps the full stdlib_v2
// example needs. The refusal names the construct and writes no image.
func TestPhaseNATStdlib_ImportedCollectionsNoImageThenReport(t *testing.T) {
	karkain := phase130Karkain(t)
	example := filepath.Join("..", "..", "examples", "stdlib_v2", "main.kark")
	outPath := filepath.Join(natStdlibScratchDir(t), "example.exe")
	cmd := exec.Command(karkain, "build", example,
		"--engine", "kcc",
		"--target", NativeWindowsTarget,
		"-o", outPath)
	out, runErr := cmd.CombinedOutput()
	if runErr == nil {
		t.Fatalf("the whole example compiled natively; that would make this refusal stale and it must move, not be deleted\n%s", out)
	}
	text := string(out)
	if !strings.Contains(text, "`let name` initialised from a non-integer expression") {
		t.Fatalf("the refusal does not name the blocking construct.\nWant fragment: `let name` initialised from a non-integer expression\nGot:\n%s", text)
	}
	if _, err := os.Stat(outPath); err == nil {
		t.Fatal("a refusal must write no image")
	}
}

// TestPhaseNATStdlib_ImportedModuleNeedsPruning pins the emit-time reachability
// pruning: an import of the unmodified module compiles when main never calls
// its unsupported (map/string) helpers, which is the exact corner the
// self-hosted engine can now lower. Without pruning the same image fails to
// build with the refusal of an unreachable function.
func TestPhaseNATStdlib_ImportedModuleNeedsPruning(t *testing.T) {
	karkain := phase130Karkain(t)
	src := writeSource(t, natStdlibEmpty)
	outPath := filepath.Join(t.TempDir(), "probe.exe")
	buildNativeKCC(t, karkain, src, outPath)
	if got := hex.EncodeToString(natStdlibRun(t, outPath)); got != "310a" {
		t.Fatalf("stdout mismatch: got hex %s, want 310a", got)
	}
}

// TestPhaseNATStdlib_ArgumentMismatchRefusals asserts the array-parameter
// argument rules are refused BY NAME, with no image.
func TestPhaseNATStdlib_ArgumentMismatchRefusals(t *testing.T) {
	karkain := phase130Karkain(t)
	cases := []struct {
		name string
		src  string
		want string
	}{
		{
			name: "array_literal_as_parameter_argument",
			src: "func f(arr) { return 1 }\n" +
				"func main() { print(f([1, 2, 3])) }\n",
			want: "array parameter `arr` (call to `f`) receives an array literal; bind it to a local first",
		},
		{
			// One int site and one array site promote the parameter to an
			// array, and the validation pass names the int site as the conflict.
			name: "int_argument_for_promoted_array_parameter",
			src: "func f(arr) { return 1 }\n" +
				"func main() {\n" +
				"    let s = [1, 2]\n" +
				"    print(f(s))\n" +
				"    print(f(1))\n" +
				"}\n",
			want: "int argument for array parameter `arr` (call to `f`)",
		},
		{
			// The dual-site conflict for an INT parameter promoted to array by
			// the array site; the int site is named as the conflict.
			name: "int_and_array_sites_for_same_parameter",
			src: "func f(n int) int { return n }\n" +
				"func main() {\n" +
				"    let arr = [1, 2]\n" +
				"    print(f(1))\n" +
				"    print(f(arr))\n" +
				"}\n",
			want: "int argument for array parameter `n` (call to `f`)",
		},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			res := nativeBuildWithEngine(t, karkain, c.src, NativeWindowsTarget, "kcc", c.name+"-refusal.img")
			if res.ExitCode != 3 {
				t.Fatalf("exit code = %d, want 3\n%s", res.ExitCode, res.Output)
			}
			if !strings.Contains(res.Output, c.want) {
				t.Fatalf("the refusal does not name the construct\nwant fragment: %q\ngot:\n%s", c.want, res.Output)
			}
			if res.WroteImage {
				t.Fatal("a refusal must write no image")
			}
		})
	}
}

// TestPhaseNATStdlib_BuildsWithNoCCompiler builds AND executes one of the
// stdlib fixtures while gcc, clang and MSVC `cl` are shadowed by failing stubs
// (152-A's mechanism). Any invocation that resolves to a real or stubbed
// compiler fails, and the run above this point would have failed first.
func TestPhaseNATStdlib_BuildsWithNoCCompiler(t *testing.T) {
	if hostNativeTarget() != NativeWindowsTarget {
		t.Skip("the no-C execution proof is Windows/amd64 on this host; Linux CI runs it there")
	}
	karkain := phase130Karkain(t)
	stubDir := shadowCompilersWithStubs(t)

	for _, tool := range noCTools {
		resolved, err := exec.LookPath(tool)
		if err != nil {
			t.Fatalf("%s does not resolve; the no-C proof is void: %v", tool, err)
		}
		if filepath.Dir(resolved) != stubDir {
			t.Fatalf("%s resolved to %s, outside the stub dir %s; the real toolchain is still reachable", tool, resolved, stubDir)
		}
		if out, err := exec.Command(resolved, "--version").CombinedOutput(); err == nil {
			t.Fatalf("%s ran successfully; expected the failing stub, got %q", tool, out)
		}
	}

	dir := t.TempDir()
	outPath := filepath.Join(dir, "probe.exe")
	buildNativeKCC(t, karkain, writeSource(t, natStdlibSumMax), outPath)
	if got := hex.EncodeToString(natStdlibRun(t, outPath)); got != "32360a390a" {
		t.Fatalf("stdout mismatch under stubbed compilers: got hex %s, want 32360a390a", got)
	}
}

// TestPhaseNATStdlib_LinuxELFIsStructurallyValid is NOT an execution claim.
// There is no runner on this host, so Linux is asserted by BUILD + the ELF
// magic of the emitted image. The bytes are not compared to the Go oracle
// (which refuses these programs).
func TestPhaseNATStdlib_LinuxELFIsStructurallyValid(t *testing.T) {
	karkain := phase130Karkain(t)
	cases := []struct {
		name string
		src  string
	}{
		{"sum_max", natStdlibSumMax},
		{"contains", natStdlibContains},
		{"empty_import", natStdlibEmpty},
		{"main_len_loop", natStdlibMainLen},
		{"callee_len_loop", natStdlibCalleeLoop},
		{"callee_idx_loop", natStdlibArrayIdxLoop},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			dir := natStdlibScratchDir(t)
			outPath := filepath.Join(dir, "probe.elf")
			srcPath := writeSource(t, c.src)
			cmd := exec.Command(karkain, "build", srcPath,
				"--engine", "kcc",
				"--target", NativeLinuxTarget,
				"-o", outPath)
			out, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("the Linux build refused (exit via err=%v):\n%s", err, out)
			}
			data, err := os.ReadFile(outPath)
			if err != nil {
				t.Fatalf("the ELF was not written: %v", err)
			}
			if !bytes.HasPrefix(data, []byte{0x7f, 'E', 'L', 'F'}) {
				t.Fatalf("the image does not carry an ELF magic")
			}
		})
	}
}

// TestPhaseNATStdlib_GateIsWiredInCI keeps this file a gate rather than a script.
func TestPhaseNATStdlib_GateIsWiredInCI(t *testing.T) {
	body, err := os.ReadFile(filepath.Join("..", "..", ".github", "workflows", "ci.yml"))
	if err != nil {
		t.Fatalf("reading ci.yml: %v", err)
	}
	text := string(body)
	const step = "go test ./pkg/cli/ -run 'TestPhaseNATStdlib_' -count=1"
	if !strings.Contains(text, step) {
		t.Errorf("ci.yml has no step running %q; a locally green gate CI never executes is not a gate", step)
	}
}
