// NAT-CALL qualification addendum — the measurements the v1.3.0 "NAT-CALL"
// milestone needs before it can be called complete, layered on top of the
// committed gate phase151e_call_test.go (which stays as the frozen record of
// the slice that LANDED the call lowering).
//
// The committed gate proved that a real .kark function called from main
// builds and EXECUTES through the C-free native path, and it pins byte
// identity against the Go oracle, the frame rule, the Windows alignment and
// a by-name refusal table. What it does not cover is the half of the
// engineering brief this increment closes:
//
//   * the ANSWERING half of the contract a user actually meets — diagnostics
//     for an undefined function, a wrong argument count, a wrong argument
//     type, and a refused return shape — each asserted on the real CLI with
//     the exit code AND the diagnostic, not just "an error happened";
//   * explicit, executed tests for the two capability questions the brief
//     asks about (forward references and recursion), which work and are
//     pinned here so they cannot regress silently;
//   * the argument evaluation order, pinned as an executed golden;
//   * locals live across a call — ten of them — proving no register or slot
//     is corrupted across a call boundary;
//   * an honest separation of what WORKS (executed) from what is REFUSED
//     (by name) and of what is claimed for Linux (byte identity only — there
//     is no Linux runner on the development host and CI is billing-blocked),
//     and what is claimed for Windows (execution);
//   * a wiring check so this file is a gate, not a script.
//
// Evidence discipline: every execution claim below is an executed PE on the
// windows/amd64 development host, with EXACT stdout bytes (never trimmed).
// No Linux ELF in this file is run; Linux is claimed via byte identity
// against the real Go oracle only. Every refusal claim asserts exit code 3,
// the diagnostic code and no image written. Nothing here reads a wall clock.
package cli

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// natCallQualCase is one executed program in the qualification corpus. Every
// wantHex value below was measured from the committed driver on this host,
// never derived by hand.
type natCallQualCase struct {
	name    string
	src     string
	wantHex string
	// kccOnly marks a shape the self-hosted driver lowers but the Go oracle
	// refuses (measured: "cannot determine return kind of recursive function"
	// and "int argument for float parameter"). Those two are NOT claimed to
	// be byte-identical, because the oracle emits nothing to compare. They
	// stay in the execution corpus and are pinned as divergences instead.
	kccOnly bool
}

var natCallQualCorpus = []natCallQualCase{
	{
		// The two capability questions the brief asks about: a function
		// declared AFTER main (forward reference) that is called from main.
		name:    "forward_reference",
		src:     "func main() { print(later(1)) }\nfunc later(n int) int { return n + 1 }\n",
		wantHex: "320a",
	},
	{
		// Recursion: the callee body contains a call to itself. NAT-CALL
		// lowers it without inlining (inlining would need a cycle walk).
		name:    "recursion",
		src:     "func fact(n int) int { if (n <= 1) { return 1 } return n * fact(n - 1) }\nfunc main() { print(fact(5)) }\n",
		wantHex: "3132300a",
		kccOnly: true,
	},
	{
		// Argument evaluation order is left to right. This program has two
		// side-effecting callables in argument position; the observable
		// output is only satisfiable if the left operand is evaluated first.
		name: "argument_evaluation_order",
		src: "func i() int { print(100) return 1 }\n" +
			"func j() int { print(200) return 2 }\n" +
			"func main() { print(i() + j()) }\n",
		wantHex: "3130300a3230300a330a",
	},
	{
		// Ten int locals live across a call. The locals are the callee-saved
		// register pressure case; a broken preservation discipline corrupts
		// one of them silently.
		name: "locals_live_across_call",
		src: "func inc(n int) int { return n + 1 }\n" +
			"func main() {\n" +
			"    let a = 1\n    let b = 2\n    let c = 3\n    let d = 4\n    let e = 5\n" +
			"    let f = 6\n    let g = 7\n    let h = 8\n    let i = 9\n    let j = 10\n" +
			"    print(a + b + c + d + e + f + g + h + i + j + inc(0))\n}\n",
		wantHex: "35360a", // 1+2+...+10 = 55, +inc(0)=1 -> 56 = "56\n"
	},
	{
		// A callee that binds its own local AND calls another function that
		// prints: confirms frames of consecutive calls do not alias.
		name:    "callee_local_from_call",
		src:     "func g() int { return 7 }\nfunc f() int { let m = g() return m }\nfunc main() { print(f()) }\n",
		wantHex: "370a",
	},
	{
		// A call in arithmetic position: the call is an integer expression,
		// not merely a statement.
		name:    "call_in_arithmetic",
		src:     "func f() int { return 20 }\nfunc main() { print(f() + 2) }\n",
		wantHex: "32320a",
	},
	{
		// Lowercase user function names with digits: symbol resolution goes
		// through the same rule as any other identifier.
		name:    "simple_name_with_digits",
		src:     "func add1(a int) int { return a + 1 }\nfunc main() { print(add1(41)) }\n",
		wantHex: "34320a",
	},
	{
		// An int literal passed where the parameter is a float executes
		// today (the argument is staged as an int unit). Pinned so any
		// future change is loud; NOT an endorsement of mixed int/float
		// typing, which the language does not have.
		name:    "int_argument_to_float_parameter",
		src:     "func f(x float) int { return 3 }\nfunc main() { print(f(1)) }\n",
		wantHex: "330a",
		kccOnly: true,
	},
	{
		// A call inside a loop: the for-free and while-backed lowering both
		// re-enter EmitCall once per iteration.
		name:    "call_in_while_loop",
		src:     "func dbl(n int) int { return n * 2 }\nfunc main() {\n    let i = 0\n    while (i < 4) { print(dbl(i)) i = i + 1 }\n}\n",
		wantHex: "300a320a340a360a",
	},
}

