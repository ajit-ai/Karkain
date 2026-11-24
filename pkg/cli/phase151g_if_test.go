package cli

import (
	"encoding/hex"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"karkain/pkg/native"
)

// NAT-IF -- a conditional branch in the kcc native driver.
//
// The slice adds `if cond { ... }` and `if cond { ... } else { ... }` over
// INTEGER comparisons to the driver that increment 151's NAT-CALL opened. It is
// the smallest construct that makes the native path a language rather than a
// straight-line evaluator, and it is the first one that introduces a BRANCH, so
// it is also the first one where the emitted bytes depend on a label.
//
// What the slice deliberately does NOT do, and refuses by name instead:
//
//   - `while`, C-style `for`, `break`, `continue` -- still refused. They are the
//     next slice, and a loop needs a bound the arena does not yet have.
//   - a string or float comparison. The oracle's emitCond has a string path
//     and a float path; the self-hosted driver's string layer is main-only and
//     it has no float statement lowering yet.
//
// CORRECTIVE CHANGE (v1.2.7, NAT-STREQ): the STRING half of that bullet no
// longer holds. String `==`/`!=` in a condition IS lowered now, by name
// (natNativeScanStrEqCond), and the refusal row that pinned the old boundary
// was REPLACED with string ORDERING rather than deleted -- see the table below.
// The float half still holds.
//   - a STRING binding inside a branch. The rodata cursor is threaded per
//     statement list in this driver, so a literal in a branch would need a
//     cursor the recursion does not carry. An integer binding inside a branch
//     IS supported, and it gets the slot the oracle gives it.
//
// The evidence is the same four layers NAT-CALL established, because they are
// the ones that survive a wrong implementation:
//
//	1. EXECUTED PE goldens -- exact stdout bytes and exit codes on this host.
//	2. A Linux ELF BYTE-IDENTITY DIFFERENTIAL against the Go oracle. The
//	   expected bytes come from calling the oracle at test time; nothing is
//	   pinned as a golden, because for most of increment 151 the bytes were
//	   wrong and a golden would have frozen the divergence.
//	3. A REFUSAL TABLE -- exit code AND no image, for every construct the
//	   slice declines.
//	4. The branch-scoped binding, asserted to be AGREED ON by both engines
//	   rather than refused by one of them.
//
// Plus determinism, non-vacuity, and the CI-wiring check.

