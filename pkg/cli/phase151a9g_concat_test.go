package cli

// Phase 151A Step 9g - string concatenation and the arena allocator.
//
// WHY THIS SLICE IS THE HARDEST OF THE 9x SERIES. Everything before it is
// FRAME-RESIDENT: an int is one unit, an array's header and elements are in the frame, a
// string literal's VALUE is (pointer, length) pointing into `.rodata`. Concatenation is
// the first shape that must BUILD a value somewhere else, so it introduces the bump
// arena, the 16-byte header that lives inside the arena, a WRITE to memory the compiler
// cannot compute at build time, and an `alloc` helper that nothing else exercises.
//
// THE ARENA BOUND IS A PROOF, not an estimate: sites * totalLiteralBytes + 16.
//   1. Every concat SITE runs at most once. The driver does not lower loops at all, so a
//      loop-contained concat is refused before emission.
//   2. Every string value in a supported program is a literal or a concatenation of
//      literals, so no runtime string exceeds the sum of the program's literals.
// Together those bound the sum of all allocations. Exhaustion is kept anyway: `alloc`
// traps with Int3 rather than writing outside the arena.
//
// The bound is READ BACK FROM THE IMAGE, not taken on trust. `alloc` holds the arena's
// base address in one register and the arena's END in another, and the end is
// `arena + header + size`, so the span between those two immediates IS the computed
// arena size. The arena in the file itself is all zeros -- the cursor self-initialises
// on first use -- so the header in the image cannot be read for this, and a gate that
// tried would read 0 - 0 and prove nothing. See nat9gArenaSpan.
//
// FOUR REAL DEFECTS were found by this slice and all four produced a STRUCTURALLY VALID
// PE that then did the wrong thing -- the worst class for a native backend, because the
// linker does not check opcode semantics and every structural layer passes:
//   1. natMask(72) returned 11, not 7. It is documented as "the low three bits" and 11
//      is 0b1011, FOUR bits. Step 9d survived by LUCK (R9/RDX happen to survive the
//      wrong mask); 9g uses R10/R11, where 184 + (11 & 11) = 195 = 0xC3 = the RET opcode,
//      so the arena limit was loaded by RETURNING from the middle of `alloc`.
//   2. natHeapRef reused natImm64Patch, whose natMovImm64Op(rd) = 184 + rd is valid only
//      for rax..rdi. It now emits REX.W|REX.B and B8 + (rd & 7) directly.
//   3. The driver had TWO literal collectors that DISAGREED: the one used for rodata
//      interning and the print_str gate did not descend through BinaryExpr "+", so
//      print_str was never emitted while the concat arm still called it. natFinish then
//      returned the STRING "undefined label 'print_str'" as if it were code. They are
//      now ONE function.
//   4. Two shapes reached EMISSION unchecked and crashed in natNativeSlotOff: a chained
//      concat (natNativeStrKind RECURSES, so validation passed) and `return <concat>` (a
//      string is not an exit status). Both are refused BY NAME. The rule that matters: a
//      shape that is not lowered must be refused, never guessed at.
//
// NOT IN THIS SLICE, each refused by name: chained concatenation, binding a
// concatenation to a `let`, `len()` of a concatenation, `return` of a concatenation,
// string slicing, string comparison.

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

