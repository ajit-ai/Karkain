package cli

// Phase 151A Step 8e - floats through the call ABI in kcc.
//
// This slice exists to PROVE a claim rather than to add emission, and the claim
// is the load-bearing one for the whole float kind:
//
//     FLOATS NEED NO NEW ABI CODE.
//
// `kindUnits(KindFloat) == 1` in the oracle, and `natKindUnits` in kcc already
// returns 1 for a float, so a float parameter occupies exactly one frame slot and
// a float argument is staged, loaded and passed by the byte-for-byte identical
// path an int uses. What Step 8d built is already float-capable; this slice
// MEASURES that rather than asserting it.
//
// THE STRONGEST LAYER IS A CROSS-CORPUS COMPARISON, not a differential against a
// fresh expectation. Arm 1 stages the INT literal 42 through the FLOAT call
// shape, so it can be compared byte-for-byte against Step 8d's one-argument int
// call: the two must differ ONLY in the 10-byte movabs immediate. That is a
// stronger claim than "the float shape looks right" -- it says the float path
// contributes no ABI emission at all, and it is falsified by any new float-only
// instruction.
//
// NO EXECUTION EVIDENCE IS CLAIMED. Four reference shapes, not a program.

import (
	"bytes"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"

	"karkain/pkg/native"
)

const floatCallArms = 4

func decode151A8e(t *testing.T, h string) []byte {
	t.Helper()
	b, err := hex.DecodeString(h)
	if err != nil {
		t.Fatalf("corpus line %q is not hex: %v", h, err)
	}
	return b
}

func goFloatCall8e(arm int) []byte {
	e := native.NewEmitter()
	switch arm {
	case 0, 1: // a float argument, then the int literal 42 for comparison
		v := uint64(42)
		if arm == 0 {
			v = uint64(f15)
		}
		e.MovRegImm64(native.RAX, v)
		e.StoreStack(native.RAX, argTemp8d(0, callFrame))
		e.LoadStack(native.RDI, argTemp8d(0, callFrame))
		e.Call("fn_f")
		e.Mark("fn_f")
	case 2: // a float parameter homed from RDI, exactly as an int is
		e.SubRegImm32(native.RSP, int32(callFrame))
		e.StoreStack(native.RDI, callLocals)
	case 3: // a float return: the bits are in RAX, then a jmp to $ret
		e.MovRegImm64(native.RAX, uint64(f15))
		e.Jmp("$ret")
		e.Mark("$ret")
	default:
		panic("bad arm")
	}
	return e.Bytes()
}

func floatCallCorpus8e(t *testing.T) []string {
	t.Helper()
	karkain := phase130Karkain(t)
	got := runKCCStep2(t, karkain, "native-value-floatcall")
	if len(got) != floatCallArms {
		t.Fatalf("kcc native-value-floatcall produced %d lines, want %d:\n%v",
			len(got), floatCallArms, got)
	}
	return got
}

// TestPhase151A8e_FloatCallShapesMatchTheOracle is the RAW differential: every
// shape equals what the real pkg/native Emitter produces.
func TestPhase151A8e_FloatCallShapesMatchTheOracle(t *testing.T) {
	got := floatCallCorpus8e(t)
	for arm := 0; arm < floatCallArms; arm++ {
		g := decode151A8e(t, got[arm])
		w := goFloatCall8e(arm)
		if !bytes.Equal(g, w) {
			t.Errorf("arm %d: kcc=% x\n          oracle=% x", arm, g, w)
		}
	}
}

// TestPhase151A8e_FloatArgPathIsIdenticalToTheIntPath is the slice's actual
// claim.
//
// Arm 1 stages the int literal 42 through the float call shape; Step 8d's arm 0
// stages the int literal 7 through the int call shape. Both are the same
// sequence with a different immediate, so everything AFTER the 10-byte movabs must
// be byte-identical: the spill address, the RDI load and the call. If any
// float-only instruction existed, this fails.
func TestPhase151A8e_FloatArgPathIsIdenticalToTheIntPath(t *testing.T) {
	karkain := phase130Karkain(t)
	floatArm := decode151A8e(t, runKCCStep2(t, karkain, "native-value-floatcall")[1])
	intArm := decode151A8e(t, runKCCStep2(t, karkain, "native-value-call")[0])

	const movabs = 10 // REX.W + B8+rd + imm64
	if len(floatArm) != len(intArm) {
		t.Fatalf("float arm is %d bytes, int arm is %d; the float path must "+
			"contribute no ABI emission at all", len(floatArm), len(intArm))
	}
	// The immediates must genuinely differ, or the comparison below is vacuous.
	if bytes.Equal(floatArm[:movabs], intArm[:movabs]) {
		t.Fatal("the float arm's literal equals the int arm's, so this " +
			"comparison would pass even if the rest differed")
	}
	if !bytes.Equal(floatArm[movabs:], intArm[movabs:]) {
		t.Errorf("the float call path diverges from the int path after the literal:\n"+
			"  float % x\n  int   % x", floatArm[movabs:], intArm[movabs:])
	}
}

