package cli

// Phase 151A Step 8f - strings through the call ABI in kcc.
//
// A string is the first TWO-UNIT kind: kindUnits(KindString) == 2. So this slice
// is where stageUnit's high-half rule is exercised at all, and where an
// argument's units can begin at an ODD global index -- arm 1 passes an int first,
// so the string's pointer rides RSI (global unit 1) and its length rides RDX
// (global unit 2), and the sequence still has to be correct.
//
// WHY THAT STILL WORKS, and it is the load-bearing subtlety. The high-half rule
// keys on the REGISTER (`if r == RSI { off += 8 }`), not on the unit index, while
// the load-back loop reads `argTemp(i) + k*8` where k is the unit index WITHIN
// the argument. The two agree because stageUnit is called with RSI in exactly one
// place -- the string branch passes RDI for the pointer and RSI for the length,
// and every other kind passes RAX. A corpus that only ever passed a string first
// could not distinguish an (arg, k) reader from a global-unit reader, which is
// exactly why arm 1 exists.
//
// NO EXECUTION EVIDENCE IS CLAIMED. Four reference shapes, not a program.
//
// RAW DIFFERENTIAL: a string LOCAL is the source rather than a string LITERAL, so
// no rodata reference is involved and no masking is needed. A literal would need
// one, which is unresolved at this layer and is what forced Step 8b's masked
// comparison.

import (
	"bytes"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"

	"karkain/pkg/native"
)

const (
	stringCallArms = 4
	// extrasCall8f is Step 8d's measured extrasBase, reused so both slices agree
	// on the layout rather than each carrying its own copy.
	extrasCall8f = 608
)

func decode151A8f(t *testing.T, h string) []byte {
	t.Helper()
	b, err := hex.DecodeString(h)
	if err != nil {
		t.Fatalf("corpus line %q is not hex: %v", h, err)
	}
	return b
}

func stringCallCorpus8f(t *testing.T) []string {
	t.Helper()
	karkain := phase130Karkain(t)
	got := runKCCStep2(t, karkain, "native-value-stringcall")
	if len(got) != stringCallArms {
		t.Fatalf("kcc native-value-stringcall produced %d lines, want %d:\n%v",
			len(got), stringCallArms, got)
	}
	return got
}

// goStringCall8f builds one arm with the REAL Go emitter, transcribing stageUnit
// and the load-back loop as the oracle writes them.
//
// stageUnit's rule: a unit inside the register budget goes to argTemp(arg), and
// the high half -- the one passed in RSI -- goes 8 bytes higher.
func goStringCall8f(arm int) []byte {
	e := native.NewEmitter()
	// stage takes the argument AND its own unit index within that argument. A
	// first draft folded them together and used arg*2 for both halves of a
	// string, which silently stored the length at the extras base instead of 8
	// bytes higher -- the register-based high-half rule is applied inside the
	// spill branch only, exactly as stageUnit does, so the extras branch must
	// still be told which unit it is holding.
	stage := func(arg, unit int, r native.Reg) {
		if unit < 6 {
			off := argTemp8d(arg, callFrame)
			if r == native.RSI {
				off += 8
			}
			e.StoreStack(r, off)
			return
		}
		e.StoreStack(r, extrasCall8f+(unit-6)*8)
	}
	// loadBack indexes by (arg, k), NOT by global unit -- the point of arm 1.
	loadBack := func(args int) {
		unit := 0
		for a := 0; a < args; a++ {
			if unit < 6 {
				e.LoadStack(argReg8d(unit), argTemp8d(a, callFrame))
				e.LoadStack(argReg8d(unit+1), argTemp8d(a, callFrame)+8)
			}
			unit += 2
		}
		if unit > 6 {
			e.LeaRegStack(native.R10, extrasCall8f)
		}
	}
	stringLocal := func(loc int) {
		e.LoadStack(native.RDI, loc)
		e.LoadStack(native.RSI, loc+8)
	}
	switch arm {
	case 0: // one string argument
		stringLocal(callLocals)
		stage(0, 0, native.RDI)
		stage(0, 1, native.RSI)
		loadBack(1)
		e.Call("fn_f")
		e.Mark("fn_f")
	case 1: // an int, then a string whose units start at an ODD global index
		e.MovRegImm64(native.RAX, 7)
		e.StoreStack(native.RAX, argTemp8d(0, callFrame))
		stringLocal(callLocals)
		stage(1, 2, native.RDI)
		stage(1, 3, native.RSI)
		e.LoadStack(native.RDI, argTemp8d(0, callFrame))
		e.LoadStack(native.RSI, argTemp8d(1, callFrame))
		e.LoadStack(native.RDX, argTemp8d(1, callFrame)+8)
		e.Call("fn_f")
		e.Mark("fn_f")
	case 2: // callee homing a two-unit parameter
		e.SubRegImm32(native.RSP, int32(callFrame))
		e.StoreStack(native.RDI, callLocals)
		e.StoreStack(native.RSI, callLocals+8)
	case 3: // four string arguments: eight units, so units 6 and 7 go to extras
		for a := 0; a < 4; a++ {
			stringLocal(callLocals + a*16)
			stage(a, a*2, native.RDI)
			stage(a, a*2+1, native.RSI)
		}
		loadBack(4)
		e.Call("fn_f")
		e.Mark("fn_f")
	default:
		panic("bad arm")
	}
	return e.Bytes()
}

