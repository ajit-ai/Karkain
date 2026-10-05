package cli

// Phase 151A Step 8b - print_float in kcc.
//
// Step 7 ported print_int and Step 8a the eight SSE2 primitives. This ports
// the OTHER mandatory output helper, so a float has an observable rendering
// path at all. It is the largest single helper in the native backend:
// Builder.emitPrintFloatHelper (pkg/native/program.go:4185-4390) is ~205 lines
// of Go and 744 bytes of x86.
//
// NO EXECUTION EVIDENCE IS CLAIMED. The corpus is the helper, not a program:
// no `main`, no entry stub, no resolved rodata. The argument is transitive --
// kcc's bytes are byte-identical to the oracle's, and the oracle's PE images
// ARE executed on this host by increments 145-150 -- and it is stated as
// transitive rather than dressed up as direct proof.
//
// WHY THE DIFFERENTIAL EXTRACTS FROM A REAL IMAGE. The obvious approach --
// rebuild the helper with the Go emitter in this file -- is not available:
// Emitter.imm64Patch is UNEXPORTED, so pkg/cli cannot produce the unresolved
// rodata reference the helper contains, and a hand-written transcription would
// be exactly the "golden against my own transcription" failure mode. So the
// oracle side is read out of a REAL compiled image instead, which is stronger
// evidence anyway: it is pkg/native's actual emission, not this file's idea.

import (
	"bytes"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"

	"karkain/pkg/lexer"
	"karkain/pkg/native"
	"karkain/pkg/parser"
)

// oraclePrintFloat151A8b extracts print_float from a real oracle image.
//
// Locating a function without labels (labels do not survive into the image)
// uses two anchors and a structural count, not a length:
//
//   - START: the 17-byte prologue `sub rsp,0x40; mov rax,rdi;
//     movabs rcx,0x8000000000000000`, asserted to occur exactly once. That
//     `movabs rcx, 1<<63` constant is what makes it unique -- print_int's
//     prologue is `sub rsp,0x40; mov rax,rdi; movabs rcx,0x7FFFFFFFFFFFFFFF`,
//     a different third instruction.
//   - END: the THIRD `add rsp,0x40; ret` after the start. Three, not one,
//     because print_float has two early returns (the special/inf path and the
//     nan path) before its fallthrough return, and each epilogue is exactly
//     that pair. The count is asserted rather than assumed, so a change to the
//     oracle's return structure fails loudly instead of silently re-extracting
//     a different span.
func oraclePrintFloat151A8b(t *testing.T) []byte {
	t.Helper()
	src := "func main() {\n\tlet a = 1.5\n\tprint(a)\n}\n"
	l := lexer.New(src)
	p := parser.New(l)
	prog := p.ParseProgram()
	if len(p.Errors) > 0 {
		t.Fatalf("oracle source parse: %v", p.Errors)
	}
	img, err := native.CompileProgramForOS(prog, native.OSLinux)
	if err != nil {
		t.Fatalf("oracle compile: %v", err)
	}
	_, off, perr := native.Parse(img)
	if perr != nil {
		t.Fatalf("oracle image parse: %v", perr)
	}
	text := img[off:]

	startPat, err := hex.DecodeString("4883ec404889f848b90000000000000080")
	if err != nil {
		t.Fatalf("start pattern: %v", err)
	}
	if c := bytes.Count(text, startPat); c != 1 {
		t.Fatalf("print_float prologue occurs %d times, want exactly 1", c)
	}
	start := bytes.Index(text, startPat)

	endPat := []byte{0x48, 0x83, 0xC4, 0x40, 0xC3} // add rsp,0x40; ret
	var ends []int
	for i := start; i+len(endPat) <= len(text); i++ {
		if bytes.Equal(text[i:i+len(endPat)], endPat) {
			ends = append(ends, i)
		}
	}
	if len(ends) < 3 {
		t.Fatalf("found %d `add rsp,0x40; ret` sites after the prologue, want at "+
			"least 3 (two early returns plus the fallthrough)", len(ends))
	}
	if len(ends) != 3 {
		t.Errorf("found %d `add rsp,0x40; ret` sites after the prologue, want "+
			"exactly 3; the end anchor is ambiguous", len(ends))
	}
	return text[start : ends[len(ends)-1]+len(endPat)]
}

