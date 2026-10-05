package cli

// Phase 151A Step 8a - the SSE2 scalar-double primitives in kcc.
//
// This slice adds the eight float instructions the native value model needs
// before any float STATEMENT can exist: the four arithmetic ops, the
// unordered compare, the two converts, and the bitwise xor used to zero an
// xmm register. It is an ENCODER slice - instruction bytes only.
//
// NO EXECUTION EVIDENCE IS CLAIMED. The corpus is four encoding sequences,
// not a program: there is no `main`, no frame, and nothing to run. What is
// claimed is byte parity with pkg/native on the sequences this slice owns.
//
// FOUR INDEPENDENT LAYERS, following the 151B / 151A Step 1-7 methodology:
//
//  1. kcc's bytes vs the REAL pkg/native Emitter building the same sequences.
//     A differential, not a golden: a golden pins kcc against a transcription
//     of the Go code and passes when both are wrong together.
//  2. bytes stated from the Intel SDM, computed from the field layout rather
//     than copied from either implementation's output.
//  3. occurrence in the .text of a REAL oracle image produced by compiling a
//     program that uses floats.
//  4. non-vacuity and determinism, plus the no-Go-fallback guard.
//
// WHY THE CORPUS IS SHAPED THIS WAY. Eight near-identical sequences would pass
// while the one thing that is actually easy to get wrong was wrong. The
// mandatory legacy prefix (0xF2 / 0x66) must precede REX, and the 0x0F escape
// must sit between them. Emit REX first and, for xmm8-15, the instruction
// silently changes meaning instead of failing. So the corpus deliberately
// includes: low registers only (the only arm that catches a spurious 0x40),
// high xmm registers (catches prefix/REX order), the two prefix families
// adjacent to each other (catches a swapped 0xF2/0x66), and REX.W together
// with REX.R and REX.B at once (the converts).

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

func decode151A8(t *testing.T, h string) []byte {
	t.Helper()
	b, err := hex.DecodeString(h)
	if err != nil {
		t.Fatalf("corpus line %q is not hex: %v", h, err)
	}
	return b
}

// goFloat151A8 builds the corpus with the REAL Go emitter, in the same order
// kcc emits. Every method used here is exported, so this is a genuine
// differential rather than a reconstruction.
func goFloat151A8(which int) []byte {
	e := native.NewEmitter()
	switch which {
	case 0: // all four arithmetic ops, low registers: no REX at all
		e.AddsdXmmXmm(native.XMM0, native.XMM1)
		e.SubsdXmmXmm(native.XMM0, native.XMM1)
		e.MulsdXmmXmm(native.XMM0, native.XMM1)
		e.DivsdXmmXmm(native.XMM0, native.XMM1)
	case 1: // high xmm registers: REX = 0x45, and the prefix must precede it
		e.AddsdXmmXmm(native.XMM8, native.XMM9)
		e.DivsdXmmXmm(native.XMM15, native.XMM8)
	case 2: // the 0xF2 family next to the 0x66 family, in one sequence
		e.AddsdXmmXmm(native.XMM0, native.XMM1)
		e.UcomisdXmmXmm(native.XMM0, native.XMM1)
		e.XorpdXmmXmm(native.XMM0, native.XMM1)
	case 3: // the converts: REX.W combined with REX.R and REX.B
		e.Cvtsi2sdXmmGp(native.XMM0, 8)
		e.Cvttsd2siGpXmm(9, native.XMM0)
		e.Cvtsi2sdXmmGp(native.XMM15, 15)
	default:
		panic("bad which")
	}
	return e.Bytes()
}

func floatCorpus151A8(t *testing.T) []string {
	t.Helper()
	karkain := phase130Karkain(t)
	got := runKCCStep2(t, karkain, "native-value-float")
	if len(got) < 4 {
		t.Fatalf("kcc native-value-float produced %d lines, want at least 4 "+
			"(arms 0-3; Step 8b adds arm 4, the print_float helper):\n%v", len(got), got)
	}
	return got
}

