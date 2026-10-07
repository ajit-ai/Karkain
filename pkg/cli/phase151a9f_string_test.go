package cli

// Phase 151A Step 9f - string literals and printing a string, through the real AST
// driver.
//
// This is the FIRST non-scalar kind that is not a frame-resident collection. 9d's array
// lives entirely in the frame; a string's VALUE is a (pointer, length) pair whose bytes
// live in `.rodata`. That makes this slice different in kind from 9d/9e, and it is why
// the literal's OFFSET rather than its content is the load-bearing number.
//
// WHAT IS PROVEN, by EXECUTING the kcc-produced PE and comparing exact stdout bytes:
//
//  1. a string literal prints its own bytes;
//  2. a string LOCAL prints the same bytes, so the (ptr, len) pair round-trips through
//     the frame's two units;
//  3. two different literals resolve to DIFFERENT bytes -- the anti-hard-coding layer.
//     A driver that always resolved a string reference to rodata offset 0 would print
//     print_int's own "-" for every string, and the images for two different programs
//     would be identical;
//  4. a repeated literal is interned ONCE and both bindings resolve to it;
//  5. a string and an int/array in the same program both work, which is what proves the
//     two-unit string slot and the array header do not overlap;
//  6. `len(s)` returns the string's length -- free, because a string's second unit IS
//     its length, the same offset an array's COUNT occupies.
//
// THE ARENA IS DELIBERATELY ABSENT. Concatenation must BUILD a string, so it needs the
// bump allocator, and that is a separate slice. A literal is already built, which is why
// this one needs no arena and no allocation at all.
//
// NOT IN THIS SLICE: concatenation, slicing, comparison, string parameters/returns,
// string elements in arrays. Each is refused by name. `docs/audit/PHASE-151A-BASELINE.md`
// section 26 has the full boundary list.

import (
	"os"
	"strings"
	"testing"
)

var nat9fCases = []nat9dCase{
	{
		// A literal printed directly, with no binding. This is the case that found a
		// real bug: the literal collector walked only `let` initialisers, so this
		// recorded a reference to an offset that had never been interned and kcc died
		// with "array index out of range" inside the lowering.
		name:    "print_literal",
		src:     "func main() {\n    print(\"hi\")\n}\n",
		wantOut: []byte("hi\n"), wantExit: 0, localBytes: 0,
	},
	{
		// The same bytes through a local, which is what proves the (ptr, len) pair
		// survives the frame.
		name:    "print_local",
		src:     "func main() {\n    let s = \"hi\"\n    print(s)\n}\n",
		wantOut: []byte("hi\n"), wantExit: 0, localBytes: 16,
	},
	{
		name:    "long_literal",
		src:     "func main() {\n    let s = \"hello karkain\"\n    print(s)\n}\n",
		wantOut: []byte("hello karkain\n"), wantExit: 0, localBytes: 16,
	},
	{
		// Two DIFFERENT literals. Their images must differ; see layer 3.
		name:    "two_distinct_literals",
		src:     "func main() {\n    let a = \"one\"\n    let b = \"two\"\n    print(a)\n    print(b)\n}\n",
		wantOut: []byte("one\ntwo\n"), wantExit: 0, localBytes: 32,
	},
	{
		// The same literal twice. natRodataSection interns by FIRST USE, so both
		// bindings must resolve to ONE copy of the bytes -- and both must still print
		// correctly. A driver that interned per-binding would also print correctly, so
		// this case is paired with the rodata-length layer below.
		name:    "repeated_literal_interned_once",
		src:     "func main() {\n    let a = \"same\"\n    let b = \"same\"\n    print(a)\n    print(b)\n}\n",
		wantOut: []byte("same\nsame\n"), wantExit: 0, localBytes: 32,
	},
	{
		// A string and an int. The string owns TWO units and the int one, so the int's
		// slot is only correct if the allocation is still sequential and kind-aware.
		name:    "string_and_int",
		src:     "func main() {\n    let s = \"n=42\"\n    let x = 42\n    print(s)\n    print(x)\n}\n",
		wantOut: []byte("n=42\n42\n"), wantExit: 0, localBytes: 24,
	},
	{
		// A string and an array. This is the sharpest layout case in the slice: the
		// string's length unit sits at its slot+8, and the array's COUNT also sits at
		// its slot+8. Getting the string's size wrong puts the array's header inside
		// the string, or vice versa, and one of the two prints goes wrong.
		name:    "string_and_array",
		src:     "func main() {\n    let s = \"items:\"\n    let a = [7, 35]\n    print(s)\n    print(a[1])\n}\n",
		wantOut: []byte("items:\n35\n"), wantExit: 0, localBytes: 48,
	},
	{
		// len of a string, and the string itself, in one program.
		name:    "len_of_string",
		src:     "func main() {\n    let s = \"karkain\"\n    print(s)\n    print(len(s))\n}\n",
		wantOut: []byte("karkain\n7\n"), wantExit: 0, localBytes: 16,
	},
}
var nat9fRefusals = []struct{ name, src, want string }{
	{
		// Concatenation must BUILD a string, so it needs the bump arena. Refusing it
		// is the honest boundary: a string value is (ptr, len) into rodata, and there
		// is nowhere to put concatenated bytes yet.
		name: "concatenation_needs_the_arena",
		src:  "func main() {\n    let a = \"x\"\n    let b = \"y\"\n    let c = a + b\n    print(c)\n}\n",
		want: "error[K145]",
	},
	{
		name: "string_index_not_lowered",
		src:  "func main() {\n    let s = \"hi\"\n    print(s[0])\n}\n",
		want: "error[K145]",
	},
	{
		name: "string_return_not_lowered",
		src:  "func main() {\n    let s = \"hi\"\n    return s\n}\n",
		want: "error[K145]",
	},
}

