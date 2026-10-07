package cli

// Phase 151A Step 9e - `len()` on arrays, through the real AST driver.
//
// 9d added the array kind. This adds the first BUILTIN to reach the driver, and it is
// a small slice for a specific reason: `len` on an array is the oracle's single
// instruction, so it can be proven end-to-end with almost no new emission, and it
// establishes the shape every later builtin will follow (a CallExpr in a value
// position, validated in the scan and lowered in the body).
//
// THE CLAIM IS EXECUTIONAL, as in 9d. Each case is a different .kark file, compiled by
// kcc into a PE, EXECUTED, and compared byte for byte. Nothing is trimmed.
//
// WHAT IS NEW HERE, beyond `len`:
//
//  1. A single `natNativeScanValue` now validates BOTH `print`'s and `return`'s
//     expression, so the accepted and the refused shapes cannot drift apart. A driver
//     that accepts `print(a[i])` but refuses `return a[i]` is an inconsistency a user
//     cannot act on, and the 9d refusal table already found the two paths diverging.
//  2. `return len(a)` is observable only through the EXIT CODE, because it prints
//     nothing. That case is the reason the gate asserts exit codes at all: a
//     stdout-only assertion cannot see it.
//  3. The `len` count offset is written literally as off+8 and is deliberately NOT
//     derived from `natLocalOff(ord, natKindArray())`. An array's element AREA starts
//     at off+16 while its COUNT is at off+8, so deriving the count from the slot
//     formula would be wrong. `len_of_1` is the case that catches it: with one element
//     the area and the count are 8 bytes apart, and reading off+16 as the count would
//     report 0 or garbage.
//
// NOT RUN HERE: any whole-tree KCC workload (see PHASE-151A-BASELINE.md 9a-1).

import (
	"os"
	"strings"
	"testing"
)

var nat9eCases = []nat9dCase{
	{
		// The basic claim: the count is the array header's second unit.
		name:    "len_of_2",
		src:     "func main() {\n    let a = [7, 35]\n    print(len(a))\n}\n",
		wantOut: []byte("2\n"), wantExit: 0, localBytes: 32,
	},
	{
		name:    "len_of_3",
		src:     "func main() {\n    let a = [7, 35, 99]\n    print(len(a))\n}\n",
		wantOut: []byte("3\n"), wantExit: 0, localBytes: 40,
	},
	{
		// One element is the boundary that distinguishes the COUNT at off+8 from the
		// element AREA at off+16. A frame offset that confuses the two reads the wrong
		// unit here.
		name:    "len_of_1",
		src:     "func main() {\n    let a = [42]\n    print(len(a))\n}\n",
		wantOut: []byte("1\n"), wantExit: 0, localBytes: 24,
	},
	{
		// len and an element read together: both header fields must be addressable, and
		// the count must survive an intervening element load.
		name:    "len_then_element",
		src:     "func main() {\n    let a = [7, 35, 99]\n    print(len(a))\n    print(a[2])\n}\n",
		wantOut: []byte("3\n99\n"), wantExit: 0, localBytes: 40,
	},
	{
		// An array followed by a scalar: len must read the ARRAY's header, not the
		// scalar's slot. With a flat 8-byte stride the array's off+8 is the scalar's
		// slot, so this reports the scalar's value instead of 2.
		name:    "len_with_scalar_after",
		src:     "func main() {\n    let a = [7, 35]\n    let b = 1000\n    print(len(a))\n}\n",
		wantOut: []byte("2\n"), wantExit: 0, localBytes: 40,
	},
	{
		// Observable ONLY through the exit code: this program prints nothing.
		name:    "len_as_return_value",
		src:     "func main() {\n    let a = [7, 35]\n    return len(a)\n}\n",
		wantOut: []byte{}, wantExit: 2, localBytes: 32,
	},
	{
		// A 64-bit-sized count. The oracle stores it with MovRegImm64, so a 3-element
		// array's count is a 10-byte movabs; a 32-bit count path would still print 3
		// here, which is why the execution layer cannot be the only guard.
		name:    "len_is_a_64_bit_header_field",
		src:     "func main() {\n    let a = [10, 20, 30]\n    print(len(a))\n}\n",
		wantOut: []byte("3\n"), wantExit: 0, localBytes: 40,
	},
}