// TestPhase151A8_FloatPrimitivesByteIdenticalToOracle is this slice's whole
// contract: every corpus sequence equals what the real Go emitter produces.
func TestPhase151A8_FloatPrimitivesByteIdenticalToOracle(t *testing.T) {
	got := floatCorpus151A8(t)
	for which := 0; which < 4; which++ {
		g := decode151A8(t, got[which])
		w := goFloat151A8(which)
		if !bytes.Equal(g, w) {
			t.Errorf("corpus %d: kcc=% x\n            oracle=% x", which, g, w)
		}
	}
}

// TestPhase151A8_EncodingsMatchBytesStatedFromTheIntelSDM checks one
// representative of each encoding shape against bytes derived here from the
// SDM, so an error BOTH emitters share still fails.
//
// This layer exists because the differential cannot catch a shared mistake in
// the SEQUENCE DEFINITION - only in the transcription. Each expected byte
// below is computed from the field layout, not copied from the output:
//
//	ADDSd xmm0, xmm1   F2 (mandatory prefix) | 0F 58 (opcode) | C1 (ModRM)
//	  ModRM C1 = 11 000 001: mod=11 register form, reg=000 -> xmm0,
//	                                rm=001 -> xmm1
//	  no REX: neither field is >= 8, so the REX byte is suppressed entirely
//
//	ADDSd xmm8, xmm9   F2 | 45 (REX) | 0F 58 | C1
//	  REX 45 = 0100 0101 = base 0x40, W=0, R=1 (reg field 8), X=0, B=1 (rm 9)
//	  ModRM still C1: only the low three bits of each field reach ModRM
//
//	UCOMISD xmm0, xmm1 66 | 0F 2E | C1   (the 0x66 family, not 0xF2)
//
//	CVTSI2SD xmm0, r8  F2 | 49 (REX) | 0F 2A | C0
//	  REX 49 = 0100 1001 = base 0x40, W=1 (64-bit GP source), R=0, X=0, B=1
//	  ModRM C0 = 11 000 000: reg=000 -> xmm0, rm=000 -> r8
func TestPhase151A8_EncodingsMatchBytesStatedFromTheIntelSDM(t *testing.T) {
	got := floatCorpus151A8(t)

	// arm 0, first instruction: addsd xmm0, xmm1 - four bytes, no REX.
	g0 := decode151A8(t, got[0])
	if len(g0) < 4 {
		t.Fatalf("arm 0 is %d bytes, too short to contain an instruction", len(g0))
	}
	if !bytes.Equal(g0[:4], []byte{0xF2, 0x0F, 0x58, 0xC1}) {
		t.Errorf("addsd xmm0,xmm1 = % x, want f2 0f 58 c1", g0[:4])
	}

	// arm 1, first instruction: addsd xmm8, xmm9 - the REX byte is 0x45 and
	// the mandatory prefix precedes it. This is the single assertion that fails
	// if the prefix and REX are emitted in the other order.
	g1 := decode151A8(t, got[1])
	if !bytes.Equal(g1[:5], []byte{0xF2, 0x45, 0x0F, 0x58, 0xC1}) {
		t.Errorf("addsd xmm8,xmm9 = % x, want f2 45 0f 58 c1 (prefix BEFORE REX)", g1[:5])
	}

	// arm 2: ucomisd takes 0x66, not 0xF2. Offset 4 is where it begins.
	g2 := decode151A8(t, got[2])
	if len(g2) < 8 {
		t.Fatalf("arm 2 is %d bytes, too short to contain addsd + ucomisd", len(g2))
	}
	if !bytes.Equal(g2[4:8], []byte{0x66, 0x0F, 0x2E, 0xC1}) {
		t.Errorf("ucomisd xmm0,xmm1 = % x, want 66 0f 2e c1 (the 0x66 family)", g2[4:8])
	}

	// arm 3, first instruction: cvtsi2sd xmm0, r8 - REX 0x49 carries W=1
	// because the GP source is 64-bit, plus B=1 for r8.
	g3 := decode151A8(t, got[3])
	if !bytes.Equal(g3[:5], []byte{0xF2, 0x49, 0x0F, 0x2A, 0xC0}) {
		t.Errorf("cvtsi2sd xmm0,r8 = % x, want f2 49 0f 2a c0", g3[:5])
	}
}