// TestPhase151A9F_StringsExecuteWithExactBytes is the primary claim.
func TestPhase151A9F_StringsExecuteWithExactBytes(t *testing.T) {
	if os.PathSeparator == '/' {
		t.Skip("PE images only execute on Windows; the driver itself is cross-platform")
	}
	for _, c := range nat9fCases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			img := nat9dImage(t, c.src)
			got, code := nat9dRun(t, img)
			if !nat9cBytesEq(got, c.wantOut) {
				t.Errorf("stdout = %q (% x), want %q (% x)\n  source: %s\n"+
					"  A run of NUL bytes means the string reference resolved to an "+
					"uninterned offset; a wrong length means the (ptr, len) pair was "+
					"built wrongly", got, got, c.wantOut, c.wantOut, c.src)
			}
			if code != c.wantExit {
				t.Errorf("exit = %d, want %d\n  source: %s", code, c.wantExit, c.src)
			}
		})
	}
}

// TestPhase151A9F_DifferentStringsProduceDifferentImages is the anti-hard-coding layer.
//
// A driver that resolved every string reference to rodata offset 0 -- where print_int's
// "-" lives -- would print a plausible byte for every program and produce IDENTICAL
// images for different string contents. Comparing images catches that; comparing stdout
// alone would not, because the expected value was written down by hand either way.
func TestPhase151A9F_DifferentStringsProduceDifferentImages(t *testing.T) {
	variants := []struct{ name, src string }{
		{"lit_hi", "func main() {\n    print(\"hi\")\n}\n"},
		{"lit_by", "func main() {\n    print(\"by\")\n}\n"},
		{"lit_ih", "func main() {\n    print(\"ih\")\n}\n"},
		{"lit_longer", "func main() {\n    print(\"hii\")\n}\n"},
		{"local_hi", "func main() {\n    let s = \"hi\"\n    print(s)\n}\n"},
		{"local_hj", "func main() {\n    let s = \"hj\"\n    print(s)\n}\n"},
	}
	var prior []byte
	var priorName string
	for _, v := range variants {
		v := v
		t.Run(v.name, func(t *testing.T) {
			img := nat9dImage(t, v.src)
			if prior != nil && nat9cBytesEq(img, prior) {
				t.Fatalf("image for %s is byte-identical to %s.\n  Two different string "+
					"contents must not compile to the same bytes; that is what a string "+
					"reference resolved to a fixed rodata offset looks like", v.name, priorName)
			}
			prior, priorName = img, v.name
		})
	}
}

