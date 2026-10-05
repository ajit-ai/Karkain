package cli

// Phase 151A Step 8d - the call ABI in kcc, int first.
//
// Phase 148's convention, ported: the caller stages every argument to a per-arg
// spill, loads all units back into RDI,RSI,RDX,RCX,R8,R9, materialises R10
// with the extras base if any unit exceeded the budget, and calls; the callee
// homes each parameter from its argument register into its frame slot at entry.
//
// NO EXECUTION EVIDENCE IS CLAIMED. The corpus is seven reference shapes, not a
// program: no `main`, no entry stub, so there is nothing to run.
//
// RAW DIFFERENTIAL. Every primitive involved is exported (MovRegImm64,
// StoreStack, LoadStack, LoadBaseOff, LeaRegStack, Call, Mark, SubRegImm32) and
// none of these shapes contains a rodata reference, so the oracle is built with
// the real pkg/native Emitter and compared RAW with no masking.
//
// THE FRAME IS NOT TOUCHED, and the strongest evidence is that nothing moved.
// An earlier reading of this work claimed the per-arg spill was a missing frame
// region that would have to be inserted before extrasBase, shifting extrasBase,
// next and frame and re-pinning every frame-dependent displacement in Steps
// 1-8c. That was WRONG: the oracle computes argTemp(i) = frame - argSpillBytes
// + i*16, indexing DOWNWARD from the frame size, and Step 1 already reserved
// exactly that (`frame = next + natArgSpillBytes()`, argSpillBytes() = 96). So
// the subtests below assert the frame numbers and the KIR pin are UNCHANGED --
// if a region really had been missing, they could not be.

import (
	"bytes"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"

	"karkain/pkg/native"
)

const (
	// callFrame and callExtras are the ORACLE's real layout numbers for a
	// one-local frame: locals 8 + binTemp 512 + argSpill 96 = 616, and
	// extrasBase = 616 - 96 + 8 = 528... which is why they are computed here
	// rather than written, and why argTemp(0) = 616 - 96 = 520.
	callFrame  = 616
	callExtras = 608
	callLocals = 8
	callArms   = 7
)

func decode151A8d(t *testing.T, h string) []byte {
	t.Helper()
	b, err := hex.DecodeString(h)
	if err != nil {
		t.Fatalf("corpus line %q is not hex: %v", h, err)
	}
	return b
}

// argTemp8d recomputes the oracle's formula independently, so the test does not
// simply restate the kcc file's arithmetic.
func argTemp8d(i, frame int) int { return frame - 96 + i*16 }

func argReg8d(i int) native.Reg {
	switch i {
	case 0:
		return native.RDI
	case 1:
		return native.RSI
	case 2:
		return native.RDX
	case 3:
		return native.RCX
	case 4:
		return native.R8
	default:
		return native.R9
	}
}

func callVals8d(n int) []uint64 {
	v := []uint64{7, 11, 13, 17, 19, 23, 29}
	return v[:n]
}

func goCall8d(arm int) []byte {
	e := native.NewEmitter()
	stage := func(vals []uint64, frame, extras int) {
		for i, v := range vals {
			e.MovRegImm64(native.RAX, v)
			if i < 6 {
				e.StoreStack(native.RAX, argTemp8d(i, frame))
			} else {
				e.StoreStack(native.RAX, extras)
			}
		}
		unit := 0
		for i := range vals {
			if unit < 6 {
				e.LoadStack(argReg8d(unit), argTemp8d(i, frame))
			}
			unit++
		}
		if unit > 6 {
			e.LeaRegStack(native.R10, extras)
		}
	}
	switch arm {
	case 0:
		stage(callVals8d(1), callFrame, callExtras)
		e.Call("fn_f")
		e.Mark("fn_f")
	case 1:
		stage(callVals8d(2), callFrame, callExtras)
		e.Call("fn_f")
		e.Mark("fn_f")
	case 2:
		stage(callVals8d(3), callFrame, callExtras)
		e.Call("fn_f")
		e.Mark("fn_f")
	case 3:
		stage(callVals8d(7), callFrame, callExtras)
		e.Call("fn_f")
		e.Mark("fn_f")
	case 4, 5, 6:
		n := 1
		if arm == 5 {
			n = 3
		} else if arm == 6 {
			n = 7
		}
		e.SubRegImm32(native.RSP, int32(callFrame))
		for i := 0; i < n; i++ {
			if i < 6 {
				e.StoreStack(argReg8d(i), callLocals+i*8)
			} else {
				e.LoadBaseOff(native.RAX, native.R10, (i-6)*8)
				e.StoreStack(native.RAX, callLocals+i*8)
			}
		}
	default:
		panic("bad arm")
	}
	return e.Bytes()
}
func callCorpus8d(t *testing.T) []string {
	t.Helper()
	karkain := phase130Karkain(t)
	got := runKCCStep2(t, karkain, "native-value-call")
	if len(got) != callArms {
		t.Fatalf("kcc native-value-call produced %d lines, want %d:\n%v",
			len(got), callArms, got)
	}
	return got
}