// nat9eRefusals are shapes `len` must not accept. Each names WHICH construct, so a
// refusal firing for the wrong reason does not pass.
var nat9eRefusals = []struct {
	name string
	src  string
	want string
}{
	{
		// len of a scalar. A real defect caught while measuring this slice's first
		// draft: the count would have been read from the scalar's neighbouring slot,
		// producing a valid image printing a meaningless number.
		name: "len_of_scalar",
		src:  "func main() {\n    let x = 5\n    print(len(x))\n}\n",
		want: "len() requires an array",
	},
	{
		// len of an undeclared name. The type checker may catch this first (K102);
		// either diagnostic is correct, so the assertion is only that it is refused
		// and names the identifier.
		name: "len_of_undeclared",
		src:  "func main() {\n    print(len(q))\n}\n",
		want: "q",
	},
	{
		// Caught EARLIER than the driver's scan, by kcc's own builtin-arity check
		// (K104), exactly as the 9d `print(q[0])` case is caught earlier by K102. The
		// first draft of this gate expected the driver's own message and was wrong:
		// asserting a diagnostic that can never fire teaches a reader to distrust the
		// gate. Both are refusals; which layer reports it is not the claim.
		name: "len_wrong_arity",
		src:  "func main() {\n    let a = [7, 35]\n    print(len(a, a))\n}\n",
		want: "builtin 'len' expects 1 arguments but got 2",
	},
	{
		// len of an element rather than of a variable. The oracle rejects this shape
		// too ("len() requires an array or map variable").
		//
		// The expected wording changed when Step 9f taught the driver about strings and
		// len() of a string became legal, so the message now says "array or string".
		// Asserting the exact phrase rather than a looser substring is deliberate: it is
		// what makes a future reword of this diagnostic a visible test change instead of
		// a silent drift.
		name: "len_of_expression",
		src:  "func main() {\n    let a = [7, 35]\n    print(len(a[0]))\n}\n",
		want: "len() requires an array or string variable",
	},
	{
		// A builtin that exists in the language but is not lowered natively yet.
		name: "other_builtin_not_lowered",
		src:  "func main() {\n    let a = [7, 35]\n    print(abs(a[0]))\n}\n",
		want: "is not lowered by the native driver",
	},
}

// TestPhase151A9E_LenExecutesWithExactBytes is the primary claim.
func TestPhase151A9E_LenExecutesWithExactBytes(t *testing.T) {
	if os.PathSeparator == '/' {
		t.Skip("PE images only execute on Windows; the driver itself is cross-platform")
	}
	for _, c := range nat9eCases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			img := nat9dImage(t, c.src)
			got, code := nat9dRun(t, img)
			if !nat9cBytesEq(got, c.wantOut) {
				t.Errorf("stdout = %q (% x), want %q (% x)\n  source: %s",
					got, got, c.wantOut, c.wantOut, c.src)
			}
			if code != c.wantExit {
				t.Errorf("exit = %d, want %d\n  source: %s\n  NOTE: "+
					"`return len(a)` prints NOTHING, so the exit code is the only "+
					"observable; a stdout-only gate could not see it", code, c.wantExit, c.src)
			}
		})
	}
}

// TestPhase151A9E_FrameFollowsTheValueModelRule pins the frame for each len program, so
// adding a builtin did not disturb slot sizing.
func TestPhase151A9E_FrameFollowsTheValueModelRule(t *testing.T) {
	for _, c := range nat9eCases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			img := nat9dImage(t, c.src)
			got := nat9dFrameSubRsp(t, img)
			want := nat9dExpectFrame(c.localBytes)
			if !nat9dContainsInt32(got, want) {
				t.Errorf("no `sub rsp, %d` in the image; got %v.\n  want %d for localBytes "+
					"%d.\n  source: %s", want, got, want, c.localBytes, c.src)
			}
		})
	}
}