// natIfCorpus is the execution corpus. Every entry is a different .kark source
// compiled through kcc's native driver.
//
// `wantHex` is the EXPECTED stdout as exact bytes. It is never trimmed: the
// Step 9b defect was a MISSING newline, and trimming is exactly the tool that
// hides a missing byte.
var natIfCorpus = []struct {
	name     string
	src      string
	wantHex  string
	wantExit int
	// expectDiff marks a case whose images are EXPECTED to differ between the
	// two engines. It exists because the byte-identity layer would otherwise
	// have to skip the case, and a skipped case is not evidence: an
	// unexplained difference is exactly what a "known difference" list is
	// for. The reason is in TestPhaseNATIf_TheRegisterAllocatorBoundaryIsPinned.
	expectDiff bool
}{
	{
		// The condition holds, so the consequence runs and the else is skipped.
		name:    "if_taken",
		src:     "func main() {\n    let x = 5\n    if x > 3 {\n        print(1)\n    }\n}\n",
		wantHex: "310a",
	},
	{
		// The condition does not hold and there is no else: the program prints
		// nothing at all and exits 0. The empty stdout is the evidence that
		// the FALSE branch was taken and skipped the body.
		name:    "if_not_taken",
		src:     "func main() {\n    let x = 5\n    if x > 30 {\n        print(1)\n    }\n}\n",
		wantHex: "",
	},
	{
		name:    "if_else_taken",
		src:     "func main() {\n    let x = 5\n    if x > 3 {\n        print(1)\n    } else {\n        print(2)\n    }\n}\n",
		wantHex: "310a",
	},
	{
		name:    "if_else_not_taken",
		src:     "func main() {\n    let x = 5\n    if x > 30 {\n        print(1)\n    } else {\n        print(2)\n    }\n}\n",
		wantHex: "320a",
	},
	{
		// The six comparison operators, one program each. The opcode is the
		// only thing that varies between them, so a corpus with one operator
		// would prove nothing about the other five.
		name:    "cmp_eq",
		src:     "func main() {\n    let x = 5\n    if x == 5 {\n        print(1)\n    }\n}\n",
		wantHex: "310a",
	},
	{
		name:    "cmp_ne",
		src:     "func main() {\n    let x = 5\n    if x != 5 {\n        print(1)\n    }\n}\n",
		wantHex: "",
	},
	{
		name:    "cmp_lt",
		src:     "func main() {\n    let x = 5\n    if x < 9 {\n        print(1)\n    }\n}\n",
		wantHex: "310a",
	},
	{
		name:    "cmp_le",
		src:     "func main() {\n    let x = 5\n    if x <= 5 {\n        print(1)\n    }\n}\n",
		wantHex: "310a",
	},
	{
		name:    "cmp_gt",
		src:     "func main() {\n    let x = 5\n    if x > 3 {\n        print(1)\n    }\n}\n",
		wantHex: "310a",
	},
	{
		name:    "cmp_ge",
		src:     "func main() {\n    let x = 5\n    if x >= 5 {\n        print(1)\n    }\n}\n",
		wantHex: "310a",
	},
	{
		// A nested `if`. The inner pair of labels must not collide with the
		// outer pair, which is why the labels carry a statement PATH and not
		// just an ordinal.
		name:    "nested_if",
		src:     "func main() {\n    let x = 5\n    if x > 3 {\n        if x > 4 {\n            print(1)\n        } else {\n            print(2)\n        }\n    }\n}\n",
		wantHex: "310a",
	},
	{
		// `else if` arrives from the parser as a one-element list holding the
		// inner `if`, so it exercises the same recursive lowering.
		name:    "else_if",
		src:     "func main() {\n    let x = 5\n    if x > 30 {\n        print(1)\n    } else if x > 3 {\n        print(2)\n    } else {\n        print(3)\n    }\n}\n",
		wantHex: "320a",
	},
	{
		// A branch inside a CALLED function. The callee has its own frame and
		// its own scratch region, so the condition must stage into the
		// CALLEE's binTemp and not main's.
		name:    "if_in_callee",
		src:     "func f(n int) int {\n    if n > 3 {\n        return 1\n    }\n    return 0\n}\nfunc main() {\n    print(f(5))\n}\n",
		wantHex: "310a",
	},
	{
		// A CALL in the condition. The callee's result must survive the
		// spill/reload the condition performs.
		name:    "call_in_condition",
		src:     "func f(n int) int {\n    return n + 1\n}\nfunc main() {\n    if f(1) == 2 {\n        print(1)\n    }\n}\n",
		wantHex: "310a",
	},
	{
		// Arithmetic in the condition. The operands are evaluated at the
		// oracle's depth 1, so a nested binary stages one slot deeper.
		name:    "arith_condition",
		src:     "func main() {\n    let a = 2\n    let b = 5\n    if a + 1 < b {\n        print(1)\n    }\n}\n",
		wantHex: "310a",
	},
	{
		// A literal-only condition, so the comparison has no local to read.
		name:    "literal_condition",
		src:     "func main() {\n    if 1 < 2 {\n        print(1)\n    }\n}\n",
		wantHex: "310a",
	},
	{
		// A binding INSIDE a branch. The oracle's layout pass descends into a
		// branch and allocates the slot at the point the `if` is reached, so
		// this program's three bindings occupy slots 0, 1 and 2 -- the branch's
		// binding is the MIDDLE one. A plan that allocated it last would give
		// the same output and a different frame, which is why the byte
		// identity layer, not this one, is the real evidence.
		name:    "branch_scoped_let",
		src:     "func main() {\n    let a = 1\n    if a > 0 {\n        let b = 2\n        print(b)\n    }\n    let c = 3\n    print(c)\n}\n",
		wantHex: "320a330a",
	},
	{
		// Five live locals and a branch: the shape at which the Go oracle's
		// Phase-150C register allocator ENGAGES, so the two engines encode it
		// differently. The self-hosted driver keeps every local in its frame
		// slot, which is the pre-150C path and is correct -- so this case is
		// about BEHAVIOUR being identical while the BYTES are not.
		//
		// It is in the corpus (so the execution layer proves the output) and
		// marked expectDiff (so the byte-identity layer asserts the
		// difference instead of skipping it).
		name:       "five_locals_with_branch",
		src:        "func main() {\n    let v1 = 1\n    let v2 = 2\n    let v3 = 3\n    let v4 = 4\n    let v5 = 5\n    if v1 < v2 {\n        print(v3)\n    } else {\n        print(v4)\n    }\n    print(v5)\n}\n",
		wantHex:    "330a350a",
		expectDiff: true,
	},
	{
		// A `return` inside a branch. The exit code is the only observable
		// here, because the program prints nothing -- which is exactly why the
		// gate asserts the exit code separately.
		name:     "return_in_branch",
		src:      "func main() {\n    let x = 5\n    if x > 3 {\n        return 7\n    }\n    return 1\n}\n",
		wantHex:  "",
		wantExit: 7,
	},
}