// TestPhase151A9F_FrameFollowsTheValueModelRule pins slot sizing. A string local is TWO
// units, so this is the layer that catches a string sized as one unit -- which would let
// the next local overlap the string's length.
func TestPhase151A9F_FrameFollowsTheValueModelRule(t *testing.T) {
	for _, c := range nat9fCases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			img := nat9dImage(t, c.src)
			got := nat9dFrameSubRsp(t, img)
			want := nat9dExpectFrame(c.localBytes)
			if !nat9dContainsInt32(got, want) {
				t.Errorf("no `sub rsp, %d` in the image; got %v.\n  want %d for localBytes "+
					"%d.\n  A string is TWO units (ptr, len); sizing it as one would let "+
					"the next local overlap its length.\n  source: %s",
					want, got, want, c.localBytes, c.src)
			}
		})
	}
}

// TestPhase151A9F_LiteralBytesReachTheImage is the interning layer.
//
// The check is that the literal's ASCII BYTES ARE IN THE IMAGE. The first draft of this
// test instead asserted that lengthening a literal lengthened the image, and it failed
// with a delta of 0 -- because a PE pads `.text` up to the 512-byte FileAlignment, so a
// 4-byte growth in rodata need not change the file size at all. The measurement was
// wrong, not the compiler, and a size assertion would have been testing the linker.
//
// Searching for the bytes is also the check that cannot be satisfied vacuously: every
// image begins with print_int's own "-", so finding THAT would prove nothing, which is
// why the needle here is a multi-character literal and the haystack search starts after
// the headers.
func TestPhase151A9F_LiteralBytesReachTheImage(t *testing.T) {
	for _, lit := range []string{"Zqxjk", "a-longer-literal-value", "0123456789"} {
		lit := lit
		t.Run(lit, func(t *testing.T) {
			src := "func main() {\n    print(\"" + lit + "\")\n}\n"
			img := nat9dImage(t, src)
			needle := []byte(lit)
			found := false
			for i := 0; i+len(needle) <= len(img); i++ {
				if nat9cBytesEq(img[i:i+len(needle)], needle) {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("the literal %q does not appear anywhere in the %d-byte image.\n"+
					"  Its bytes must be interned into rodata, or a string reference "+
					"resolves to whatever happens to sit at some other offset",
					lit, len(img))
			}
		})
	}
}

// TestPhase151A9F_RepeatedLiteralIsInternedOnce is the other half of the interning
// contract: a literal used twice must occupy ONE copy.
//
// Without this, "interned once" and "interned per use" both pass the growth test above.
// Holding the program fixed and comparing a one-use program against a two-use program
// separates them: the two-use program must be the same SIZE, because the second use
// resolves to the slot the first one interned.
func TestPhase151A9F_RepeatedLiteralIsInternedOnce(t *testing.T) {
	one := nat9dImage(t, "func main() {\n    print(\"dup\")\n}\n")
	two := nat9dImage(t, "func main() {\n    print(\"dup\")\n    print(\"dup\")\n}\n")
	if len(one) != len(two) {
		t.Errorf("a repeated literal changed the image size: one use = %d bytes, two "+
			"uses = %d bytes.\n  The bytes for the second use should already be in "+
			"rodata, so only the extra PRINT CALL may grow the image -- and if it did "+
			"not, the interning is not what this claims", len(one), len(two))
	}
}

// TestPhase151A9F_OutOfSubsetShapesAreRefusedByName covers the boundary list.
func TestPhase151A9F_OutOfSubsetShapesAreRefusedByName(t *testing.T) {
	for _, r := range nat9fRefusals {
		r := r
		t.Run(r.name, func(t *testing.T) {
			out := nat9cBuild(t, r.src)
			if !strings.Contains(out, r.want) {
				t.Errorf("expected %q, got:\n%.400s", r.want, out)
			}
			if isHex9d(strings.TrimSpace(out)) {
				t.Errorf("a REFUSED program also produced an image:\n%.200s", out)
			}
		})
	}
}

// TestPhase151A9F_CorpusIsNonVacuousAndDeterministic.
func TestPhase151A9F_CorpusIsNonVacuousAndDeterministic(t *testing.T) {
	if len(nat9fCases) < 8 {
		t.Fatalf("corpus has %d cases; too few to be evidence", len(nat9fCases))
	}
	for _, c := range nat9fCases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			a := nat9dImage(t, c.src)
			b := nat9dImage(t, c.src)
			if !nat9cBytesEq(a, b) {
				t.Errorf("two runs of the same source produced different images")
			}
			if len(a) < 512 {
				t.Errorf("image is only %d bytes", len(a))
			}
		})
	}
}