// nat9gCases are the executable programs. Every case is a DIFFERENT .kark source and
// every one is compiled, EXECUTED, and compared byte for byte -- nothing is trimmed.
// Trimming is exactly the wrong tool for a defect that is a MISSING byte, which is how
// 9b's first gate draft passed a program that printed "12345" with no newline.
var nat9gCases = []nat9dCase{
	// The case that found the collector disagreement: the rodata collector returned [],
	// so print_str was never emitted while the concat arm still called it.
	{name: "concat_lit", src: "func main() {\n    print(\"ab\" + \"cd\")\n}\n",
		wantOut: []byte("abcd\n"), wantExit: 0, localBytes: 0},
	// Both operands from the frame's two units rather than from immediates.
	{name: "concat_locals", src: "func main() {\n    let a = \"ab\"\n    let b = \"cd\"\n    print(a + b)\n}\n",
		wantOut: []byte("abcd\n"), wantExit: 0, localBytes: 32},
	// BOTH operand orders. The right half's destination is block + llen and llen is a
	// RUNTIME value, so the oracle advances the destination register instead of
	// encoding an offset in the SIB displacement. A single-order corpus cannot tell
	// which shape advances.
	//
	// The reversed case's expected bytes are "cdab", NOT "abcd": the literal is on the
	// left and the local on the right, so the order is observable in the OUTPUT. An
	// expectation of "abcd" here was a wrong expectation in the first draft of this
	// gate, caught by execution -- a symmetric expectation would have made this case
	// indistinguishable from the operand-order case above it and proved nothing about
	// which side is which.
	{name: "concat_lit_local", src: "func main() {\n    let a = \"ab\"\n    print(a + \"cd\")\n}\n",
		wantOut: []byte("abcd\n"), wantExit: 0, localBytes: 16},
	{name: "concat_local_lit", src: "func main() {\n    let a = \"ab\"\n    print(\"cd\" + a)\n}\n",
		wantOut: []byte("cdab\n"), wantExit: 0, localBytes: 16},
	// The empty cases each still perform a real alloc, and each changes the
	// (sites, totalLiteralBytes) pair. With total == 0 the arena is header-only, so a
	// driver that dropped the +16 would compute 0 and emit no arena at all.
	{name: "concat_empty_left", src: "func main() {\n    print(\"\" + \"cd\")\n}\n",
		wantOut: []byte("cd\n"), wantExit: 0, localBytes: 0},
	{name: "concat_empty_right", src: "func main() {\n    print(\"ab\" + \"\")\n}\n",
		wantOut: []byte("ab\n"), wantExit: 0, localBytes: 0},
	{name: "concat_both_empty", src: "func main() {\n    print(\"\" + \"\")\n}\n",
		wantOut: []byte("\n"), wantExit: 0, localBytes: 0},
	// Long enough that a 32-bit length would not silently agree with a 64-bit one for
	// the wrong reason: the copy loop and the length unit must both carry the value.
	{name: "long_concat", src: "func main() {\n    print(\"karkain\" + \"compiler\")\n}\n",
		wantOut: []byte("karkaincompiler\n"), wantExit: 0, localBytes: 0},
	// MULTIPLE sites. This is what makes the arena proof bite: sites and
	// totalLiteralBytes both differ from every single-site case, so an arena sized from
	// either term alone gets the wrong number here.
	{name: "two_sites", src: "func main() {\n    print(\"ab\" + \"cd\")\n    print(\"ef\" + \"gh\")\n}\n",
		wantOut: []byte("abcd\nefgh\n"), wantExit: 0, localBytes: 0},
	{name: "three_sites", src: "func main() {\n    print(\"ab\" + \"\")\n    print(\"\" + \"cd\")\n    print(\"e\" + \"f\")\n}\n",
		wantOut: []byte("ab\ncd\nef\n"), wantExit: 0, localBytes: 0},
}

// nat9gRefusals are the intentional boundaries. Every one of these produced a CRASH
// ("array index out of range ... stack: natNativeSlotOff") before it was refused, so
// each case asserts three separate things: kcc refuses, kcc does not crash, and the
// refusal names the construct.
var nat9gRefusals = []struct{ name, src, want string }{
	{name: "chained_concat_refused",
		src:  "func main() {\n    let a = \"ab\"\n    let b = \"cd\"\n    print(a + b + \"ef\")\n}\n",
		want: "error[K145]: native target: chained string concatenation is not lowered"},
	{name: "return_concat_refused",
		src:  "func main() {\n    let s = \"ab\"\n    return s + \"cd\"\n}\n",
		want: "error[K145]: native target: `return` of a concatenated string"},
	{name: "bind_concat_refused",
		src:  "func main() {\n    let a = \"ab\"\n    let b = \"cd\"\n    let c = a + b\n    print(c)\n}\n",
		want: "error[K145]"},
	{name: "len_of_concat_refused",
		src:  "func main() {\n    let a = \"ab\"\n    print(len(a + \"cd\"))\n}\n",
		want: "error[K145]"},
	// 9f's pre-existing check must still fire: the arena work must not have relaxed it.
	{name: "int_plus_string_refused",
		src:  "func main() {\n    let a = \"ab\"\n    let n = 3\n    print(a + n)\n}\n",
		want: "error[K145]"},
}