// natIfRefusals is the refusal table. Every entry must be REFUSED by name:
// a non-zero exit, a message naming the construct, and NO image on disk.
//
// The message is matched as a substring because the exact wording carries the
// reason, but the REASON is the assertion: a refusal that does not say what it
// refused leaves the reader guessing, and the prompt's Phase B/D requirement
// is that unsupported constructs produce ACCURATE diagnostics.
var natIfRefusals = []struct {
	name string
	src  string
	want string
}{
	{
		// A condition that is not a comparison. The oracle's own wording
		// (program.go:5074), so the two engines say the same thing.
		name: "non_comparison_condition",
		src:  "func main() {\n    let x = 5\n    if x {\n        print(1)\n    }\n}\n",
		want: "condition must be a comparison (==, !=, <, <=, >, >=) over ints or floats",
	},
	{
		// The operator is named in the refusal, so the reader does not have to
		// guess which of the six was rejected.
		name: "arithmetic_condition",
		src:  "func main() {\n    let x = 5\n    if x + 1 {\n        print(1)\n    }\n}\n",
		want: "found '+'",
	},
	{
		// CORRECTIVE CHANGE against the frozen NAT-IF baseline (v1.2.7,
		// NAT-STREQ). This row used to pin `if s == "hi"` as refused, because
		// the self-hosted driver's comparison path was not ported. v1.2.7 ported
		// STRING EQUALITY, so the program is now ACCEPTED and the row failed with
		// "kcc ACCEPTED a program it cannot lower (exit 0)" -- measured before
		// this edit. The row is REPLACED, not deleted: this table's property is
		// "a construct the driver cannot lower is refused BY NAME and writes no
		// image", and the boundary the slice actually draws is ORDERING.
		// `==`/`!=` compare content; `<`, `<=`, `>`, `>=` over strings have no
		// defined semantics in this value model and are refused by name.
		// The accepted half is gated by TestPhaseV127Streq_ (nested and
		// loop-contained cases included).
		name: "string_ordering_condition",
		src:  "func main() {\n    let s = \"hi\"\n    if s < \"ho\" {\n        print(1)\n    }\n}\n",
		want: "string ordering ('<') is not supported",
	},
	{
		name: "float_condition",
		src:  "func main() {\n    if 1.5 < 2.5 {\n        print(1)\n    }\n}\n",
		want: "is not an integer expression",
	},
	{
		// A CORRECTIVE CHANGE against the frozen NAT-IF baseline, recorded
		// in docs/audit/NAT-LOOP-BASELINE.md section 8. This row used to pin
		// "a `while` is not lowered", and NAT-LOOP taught the driver to lower
		// loops, so the row stopped being a refusal and the gate failed with
		// "kcc ACCEPTED a program it cannot lower (exit 0)". It was REPLACED
		// rather than deleted: this table's property is "a construct the
		// driver cannot lower is refused BY NAME and writes no image", and a
		// table that quietly shrinks stops being evidence for it.
		//
		// The replacement is the loop condition's own rule: a condition must
		// be a COMPARISON, which the oracle's emitCond requires.
		name: "loop_non_comparison_condition",
		src:  "func main() {\n    let x = 5\n    while x {\n        x = x + 1\n    }\n}\n",
		want: "condition must be a comparison",
	},
	{
		// Division is still out of scope, and it is refused in the CONDITION
		// too -- a condition is an expression, and the expression path refuses
		// the operator before the condition path ever sees it.
		name: "division_in_condition",
		src:  "func main() {\n    if 6 / 2 < 4 {\n        print(1)\n    }\n}\n",
		want: "unsupported operator '/'",
	},
}

