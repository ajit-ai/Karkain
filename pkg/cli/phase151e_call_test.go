package cli

import (
	"encoding/binary"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"karkain/pkg/native"
)

// NAT-CALL -- a real .kark function called from main, lowered by kcc's native
// driver and EXECUTED through the C-free path.
//
// This is the v1.3.0 opener, and its definition of done
// (docs/release/v1.3.0-CHECKLIST.md section 1 item 1) has three parts:
//
//  1. a real .kark function called from main builds and EXECUTES through the
//     C-free native path on both engines;
//  2. the refusal table shrinks BY NAME rather than by omission;
//  3. kcc and the Go oracle emit byte-identical images for the covered shapes.
//
// The gate is therefore organised as four independent layers, each able to fail
// while the others pass:
//
//	layer 1  EXECUTED goldens           -- exact stdout bytes + exit code, PE on
//	                                       this Windows host
//	layer 2  Linux ELF byte identity    -- a DIFFERENTIAL against the real Go
//	                                       oracle, plus kcc provenance
//	layer 3  frame numbers              -- derived independently from the rule,
//	                                       not read back from the compiler
//	layer 4  Windows-only call alignment -- `sub rsp,8` counts
//
// WHY LAYER 2 IS A DIFFERENTIAL AND NOT A GOLDEN. A golden pins the bytes of the
// engine that produced it, so it would pass with a single buggy emitter and
// would have frozen the 39-byte divergence that Phase D1/D2/D3 spent three
// slices removing. Calling the real oracle inside the test means the assertion
// can only pass if BOTH engines agree, and neither can be "pinned" as truth.
//
// WHY LAYER 3 EXISTS AT ALL, given layer 2. Byte identity proves the two engines
// agree; it cannot prove they agree on the RIGHT answer, because both could
// round a frame the same wrong way. The frame rule is therefore restated here,
// independently of src/compiler/native_value.kark and of pkg/native, and checked
// against both images:
//
//	localBytes     = 8 * (parameters + let bindings)   every local is one int unit
//	+ 512          = bin temporary area (64 slots * 8)
//	+ 8 * extras   = argument units beyond the six register slots
//	Windows: + 8 when the running total is not 16-byte aligned
//	+ 96           = argument spill area (6 * 16)
//
// Layer 3 is what caught the extras bug while this slice was being built: the
// first draft reserved `units` bytes instead of `units - 6`, which put 56 bytes
// of extras on a seven-argument caller where the oracle reserves 8. Layer 2 also
// caught it, but layer 3 named WHICH function was wrong and by how much.
//
// WHY LAYER 4 EXISTS. Win64 requires 16-byte alignment at a call, so a
// `sub rsp,8` precedes and an `add rsp,8` follows a call on Windows ONLY. Linux
// and macOS are entered with rsp%16 == 8 and a call leaves the callee aligned,
// so the sequence must be ABSENT there. This is not cosmetic: Phase D2 found the
// unconditional version shipping on all three targets, where it MISALIGNED the
// callee on Linux/macOS and still produced a working program, because no callee
// in the subset touches an aligned SSE slot. A byte gate would have caught it;
// an execution gate alone could not. Counting the sequence on both sides makes
// the asymmetry an assertion rather than a claim.
//
// EXECUTION EVIDENCE, stated precisely: the PE images are executed on this
// windows/amd64 host and the stdout is compared as EXACT BYTES (a missing
// trailing newline is a real defect, and trimming is what hid the Step 9b
// relocation bug). The ELF images are never run here -- there is no WSL and no
// Docker -- so every Linux claim in this file is structural plus byte identity,
// and the Linux CI leg is where an ELF built from these exact bytes executes.
//
// WHAT IS DELIBERATELY NOT COVERED, so the gap is not inferred from a green run:
// `/`, `%`, comparisons, `&&`/`||`/`!` on integers; `if`, `while`, `for`,
// `for-in`, `break`, `continue`; string, array, record and map values inside a
// callee; non-int parameters and returns; a call as a `let` initialiser in main
// (`let c = f(1)`, frozen by Step 9c); and an argument at index >= 1 that itself
// contains a call (the spill area is reused across calls). Every one of those is
// in the refusal table below BY NAME, which is the point of layer 5.

// natEFuncFrame is one function's contribution to the frame, stated as the two
// inputs the rule needs rather than as the frame number itself.
type natEFuncFrame struct {
	locals int // parameters + `let` bindings, one 8-byte unit each
	extras int // argument units beyond the six register slots
}

// natECase is one program in the NAT-CALL corpus.
type natECase struct {
	name string
	src  string
	// wantHex is the EXACT stdout in hex. "" means the program must print
	// nothing, which is a real expectation for two cases (main_return exits
	// with a status and prints nothing; int_call_as_stmt discards a result).
	wantHex  string
	wantExit int
	// funcs lists every function in SOURCE order, main last. The driver emits
	// functions in source order, so the frame scan sees them in this order.
	funcs []natEFuncFrame
	// winSub8 is the number of `sub rsp,8` sequences expected in the PE .text:
	// one per user CALL SITE plus one per PRINT SITE, on Windows only. SITES
	// means emission sites, not dynamic executions -- a print inside a callee
	// executed twice from a loop-free body is still emitted once, and a nested
	// call `inc(inc(1))` is two sites. Counting the source is therefore possible
	// but easy to get wrong in exactly those two ways, which is why every count
	// below was measured rather than derived.
	winSub8 int
	// quirk records a measured PARITY LIMITATION this case pins rather than
	// endorses. Non-empty means the program exercises a pre-existing checker
	// gap that BOTH engines accept identically; the case is here so a future
	// divergence is loud, not so the behaviour is claimed to be correct.
	quirk string
}