// TestPhase151A8f_StringCallShapesMatchTheOracle is the RAW differential.
func TestPhase151A8f_StringCallShapesMatchTheOracle(t *testing.T) {
	got := stringCallCorpus8f(t)
	for arm := 0; arm < stringCallArms; arm++ {
		g := decode151A8f(t, got[arm])
		w := goStringCall8f(arm)
		if !bytes.Equal(g, w) {
			t.Errorf("arm %d: kcc=% x\n          oracle=% x", arm, g, w)
		}
	}
}

// disp32After reads the little-endian disp32 that follows a 4-byte ModRM+SIB
// prefix. Every stack slot in these sequences is >= 508, which does not fit a
// signed disp8, so the encodings are always mod=10 with a disp32 -- checking the
// RANGE first rather than assuming disp8 is the lesson Step 8d's test had to
// learn the hard way.
func disp32After(t *testing.T, b, prefix []byte) int {
	t.Helper()
	i := bytes.Index(b, prefix)
	if i < 0 {
		t.Fatalf("no % x in % x", prefix, b)
	}
	d := b[i+len(prefix) : i+len(prefix)+4]
	return int(int32(uint32(d[0]) | uint32(d[1])<<8 | uint32(d[2])<<16 | uint32(d[3])<<24))
}

// TestPhase151A8f_OddIndexedStringUsesTheArgRelativeConvention is the layer that
// justifies arm 1.
//
// The string in arm 1 begins at GLOBAL unit index 1, so its pointer is staged
// from RSI and its length from RDX. A reader indexing by global unit rather than
// by (arg, k) would deliver them to the wrong registers here.
func TestPhase151A8f_OddIndexedStringUsesTheArgRelativeConvention(t *testing.T) {
	got := stringCallCorpus8f(t)
	b := decode151A8f(t, got[1])

	// arg0's int into RDI, then the string's low half into RSI and its high half
	// into RDX -- the (arg, k) sequence, spelled out with disp32 operands.
	want := []byte{
		0x48, 0x8B, 0xBC, 0x24, 0x08, 0x02, 0x00, 0x00, // mov rdi, [rsp+argTemp(0)]
		0x48, 0x8B, 0xB4, 0x24, 0x18, 0x02, 0x00, 0x00, // mov rsi, [rsp+argTemp(1)]
		0x48, 0x8B, 0x94, 0x24, 0x20, 0x02, 0x00, 0x00, // mov rdx, [rsp+argTemp(1)+8]
		0xE8, 0x00, 0x00, 0x00, 0x00, // call rel32 (+0)
	}
	if !bytes.Contains(b, want) {
		t.Errorf("arm 1's load-back is not the (arg, k) sequence:\n  got  % x\n  want % x",
			b, want)
	}
	// And the staging put the string's low half at argTemp(1) exactly, which is
	// what the 16-byte stride is for.
	if !bytes.Contains(b, []byte{0x48, 0x89, 0xBC, 0x24, 0x18, 0x02, 0x00, 0x00}) {
		t.Errorf("arm 1 does not stage the string's pointer at argTemp(1): % x", b)
	}
}