// natIfImage compiles one source through the kcc native driver and returns the
// PE image. It goes through `native-ast`, the driver entry point, so a failure
// here is a driver failure and never a CLI-dispatch artefact.
func natIfImage(t *testing.T, src string) []byte {
	t.Helper()
	out := nat9cBuild(t, src)
	if strings.Contains(out, "error[K") {
		t.Fatalf("kcc refused a program this gate expects to compile:\n%.400s", out)
	}
	img := nat9cDecodeHex(t, out, "driver image")
	if _, _, err := native.ParsePE(img); err != nil {
		t.Fatalf("driver emitted an invalid PE: %v", err)
	}
	return img
}

// natIfHex decodes a hex expectation, failing loudly rather than comparing
// against an empty slice -- which would make an empty expectation pass for a
// program that printed nothing at all.
func natIfHex(t *testing.T, s, what string) []byte {
	t.Helper()
	if s == "" {
		return nil
	}
	b, err := hex.DecodeString(s)
	if err != nil {
		t.Fatalf("%s is not hex (%q): %v", what, s, err)
	}
	return b
}

// TestPhaseNATIf_ProgramsExecuteWithExactBytes is layer 1: the branch is
// OBSERVABLE.
//
// Each corpus entry is a different .kark file compiled through kcc's native
// driver, written to disk and EXECUTED on this host. stdout is compared as
// exact bytes and the exit code separately, because `return_in_branch` is
// observable only through the exit code and `if_not_taken` only through the
// absence of output.
func TestPhaseNATIf_ProgramsExecuteWithExactBytes(t *testing.T) {
	if os.PathSeparator == '/' {
		t.Skip("PE images only execute on Windows; every other layer is cross-platform")
	}
	if len(natIfCorpus) < 10 {
		t.Fatalf("corpus has %d cases; NAT-IF was measured over at least 10 and a "+
			"shrinking corpus is how a gate stops being evidence", len(natIfCorpus))
	}

	for _, c := range natIfCorpus {
		c := c
		t.Run(c.name, func(t *testing.T) {
			img := natIfImage(t, c.src)
			got, code := nat9dRun(t, img)
			want := natIfHex(t, c.wantHex, c.name+" expected stdout")
			if !nat9cBytesEq(got, want) {
				t.Errorf("stdout = % x, want % x\n  source:\n%s\n"+
					"  Compared as EXACT BYTES: a missing or extra trailing newline is a "+
					"real defect, and trimming would hide it.", got, want, c.src)
			}
			if code != c.wantExit {
				t.Errorf("exit = %d, want %d\n  source:\n%s", code, c.wantExit, c.src)
			}
		})
	}
}