// natECorpus is measured, not invented: every wantHex, wantExit, frame list and
// winSub8 count below was read off the committed compiler before this file was
// written, and each case is a DIFFERENT .kark source.
var natECorpus = []natECase{
	{
		// The minimum claim: a two-parameter function, a call, a print.
		name:     "call_add",
		src:      "func add(a int, b int) int { return a + b }\nfunc main() { print(add(2, 3)) }\n",
		wantHex:  "350a",
		funcs:    []natEFuncFrame{{2, 0}, {0, 0}},
		winSub8:  2,
		wantExit: 0,
	},
	{
		// Two call SITES in one function: the staging path must be re-entered,
		// not amortised across calls.
		name: "two_calls",
		src: "func dbl(n int) int { return n * 2 }\n" +
			"func main() {\n    print(dbl(4))\n    print(dbl(8))\n}\n",
		wantHex:  "380a31360a",
		funcs:    []natEFuncFrame{{1, 0}, {0, 0}},
		winSub8:  4,
		wantExit: 0,
	},
	{
		// A callee calling a callee: `inc` is emitted before `twice`, so the
		// call inside `twice` resolves against an already-marked label, and
		// the argument is itself a call (argument index 0, which is allowed).
		name: "nested",
		src: "func inc(n int) int { return n + 1 }\n" +
			"func twice(n int) int { return inc(inc(n)) }\n" +
			"func main() { print(twice(40)) }\n",
		wantHex:  "34320a",
		funcs:    []natEFuncFrame{{1, 0}, {1, 0}, {0, 0}},
		winSub8:  4,
		wantExit: 0,
	},
	{
		// A callee with its OWN let binding: the callee's frame is wider than
		// its parameter list, and the local's slot must not alias the parameter.
		name: "callee_local",
		src: "func f(n int) int {\n    let m = n * 2\n    return m + 1\n}\n" +
			"func main() { print(f(5)) }\n",
		wantHex:  "31310a",
		funcs:    []natEFuncFrame{{2, 0}, {0, 0}},
		winSub8:  2,
		wantExit: 0,
	},
	{
		// SEVEN arguments: units 6 and 7 leave the register budget and travel
		// in the caller's extras area, so the CALLER's frame grows by 8 and
		// nothing changes in the callee's. This is the case that caught the
		// extras = units-6 bug.
		name: "seven",
		src: "func s(a int, b int, c int, d int, e int, f int, g int) int { return a + g }\n" +
			"func main() { print(s(1, 2, 3, 4, 5, 6, 7)) }\n",
		wantHex:  "380a",
		funcs:    []natEFuncFrame{{7, 0}, {0, 1}},
		winSub8:  2,
		wantExit: 0,
	},
	{
		// Unary minus on a parameter: `natEmitIntExpr`'s unary arm, reached
		// through a call boundary.
		name:     "unary",
		src:      "func neg(n int) int { return -n }\nfunc main() { print(neg(9)) }\n",
		wantHex:  "2d390a",
		funcs:    []natEFuncFrame{{1, 0}, {0, 0}},
		winSub8:  2,
		wantExit: 0,
	},
	{
		// Precedence across three parameters: `a * b + c` is 10, and a
		// left-to-right evaluator that ignored precedence would print 20.
		name: "prec",
		src: "func g(a int, b int, c int) int { return a * b + c }\n" +
			"func main() { print(g(2, 3, 4)) }\n",
		wantHex:  "31300a",
		funcs:    []natEFuncFrame{{3, 0}, {0, 0}},
		winSub8:  2,
		wantExit: 0,
	},
	{
		// A call in RETURN position. The exit status is the ONLY observable,
		// which is exactly why it is asserted: a driver that dropped the
		// return value would exit 0 and print nothing, and stdout alone could
		// not tell the two apart.
		name:     "main_return",
		src:      "func one(n int) int { return n }\nfunc main() { return one(1) }\n",
		wantHex:  "",
		funcs:    []natEFuncFrame{{1, 0}, {0, 0}},
		winSub8:  1,
		wantExit: 1,
	},
	{
		// A callee that prints and returns nothing, called as a STATEMENT.
		name:     "callee_prints",
		src:      "func show(x int) { print(x) }\nfunc main() { show(7) }\n",
		wantHex:  "370a",
		funcs:    []natEFuncFrame{{1, 0}, {0, 0}},
		winSub8:  2,
		wantExit: 0,
	},
	{
		// The same seven-argument extras pressure on a VOID callee, so the
		// extras mechanism is exercised on a path that does not return a value.
		name: "stmt_seven",
		src: "func s(a int, b int, c int, d int, e int, f int, g int) { print(a + g) }\n" +
			"func main() { s(1, 2, 3, 4, 5, 6, 7) }\n",
		wantHex:  "380a",
		funcs:    []natEFuncFrame{{7, 0}, {0, 1}},
		winSub8:  2,
		wantExit: 0,
	},
	{
		// A two-level void call chain: main calls `twice`, `twice` calls
		// `show` twice. The output order is the call order, so a driver that
		// reordered emission would still compile and print 41 then 40.
		name: "void_chain",
		src: "func show(x int) { print(x) }\n" +
			"func twice(n int) {\n    show(n)\n    show(n + 1)\n}\n" +
			"func main() { twice(40) }\n",
		wantHex:  "34300a34310a",
		funcs:    []natEFuncFrame{{1, 0}, {1, 0}, {0, 0}},
		winSub8:  4,
		wantExit: 0,
	},
	{
		// A call forwarded straight through a parameter at index 0: the value
		// passes through two frames with no local to hold it.
		name: "arg0_call",
		src: "func inc(n int) int { return n + 1 }\n" +
			"func wrap(n int) int { return inc(n) }\n" +
			"func main() { print(wrap(3)) }\n",
		wantHex:  "340a",
		funcs:    []natEFuncFrame{{1, 0}, {1, 0}, {0, 0}},
		winSub8:  3, // wrap + inc + print
		wantExit: 0,
	},
	{
		// index 0 of a call argument is itself a call -- the one nesting shape
		// the spill area permits, because nothing earlier needs to survive it.
		name: "nested_arg",
		src: "func inc(n int) int { return n + 1 }\n" +
			"func main() { print(inc(inc(1))) }\n",
		wantHex:  "330a",
		funcs:    []natEFuncFrame{{1, 0}, {0, 0}},
		winSub8:  3, // two inc sites + print
		wantExit: 0,
	},
	{
		// A call whose RESULT is an argument to another call, plus a second
		// direct call. `show(id(3))` is argument index 0 and `print(id(4))`
		// is a print of a call, so both call-bearing shapes appear once.
		name: "void_then_call",
		src: "func show(x int) { print(x) }\nfunc id(n int) int { return n }\n" +
			"func main() {\n    show(id(3))\n    print(id(4))\n}\n",
		wantHex:  "330a340a",
		funcs:    []natEFuncFrame{{1, 0}, {1, 0}, {0, 0}},
		winSub8:  5,
		wantExit: 0,
	},
	{
		// A zero-parameter function: the staging loop must handle an empty
		// argument list, and the frame must NOT grow.
		name:     "zero_arg",
		src:      "func z() int { return 7 }\nfunc main() { print(z()) }\n",
		wantHex:  "370a",
		funcs:    []natEFuncFrame{{0, 0}, {0, 0}},
		winSub8:  2,
		wantExit: 0,
	},
	{
		// PARITY LIMITATION, pinned not endorsed: a void call in VALUE
		// position. Neither checker rejects it, and both engines read the
		// callee's implicit `return 0`, so this prints the callee's own `1`
		// and then `0`. It was measured on the Go oracle FIRST (same source,
		// C path, same two lines) before being written down, so this is a
		// pre-existing language gap rather than a NAT-CALL behaviour. Closing
		// it means changing BOTH checkers, which is its own slice.
		name:    "void_in_value",
		src:     "func v(n int) { print(n) }\nfunc main() { print(v(1)) }\n",
		wantHex: "310a300a",
		funcs:   []natEFuncFrame{{1, 0}, {0, 0}},
		winSub8: 3,
		quirk: "a void call in value position is accepted by both checkers and " +
			"yields the callee's implicit 0",
	},
	{
		// The same gap with an empty callee, so the value cannot be confused
		// with the callee's own output: exactly `0\n`.
		name:    "void_empty_in_value",
		src:     "func v(n int) { }\nfunc main() { print(v(1)) }\n",
		wantHex: "300a",
		funcs:   []natEFuncFrame{{1, 0}, {0, 0}},
		winSub8: 2,
		quirk:   "void_in_value",
	},
	{
		// A value-returning call used as a STATEMENT: the result is discarded
		// and stdout is empty. Byte identity still applies, so this pins that
		// a discarded result is lowered identically rather than elided.
		name:     "int_call_as_stmt",
		src:      "func f(n int) int { return n + 1 }\nfunc main() { f(1) }\n",
		wantHex:  "",
		funcs:    []natEFuncFrame{{1, 0}, {0, 0}},
		winSub8:  1,
		wantExit: 0,
	},
	{
		// A void call as a statement, which IS the supported shape for a
		// callee that only prints.
		name:     "void_call_as_stmt",
		src:      "func v(n int) { print(n) }\nfunc main() { v(9) }\n",
		wantHex:  "390a",
		funcs:    []natEFuncFrame{{1, 0}, {0, 0}},
		winSub8:  2,
		wantExit: 0,
	},
	{
		// `return v(1)` where `v` returns nothing: the callee prints, and main
		// exits 0 with the implicit zero. Same checker gap as void_in_value,
		// one position further out, and identical on both engines.
		name:     "return_void_call",
		src:      "func v(n int) { print(n) }\nfunc main() { return v(1) }\n",
		wantHex:  "310a",
		funcs:    []natEFuncFrame{{1, 0}, {0, 0}},
		winSub8:  2,
		wantExit: 0,
	},
}

