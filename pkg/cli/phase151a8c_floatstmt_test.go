package cli

// Phase 151A Step 8c - float statement lowering in kcc.
//
// Step 8a ported the SSE2 primitives and Step 8b the print_float helper; both
// were unreachable from an actual statement. This slice adds the LOWERING, and
// it is what makes whole-image parity testable for a real program.
//
// NO EXECUTION EVIDENCE IS CLAIMED. The corpus is thirteen reference shapes, not
// a program: no `main`, no entry stub, so there is nothing to run. What is
// claimed is byte parity with pkg/native on the shapes this slice owns.
//
// WHY THIS SLICE GETS A DIRECT DIFFERENTIAL where Step 8b could not: every
// primitive involved here is EXPORTED (MovRegImm64, StoreStack, LoadStack,
// MovXmmRegGp, MovGpRegXmm, the four arithmetic ops, XorpdXmmXmm,
// UcomisdXmmXmm, Jp, Jnz, Jmp, Jbe, Jb), and none of these shapes contains a
// rodata reference. Step 8b had to extract from a real image because
// Emitter.imm64Patch is unexported; there is no such obstacle here, so the
// oracle is built with the real Emitter directly and compared RAW.
//
// THE LOAD-BEARING FACT this slice turns on: a float local is ONE 8-byte unit
// in exactly the frame slot an int occupies. emitLet's float path is therefore
// the int path's `StoreStack(RAX, off)`, with the f64 bits left in RAX. Nothing
// about the frame changes between kinds and rsp never moves.

import (
	"bytes"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"

	"karkain/pkg/native"
)

const (
	f15       int64  = 4609434218613702656 // float64bits(1.5)  = 0x3FF8000000000000
	f225      int64  = 4612248968380809216 // float64bits(2.25) = 0x4002000000000000
	signMaskI uint64 = 0x8000000000000000  // 1<<63
	// binTempBase is the oracle's binTemp offset for a ONE-local frame: locals
	// occupy 0..7, so binTemp starts at 8. Using the oracle's real staging
	// offset is the point -- the staged bytes are part of the shape.
	binTempBase = 8
)

const floatStmtArms = 13

func decode151A8c(t *testing.T, h string) []byte {
	t.Helper()
	b, err := hex.DecodeString(h)
	if err != nil {
		t.Fatalf("corpus line %q is not hex: %v", h, err)
	}
	return b
}

// stageFloat151A8c writes the left operand into binTemp, evaluates the right into
// RAX, then reloads the left into xmm0 -- the oracle's order, and the reason the
// RIGHT operand is evaluated second.
func stageFloat151A8c(e *native.Emitter, l, r int64) {
	e.MovRegImm64(native.RAX, uint64(l))
	e.StoreStack(native.RAX, binTempBase)
	e.MovRegImm64(native.RAX, uint64(r))
	e.MovXmmRegGp(native.XMM1, native.RAX)
	e.LoadStack(native.RAX, binTempBase)
	e.MovXmmRegGp(native.XMM0, native.RAX)
}

func goFloatStmt151A8c(arm int) []byte {
	e := native.NewEmitter()
	switch arm {
	case 0: // let of a literal: the int path's store, f64 bits in RAX
		e.MovRegImm64(native.RAX, uint64(f15))
		e.StoreStack(native.RAX, 0)
	case 1: // a local reload
		e.LoadStack(native.RAX, 0)
	case 2:
		stageFloat151A8c(e, f15, f225)
		e.AddsdXmmXmm(native.XMM0, native.XMM1)
		e.MovGpRegXmm(native.RAX, native.XMM0)
	case 3:
		stageFloat151A8c(e, f15, f225)
		e.SubsdXmmXmm(native.XMM0, native.XMM1)
		e.MovGpRegXmm(native.RAX, native.XMM0)
	case 4:
		stageFloat151A8c(e, f15, f225)
		e.MulsdXmmXmm(native.XMM0, native.XMM1)
		e.MovGpRegXmm(native.RAX, native.XMM0)
	case 5:
		stageFloat151A8c(e, f15, f225)
		e.XorpdXmmXmm(native.XMM2, native.XMM2)
		e.UcomisdXmmXmm(native.XMM1, native.XMM2)
		e.Jnz("fdiv")
		e.XorpdXmmXmm(native.XMM0, native.XMM0)
		e.Jmp("fdivend")
		e.Mark("fdiv")
		e.DivsdXmmXmm(native.XMM0, native.XMM1)
		e.Mark("fdivend")
		e.MovGpRegXmm(native.RAX, native.XMM0)
	case 6: // unary negation: XOR the sign bit, never 0.0 - x
		e.MovRegImm64(native.RCX, signMaskI)
		e.MovXmmRegGp(native.XMM1, native.RCX)
		e.MovXmmRegGp(native.XMM0, native.RAX)
		e.XorpdXmmXmm(native.XMM0, native.XMM1)
		e.MovGpRegXmm(native.RAX, native.XMM0)
	case 7: // == : ordered AND bit-equal
		stageFloat151A8c(e, f15, f225)
		e.UcomisdXmmXmm(native.XMM0, native.XMM1)
		e.Jp("false")
		e.Jnz("false")
		e.Mark("false")
	case 8: // != : unordered OR not-equal, needing the explicit hold label
		stageFloat151A8c(e, f15, f225)
		e.UcomisdXmmXmm(native.XMM0, native.XMM1)
		e.Jp("fne")
		e.Jnz("fne")
		e.Jmp("false")
		e.Mark("fne")
		e.Mark("false")
	case 9: // <
		stageFloat151A8c(e, f15, f225)
		e.UcomisdXmmXmm(native.XMM1, native.XMM0)
		e.Jbe("false")
		e.Mark("false")
	case 10: // <=
		stageFloat151A8c(e, f15, f225)
		e.UcomisdXmmXmm(native.XMM1, native.XMM0)
		e.Jb("false")
		e.Mark("false")
	case 11: // >
		stageFloat151A8c(e, f15, f225)
		e.UcomisdXmmXmm(native.XMM0, native.XMM1)
		e.Jbe("false")
		e.Mark("false")
	case 12: // >=
		stageFloat151A8c(e, f15, f225)
		e.UcomisdXmmXmm(native.XMM0, native.XMM1)
		e.Jb("false")
		e.Mark("false")
	default:
		panic("bad arm")
	}
	return e.Bytes()
}