// TestPhaseNATIf_LinuxELFIsByteIdenticalToTheOracle is layer 2, and it is the
// byte-identity evidence.
//
// Both engines build the SAME source for native-x86_64-linux and the image
// bytes are compared. The kcc outcome must also carry the provenance marker,
// which only src/compiler/main.kark can print -- so a build that quietly fell
// through to the Go backend would agree byte for byte and still fail here.
//
// The expected bytes are produced by calling the oracle at test time. A
// GOLDEN would be wrong here: for most of increment 151 the emitted bytes were
// wrong, so a golden pins the divergence and calls it expected.
func TestPhaseNATIf_LinuxELFIsByteIdenticalToTheOracle(t *testing.T) {
	karkain := phase130Karkain(t)

	for _, c := range natIfCorpus {
		c := c
		t.Run(c.name, func(t *testing.T) {
			kccRes := nativeBuildWithEngine(t, karkain, c.src, NativeLinuxTarget, "kcc", c.name+"-kcc.elf")
			if kccRes.ExitCode != 0 || !kccRes.WroteImage {
				t.Fatalf("kcc produced no ELF (exit %d):\n%s", kccRes.ExitCode, kccRes.Output)
			}
			if !kccProducedImage(kccRes.Output) {
				t.Fatalf("the kcc build did not carry the self-hosted provenance marker "+
					"(%q), so a Go-backend fallback cannot be excluded:\n%s",
					kccNativeProvenance, kccRes.Output)
			}
			if _, _, err := native.Parse(kccRes.Image); err != nil {
				t.Fatalf("kcc emitted an ELF the ORACLE's Parse rejects: %v", err)
			}

			goRes := nativeBuildWithEngine(t, karkain, c.src, NativeLinuxTarget, "go", c.name+"-go.elf")
			if goRes.ExitCode != 0 || !goRes.WroteImage {
				t.Fatalf("the Go oracle produced no ELF (exit %d):\n%s", goRes.ExitCode, goRes.Output)
			}

			if c.expectDiff {
				// A KNOWN difference, asserted rather than skipped. See the
				// expectDiff field's comment and
				// TestPhaseNATIf_TheRegisterAllocatorBoundaryIsPinned for why the
				// two engines differ here.
				if nat9cBytesEq(kccRes.Image, goRes.Image) {
					t.Errorf("this case is marked expectDiff, but the two engines "+
						"produced IDENTICAL images.\n  sha %s\n"+
						"  The Go oracle's Phase-150C register allocator engages at 5 "+
						"simultaneously-live int locals in a function with control flow. "+
						"If the allocator's rule changed, the boundary must be "+
						"re-measured rather than assumed.", kccRes.SHA)
				}
				return
			}
			if !nat9cBytesEq(kccRes.Image, goRes.Image) {
				t.Errorf("kcc and the Go oracle emitted DIFFERENT ELF images for the "+
					"same program.\n  kcc: %d bytes sha %s\n  go : %d bytes sha %s\n"+
					"  source:\n%s\n"+
					"  The corpus is meant to cover the shapes that ARE identical. If a "+
					"difference is expected, it belongs in the refusal table or in a "+
					"newly baselined slice -- not silently in this list.",
					len(kccRes.Image), kccRes.SHA, len(goRes.Image), goRes.SHA, c.src)
			}
		})
	}
}

// TestPhaseNATIf_RefusalsAreByName is layer 3.
//
// Every declined construct must be refused BY NAME, must exit non-zero, and
// must leave NO IMAGE behind. The last part is the one that matters most: a
// refusal that still wrote a file would leave a user with an executable they
// believe is the program they wrote.
func TestPhaseNATIf_RefusalsAreByName(t *testing.T) {
	karkain := phase130Karkain(t)

	for _, c := range natIfRefusals {
		c := c
		t.Run(c.name, func(t *testing.T) {
			dir := t.TempDir()
			src := writeKark(t, dir, c.name+".kark", c.src)
			out := filepath.Join(dir, c.name+".elf")

			res := nativeBuildWithEngine(t, karkain, c.src, NativeLinuxTarget, "kcc", c.name+".elf")
			if res.ExitCode == 0 {
				t.Fatalf("kcc ACCEPTED a program it cannot lower (exit 0):\n%s", res.Output)
			}
			if !strings.Contains(res.Output, c.want) {
				t.Errorf("the refusal does not name the construct.\n  want the message to "+
					"contain %q\n  got:\n%s", c.want, res.Output)
			}
			if res.WroteImage {
				t.Errorf("a refused program still wrote an image (%d bytes)", len(res.Image))
			}
			if _, err := os.Stat(out); err == nil {
				t.Errorf("a refused program still wrote an image to %s", out)
			}
			_ = src
		})
	}
}