// TestPhaseNATCallQual_ExecutedGoldens executes every corpus program as a PE
// on this windows/amd64 host and compares EXACT stdout bytes.
//
// Skips on other platforms: there is no PE execution elsewhere, and a skip,
// not a pass, is the honest outcome.
func TestPhaseNATCallQual_ExecutedGoldens(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("native PE execution is a Windows-only evidence; this host is windows/amd64")
	}
	karkain := phase130Karkain(t)
	for _, c := range natCallQualCorpus {
		c := c
		t.Run(c.name, func(t *testing.T) {
			dir := t.TempDir()
			srcPath := filepath.Join(dir, "probe.kark")
			outPath := filepath.Join(dir, "probe.exe")
			if err := os.WriteFile(srcPath, []byte(c.src), 0o644); err != nil {
				t.Fatal(err)
			}
			out, err := exec.Command(karkain, "build", srcPath, "--target", NativeWindowsTarget, "-o", outPath).CombinedOutput()
			if err != nil {
				t.Fatalf("build refused a program the qualification corpus expects to compile:\n%s", out)
			}
			stdout := runAndCapture(t, outPath)
			got := hexOf(stdout)
			if got != c.wantHex {
				t.Fatalf("stdout mismatch for %s\n got hex: %s\n want hex: %s", c.name, got, c.wantHex)
			}
		})
	}
}

// TestPhaseNATCallQual_LinuxELFByteIdentity asserts that for the covered
// shapes the kcc-emitted Linux ELF is byte-identical to the one the Go
// oracle produces. This is a DIFFERENTIAL, never a golden: the expected bytes
// come from pkg/native at test time, so neither engine can be pinned as the
// truth. No ELF is executed here.
func TestPhaseNATCallQual_LinuxELFByteIdentity(t *testing.T) {
	karkain := phase130Karkain(t)
	for _, c := range natCallQualCorpus {
		c := c
		if c.kccOnly {
			t.Run(c.name+"_kcc_only", func(t *testing.T) {
				t.Skip("kcc-only shape; the Go oracle refuses it (pinned in TestPhaseNATCallQual_KccAcceptsWhatTheGoOracleRefuses)")
			})
			continue
		}
		t.Run(c.name, func(t *testing.T) {
			kccRes := nativeBuildWithEngine(t, karkain, c.src, NativeLinuxTarget, "kcc", c.name+"-kcc.elf")
			goRes := nativeBuildWithEngine(t, karkain, c.src, NativeLinuxTarget, "go", c.name+"-go.elf")
			if kccRes.ExitCode != 0 || goRes.ExitCode != 0 || !kccRes.WroteImage || !goRes.WroteImage {
				t.Fatalf("both engines must accept %s (kcc exit %d, go exit %d)\n  kcc:\n%s\n  go:\n%s",
					c.name, kccRes.ExitCode, goRes.ExitCode, kccRes.Output, goRes.Output)
			}
			if !strings.Contains(kccRes.Output, kccNativeProvenance) {
				t.Fatalf("the kcc build did not carry %q; provenance cannot be established:\n%s",
					kccNativeProvenance, kccRes.Output)
			}
			if !bytes.Equal(kccRes.Image, goRes.Image) {
				t.Fatalf("kcc and the Go oracle DISAGREE on %s\n  kcc: %d bytes\n  go:  %d bytes",
					c.name, len(kccRes.Image), len(goRes.Image))
			}
		})
	}
}