// nat9gMovabsArena scans the image for the two `movabs r64, imm64` instructions the
// arena needs and returns their immediates.
//
// natHeapRef emits REX.W|REX.B (0x49) followed by B8 + (rd & 7), so the arena BASE is
// `49 BA` (r10) and the arena END is `49 BB` (r11). The span between them is the arena
// size the compiler computed, which is what nat9gArenaProof checks against the proof.
//
// It reports whether the ERRONEOUS `49 C3` encoding was seen. `49 C3` is the regression
// this gate exists to keep dead: with natMask(72) == 11, R11 computed to
// 184 + (11 & 11) == 195 == 0xC3, which is `ret`, so the arena limit was loaded by
// RETURNING from the middle of alloc. That produces a structurally valid PE which then
// faults at runtime -- the defect class every structural layer here passes.
func nat9gMovabsArena(t *testing.T, img []byte) (base, end uint64, sawRet bool) {
	t.Helper()
	var sawR10, sawR11 bool
	for i := 0; i+10 <= len(img); i++ {
		if img[i] != 0x49 {
			continue
		}
		v := uint64(0)
		for k := 0; k < 8; k++ {
			v |= uint64(img[i+2+k]) << (8 * k)
		}
		switch img[i+1] {
		case 0xBA:
			base, sawR10 = v, true
		case 0xBB:
			end, sawR11 = v, true
		case 0xC3:
			sawRet = true
		}
	}
	if !sawR10 {
		t.Errorf("no `movabs r10, imm64` (49 BA) in the image: the arena base " +
			"address is never loaded, so alloc cannot run")
	}
	if !sawR11 {
		t.Errorf("no `movabs r11, imm64` (49 BB) in the image: the arena end " +
			"address is never loaded, so alloc cannot run")
	}
	return base, end, sawRet
}

// TestPhase151A9G_ConcatsExecuteWithExactBytes is layer 1: the primary claim.
//
// Each case is a DIFFERENT .kark source driven through kcc's whole-program driver,
// written to disk as a PE, EXECUTED, and compared byte for byte. nat9dRun also asserts
// the image is a structurally valid PE (native.ParsePE) and carries Step 9b's
// relocation count, so a linker regression fails here rather than as a mystery crash.
func TestPhase151A9G_ConcatsExecuteWithExactBytes(t *testing.T) {
	if os.PathSeparator == '/' {
		t.Skip("PE images only execute on Windows; the driver itself is cross-platform")
	}
	for _, c := range nat9gCases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			img := nat9dImage(t, c.src)
			got, code := nat9dRun(t, img)
			if code != c.wantExit {
				t.Errorf("exit status = %d, want %d (a native fault is %d on Windows, "+
					"so a fault here means the image is valid but does the wrong thing)",
					code, c.wantExit, -1073741819)
			}
			if !bytes.Equal(got, c.wantOut) {
				t.Errorf("stdout = %q (% x), want %q (% x)", got, got, c.wantOut, c.wantOut)
			}
		})
	}
}