// TestPhase151A8f_HighHalfRuleKeysOnTheRegister pins the rule stageUnit uses.
//
// It picks the slot with `if r == RSI { off += 8 }` -- a test on the REGISTER,
// not on the unit index. That is sound only because RSI is passed in exactly one
// place; a change passing RSI for something that was not a string's second unit
// would swap the halves silently. This asserts the rule at both call sites that
// matter: the string's pointer at +0 and its length at +8.
func TestPhase151A8f_HighHalfRuleKeysOnTheRegister(t *testing.T) {
	got := stringCallCorpus8f(t)
	b := decode151A8f(t, got[0]) // one string argument

	low := disp32After(t, b, []byte{0x48, 0x89, 0xBC, 0x24})  // mov [rsp+d], rdi
	high := disp32After(t, b, []byte{0x48, 0x89, 0xB4, 0x24}) // mov [rsp+d], rsi

	if low != argTemp8d(0, callFrame) {
		t.Errorf("string pointer staged at %d, want argTemp(0)=%d",
			low, argTemp8d(0, callFrame))
	}
	if want := argTemp8d(0, callFrame) + 8; high != want {
		t.Errorf("string length staged at %d, want %d (argTemp(0)+8)", high, want)
	}
	if high-low != 8 {
		t.Errorf("the two halves are %d apart, want 8", high-low)
	}
}

// TestPhase151A8f_ExtrasUnitsLeaveTheSpill checks that units 6 and 7 -- the last
// two of four string arguments -- travel in the caller's extras area rather than
// to a per-arg spill slot, and that R10 is materialised to point at them.
//
// This is the first case that combines the extras region with the two-unit
// stride, so a corpus without it would never exercise both at once.
func TestPhase151A8f_ExtrasUnitsLeaveTheSpill(t *testing.T) {
	got := stringCallCorpus8f(t)
	b := decode151A8f(t, got[3])

	// The 4th string's ptr/len are the only stores at the extras base and +8.
	want := []byte{
		0x48, 0x89, 0xBC, 0x24, 0x60, 0x02, 0x00, 0x00, // mov [rsp+extras], rdi
		0x48, 0x89, 0xB4, 0x24, 0x68, 0x02, 0x00, 0x00, // mov [rsp+extras+8], rsi
	}
	if !bytes.Contains(b, want) {
		t.Errorf("arm 3 does not stage units 6 and 7 at the extras base: % x", b)
	}
	// And R10 is materialised, since two units exceeded the budget.
	if !bytes.Contains(b, []byte{0x4C, 0x8D, 0x94, 0x24, 0x60, 0x02, 0x00, 0x00}) {
		t.Errorf("arm 3 does not materialise R10 with the extras base: % x", b)
	}
}

// TestPhase151A8f_CorpusIsNonVacuousAndDeterministic guards an empty or
// unstable corpus.
func TestPhase151A8f_CorpusIsNonVacuousAndDeterministic(t *testing.T) {
	karkain := phase130Karkain(t)
	first := runKCCStep2(t, karkain, "native-value-stringcall")
	if len(first) != stringCallArms {
		t.Fatalf("corpus has %d lines, want %d", len(first), stringCallArms)
	}
	total := 0
	for i, h := range first {
		if len(h) == 0 {
			t.Fatalf("corpus line %d is empty; an empty comparison is not evidence", i)
		}
		total += len(decode151A8f(t, h))
	}
	if total < 150 {
		t.Errorf("corpus carries only %d bytes, too few for four string call shapes", total)
	}
	for run := 0; run < 2; run++ {
		again := runKCCStep2(t, karkain, "native-value-stringcall")
		for i := range first {
			if again[i] != first[i] {
				t.Fatalf("run %d line %d differs", run+1, i)
			}
		}
	}
}

// TestPhase151A8f_NoGoFallback guards the measurement contract.
func TestPhase151A8f_NoGoFallback(t *testing.T) {
	src, err := os.ReadFile(filepath.Join(repoRoot(t), "pkg", "cli", "native_encode.go"))
	if err != nil {
		t.Fatalf("read native_encode.go: %v", err)
	}
	if !bytes.Contains(src, []byte(`return kccSubcommand(w, "native-value-stringcall")`)) {
		t.Error("KCCNativeValueStringCallCommand does not delegate to kccSubcommand")
	}
}