// natCallQualRefusal is one refused program: exit code, diagnostic code and
// fragment, and NO image.
type natCallQualRefusal struct {
	name    string
	src     string
	code    string
	want    string
}

func natCallQualRefusals() []natCallQualRefusal {
	return []natCallQualRefusal{
		{
			// The checker rejects the program before the driver sees it;
			// native lowering does not need to.
			name: "undefined_function",
			src:  "func main() { print(nosuch(1)) }\n",
			code: "error[K101]", want: "undefined function 'nosuch'",
		},
		{
			name: "wrong_argument_count",
			src:  "func f(n int) int { return n }\nfunc main() { print(f(1, 2)) }\n",
			code: "error[K103]", want: "function 'f' expects 1 arguments but got 2",
		},
		{
			// A callee that miscounts its own caller's arguments is rejected
			// the same way, so the diagnostic is engine-local, not
			// main-local.
			name: "wrong_argument_count_inside_callee",
			src: "func u(n int) int { return n }\n" +
				"func g() int { return u(1, 2) }\n" +
				"func main() { print(g()) }\n",
			code: "error[K103]", want: "function 'u' expects 1 arguments but got 2",
		},
		{
			// Every parameter is an int to the native driver (the parser
			// discards annotations), so a non-int argument is refused AT THE
			// CALL SITE by the argument's own kind.
			name: "string_argument",
			src:  "func f(n int) int { return n }\nfunc main() { print(f(\"hi\")) }\n",
			code: "error[K145]", want: "string argument for int parameter `n`",
		},
		{
			name: "float_argument_to_float_parameter",
			src:  "func f(x float) int { return 3 }\nfunc main() { print(f(1.5)) }\n",
			code: "error[K145]", want: "float argument for int parameter `x`",
		},
		{
			// A float return value is refused: the native subset is int.
			name: "float_return",
			src:  "func f() float { return 1.5 }\nfunc main() { print(f()) }\n",
			code: "error[K145]", want: "kind 22 is neither an integer literal",
		},
		{
			name: "string_return",
			src:  "func f() string { return \"x\" }\nfunc main() { print(f()) }\n",
			code: "error[K145]", want: "`return` of a string is not supported",
		},
		{
			name: "main_let_from_call",
			src:  "func f(n int) int { return n }\nfunc main() {\n    let c = f(1)\n    print(c)\n}\n",
			code: "error[K145]", want: "`let c` initialised from a non-integer expression",
		},
		{
			name: "bind_void_call_result",
			src:  "func v() {}\nfunc main() {\n    let x = v()\n    print(x)\n}\n",
			code: "error[K145]", want: "`let x` initialised from a non-integer expression",
		},
		{
			// The parser discards parameter annotations, so a `bool` parameter
			// cannot be expressed to the driver either; pin what the checker
			// does with an int literal in that position instead.
			name: "callee_binds_string",
			src:  "func f(n int) int {\n    let s = \"hi\"\n    return n\n}\nfunc main() { print(f(1)) }\n",
			code: "error[K145]", want: "`StringLiteral` is not an integer expression",
		},
	}
}

// TestPhaseNATCallQual_KccAcceptsWhatTheGoOracleRefuses pins the two measured
// divergences. kcc's driver lowers these shapes; the Go oracle refuses them
// before any byte is emitted. The Go diagnostic is asserted so a future
// alignment of the two engines is loud and deliberate.
func TestPhaseNATCallQual_KccAcceptsWhatTheGoOracleRefuses(t *testing.T) {
	karkain := phase130Karkain(t)
	divergences := []struct {
		name string
		src  string
		goWant string
	}{
		{
			name: "recursion",
			src:  "func fact(n int) int { if (n <= 1) { return 1 } return n * fact(n - 1) }\nfunc main() { print(fact(5)) }\n",
			goWant: "cannot determine return kind of recursive function 'fact'",
		},
		{
			name: "int_argument_to_float_parameter",
			src:  "func f(x float) int { return 3 }\nfunc main() { print(f(1)) }\n",
			goWant: "int argument for float parameter 'x'",
		},
	}
	for _, d := range divergences {
		d := d
		t.Run(d.name, func(t *testing.T) {
			kccRes := nativeBuildWithEngine(t, karkain, d.src, NativeWindowsTarget, "kcc", d.name+"-qual-kcc.img")
			if kccRes.ExitCode != 0 {
				t.Fatalf("kcc refused %s (exit %d):\n%s", d.name, kccRes.ExitCode, kccRes.Output)
			}
			if !strings.Contains(kccRes.Output, kccNativeProvenance) {
				t.Fatalf("kcc build for %s did not carry %q", d.name, kccNativeProvenance)
			}
			goRes := nativeBuildWithEngine(t, karkain, d.src, NativeWindowsTarget, "go", d.name+"-qual-go.img")
			if goRes.ExitCode == 0 {
				t.Fatalf("the Go oracle ACCEPTED %s; that is a change to record, not to rely on", d.name)
			}
			if !strings.Contains(goRes.Output, d.goWant) {
				t.Fatalf("the Go oracle's refusal for %s changed:\nwant fragment: %q\n%s", d.name, d.goWant, goRes.Output)
			}
		})
	}
}