// natEFrameExpect is the frame rule above, written from the rule rather than
// from the compiler. gcc/ld never see this number, so a disagreement means one
// of the two derivations is wrong -- which is the whole point of a third layer.
func natEFrameExpect(f natEFuncFrame, windows bool) int32 {
	next := f.locals*8 + 512 + f.extras*8
	if windows && next%16 != 0 {
		next += 8
	}
	return int32(next + 96)
}

// natEHex decodes a hex expectation and fails loudly rather than silently
// comparing against an empty slice, which would make an empty expectation pass
// for a program that printed nothing at all.
func natEHex(t *testing.T, s, what string) []byte {
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

// natEPETextRange returns the raw file offset and size of a PE's .text.
func natEPETextRange(t *testing.T, img []byte) (int, int) {
	t.Helper()
	lfanew := binary.LittleEndian.Uint32(img[0x3c:])
	numSec := int(binary.LittleEndian.Uint16(img[lfanew+4+2:]))
	optSize := int(binary.LittleEndian.Uint16(img[lfanew+4+16:]))
	secTab := int(lfanew) + 4 + 20 + optSize
	for i := 0; i < numSec; i++ {
		s := secTab + i*40
		if string(img[s:s+5]) == ".text" {
			return int(binary.LittleEndian.Uint32(img[s+20:])),
				int(binary.LittleEndian.Uint32(img[s+16:]))
		}
	}
	t.Fatal("no .text section in the PE")
	return 0, 0
}

// natEFramesOrdered returns the imm32 of every `sub rsp, imm32` (REX.W 81 /5,
// i.e. 48 81 EC) in [start, start+size), in FILE ORDER and WITHOUT sorting.
//
// Order matters here in a way it does not for Step 9d's helper: with more than
// one function in the image, the sequence of frames is the sequence of
// functions, so a driver that emitted main before its callees would produce the
// same SET and a different LIST. Sorting would hide exactly that.
func natEFramesOrdered(t *testing.T, img []byte, start, size int) []int32 {
	t.Helper()
	if size <= 0 || start+size > len(img) {
		t.Fatalf("frame scan range [%d,%d) is outside a %d-byte image", start, start+size, len(img))
	}
	var out []int32
	end := start + size
	for i := start; i+6 <= end; i++ {
		if img[i] == 0x48 && img[i+1] == 0x81 && img[i+2] == 0xEC {
			out = append(out, int32(binary.LittleEndian.Uint32(img[i+3:])))
		}
	}
	return out
}

// natECountSeq counts non-overlapping occurrences of pat inside [start,size).
func natECountSeq(img []byte, start, size int, pat []byte) int {
	n := 0
	end := start + size
	for i := start; i+len(pat) <= end; i++ {
		ok := true
		for k := range pat {
			if img[i+k] != pat[k] {
				ok = false
				break
			}
		}
		if ok {
			n++
		}
	}
	return n
}

// natEWinCallAlign and natEWinCallAlignUndo are the Win64 alignment pair a call
// site emits on Windows ONLY: `sub rsp,8` before and `add rsp,8` after.
var (
	natEWinCallAlign     = []byte{0x48, 0x83, 0xEC, 0x08}
	natEWinCallAlignUndo = []byte{0x48, 0x83, 0xC4, 0x08}
)

// natEImage compiles one source through the kcc native driver and returns the
// PE image, failing if the driver refused it or emitted something that is not a
// PE. It goes through `native-ast`, the driver entry point, so a failure here is
// a driver failure and never a CLI-dispatch artefact.
func natEImage(t *testing.T, src string) []byte {
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

// TestPhaseNATCall_ProgramsExecuteWithExactBytes is layer 1.
//
// Each corpus entry is a different .kark file compiled through kcc's native
// driver, written to disk and EXECUTED. stdout is compared as exact bytes and
// the exit code is compared separately, because two of the cases are observable
// only through one of the two (main_return prints nothing; int_call_as_stmt
// exits 0 with no output at all).
func TestPhaseNATCall_ProgramsExecuteWithExactBytes(t *testing.T) {
	if os.PathSeparator == '/' {
		t.Skip("PE images only execute on Windows; every other layer is cross-platform")
	}
	if len(natECorpus) < 15 {
		t.Fatalf("corpus has %d cases; NAT-CALL was measured over at least 15 and a "+
			"shrinking corpus is how a gate stops being evidence", len(natECorpus))
	}

	for _, c := range natECorpus {
		c := c
		t.Run(c.name, func(t *testing.T) {
			img := natEImage(t, c.src)
			got, code := nat9dRun(t, img)
			want := natEHex(t, c.wantHex, c.name+" expected stdout")
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

// TestPhaseNATCall_LinuxELFIsByteIdenticalToTheOracle is layer 2, and it is the
// direct evidence for the third part of the slice's definition of done.
//
// Both engines build the SAME source for native-x86_64-linux and the image bytes
// are compared. The kcc outcome must also carry the provenance marker, which
// only src/compiler/main.kark can print -- so a build that quietly fell through
// to the Go backend would agree byte for byte and still fail here.
func TestPhaseNATCall_LinuxELFIsByteIdenticalToTheOracle(t *testing.T) {
	karkain := phase130Karkain(t)

	for _, c := range natECorpus {
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

// TestPhaseNATCall_FramesFollowTheDerivedRule is layer 3.
//
// The frame of every function, in emission order, is compared against a number
// derived from the rule and the function's own source shape -- never against a
// number read back from kcc or from pkg/native. Both images are checked, and the
// Windows and Linux lists must differ only in the 16-byte rounding.
func TestPhaseNATCall_FramesFollowTheDerivedRule(t *testing.T) {
	karkain := phase130Karkain(t)
	sawWindowsRounding := false

	for _, c := range natECorpus {
		c := c
		t.Run(c.name, func(t *testing.T) {
			// A local list long enough to derive every expectation before any
			// image is read, so a construction failure cannot be mistaken for
			// an emission difference.
			var wantWin, wantLin []int32
			for _, f := range c.funcs {
				wantWin = append(wantWin, natEFrameExpect(f, true))
				wantLin = append(wantLin, natEFrameExpect(f, false))
				if natEFrameExpect(f, true) != natEFrameExpect(f, false) {
					sawWindowsRounding = true
				}
			}

			peImg := natEImage(t, c.src)
			peOff, peSize := natEPETextRange(t, peImg)
			gotWin := natEFramesOrdered(t, peImg, peOff, peSize)
			if len(gotWin) != len(wantWin) || !natEInt32SlicesEq(gotWin, wantWin) {
				t.Errorf("PE frames = %v, want %v (source order, main last)\n  source:\n%s\n"+
					"  Derived from: 8*locals + 512 + 8*extras, +8 when not 16-aligned "+
					"on Windows, +96 spill.", gotWin, wantWin, c.src)
			}

			kccRes := nativeBuildWithEngine(t, karkain, c.src, NativeLinuxTarget, "kcc", c.name+"-frames.elf")
			if kccRes.ExitCode != 0 || !kccRes.WroteImage {
				t.Fatalf("kcc produced no ELF (exit %d):\n%s", kccRes.ExitCode, kccRes.Output)
			}
			// The whole file is scanned for the ELF because the writer emits no
			// section table: the executable PT_LOAD is laid out directly after
			// the headers and .text follows them. A spurious hit inside the
			// headers or the read-only data would show up as an extra frame and
			// fail the ordered comparison, so the scan is safe as a CHECK even
			// though it is not a parser.
			gotLin := natEFramesOrdered(t, kccRes.Image, 0, len(kccRes.Image))
			if len(gotLin) != len(wantLin) || !natEInt32SlicesEq(gotLin, wantLin) {
				t.Errorf("ELF frames = %v, want %v (source order, main last)\n  source:\n%s",
					gotLin, wantLin, c.src)
			}
		})
	}

	if !sawWindowsRounding {
		t.Error("no corpus case had a frame that the Windows 16-byte rounding moves; " +
			"the rounding layer is then untested, and a case with an odd unit count " +
			"(one parameter, or seven arguments) belongs in the corpus")
	}
}

// natEInt32SlicesEq compares two frame lists element by element.
func natEInt32SlicesEq(a, b []int32) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// TestPhaseNATCall_WindowsAlignmentIsEmittedOnWindowsOnly is layer 4.
//
// The Win64 call-alignment pair must appear on Windows and NOWHERE else. The
// expected Windows count is stated per case as calls + prints, which is a number
// a reader can count in the source; the expected Linux count is zero, always.
func TestPhaseNATCall_WindowsAlignmentIsEmittedOnWindowsOnly(t *testing.T) {
	karkain := phase130Karkain(t)
	totalWin := 0

	for _, c := range natECorpus {
		c := c
		t.Run(c.name, func(t *testing.T) {
			peImg := natEImage(t, c.src)
			peOff, peSize := natEPETextRange(t, peImg)
			gotSub := natECountSeq(peImg, peOff, peSize, natEWinCallAlign)
			gotAdd := natECountSeq(peImg, peOff, peSize, natEWinCallAlignUndo)
			if gotSub != c.winSub8 {
				t.Errorf("PE `sub rsp,8` count = %d, want %d (one per user call plus "+
					"one per print)\n  source:\n%s", gotSub, c.winSub8, c.src)
			}
			if gotAdd != c.winSub8 {
				t.Errorf("PE `add rsp,8` count = %d, want %d -- the pair must balance, "+
					"or the frame is left shifted\n  source:\n%s", gotAdd, c.winSub8, c.src)
			}
			totalWin += gotSub

			kccRes := nativeBuildWithEngine(t, karkain, c.src, NativeLinuxTarget, "kcc", c.name+"-align.elf")
			if kccRes.ExitCode != 0 || !kccRes.WroteImage {
				t.Fatalf("kcc produced no ELF (exit %d):\n%s", kccRes.ExitCode, kccRes.Output)
			}
			if n := natECountSeq(kccRes.Image, 0, len(kccRes.Image), natEWinCallAlign); n != 0 {
				t.Errorf("ELF has %d `sub rsp,8` sequences; the Win64 alignment must not "+
					"be emitted on Linux. Phase D2 found this exact defect shipping on all "+
					"three targets, where it MISALIGNED the callee and still ran.\n  source:\n%s",
					n, c.src)
			}
			if n := natECountSeq(kccRes.Image, 0, len(kccRes.Image), natEWinCallAlignUndo); n != 0 {
				t.Errorf("ELF has %d `add rsp,8` sequences; see above", n)
			}
		})
	}

	if totalWin == 0 {
		t.Error("no Windows alignment sequence was found anywhere in the corpus, so the " +
			"layer that distinguishes the targets tested nothing")
	}
}

// natERefusal is one row of the refusal table.
type natERefusal struct {
	name string
	src  string
	// code is the diagnostic family that must fire. Most rows are the native
	// driver's own K145; two are kcc's checker, which rejects the program
	// before the driver sees it, and saying WHICH layer refuses is part of the
	// evidence -- a K145 that should be a K101 would mean the driver ran on a
	// program that does not type-check.
	code string
	want string
}

// natERefusals is the shape of the table the slice's definition of done asks
// for: every construct the driver does not lower is refused BY NAME, with no
// image written.
//
// The messages below were captured from the committed compiler rather than
// composed here, and the `want` fragments are the actionable part of each
// diagnostic (the construct, the offending name, or the fix).
var natERefusals = []natERefusal{
	{
		// A callee may not bind a string: its literals are never interned, so
		// the rodata list main builds does not contain them.
		name: "callee_binds_string",
		src:  "func f(n int) int {\n    let s = \"hi\"\n    return n\n}\nfunc main() { print(f(1)) }\n",
		code: "error[K145]", want: "`StringLiteral` is not an integer expression",
	},
	{
		name: "callee_binds_array",
		src:  "func f(n int) int {\n    let a = [1, 2]\n    return n\n}\nfunc main() { print(f(1)) }\n",
		code: "error[K145]", want: "`ArrayLiteral` is not an integer expression",
	},
	{
		name: "callee_binds_map",
		src:  "func f(n int) int {\n    let m = {1: 2}\n    return n\n}\nfunc main() { print(f(1)) }\n",
		code: "error[K145]", want: "`MapLiteral` is not an integer expression",
	},
	{
		name: "callee_binds_record",
		src: "type P struct { x int }\nfunc f(n int) int {\n    let p = P{x: 1}\n" +
			"    return n\n}\nfunc main() { print(f(1)) }\n",
		code: "error[K145]", want: "`StructLiteral` is not an integer expression",
	},
	{
		name: "callee_binds_float",
		src:  "func f(n int) int {\n    let x = 1.5\n    return n\n}\nfunc main() { print(f(1)) }\n",
		code: "error[K145]", want: "`FloatLiteral` is not an integer expression",
	},
	{
		name: "callee_len_of_expression",
		src:  "func f(n int) int { return len([1, 2]) }\nfunc main() { print(f(1)) }\n",
		code: "error[K145]", want: "len() requires an array or string variable, not an expression",
	},
	{
		// Division, modulo, comparison and the logical operators are refused
		// by name rather than lowered, because each needs its own fidelity
		// argument against the oracle (signed division truncation, the six
		// comparison forms, short-circuit evaluation).
		name: "callee_division",
		src:  "func f(n int) int { return n / 2 }\nfunc main() { print(f(1)) }\n",
		code: "error[K145]", want: "unsupported operator '/' (want +, - or *)",
	},
	{
		name: "callee_modulo",
		src:  "func f(n int) int { return n % 2 }\nfunc main() { print(f(1)) }\n",
		code: "error[K145]", want: "unsupported operator '%'",
	},
	{
		name: "callee_comparison",
		src:  "func f(n int) int { return n < 2 }\nfunc main() { print(f(1)) }\n",
		code: "error[K145]", want: "unsupported operator '<'",
	},
	{
		name: "callee_logical_and",
		src:  "func f(n int) int { return n && 1 }\nfunc main() { print(f(1)) }\n",
		code: "error[K145]", want: "unsupported operator '&&'",
	},
	{
		name: "callee_logical_not",
		src:  "func f(n int) int { return !n }\nfunc main() { print(f(1)) }\n",
		code: "error[K145]", want: "unsupported unary operator '!'",
	},
	{
		// Control flow: the driver lowers no branch. Naming the construct is
		// the difference between a refusal a user can act on and "statement
		// kind 5".
		name: "callee_if",
		src: "func f(n int) int {\n    if n < 2 { return 1 }\n    return 2\n}\n" +
			"func main() { print(f(1)) }\n",
		code: "error[K145]",
		want: "`if` statement is not lowered by the native driver in a called function",
	},
	{
		name: "callee_while",
		src: "func f(n int) int {\n    while n < 2 { n = n + 1 }\n    return n\n}\n" +
			"func main() { print(f(1)) }\n",
		code: "error[K145]",
		want: "`while` statement is not lowered by the native driver in a called function",
	},
	{
		// main keeps its pre-NAT-CALL wording byte for byte, which is why the
		// suffix differs from the two rows above.
		name: "main_if",
		src:  "func main() {\n    if 1 < 2 { print(1) }\n}\n",
		code: "error[K145]",
		want: "`if` statement is not lowered by the native driver in `func main`",
	},
	{
		// The entry stub calls main with no arguments, so a parameter has
		// nowhere to come from.
		name: "main_with_parameters",
		src:  "func main(x int) { print(x) }\n",
		code: "error[K145]", want: "`main` must take no parameters on the native target",
	},
	{
		// Step 9c froze main's `let` arm at literal initialisers. The reason
		// is not arbitrary: widening it would make main's frame depend on a
		// callee's return kind, which the parser does not record.
		name: "main_let_from_call",
		src:  "func f(n int) int { return n }\nfunc main() {\n    let c = f(1)\n    print(c)\n}\n",
		code: "error[K145]", want: "`let c` initialised from a non-integer expression",
	},
	{
		// Every parameter is an int in this slice, so a non-int argument is
		// refused at the CALL SITE by kind, naming both the parameter and the
		// callee.
		name: "string_argument",
		src:  "func f(n int) int { return n }\nfunc main() { print(f(\"hi\")) }\n",
		code: "error[K145]", want: "string argument for int parameter `n` (call to `f`)",
	},
	{
		name: "bool_argument",
		src:  "func f(n int) int { return n }\nfunc main() { print(f(true)) }\n",
		code: "error[K145]", want: "bool argument for int parameter `n` (call to `f`)",
	},
	{
		name: "float_argument",
		src:  "func f(n int) int { return n }\nfunc main() { print(f(1.5)) }\n",
		code: "error[K145]", want: "float argument for int parameter `n` (call to `f`)",
	},
	{
		// The latent spill-clobber bound: an argument at index >= 1 that
		// contains a call would be overwritten when the inner call reuses the
		// spill area. Refusing it is honest; silently mis-passing is not.
		name: "call_in_later_argument",
		src: "func h(n int) int { return n }\nfunc f(a int, b int) int { return a + b }\n" +
			"func main() { print(f(h(1), h(2))) }\n",
		code: "error[K145]", want: "argument 2 of the call to `f` contains another call",
	},
	{
		// A builtin is not a user function, so `abs` has no label to call.
		name: "builtin_in_callee",
		src:  "func f(n int) int { return abs(n) }\nfunc main() { print(f(1)) }\n",
		code: "error[K145]", want: "call to builtin `abs` is not lowered by the native driver",
	},
	{
		// A callee's `print` is int-only. Before this refusal existed the
		// program CRASHED kcc with "array index out of range", because the
		// literal was never interned and the emission path indexed the rodata
		// list anyway. The refusal is the repair.
		name: "callee_prints_string",
		src:  "func f(n int) {\n    print(\"hi\")\n}\nfunc main() { f(1) }\n",
		code: "error[K145]", want: "`StringLiteral` is not an integer expression",
	},
	{
		// The checker refuses these BEFORE the driver, which is the correct
		// layering: the driver must never emit for a program that does not
		// type-check.
		name: "undefined_function",
		src:  "func main() { print(nope(1)) }\n",
		code: "error[K101]", want: "undefined function 'nope'",
	},
	{
		name: "arity_mismatch",
		src:  "func f(a int) int { return a }\nfunc main() { print(f(1, 2)) }\n",
		code: "error[K103]", want: "function 'f' expects 1 arguments but got 2",
	},
}

// TestPhaseNATCall_RefusalsAreByName is layer 5: the table shrank by name.
//
// Every row asserts four things at once: the exit status, that NO image was
// written, that the named code fired, and that the message contains the
// construct. A refusal that exits 3 with an empty message would satisfy a
// status-only check while telling a user nothing, and a refusal that still
// wrote an image would be worse than either.
func TestPhaseNATCall_RefusalsAreByName(t *testing.T) {
	karkain := phase130Karkain(t)
	if len(natERefusals) < 20 {
		t.Fatalf("refusal table has %d rows; NAT-CALL measured at least 20 distinct "+
			"refused constructs", len(natERefusals))
	}

	for _, c := range natERefusals {
		c := c
		t.Run(c.name, func(t *testing.T) {
			res := nativeBuildWithEngine(t, karkain, c.src, NativeWindowsTarget, "kcc", c.name+".exe")
			if res.ExitCode != 3 {
				t.Errorf("exit = %d, want 3 (compile error)\n  output:\n%.600s", res.ExitCode, res.Output)
			}
			if res.WroteImage {
				t.Fatalf("a refused program WROTE AN IMAGE (%d bytes); a refusal that "+
					"still emits is not a refusal\n  output:\n%.600s", len(res.Image), res.Output)
			}
			if !strings.Contains(res.Output, c.code) {
				t.Errorf("output does not contain %s\n  output:\n%.800s", c.code, res.Output)
			}
			if !strings.Contains(res.Output, c.want) {
				t.Errorf("output does not name the construct %q\n  output:\n%.800s",
					c.want, res.Output)
			}
			// The provenance marker is printed BEFORE the refusal, and it is
			// what proves the refusal came from kcc rather than from a Go-side
			// preflight that never reached the driver.
			if !strings.Contains(res.Output, kccNativeProvenance) &&
				!strings.HasPrefix(c.code, "error[K1") {
				t.Errorf("the refusal did not come from the self-hosted driver (no %q); "+
					"a Go-side rejection would look identical to a user\n  output:\n%.600s",
					kccNativeProvenance, res.Output)
			}
		})
	}
}

// TestPhaseNATCall_DeterminismAndNonVacuity is layer 6.
//
// Two runs of the same source must be identical, two different sources must not
// be, and the corpus must actually exercise what it claims: a corpus that
// printed nothing would pass an equality-only check while proving nothing about
// the call path.
func TestPhaseNATCall_DeterminismAndNonVacuity(t *testing.T) {
	var totalOut int
	var withOutput, quirkCases int

	for _, c := range natECorpus {
		c := c
		t.Run(c.name, func(t *testing.T) {
			first := natEImage(t, c.src)
			second := natEImage(t, c.src)
			if !nat9cBytesEq(first, second) {
				t.Fatalf("two runs of the same source produced different images (%d and "+
					"%d bytes); the driver must be deterministic", len(first), len(second))
			}
			if len(first) < 300 {
				t.Errorf("image is only %d bytes, which is too small to contain the "+
					"helper set; the build did not do what this case assumes", len(first))
			}
			totalOut += len(natEHex(t, c.wantHex, c.name))
			if c.wantHex != "" {
				withOutput++
			}
			if c.quirk != "" {
				quirkCases++
			}
		})
	}

	// Two different sources must not collide. Comparing the FIRST case with
	// every other one is enough to catch a driver that ignores its input, and
	// it keeps the layer linear rather than quadratic.
	base := natEImage(t, natECorpus[0].src)
	same := 0
	for _, c := range natECorpus[1:] {
		if nat9cBytesEq(base, natEImage(t, c.src)) {
			same++
			t.Errorf("case %q produced the same image as %q; a driver that ignores "+
				"its input would look like this", c.name, natECorpus[0].name)
		}
	}

	if totalOut == 0 {
		t.Error("the corpus expects no stdout at all, so an image that printed nothing " +
			"would pass every equality check in this file")
	}
	if withOutput < len(natECorpus)-3 {
		t.Errorf("only %d of %d cases expect output; the corpus is meant to be "+
			"execution evidence, not a compile-only sweep", withOutput, len(natECorpus))
	}
	if same != 0 {
		t.Errorf("%d cases collided with the first case's image", same)
	}
	if quirkCases != 2 {
		t.Errorf("%d corpus cases are marked as parity quirks, want 2 (the void-call-in-"+
			"value pair); an unmarked quirk is how a limitation becomes a claim",
			quirkCases)
	}
}

// TestPhaseNATCall_NoGoFallback is layer 7.
//
// The dishonesty increment 151 exists to remove is a native path that LOOKS like
// kcc's and is not. Byte identity alone cannot exclude it -- a fallback produces
// IDENTICAL bytes by construction -- so the provenance marker is the assertion
// that matters, and it is checked on the user-facing build path for all three
// targets kcc owns.
func TestPhaseNATCall_NoGoFallback(t *testing.T) {
	karkain := phase130Karkain(t)
	src := "func add(a int, b int) int { return a + b }\nfunc main() { print(add(2, 3)) }\n"

	for _, target := range []string{NativeWindowsTarget, NativeLinuxTarget, NativeMacOSTarget} {
		target := target
		t.Run(target, func(t *testing.T) {
			res := nativeBuildWithEngine(t, karkain, src, target, "kcc", "fallback-check.img")
			if res.ExitCode != 0 || !res.WroteImage {
				t.Fatalf("kcc produced no image for %s (exit %d):\n%s", target, res.ExitCode, res.Output)
			}
			if !strings.Contains(res.Output, kccNativeProvenance) {
				t.Fatalf("the build did not carry %q, so it may have been produced by the "+
					"Go backend:\n%s", kccNativeProvenance, res.Output)
			}
			// A call-bearing program is the case this slice added, so each
			// container is validated by the ORACLE rather than by a check
			// written here -- a structural check in this file would share the
			// assumptions of the code under test.
			switch target {
			case NativeWindowsTarget:
				if _, _, err := native.ParsePE(res.Image); err != nil {
					t.Fatalf("kcc emitted an invalid PE: %v", err)
				}
			case NativeLinuxTarget:
				if _, _, err := native.Parse(res.Image); err != nil {
					t.Fatalf("kcc emitted an invalid ELF: %v", err)
				}
			case NativeMacOSTarget:
				if _, _, err := native.ParseMachO(res.Image); err != nil {
					t.Fatalf("kcc emitted an invalid Mach-O: %v", err)
				}
			}
		})
	}
}

// TestPhaseNATCall_QuirksAreParityNotEndorsement pins the two cases that
// exercise a pre-existing checker gap.
//
// The gap: `print(v(1))` where `v` returns nothing is accepted by BOTH checkers,
// and both engines read the callee's implicit `return 0`. That was measured on
// the Go oracle FIRST, through the C path, before it was recorded -- so it is not
// a NAT-CALL behaviour and closing it is a type-checking change that would move
// both engines.
//
// The assertion is that the engines AGREE, which is the only claim this slice can
// honestly make. If a future slice rejects the shape, this test fails and the
// corpus entry is deleted on purpose -- it cannot silently rota.
func TestPhaseNATCall_QuirksAreParityNotEndorsement(t *testing.T) {
	karkain := phase130Karkain(t)
	var quirks []natECase
	for _, c := range natECorpus {
		if c.quirk != "" {
			quirks = append(quirks, c)
		}
	}
	if len(quirks) == 0 {
		t.Fatal("no quirk case is marked, so this layer cannot distinguish a fixed " +
			"shape from an undocumented one; update the corpus deliberately")
	}

	for _, c := range quirks {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Logf("pinned parity limitation: %s", c.quirk)
			kccRes := nativeBuildWithEngine(t, karkain, c.src, NativeLinuxTarget, "kcc", c.name+"-q-kcc.elf")
			goRes := nativeBuildWithEngine(t, karkain, c.src, NativeLinuxTarget, "go", c.name+"-q-go.elf")
			if kccRes.ExitCode != 0 || goRes.ExitCode != 0 ||
				!kccRes.WroteImage || !goRes.WroteImage {
				t.Fatalf("both engines must accept this shape for it to be PARITY "+
					"(kcc exit %d, go exit %d); if one now refuses, that is a deliberate "+
					"change and the corpus entry must move with it.\n  kcc:\n%s\n  go:\n%s",
					kccRes.ExitCode, goRes.ExitCode, kccRes.Output, goRes.Output)
			}
			if !nat9cBytesEq(kccRes.Image, goRes.Image) {
				t.Fatalf("the two engines DISAGREE on a shape both accept, which is a "+
					"parity defect rather than a documented gap\n  kcc %d bytes\n  go  %d bytes",
					len(kccRes.Image), len(goRes.Image))
			}
		})
	}
}

// TestPhaseNATCall_GateIsWiredInCI keeps the gate a gate.
//
// The repository's own rule: "a gate is a test file PLUS a ci.yml entry", and it
// was earned four times over when increment gates were written, run locally,
// reported green, and never executed by CI. A test that only runs because some
// broader pattern happens to match is not wired, and the pattern below is chosen
// so it cannot be swallowed by an earlier family.
func TestPhaseNATCall_GateIsWiredInCI(t *testing.T) {
	body, err := os.ReadFile(filepath.Join("..", "..", ".github", "workflows", "ci.yml"))
	if err != nil {
		t.Fatalf("reading ci.yml: %v", err)
	}
	text := string(body)
	const step = "go test ./pkg/cli/ -run 'TestPhaseNATCall_' -count=1"
	if !strings.Contains(text, step) {
		t.Errorf("ci.yml has no step running %q; a locally green gate that CI never "+
			"executes is not a gate", step)
	}
	// The pattern must be its own step rather than a substring of a broader
	// one, because 'TestPhase151' would not match 'TestPhaseNATCall_' at all
	// and a future rename could silently orphan it.
	if strings.Contains(text, "-run 'TestPhase151_' -count=1") &&
		!strings.Contains(text, "'TestPhaseNATCall_'") {
		t.Error("only the broad 151 pattern is wired; NAT-CALL needs its own step")
	}
}