// printFloatRodataSites is how many `rodataRef` calls print_float makes: TEN,
// not the six a first draft of this file asserted.
//
//   - the infinity path: "-", "inf", and printNewline's "\n"      = 3
//   - the nan path:      "-", "nan", and printNewline's "\n"      = 3
//   - the finite path:   "-", "0", "." and printNewline's "\n"   = 4
//
// The count is a named constant rather than a literal because getting it wrong
// is the first thing that happens: a hand-count of six missed the two early
// return paths' newlines and one of the finite-path sites. The site-count
// assertion in maskRodata151A8b is what caught that, which is the argument for
// having it -- a baseline with a wrong expectation is worse than no baseline.
const printFloatRodataSites = 10

func floatPrintCorpus151A8b(t *testing.T) []string {
	t.Helper()
	karkain := phase130Karkain(t)
	got := runKCCStep2(t, karkain, "native-value-float")
	if len(got) != 5 {
		t.Fatalf("kcc native-value-float produced %d lines, want 5:\n%v", len(got), got)
	}
	return got
}

// maskRodata151A8b zeroes the 8-byte immediate of every `movabs rsi, imm64`
// site, asserting the site count.
//
// `48 BE` is `movabs rsi, imm64`. It is an exact marker here rather than a
// heuristic: the helper's only RSI-destination movabs are its rodata
// references, while every other movabs targets RCX or R10 (`48 B9`, `49 BA`).
// The count assertion stops a changed site count from passing unnoticed.
func maskRodata151A8b(t *testing.T, b []byte, who string, want int) []byte {
	t.Helper()
	out := bytes.Clone(b)
	const siteLen = 10
	sites := 0
	for i := 0; i+siteLen <= len(out); i++ {
		if out[i] == 0x48 && out[i+1] == 0xBE {
			sites++
			for j := i + 2; j < i+siteLen; j++ {
				out[j] = 0
			}
			i += siteLen - 1
		}
	}
	if sites != want {
		t.Fatalf("%s contains %d `movabs rsi` rodata sites, want %d", who, sites, want)
	}
	return out
}

// TestPhase151A8b_PrintFloatMatchesOracleExceptRodata is this slice's contract:
// every byte of the 744-byte helper matches pkg/native's real emission except
// the ten link-time rodata immediates.
func TestPhase151A8b_PrintFloatMatchesOracleExceptRodata(t *testing.T) {
	got := floatPrintCorpus151A8b(t)
	kcc := decode151A8(t, got[4])
	oracle := oraclePrintFloat151A8b(t)

	if len(kcc) != len(oracle) {
		t.Fatalf("print_float length: kcc=%d oracle=%d", len(kcc), len(oracle))
	}
	gm := maskRodata151A8b(t, kcc, "kcc print_float", printFloatRodataSites)
	om := maskRodata151A8b(t, oracle, "oracle print_float", printFloatRodataSites)
	if !bytes.Equal(gm, om) {
		for i := range gm {
			if gm[i] != om[i] {
				lo, hi := i-8, i+8
				if lo < 0 {
					lo = 0
				}
				if hi > len(gm) {
					hi = len(gm)
				}
				t.Fatalf("print_float differs at byte %d:\n  kcc    % x\n  oracle % x",
					i, gm[lo:hi], om[lo:hi])
			}
		}
		t.Fatal("print_float differs from the oracle but no differing byte was found")
	}
}

// TestPhase151A8b_RodataPlaceholdersAreUnresolved pins the placeholder form: all
// six rodata immediates are ZERO in kcc's output, because the address is only
// knowable at link time.
func TestPhase151A8b_RodataPlaceholdersAreUnresolved(t *testing.T) {
	got := floatPrintCorpus151A8b(t)
	kcc := decode151A8(t, got[4])
	const siteLen = 10
	sites := 0
	for i := 0; i+siteLen <= len(kcc); i++ {
		if kcc[i] == 0x48 && kcc[i+1] == 0xBE {
			sites++
			for j := i + 2; j < i+siteLen; j++ {
				if kcc[j] != 0 {
					t.Errorf("rodata site at byte %d has a non-zero immediate byte "+
						"(%#x); kcc must not invent an address", i, kcc[j])
				}
			}
			i += siteLen - 1
		}
	}
	if sites != printFloatRodataSites {
		t.Errorf("kcc print_float has %d rodata sites, want %d", sites, printFloatRodataSites)
	}
}