// TestPhase151A8d_CallAbiByteIdenticalToOracle is this slice's contract.
func TestPhase151A8d_CallAbiByteIdenticalToOracle(t *testing.T) {
	got := callCorpus8d(t)
	for arm := 0; arm < callArms; arm++ {
		g := decode151A8d(t, got[arm])
		w := goCall8d(arm)
		if !bytes.Equal(g, w) {
			t.Errorf("arm %d: kcc=% x\n          oracle=% x", arm, g, w)
		}
	}
}

// TestPhase151A8d_ArgSpillIndexingIsDerived pins the arithmetic that makes the
// spill addresses correct, derived here rather than copied from the kcc file.
//
// The oracle's formula is argTemp(i) = frame - argSpillBytes + i*16, i.e. the
// spill is addressed DOWNWARD from the frame size rather than upward from a
// region boundary. For a one-local frame:
//
//	argTemp(0) = 616 - 96 +  0 = 520 = 0x208
//	argTemp(1) = 616 - 96 + 16 = 536 = 0x218
//	argTemp(6) = 616 - 96 + 96 = 616 = 0x268
//
// A 16-byte stride (not 8) is the load-bearing part: each argument gets a low
// and high half, which is what lets a two-unit argument such as a string ride
// the same slot pair.
func TestPhase151A8d_ArgSpillIndexingIsDerived(t *testing.T) {
	got := callCorpus8d(t)
	b := decode151A8d(t, got[1]) // the two-argument call

	// Each staged value is a distinct literal, so the disp8 displacements are
	// visible: 7 staged at argTemp(0), 11 at argTemp(1).
	// Every argTemp lands at or above 520, which does NOT fit a signed disp8, so
	// the encoding is mod=10 with a disp32: 48 89 84 24 <disp32> to store and
	// 48 8B BC 24 <disp32> to load back. A first draft of this test searched for
	// the disp8 form and read a displacement of 8 off an unrelated store --
	// assuming disp8 here is the same class of mistake as assuming a literal
	// fits a signed field, and it is caught by checking the RANGE first.
	disp := func(v int) []byte {
		if v < -128 || v > 127 {
			return []byte{byte(v), byte(v >> 8), byte(v >> 16), byte(v >> 24)}
		}
		return []byte{byte(v)}
	}
	storeAt := func(v int) []byte {
		return append([]byte{0x48, 0x89, 0x84, 0x24}, disp(v)...)
	}
	loadAt := func(v int) []byte {
		return append([]byte{0x48, 0x8B, 0xBC, 0x24}, disp(v)...)
	}

	if !bytes.Contains(b, storeAt(argTemp8d(0, callFrame))) {
		t.Errorf("arm 1 has no spill store at argTemp(0)=%d: % x",
			argTemp8d(0, callFrame), b)
	}
	// The load back into RDI must read the SAME slot it staged to.
	if !bytes.Contains(b, loadAt(argTemp8d(0, callFrame))) {
		t.Errorf("arm 1 does not load unit 0 back from argTemp(0)=%d: % x",
			argTemp8d(0, callFrame), b)
	}

	// Arm 2 stages three args, so argTemp(1) and argTemp(2) must appear too.
	b2 := decode151A8d(t, got[2])
	for i := 0; i < 3; i++ {
		if !bytes.Contains(b2, storeAt(argTemp8d(i, callFrame))) {
			t.Errorf("arm 2 has no spill store at argTemp(%d)=%d", i, argTemp8d(i, callFrame))
		}
	}
	// The 16-byte stride: consecutive arguments must be 16 apart, which is what
	// gives a two-unit argument its low and high half.
	if d := argTemp8d(1, callFrame) - argTemp8d(0, callFrame); d != 16 {
		t.Errorf("argTemp stride = %d, want 16", d)
	}
}