// TestPhaseNATCallQual_RefusalsAreByNameAndWriteNoImage is the diagnostic half
// of the contract: every refused program exits 3, carries a meaningful
// code, names the offending construct and produces NO image.
func TestPhaseNATCallQual_RefusalsAreByNameAndWriteNoImage(t *testing.T) {
	karkain := phase130Karkain(t)
	for _, c := range natCallQualRefusals() {
		c := c
		t.Run(c.name, func(t *testing.T) {
			res := nativeBuildWithEngine(t, karkain, c.src, NativeWindowsTarget, "kcc", c.name+"-refusal.img")
			if res.ExitCode != 3 {
				t.Fatalf("%s: exit code = %d, want 3\n%s", c.name, res.ExitCode, res.Output)
			}
			if !strings.Contains(res.Output, c.code) {
				t.Fatalf("%s: output does not carry %q\n%s", c.name, c.code, res.Output)
			}
			if !strings.Contains(res.Output, c.want) {
				t.Fatalf("%s: output does not name the construct (%q)\n%s", c.name, c.want, res.Output)
			}
			if res.WroteImage {
				t.Fatalf("%s: a refusal must not write an image", c.name)
			}
		})
	}
}

// TestPhaseNATCallQual_NoGoFallbackEverywhere asserts provenance for every
// accepted shape on all three owned native targets: the Go backend has no
// code path that prints the marker.
func TestPhaseNATCallQual_NoGoFallbackEverywhere(t *testing.T) {
	karkain := phase130Karkain(t)
	for _, c := range natCallQualCorpus {
		for _, target := range []string{NativeWindowsTarget, NativeLinuxTarget, NativeMacOSTarget} {
			c, target := c, target
			t.Run(c.name+"_"+target, func(t *testing.T) {
				res := nativeBuildWithEngine(t, karkain, c.src, target, "kcc", c.name+"-qual-kcc.img")
				if res.ExitCode != 0 || !res.WroteImage {
					t.Fatalf("kcc produced no image for %s/%s (exit %d):\n%s", c.name, target, res.ExitCode, res.Output)
				}
				if !strings.Contains(res.Output, kccNativeProvenance) {
					t.Fatalf("the build did not carry %q, so a Go fallback cannot be ruled out:\n%s",
						kccNativeProvenance, res.Output)
				}
			})
		}
	}
}

// TestPhaseNATCallQual_GateIsWiredInCI keeps this file a gate.
func TestPhaseNATCallQual_GateIsWiredInCI(t *testing.T) {
	body, err := os.ReadFile(filepath.Join("..", "..", ".github", "workflows", "ci.yml"))
	if err != nil {
		t.Fatalf("reading ci.yml: %v", err)
	}
	text := string(body)
	const step = "go test ./pkg/cli/ -run 'TestPhaseNATCallQual_' -count=1"
	if !strings.Contains(text, step) {
		t.Errorf("ci.yml has no step running %q; a locally green gate CI never executes is not a gate", step)
	}
}

// runAndCapture executes a compiled image and returns its stdout as raw bytes.
//
// The capture uses a pipe so the exact byte stream is observable, including a
// trailing newline: trimming here is how the 151A Step 9b defect (a missing
// newline) went undetected.
func runAndCapture(t *testing.T, path string) []byte {
	t.Helper()
	cmd := exec.Command(path)
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stdout
	if err := cmd.Run(); err != nil {
		t.Fatalf("the produced image failed to run: %v\n%s", err, stdout.String())
	}
	return stdout.Bytes()
}

func hexOf(b []byte) string {
	var sb strings.Builder
	for _, v := range b {
		sb.WriteString(hexDigit(v >> 4))
		sb.WriteString(hexDigit(v & 0x0f))
	}
	return sb.String()
}

func hexDigit(n byte) string {
	const digits = "0123456789abcdef"
	return string(digits[n])
}