func floatStmtCorpus151A8c(t *testing.T) []string {
	t.Helper()
	karkain := phase130Karkain(t)
	got := runKCCStep2(t, karkain, "native-value-floatstmt")
	if len(got) != floatStmtArms {
		t.Fatalf("kcc native-value-floatstmt produced %d lines, want %d:\n%v",
			len(got), floatStmtArms, got)
	}
	return got
}

// TestPhase151A8c_FloatStatementsByteIdenticalToOracle is this slice's contract:
// all thirteen arms equal what the real Go emitter produces, RAW.
func TestPhase151A8c_FloatStatementsByteIdenticalToOracle(t *testing.T) {
	got := floatStmtCorpus151A8c(t)
	for arm := 0; arm < floatStmtArms; arm++ {
		g := decode151A8c(t, got[arm])
		w := goFloatStmt151A8c(arm)
		if !bytes.Equal(g, w) {
			t.Errorf("arm %d: kcc=% x\n          oracle=% x", arm, g, w)
		}
	}
}

// TestPhase151A8c_ComparisonShapesMatchTheSDM checks the two facts a
// transcription error would most easily get wrong.
//
//  1. The ORDERED comparisons feed ucomisd SWAPPED for `<` and `<=`, and
//     unswapped for `>` and `>=`. ucomisd computes the compare in SOURCE order,
//     so `ucomisd xmm1, xmm0` means "right <= left", which is what lets a
//     single jbe express "not (left < right)". Swapping backwards yields a
//     comparator that compiles and computes the wrong answer.
//     ModRM tells them apart: dst in reg, src in rm, so `66 0f 2e c8` is
//     xmm1,xmm0 and `66 0f 2e c1` is xmm0,xmm1.
//  2. `<` uses JBE and `<=` uses JB. "Below or equal" is CF=1 OR ZF=1, which is
//     JBE (0F 86); "strictly below" is CF=1 alone, which is JB (0F 82).
func TestPhase151A8c_ComparisonShapesMatchTheSDM(t *testing.T) {
	got := floatStmtCorpus151A8c(t)
	ucm := []byte{0x66, 0x0F, 0x2E} // ucomisd, no REX needed for xmm0/xmm1
	be := []byte{0x0F, 0x86}        // jbe
	bt := []byte{0x0F, 0x82}        // jb

	cases := []struct {
		arm   int
		swap  bool // expect the operands swapped in ucomisd
		which []byte
	}{
		{9, true, be},   // <
		{10, true, bt},  // <=
		{11, false, be}, // >
		{12, false, bt}, // >=
	}
	for _, c := range cases {
		b := decode151A8c(t, got[c.arm])
		i := bytes.LastIndex(b, ucm)
		if i < 0 {
			t.Errorf("arm %d has no ucomisd", c.arm)
			continue
		}
		modrm := b[i+3]
		gotDst, gotSrc := int(modrm>>3&7), int(modrm&7)
		wantDst, wantSrc := 0, 1
		if c.swap {
			wantDst, wantSrc = 1, 0
		}
		if gotDst != wantDst || gotSrc != wantSrc {
			t.Errorf("arm %d: ucomisd operands reg=%d rm=%d, want reg=%d rm=%d",
				c.arm, gotDst, gotSrc, wantDst, wantSrc)
		}
		if !bytes.Contains(b, c.which) {
			t.Errorf("arm %d is missing the expected branch % x", c.arm, c.which)
		}
		// And it must NOT carry the sibling branch.
		other := be
		if bytes.Equal(c.which, be) {
			other = bt
		}
		if bytes.Contains(b, other) {
			t.Errorf("arm %d wrongly contains % x", c.arm, other)
		}
	}
}