// TestPhase151A8e_FloatParamHomingIsIdenticalToTheIntPath makes the same point
// for the callee side: homing a FLOAT parameter is byte-identical to homing an
// int, because it is one unit either way.
func TestPhase151A8e_FloatParamHomingIsIdenticalToTheIntPath(t *testing.T) {
	karkain := phase130Karkain(t)
	floatArm := decode151A8e(t, runKCCStep2(t, karkain, "native-value-floatcall")[2])
	intArm := decode151A8e(t, runKCCStep2(t, karkain, "native-value-call")[4])

	if !bytes.Equal(floatArm, intArm) {
		t.Errorf("float param homing = % x, int param homing = % x; they must be "+
			"identical", floatArm, intArm)
	}
}

// TestPhase151A8e_FloatReturnDoesNotTearDownTheFrame pins the correction this
// slice made.
//
// The oracle's ReturnStmt case is `emitReturn(...)` then `Jmp("fn_<name>$ret")`
// (program.go:4893-4900); the `AddRsp(frame)` happens AT the `$ret` label in the
// epilogue (program.go:4863-4866). A first draft emitted `sub rsp, frame` before
// the jmp, which would tear the frame down twice.
//
// It also corrects a stale reference rather than inheriting one: the Step 2 note
// recorded the `$ret` jmp at program.go:3386, and that line now holds an
// unrelated function.
func TestPhase151A8e_FloatReturnDoesNotTearDownTheFrame(t *testing.T) {
	got := floatCallCorpus8e(t)
	b := decode151A8e(t, got[3])

	// movabs rax, <bits> then jmp rel32 (+0, since $ret is marked immediately).
	want := []byte{0x48, 0xB8}
	if !bytes.HasPrefix(b, want) {
		t.Fatalf("arm 3 does not begin with a movabs: % x", b)
	}
	jmp := len(b) - 5
	if b[jmp] != 0xE9 {
		t.Fatalf("arm 3 does not end with a jmp rel32: % x", b)
	}
	// No frame teardown anywhere: neither sub nor add rsp.
	for i := 0; i+3 <= len(b); i++ {
		if b[i] == 0x48 && (b[i+1] == 0x81 || b[i+1] == 0x83) &&
			(b[i+2] == 0xEC || b[i+2] == 0xC4) {
			t.Errorf("arm 3 contains a frame teardown at byte %d (% x); the return "+
				"statement must not touch rsp", i, b[i:i+3])
		}
	}
}

// TestPhase151A8e_CorpusIsNonVacuousAndDeterministic guards an empty or unstable
// corpus.
func TestPhase151A8e_CorpusIsNonVacuousAndDeterministic(t *testing.T) {
	karkain := phase130Karkain(t)
	first := runKCCStep2(t, karkain, "native-value-floatcall")
	if len(first) != floatCallArms {
		t.Fatalf("corpus has %d lines, want %d", len(first), floatCallArms)
	}
	total := 0
	for i, h := range first {
		if len(h) == 0 {
			t.Fatalf("corpus line %d is empty; an empty comparison is not evidence", i)
		}
		total += len(decode151A8e(t, h))
	}
	if total < 80 {
		t.Errorf("corpus carries only %d bytes, too few for four float call shapes", total)
	}
	for run := 0; run < 2; run++ {
		again := runKCCStep2(t, karkain, "native-value-floatcall")
		for i := range first {
			if again[i] != first[i] {
				t.Fatalf("run %d line %d differs", run+1, i)
			}
		}
	}
}

// TestPhase151A8e_NoGoFallback guards the measurement contract.
func TestPhase151A8e_NoGoFallback(t *testing.T) {
	src, err := os.ReadFile(filepath.Join(repoRoot(t), "pkg", "cli", "native_encode.go"))
	if err != nil {
		t.Fatalf("read native_encode.go: %v", err)
	}
	if !bytes.Contains(src, []byte(`return kccSubcommand(w, "native-value-floatcall")`)) {
		t.Error("KCCNativeValueFloatCallCommand does not delegate to kccSubcommand")
	}
}