// TestPhase151A8b_SignMaskConstantIsExact is a regression pin for a real defect
// found while writing this slice.
//
// The sign mask is 1<<63 = 9223372036854775808, one past the largest signed
// 64-bit value. Karkain's integer surface is signed 64-bit, so writing that
// literal clamps it to 9223372036854775807, and the helper then emitted
// `48 b9 ff ff ff ff ff ff ff ff` where the oracle emits
// `48 b9 00 00 00 00 00 00 00 80`. The `and r11, rcx` that extracts the sign
// would then test the wrong mask and every negative float would print without
// its sign. The fix writes the constant as `0 - 9223372036854775807 - 1`, which
// is exact, and natBytesLE's per-byte masking turns the negative into the
// correct two's-complement bytes.
//
// This is the 151B constant-class bug recurring in a new place. It was caught
// by decoding the ORACLE's bytes by hand and comparing, precisely the practice
// the 151B report says caught the original, and it is pinned here so that a
// clamped literal can never again survive on the strength of a differential
// alone.
func TestPhase151A8b_SignMaskConstantIsExact(t *testing.T) {
	got := floatPrintCorpus151A8b(t)
	kcc := decode151A8(t, got[4])

	exact, err := hex.DecodeString("48b90000000000000080")
	if err != nil {
		t.Fatalf("exact pattern: %v", err)
	}
	if bytes.Contains(kcc, exact) {
		return
	}
	clamped, err := hex.DecodeString("48b9ffffffffffffff7f")
	if err != nil {
		t.Fatalf("clamped pattern: %v", err)
	}
	if bytes.Contains(kcc, clamped) {
		t.Fatal("the sign mask was clamped to max-int64: kcc emitted " +
			"movabs rcx,0x7FFFFFFFFFFFFFFF where the oracle emits " +
			"movabs rcx,0x8000000000000000; write it as " +
			"`0 - 9223372036854775807 - 1`")
	}
	t.Fatal("print_float does not contain the sign-mask movabs at all")
}

// TestPhase151A8b_CorpusIsNonVacuousAndDeterministic guards an empty or
// unstable corpus, and pins the helper's length so a truncated helper cannot
// compare equal to a truncated oracle extraction.
func TestPhase151A8b_CorpusIsNonVacuousAndDeterministic(t *testing.T) {
	karkain := phase130Karkain(t)
	first := runKCCStep2(t, karkain, "native-value-float")
	if len(first) != 5 {
		t.Fatalf("corpus has %d lines, want 5:\n%v", len(first), first)
	}
	for i, h := range first {
		if len(h) == 0 {
			t.Fatalf("corpus line %d is empty; an empty comparison is not evidence", i)
		}
	}
	helper := decode151A8(t, first[4])
	if len(helper) != 744 {
		t.Errorf("print_float is %d bytes, want 744", len(helper))
	}
	for run := 0; run < 2; run++ {
		again := runKCCStep2(t, karkain, "native-value-float")
		for i := range first {
			if again[i] != first[i] {
				t.Fatalf("run %d line %d differs", run+1, i)
			}
		}
	}
}

// TestPhase151A8b_NoGoFallback guards the measurement contract: the Go side must
// only RUN kcc, never compute the answer.
func TestPhase151A8b_NoGoFallback(t *testing.T) {
	src, err := os.ReadFile(filepath.Join(repoRoot(t), "pkg", "cli", "native_encode.go"))
	if err != nil {
		t.Fatalf("read native_encode.go: %v", err)
	}
	if bytes.Contains(src, []byte("func goPrintFloat")) ||
		bytes.Contains(src, []byte("func oraclePrintFloat")) {
		t.Error("pkg/cli/native_encode.go appears to compute print_float in Go; " +
			"the measurement surface must only run kcc")
	}
	if !bytes.Contains(src, []byte(`return kccSubcommand(w, "native-value-float")`)) {
		t.Error("KCCNativeValueFloatCommand does not delegate to kccSubcommand")
	}
}