// TestPhase151A8c_DivideShortCircuitDisplacements checks the two rel32
// displacements in the zero-divisor guard from first principles rather than by
// reading them off the hex.
//
// The shape is: ucomisd; jnz TARGET_DIVIDE; xorpd xmm0,xmm0; jmp TARGET_END;
// TARGET_DIVIDE: divsd; TARGET_END:.
//
// From the ucomisd's own position the fall-through is the xorpd, so the jnz has
// to jump OVER it -- and over the jmp that follows -- landing exactly on the
// divsd. The jmp then jumps over that single divsd to the end.
func TestPhase151A8c_DivideShortCircuitDisplacements(t *testing.T) {
	got := floatStmtCorpus151A8c(t)
	b := decode151A8c(t, got[5])

	divsd := []byte{0xF2, 0x0F, 0x5E, 0xC1} // divsd xmm0, xmm1
	dAt := bytes.Index(b, divsd)
	if dAt < 0 {
		t.Fatalf("arm 5 has no divsd: % x", b)
	}
	jnz := bytes.LastIndex(b[:dAt], []byte{0x0F, 0x85})
	if jnz < 0 {
		t.Fatalf("arm 5 has no jnz before the divsd: % x", b)
	}
	jmp := bytes.LastIndex(b[:dAt], []byte{0xE9})
	if jmp < 0 {
		t.Fatalf("arm 5 has no jmp before the divsd: % x", b)
	}

	// The two branches have DIFFERENT opcode widths, which is a trap worth
	// naming: jnz is `0F 85 cd` (two opcode bytes, so cd is at at+2) while jmp
	// is `E9 cd` (ONE opcode byte, so cd is at at+1). Reading both with one
	// helper is off by one for the jmp, and the first draft of this test did
	// exactly that and reported a displacement of -234881024.
	readDisp2 := func(at int) int32 { // for 0F 8x forms
		return int32(uint32(b[at+2]) | uint32(b[at+3])<<8 |
			uint32(b[at+4])<<16 | uint32(b[at+5])<<24)
	}
	readDisp1 := func(at int) int32 { // for the E9 short form
		return int32(uint32(b[at+1]) | uint32(b[at+2])<<8 |
			uint32(b[at+3])<<16 | uint32(b[at+4])<<24)
	}
	// jnz at jnz, next instruction at jnz+6; it must land on the divsd.
	if got, want := readDisp2(jnz), int32(dAt-(jnz+6)); got != want {
		t.Errorf("jnz displacement = %d, want %d (land on the divsd at %d)", got, want, dAt)
	}
	// jmp at jmp, next instruction at jmp+5; it must land just past the divsd.
	if got, want := readDisp1(jmp), int32((dAt+len(divsd))-(jmp+5)); got != want {
		t.Errorf("jmp displacement = %d, want %d (land past the divsd)", got, want)
	}
}

// TestPhase151A8c_CorpusIsNonVacuousAndDeterministic guards an empty or unstable
// corpus, and pins the shared staging prefix so an arm that stopped staging
// cannot quietly report success.
func TestPhase151A8c_CorpusIsNonVacuousAndDeterministic(t *testing.T) {
	karkain := phase130Karkain(t)
	first := runKCCStep2(t, karkain, "native-value-floatstmt")
	if len(first) != floatStmtArms {
		t.Fatalf("corpus has %d lines, want %d", len(first), floatStmtArms)
	}
	total := 0
	for i, h := range first {
		if len(h) == 0 {
			t.Fatalf("corpus line %d is empty; an empty comparison is not evidence", i)
		}
		total += len(decode151A8c(t, h))
	}
	// Thirteen shapes, the shortest one instruction and the longest a dozen or
	// more; this floor catches an emitter that produced one arm for all of them.
	if total < 300 {
		t.Errorf("corpus carries only %d bytes, too few for thirteen float statements", total)
	}
	for run := 0; run < 2; run++ {
		again := runKCCStep2(t, karkain, "native-value-floatstmt")
		for i := range first {
			if again[i] != first[i] {
				t.Fatalf("run %d line %d differs", run+1, i)
			}
		}
	}
}

// TestPhase151A8c_NoGoFallback guards the measurement contract: the Go side must
// only RUN kcc, never compute the answer.
func TestPhase151A8c_NoGoFallback(t *testing.T) {
	src, err := os.ReadFile(filepath.Join(repoRoot(t), "pkg", "cli", "native_encode.go"))
	if err != nil {
		t.Fatalf("read native_encode.go: %v", err)
	}
	if bytes.Contains(src, []byte("func kccFloatStmt")) {
		t.Error("pkg/cli/native_encode.go appears to compute the float statements " +
			"in Go; the measurement surface must only run kcc")
	}
	if !bytes.Contains(src, []byte(`return kccSubcommand(w, "native-value-floatstmt")`)) {
		t.Error("KCCNativeValueFloatStmtCommand does not delegate to kccSubcommand")
	}
}