// TestPhaseNATIf_TheBranchScopedBindingIsAgreedOnByBothEngines is the evidence
// for a CORRECTION this slice made to itself.
//
// The first draft of this slice REFUSED a `let` inside a branch, on the belief
// that the layout pass gave branch-scoped bindings no frame slot and they
// would land on offset 0. That belief was WRONG: the oracle's scanLets
// descends into a branch (program.go:4674-4676) and allocates the slot at the
// point the `if` is reached. The gate that caught it was the byte-identity
// differential -- the oracle produced an image and the self-hosted driver
// refused the program.
//
// So the two engines must now AGREE, and this asserts it in both directions:
// both build the program, and both images are identical. The "both refuse"
// version of this test would have passed while the oracle silently corrupted
// the first local, which is the failure mode this assertion exists to prevent.
func TestPhaseNATIf_TheBranchScopedBindingIsAgreedOnByBothEngines(t *testing.T) {
	karkain := phase130Karkain(t)
	// Three bindings: one before the branch, one inside it, one after. The
	// branch's binding must be the MIDDLE slot, which is what the oracle's
	// walk order produces.
	const src = "func main() {\n    let x = 5\n    if x > 3 {\n        let z = 1\n        print(z)\n    }\n    let y = 9\n    print(y)\n}\n"

	kccRes := nativeBuildWithEngine(t, karkain, src, NativeLinuxTarget, "kcc", "branchlet-kcc.elf")
	if kccRes.ExitCode != 0 || !kccRes.WroteImage {
		t.Fatalf("kcc refused a program the oracle accepts (exit %d):\n%s",
			kccRes.ExitCode, kccRes.Output)
	}
	goRes := nativeBuildWithEngine(t, karkain, src, NativeLinuxTarget, "go", "branchlet-go.elf")
	if goRes.ExitCode != 0 || !goRes.WroteImage {
		t.Fatalf("the Go oracle refused the program (exit %d):\n%s", goRes.ExitCode, goRes.Output)
	}
	if !nat9cBytesEq(kccRes.Image, goRes.Image) {
		t.Errorf("the two engines disagree about a branch-scoped binding.\n"+
			"  kcc: %d bytes sha %s\n  go : %d bytes sha %s\n"+
			"  The oracle allocates the branch's slot at the point the `if` is "+
			"reached; the self-hosted plan must do the same.",
			len(kccRes.Image), kccRes.SHA, len(goRes.Image), goRes.SHA)
	}
}

// TestPhaseNATIf_DeterminismAndNonVacuity is layer 4.
//
// Determinism: the same source must produce the same image twice. A native
// backend that leaked state between builds would be a nightmare to debug, and
// the KIR contract already promises byte-determinism for the text form.
//
// Non-vacuity: DIFFERENT sources must produce DIFFERENT images. A driver that
// ignored the condition and always took the same branch would pass the
// execution layer for every case whose expected output happened to match, so
// the images themselves have to move.
func TestPhaseNATIf_DeterminismAndNonVacuity(t *testing.T) {
	if os.PathSeparator == '/' {
		t.Skip("PE images only execute on Windows")
	}

	// Determinism, on the branch case specifically.
	src := "func main() {\n    let x = 5\n    if x > 3 {\n        print(1)\n    } else {\n        print(2)\n    }\n}\n"
	a := natIfImage(t, src)
	b := natIfImage(t, src)
	if !nat9cBytesEq(a, b) {
		t.Errorf("two builds of the same source produced different images (%d vs %d bytes)",
			len(a), len(b))
	}

	// Non-vacuity: the condition's RESULT must reach the image. Two programs
	// that differ only in the comparison's outcome must differ in their bytes.
	taken := natIfImage(t, "func main() {\n    let x = 5\n    if x > 3 {\n        print(1)\n    }\n}\n")
	notTaken := natIfImage(t, "func main() {\n    let x = 5\n    if x > 30 {\n        print(1)\n    }\n}\n")
	if nat9cBytesEq(taken, notTaken) {
		t.Errorf("two programs whose conditions differ produced IDENTICAL images, so " +
			"the condition is not reaching the emitted code")
	}
}