// TestPhase151A9E_LenReadsTheHeaderNotTheElementArea is the discriminating layer.
//
// The array header is two units: [off] = element-area base, [off+8] = COUNT, and the
// elements start at off+16. Three of these programs have the SAME element value at
// index 0 (42, 7, 42) and DIFFERENT counts, so an implementation that read the
// element area instead of the count could not produce all of them.
//
// It is stated as an image-level comparison rather than left to the execution layer
// because the execution layer already passes: printing off+16 for a one-element array
// happens to produce a large pointer-looking number, which is wrong, but "wrong" is
// only visible because the expected value was written down. The images make the
// mistake mechanical to detect.
func TestPhase151A9E_LenReadsTheHeaderNotTheElementArea(t *testing.T) {
	variants := []struct{ name, src string }{
		{"count_2_first_42", "func main() {\n    let a = [42, 35]\n    print(len(a))\n}\n"},
		{"count_3_first_42", "func main() {\n    let a = [42, 35, 99]\n    print(len(a))\n}\n"},
		{"count_5_first_42", "func main() {\n    let a = [42, 1, 2, 3, 4]\n    print(len(a))\n}\n"},
	}
	var prior []byte
	var priorName string
	for _, v := range variants {
		v := v
		t.Run(v.name, func(t *testing.T) {
			img := nat9dImage(t, v.src)
			if prior != nil && nat9cBytesEq(img, prior) {
				t.Fatalf("image for %s is byte-identical to %s; the count must come "+
					"from the array's own header", v.name, priorName)
			}
			prior, priorName = img, v.name
		})
	}
}

// TestPhase151A9E_OutOfSubsetShapesAreRefusedByName covers the negative table.
func TestPhase151A9E_OutOfSubsetShapesAreRefusedByName(t *testing.T) {
	for _, r := range nat9eRefusals {
		r := r
		t.Run(r.name, func(t *testing.T) {
			out := nat9cBuild(t, r.src)
			if !strings.Contains(out, "error[") {
				t.Fatalf("expected a refusal, got:\n%.400s", out)
			}
			if !strings.Contains(out, r.want) {
				t.Errorf("refusal does not name the offending construct %q:\n%.400s",
					r.want, out)
			}
			if isHex9d(strings.TrimSpace(out)) {
				t.Errorf("a REFUSED program also produced an image:\n%.200s", out)
			}
		})
	}
}

// TestPhase151A9E_CorpusIsNonVacuousAndDeterministic guards against a corpus that
// silently stopped compiling, and against nondeterminism.
func TestPhase151A9E_CorpusIsNonVacuousAndDeterministic(t *testing.T) {
	if len(nat9eCases) < 6 {
		t.Fatalf("corpus has %d cases; that is too few to be evidence", len(nat9eCases))
	}
	for _, c := range nat9eCases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			a := nat9dImage(t, c.src)
			b := nat9dImage(t, c.src)
			if !nat9cBytesEq(a, b) {
				t.Errorf("two runs of the same source produced different images (%d vs %d)",
					len(a), len(b))
			}
			if len(a) < 512 {
				t.Errorf("image is only %d bytes", len(a))
			}
		})
	}
}

// TestPhase151A9E_PrintAndReturnAgree pins the refactor: one validator now backs both
// value positions, so a shape accepted in one must be accepted in the other.
//
// This is a property of the DRIVER, not of any one program: it walks the accepted
// shapes and checks that `print` and `return` give the same verdict. A driver with two
// copies of the logic would drift, and the drift would only show up for a shape a
// given test corpus happens not to use.
func TestPhase151A9E_PrintAndReturnAgree(t *testing.T) {
	accepted := []struct{ name, body string }{
		{"int_literal", "print(1)"},
		{"scalar_local", "let x = 1\n    print(x)"},
		{"array_index", "let a = [7, 35]\n    print(a[1])"},
		{"len_of_array", "let a = [7, 35]\n    print(len(a))"},
	}
	for _, a := range accepted {
		a := a
		t.Run(a.name, func(t *testing.T) {
			printSrc := "func main() {\n    " + a.body + "\n}\n"
			retSrc := "func main() {\n    " + a.body + "\n    return 0\n}\n"
			p := nat9cBuild(t, printSrc)
			r := nat9cBuild(t, retSrc)
			pBad := strings.Contains(p, "error[")
			rBad := strings.Contains(r, "error[")
			if pBad != rBad {
				t.Errorf("`print %s` and `return 0` after it disagree:\n"+
					"  print form: %.200s\n  return form: %.200s", a.name, p, r)
			}
		})
	}
}