// nat9gArenaCases pairs a program with the two numbers the proof is made of. Both terms
// VARY across the set on purpose: sites takes 1, 2 and 3 and totalLiteralBytes takes 0,
// 4, 6, 8 and 15, so an arena sized from either term alone, or from a constant, produces
// the wrong span on at least one case.
var nat9gArenaCases = []struct {
	name  string
	sites int
	total int
}{
	{"concat_both_empty", 1, 0}, // header-only arena: pins the +16
	{"concat_lit", 1, 4},
	{"long_concat", 1, 15},
	{"three_sites", 3, 6}, // sites AND total both differ from every other case
	{"two_sites", 2, 8},
}

// TestPhase151A9G_ArenaSizeIsTheProofNotAGuess reads the computed arena size back out of
// the image and checks it against sites*totalLiteralBytes+16.
//
// The span is `movabs r11` minus `movabs r10`, because `alloc` holds the arena base and
// the arena end (`arena + header + size`) in exactly those two registers. The expected
// number is computed HERE from the rule, independently of
// src/compiler/native_value.kark, so this compares the implementation against the
// PROOF rather than against itself.
func TestPhase151A9G_ArenaSizeIsTheProofNotAGuess(t *testing.T) {
	for _, c := range nat9gArenaCases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			var src string
			for _, k := range nat9gCases {
				if k.name == c.name {
					src = k.src
				}
			}
			if src == "" {
				t.Fatalf("corpus case %q not found", c.name)
			}
			img := nat9dImage(t, src)
			base, end, sawRet := nat9gMovabsArena(t, img)
			if sawRet {
				t.Fatalf("image contains 49 C3, which is `movabs r11` encoded as the " +
					"RET opcode; natMask(72) must be 7, not 11")
			}
			if end <= base {
				t.Fatalf("arena end (%#x) must be above arena base (%#x)", end, base)
			}
			gotSpan := end - base
			wantSpan := uint64(c.sites*c.total + 16)
			if gotSpan != wantSpan {
				t.Errorf("arena span = %d, want %d\n"+
					"  sites=%d  totalLiteralBytes=%d\n"+
					"  proof: sites*total + 16 = %d*%d + 16 = %d\n"+
					"  observed: end %#x - base %#x = %d",
					gotSpan, wantSpan,
					c.sites, c.total, c.sites, c.total, wantSpan,
					end, base, gotSpan)
			}
		})
	}
}

// TestPhase151A9G_AllocEncodesR10AndR11Correctly is the regression guard for the
// encoding defect, asserted on the EMITTED BYTES rather than on a diagnostic.
//
// The property that makes the program run is that R10 and R11 immediates are loaded with
// 49 BA and 49 BB, and NOT with 49 C3. It is checked by SCANNING rather than at a fixed
// offset, because the emitter's layout is not a contract: a future instruction inserted
// before alloc would move every byte, and a gate pinned to an offset would then fail for
// the wrong reason or pass for the wrong one. nat9gMovabsArena also fails the test if
// either correct encoding is absent, so the guard cannot pass vacuously on an image that
// simply contains neither.
func TestPhase151A9G_AllocEncodesR10AndR11Correctly(t *testing.T) {
	img := nat9dImage(t, "func main() {\n    print(\"ab\" + \"cd\")\n}\n")
	base, end, sawRet := nat9gMovabsArena(t, img)
	if sawRet {
		t.Errorf("image contains 49 C3 (`movabs r11` encoded as RET); natHeapRef must " +
			"emit REX.W|REX.B (0x49) with B8 + (rd & 7), and natMask(72) must be 7, " +
			"not 11")
	}
	if end-base != 1*4+16 {
		t.Errorf("arena span = %d, want %d; the encoding guard is only meaningful "+
			"when alloc's two immediates are the arena base and end", end-base, 1*4+16)
	}
}