// TestPhaseNATIf_TheRegisterAllocatorBoundaryIsPinned is layer 5, and it is a
// LIMITATION stated as an assertion rather than as a comment.
//
// The Go oracle has a Phase-150C register allocator. It engages only when a
// function contains control flow AND has more than `raMinLive`
// (= allocRegCount + 1 = 5) simultaneously-live int locals, because below that
// every live value fits in a register anyway and a push/pop pair would cost
// more than the load it removes.
//
// The self-hosted driver has NO register allocator: every local is a frame
// slot, which is the pre-150C path and is always CORRECT. So from five locals
// upward the two engines encode the same program DIFFERENTLY -- and the
// difference is an encoding strategy, not a wrong answer.
//
// MEASURED on this host, not derived:
//
//	4 locals + an if : kcc 454 bytes, oracle 454 bytes -- IDENTICAL
//	5 locals + an if : kcc 469 bytes, oracle 471 bytes -- DIFFER
//	6 locals + an if : kcc 484 bytes, oracle 486 bytes -- DIFFER
//
// The boundary is asserted from BOTH sides: four locals must stay identical
// (so a future change that breaks byte identity for small programs is caught
// here), and five must differ (so the gap cannot quietly become an unknown
// one). The five-local case is also EXECUTED, because "different bytes" must
// not become "different behaviour".
func TestPhaseNATIf_TheRegisterAllocatorBoundaryIsPinned(t *testing.T) {
	karkain := phase130Karkain(t)

	build := func(locals int) (string, string) {
		var b strings.Builder
		b.WriteString("func main() {\n")
		for i := 1; i <= locals; i++ {
			b.WriteString("    let v")
			b.WriteString(strconv.Itoa(i))
			b.WriteString(" = ")
			b.WriteString(strconv.Itoa(i))
			b.WriteString("\n")
		}
		b.WriteString("    if v1 < v2 {\n        print(v3)\n    } else {\n        print(v4)\n    }\n")
		b.WriteString("}\n")
		src := b.String()
		kccRes := nativeBuildWithEngine(t, karkain, src, NativeLinuxTarget, "kcc", "ra-kcc.elf")
		goRes := nativeBuildWithEngine(t, karkain, src, NativeLinuxTarget, "go", "ra-go.elf")
		if kccRes.ExitCode != 0 || goRes.ExitCode != 0 {
			t.Fatalf("a %d-local program did not build (kcc exit %d, go exit %d):\n%s\n%s",
				locals, kccRes.ExitCode, goRes.ExitCode, kccRes.Output, goRes.Output)
		}
		return kccRes.SHA, goRes.SHA
	}

	// Below the threshold: identical.
	kccSHA, goSHA := build(4)
	if kccSHA != goSHA {
		t.Errorf("a 4-local program with a branch produced DIFFERENT images.\n"+
			"  kcc sha %s\n  go  sha %s\n"+
			"  Below the allocator's engagement threshold the two engines must agree "+
			"byte for byte; a difference here means something else moved.", kccSHA, goSHA)
	}

	// At the threshold: different, and the difference is recorded here.
	kccSHA, goSHA = build(5)
	if kccSHA == goSHA {
		t.Errorf("a 5-local program with a branch produced IDENTICAL images.\n" +
			"  The oracle's Phase-150C register allocator engages at 5 simultaneously-" +
			"live int locals in a function with control flow, so the two engines are " +
			"expected to differ. If they do not, the allocator's rule changed and " +
			"this boundary must be re-measured rather than assumed.")
	}
}

// TestPhaseNATIf_GateIsWiredInCI asserts that this gate has a ci.yml step of
// its own.
//
// A gate is a test file PLUS a CI entry. Writing the test and running it
// locally is not evidence that CI ever executes it -- and this repository has
// four gates that were written, reported green locally, and never ran in CI.
func TestPhaseNATIf_GateIsWiredInCI(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join(repoRoot(t), ".github", "workflows", "ci.yml"))
	if err != nil {
		t.Fatalf("reading ci.yml: %v", err)
	}
	ci := string(raw)
	if !strings.Contains(ci, "TestPhaseNATIf_") {
		t.Errorf("ci.yml has no step running this gate's pattern 'TestPhaseNATIf_'")
	}
}