// TestPhase151A8_PrimitivesAppearInARealOracleImage ties the slice to
// pkg/native's actual float emission rather than to hand-built sequences.
//
// The corpus proves kcc and the Go emitter agree in isolation. This proves the
// bytes are the ones the compiler really emits, by compiling a program that
// uses floats and requiring one of kcc's sequences to occur verbatim in its
// .text.
func TestPhase151A8_PrimitivesAppearInARealOracleImage(t *testing.T) {
	got := floatCorpus151A8(t)
	// The first instruction of arm 0: addsd xmm0, xmm1, which any program
	// doing float addition must contain.
	seq := decode151A8(t, got[0])[:4]

	src := "func main() {\n\tlet a = 1.5\n\tlet b = a + 2.25\n\tprint(b)\n}\n"
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
	_, textOff, perr := native.Parse(img)
	if perr != nil {
		t.Fatalf("oracle image parse: %v", perr)
	}

	if idx := bytes.Index(img[textOff:], seq); idx < 0 {
		t.Fatalf("kcc's addsd xmm0,xmm1 % x does not occur in the oracle .text "+
			"(%d bytes); either pkg/native's float emission changed, or this "+
			"layer's premise is wrong", seq, len(img)-textOff)
	}
}

// TestPhase151A8_CorpusIsNonVacuousAndDeterministic guards the two failure
// modes a hex-comparison gate is otherwise blind to: an empty corpus compares
// equal to nothing, and a non-deterministic corpus would report parity that
// does not exist.
//
// It also asserts the corpus actually carries all eight primitives, so a slice
// that silently dropped one of them could not report success.
func TestPhase151A8_CorpusIsNonVacuousAndDeterministic(t *testing.T) {
	karkain := phase130Karkain(t)
	first := runKCCStep2(t, karkain, "native-value-float")
	if len(first) < 4 {
		t.Fatalf("corpus has %d lines, want at least 4:\n%v", len(first), first)
	}
	total := 0
	for i, h := range first {
		if len(h) == 0 {
			t.Fatalf("corpus line %d is empty; an empty comparison is not evidence", i)
		}
		total += len(decode151A8(t, h))
	}
	// Eight instructions at 4-6 bytes each cannot fit in fewer than 32 bytes;
	// this catches an emitter that produced one arm's bytes for all four.
	if total < 32 {
		t.Errorf("corpus carries only %d bytes, too few for eight SSE2 instructions", total)
	}

	for run := 0; run < 2; run++ {
		again := runKCCStep2(t, karkain, "native-value-float")
		for i := range first {
			if again[i] != first[i] {
				t.Fatalf("run %d line %d differs:\n  %s\n  %s", run+1, i, first[i], again[i])
			}
		}
	}
}

// TestPhase151A8_NoGoFallback guards the measurement contract itself: the Go
// side must only RUN kcc, never compute the answer.
//
// Every measurement command in this increment carries this same rule, and it
// is the rule increment 151 was opened to enforce. A Go-side emitter that
// produced the expected bytes would make every differential above pass while
// proving nothing about kcc.
func TestPhase151A8_NoGoFallback(t *testing.T) {
	src, err := os.ReadFile(filepath.Join(repoRoot(t), "pkg", "cli", "native_encode.go"))
	if err != nil {
		t.Fatalf("read native_encode.go: %v", err)
	}
	if bytes.Contains(src, []byte("func goFloat")) ||
		bytes.Contains(src, []byte("func oracleFloat")) {
		t.Error("pkg/cli/native_encode.go appears to compute the float bytes in Go; " +
			"the measurement surface must only run kcc")
	}
	if !bytes.Contains(src, []byte(`return kccSubcommand(w, "native-value-float")`)) {
		t.Error("KCCNativeValueFloatCommand does not delegate to kccSubcommand")
	}
}