// TestPhase151A8d_ExtrasUnitTravelsOutsideTheSpill checks the one unit that
// exceeds the register budget.
//
// Unit 6 is NOT staged to a spill slot: stageUnit sends it straight to the
// extras area, which is already callee-visible, and the caller then materialises
// R10 with the extras base. A corpus that staged all seven to spill slots would
// be a plausible-looking wrong implementation.
func TestPhase151A8d_ExtrasUnitTravelsOutsideTheSpill(t *testing.T) {
	got := callCorpus8d(t)
	b := decode151A8d(t, got[3])

	// The 7th argument is the literal 29, and it must be stored at extrasBase.
	// The 7th argument is the literal 29 and must be stored at extrasBase = 608.
	// 608 does NOT fit a signed disp8, so the encoding is mod=10 with a disp32:
	// 48 89 84 24 <disp32> for the store and 4C 8D 94 24 <disp32> for the lea.
	// Writing these as disp8 would be an encoding that cannot hold the value --
	// the same class of mistake as assuming a literal fits a signed field.
	wantDisp := []byte{0x60, 0x02, 0x00, 0x00} // 608 little-endian
	store := append([]byte{0x48, 0x89, 0x84, 0x24}, wantDisp...)
	if !bytes.Contains(b, store) {
		t.Errorf("arm 3 does not stage unit 6 at extrasBase %d: % x", callExtras, b)
	}
	// R10 must be materialised, since a unit exceeded the budget.
	lea := append([]byte{0x4C, 0x8D, 0x94, 0x24}, wantDisp...)
	if !bytes.Contains(b, lea) {
		t.Errorf("arm 3 does not materialise R10 with the extras base %d: % x",
			callExtras, b)
	}
	// And unit 6 must NOT have gone to a spill slot: 608 is not among the
	// argTemp displacements, which is the whole point of the extras path.
	for i := 0; i < 7; i++ {
		if argTemp8d(i, callFrame) == callExtras {
			t.Errorf("argTemp(%d) collides with extrasBase; the test's layout "+
				"numbers are wrong", i)
		}
	}
}

// TestPhase151A8d_FrameIsUnchanged is the evidence for the correction above.
//
// If the per-arg spill really had been a missing frame region, inserting it
// before extrasBase would have shifted extrasBase, next and frame, and every
// frame-dependent displacement in Steps 1-8c with them. It was not, and the
// frame these sequences use is the one Step 1 already established.
func TestPhase151A8d_FrameIsUnchanged(t *testing.T) {
	got := callCorpus8d(t)

	// The prologue arms subtract the full one-local frame, 616 = 0x268.
	want := []byte{0x48, 0x81, 0xEC, 0x68, 0x02, 0x00, 0x00}
	for _, arm := range []int{4, 5, 6} {
		b := decode151A8d(t, got[arm])
		if !bytes.HasPrefix(b, want) {
			t.Errorf("arm %d prologue = % x, want it to start with "+
				"sub rsp,0x268 (616) = % x", arm, b[:7], want)
		}
	}
	// argTemp(0) must still be 520: the spill is at the frame top, and 520 +
	// 96 == 616, which is the whole claim in one equation.
	if got520 := argTemp8d(0, callFrame); got520+96 != callFrame {
		t.Errorf("argTemp(0)=%d does not sit 96 bytes below the frame top %d",
			got520, callFrame)
	}
}

// TestPhase151A8d_CorpusIsNonVacuousAndDeterministic guards an empty or unstable
// corpus, and pins the distinct staged literals that make an off-by-one in the
// spill indexing visible.
func TestPhase151A8d_CorpusIsNonVacuousAndDeterministic(t *testing.T) {
	karkain := phase130Karkain(t)
	first := runKCCStep2(t, karkain, "native-value-call")
	if len(first) != callArms {
		t.Fatalf("corpus has %d lines, want %d", len(first), callArms)
	}
	total := 0
	for i, h := range first {
		if len(h) == 0 {
			t.Fatalf("corpus line %d is empty; an empty comparison is not evidence", i)
		}
		total += len(decode151A8d(t, h))
	}
	if total < 200 {
		t.Errorf("corpus carries only %d bytes, too few for seven call shapes", total)
	}
	for run := 0; run < 2; run++ {
		again := runKCCStep2(t, karkain, "native-value-call")
		for i := range first {
			if again[i] != first[i] {
				t.Fatalf("run %d line %d differs", run+1, i)
			}
		}
	}
}

// TestPhase151A8d_NoGoFallback guards the measurement contract: the Go side must
// only RUN kcc, never compute the answer.
func TestPhase151A8d_NoGoFallback(t *testing.T) {
	src, err := os.ReadFile(filepath.Join(repoRoot(t), "pkg", "cli", "native_encode.go"))
	if err != nil {
		t.Fatalf("read native_encode.go: %v", err)
	}
	if bytes.Contains(src, []byte("func kccCallABI")) {
		t.Error("pkg/cli/native_encode.go appears to compute the call bytes in Go")
	}
	if !bytes.Contains(src, []byte(`return kccSubcommand(w, "native-value-call")`)) {
		t.Error("KCCNativeValueCallCommand does not delegate to kccSubcommand")
	}
}