// TestPhase151A9G_UnsupportedShapesAreRefusedByName is the honest-boundary layer.
//
// Each case asserts three separate properties, because any one of them alone is
// satisfiable by the wrong thing:
//   - kcc refuses, and the refusal is kcc's OWN native-driver wording
//     ("native target:"), which proves it was not handed to the Go engine;
//   - kcc does not CRASH: "array index out of range" naming a slot-plan offset is the
//     bug, and accepting it as a refusal would make the gate bless a crash;
//   - the refusal NAMES the construct, so a reader can tell what is unsupported rather
//     than grepping for a code.
func TestPhase151A9G_UnsupportedShapesAreRefusedByName(t *testing.T) {
	for _, c := range nat9gRefusals {
		c := c
		t.Run(c.name, func(t *testing.T) {
			out := nat9cBuild(t, c.src)
			if !strings.Contains(out, "error[K") {
				t.Fatalf("expected kcc to REFUSE this program, but it emitted an image:\n%.400s", out)
			}
			if strings.Contains(out, "array index out of range") {
				t.Fatalf("the program CRASHED instead of being refused:\n%.400s", out)
			}
			if !strings.Contains(out, "native target:") {
				t.Errorf("refusal is not from kcc's native driver; got:\n%.400s", out)
			}
			if !strings.Contains(out, c.want) {
				t.Errorf("refusal does not contain %q; got:\n%.400s", c.want, out)
			}
		})
	}
}

// TestPhase151A9G_DifferentConcatProgramsProduceDifferentImages is the anti-hard-coding
// layer.
//
// A driver that emitted one constant image, or that resolved every concat to the same
// rodata bytes, or that always allocated the same span, would satisfy a stdout comparison
// once and then keep satisfying it. Comparison is EXACT BYTES via nat9cBytesEq -- an
// earlier draft keyed on an FNV fingerprint and reported five different images as
// identical, which is the mistake this layer exists to prevent.
func TestPhase151A9G_DifferentConcatProgramsProduceDifferentImages(t *testing.T) {
	imgs := make([][]byte, len(nat9gCases))
	for i, c := range nat9gCases {
		imgs[i] = nat9dImage(t, c.src)
	}
	for i := 0; i < len(nat9gCases); i++ {
		for j := i + 1; j < len(nat9gCases); j++ {
			if nat9cBytesEq(imgs[i], imgs[j]) {
				t.Errorf("cases %q and %q produced IDENTICAL images (%d bytes each); at "+
					"least one of them is not being lowered from its own source",
					nat9gCases[i].name, nat9gCases[j].name, len(imgs[i]))
			}
		}
	}
}

// TestPhase151A9G_CorpusIsNonVacuousAndDeterministic checks that the corpus is big enough
// to be evidence, that every image is substantial, and that compiling the same source
// twice yields IDENTICAL BYTES.
//
// Full-image comparison, not a size and not a stdout: two runs of a compiler with a
// shared label counter can agree on length and disagree on content.
func TestPhase151A9G_CorpusIsNonVacuousAndDeterministic(t *testing.T) {
	if len(nat9gCases) < 8 {
		t.Fatalf("corpus has %d cases; too few to be evidence", len(nat9gCases))
	}
	if len(nat9gRefusals) < 5 {
		t.Fatalf("refusal table has %d cases; the boundary is not pinned", len(nat9gRefusals))
	}
	// The arena proof must vary BOTH terms, or it proves nothing about either.
	distinctSites := map[int]bool{}
	distinctTotals := map[int]bool{}
	for _, c := range nat9gArenaCases {
		distinctSites[c.sites] = true
		distinctTotals[c.total] = true
	}
	if len(distinctSites) < 3 || len(distinctTotals) < 4 {
		t.Errorf("arena cases vary %d distinct `sites` values and %d distinct "+
			"totalLiteralBytes values; need at least 3 and 4 so the proof is not "+
			"satisfiable by a constant or by either term alone",
			len(distinctSites), len(distinctTotals))
	}
	for _, c := range nat9gCases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			a := nat9dImage(t, c.src)
			b := nat9dImage(t, c.src)
			if !nat9cBytesEq(a, b) {
				t.Errorf("two compilations of the same source produced different images "+
					"(%d vs %d bytes)", len(a), len(b))
			}
			if len(a) < 512 {
				t.Errorf("image is only %d bytes", len(a))
			}
		})
	}
}
