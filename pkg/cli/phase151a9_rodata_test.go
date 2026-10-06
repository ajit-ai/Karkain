package cli

// Phase 151A Step 9a - rodata resolution in kcc.
//
// Every absolute address is emitted as a 10-byte `mov r64, imm64` with the
// immediate left at zero and the POSITION recorded. That was always true -- it is
// what natImm64Patch produces. What was missing is the other half: nothing
// recorded WHICH rodata byte each position refers to, so print_int's two `"-"` and
// `"\n"` references had sites but no destinations. This is the piece every
// section of PHASE-151A-BASELINE names as the thing blocking 151D.
//
// NO EXECUTION EVIDENCE IS CLAIMED: this resolves addresses into a byte buffer.
// It emits no image and runs nothing. 9b links, 9c executes.
//
// Layers, each catching a class the others cannot:
//   1. interning, asserted on LENGTH so a non-interning section cannot pass;
//   2. the placeholder is zero BEFORE resolution (the ordering discipline);
//   3. the resolved immediate is exactly base + rva + offset, little-endian,
//      with the arithmetic derived rather than only the literal stated;
//   4. only the immediate bytes changed, so a shifted site or a rewritten buffer
//      fails;
//   5. an explicit site count, so a resolver that filled nothing cannot pass by
//      comparing two all-zero buffers;
//   6. the no-Go-fallback guard.

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

const (
	rodBase  uint64 = 0x140000000 // the PE base Step 5/6 use
	rodRVA   uint64 = 0x3000      // the PE .rodata RVA
	rodArms  int    = 4
	rodSites int    = 2
)

// rodOffs is the rodata offset each site refers to: 0 for "-", 2 for "abc".
var rodOffs = [...]int{0, 2}

func decodeRodata9a(t *testing.T, h string) []byte {
	t.Helper()
	b, err := hex.DecodeString(h)
	if err != nil {
		t.Fatalf("corpus line %q is not hex: %v", h, err)
	}
	return b
}

func rodataCorpus9a(t *testing.T) []string {
	t.Helper()
	karkain := phase130Karkain(t)
	got := runKCCStep2(t, karkain, "native-value-rodata")
	if len(got) != rodArms {
		t.Fatalf("kcc native-value-rodata produced %d lines, want %d:\n%v",
			len(got), rodArms, got)
	}
	return got
}

// rodFindSites9a returns the byte index of each 8-byte immediate of a `movabs rsi`
// (`48 BE`) site.
func rodFindSites9a(t *testing.T, code []byte, who string) []int {
	t.Helper()
	var sites []int
	for i := 0; i+10 <= len(code); i++ {
		if code[i] == 0x48 && code[i+1] == 0xBE {
			sites = append(sites, i)
			i += 9
		}
	}
	if len(sites) != rodSites {
		t.Fatalf("%s: found %d rodata sites, want %d (buffer % x)",
			who, len(sites), rodSites, code)
	}
	return sites
}

// TestPhase151A9a_RodataSectionInternsDuplicates checks the section.
//
// The corpus is ["-", "\n", "abc", "-"]. The oracle's internRodata deduplicates by
// first use, so the repeated "-" occupies ONE entry and the section is exactly 5
// bytes: 2d 0a 61 62 63. An implementation that appended every occurrence yields
// the same first five bytes and a 6-byte section, so the LENGTH assertion is what
// makes this non-vacuous; checking only the prefix would pass either way.
func TestPhase151A9a_RodataSectionInternsDuplicates(t *testing.T) {
	sec := decodeRodata9a(t, rodataCorpus9a(t)[0])
	want := []byte{0x2D, 0x0A, 0x61, 0x62, 0x63} // "-", newline, "abc"
	if !bytes.Equal(sec, want) {
		t.Errorf("rodata section = % x, want % x", sec, want)
	}
	if len(sec) != 5 {
		t.Errorf("rodata section is %d bytes, want 5: the repeated \"-\" was not "+
			"interned (the oracle's internRodata dedupes by first use)", len(sec))
	}
}

// TestPhase151A9a_PlaceholdersAreZeroBeforeResolution pins the ordering
// discipline: the emitter leaves the immediate at zero and the LINKER fills it. An
// emitter that filled it in would guess a number the container writer owns, and the
// base is not even known until the writer has chosen a layout.
func TestPhase151A9a_PlaceholdersAreZeroBeforeResolution(t *testing.T) {
	code := decodeRodata9a(t, rodataCorpus9a(t)[1])
	for n, at := range rodFindSites9a(t, code, "pre-resolution") {
		for j := at + 2; j < at+10; j++ {
			if code[j] != 0 {
				t.Errorf("site %d at byte %d has non-zero placeholder byte %#x; "+
					"the emitter must not fill the address in", n, at, code[j])
			}
		}
	}
}

// TestPhase151A9a_ResolvedAddressesAreBasePlusRvaPlusOffset is the slice's
// contract, with the expected values stated rather than read off the output.
func TestPhase151A9a_ResolvedAddressesAreBasePlusRvaPlusOffset(t *testing.T) {
	code := decodeRodata9a(t, rodataCorpus9a(t)[2])
	for n, at := range rodFindSites9a(t, code, "resolved") {
		v := binary.LittleEndian.Uint64(code[at+2 : at+10])
		if derived := rodBase + rodRVA + uint64(rodOffs[n]); v != derived {
			t.Errorf("site %d = %#x, want base %#x + rva %#x + offset %d = %#x",
				n, v, rodBase, rodRVA, rodOffs[n], derived)
		}
	}
}

// TestPhase151A9a_OnlyTheImmediatesChanged pins that resolution is surgical, so a
// resolver that shifted a recorded position by one, or rewrote the buffer, fails.
func TestPhase151A9a_OnlyTheImmediatesChanged(t *testing.T) {
	got := rodataCorpus9a(t)
	before := decodeRodata9a(t, got[1])
	after := decodeRodata9a(t, got[2])
	if len(before) != len(after) {
		t.Fatalf("resolution changed the buffer length: %d -> %d", len(before), len(after))
	}
	inImm := func(i int) bool {
		for _, at := range rodFindSites9a(t, after, "diff check") {
			if i >= at+2 && i < at+10 {
				return true
			}
		}
		return false
	}
	diff := 0
	for i := range before {
		if before[i] != after[i] {
			diff++
			if !inImm(i) {
				t.Errorf("byte %d changed outside any rodata immediate", i)
			}
		}
	}
	if diff == 0 {
		t.Fatal("resolution changed nothing; this corpus cannot detect a resolver " +
			"that fails to work")
	}
	// Two 8-byte immediates. Each address has four zero high bytes, so 8 differing
	// bytes is the ceiling; more means something else was touched.
	if diff > 8 {
		t.Errorf("resolution changed %d bytes, more than the two 8-byte immediates", diff)
	}
}

// TestPhase151A9a_SiteCountIsReported guards against an empty comparison: a
// resolver that filled nothing would otherwise be compared against an all-zero
// buffer and pass.
func TestPhase151A9a_SiteCountIsReported(t *testing.T) {
	n, err := strconv.Atoi(rodataCorpus9a(t)[3])
	if err != nil {
		t.Fatalf("site count is not a number: %v", err)
	}
	if n != rodSites {
		t.Errorf("kcc reported %d rodata sites, want %d", n, rodSites)
	}
}

// TestPhase151A9a_CorpusIsNonVacuousAndDeterministic re-runs, compares, and
// asserts byte-length floors so a corpus that stopped carrying real content could
// not report success.
func TestPhase151A9a_CorpusIsNonVacuousAndDeterministic(t *testing.T) {
	first := rodataCorpus9a(t)
	again := rodataCorpus9a(t)
	for i := range first {
		if first[i] != again[i] {
			t.Errorf("line %d differs between runs:\n  %s\n  %s", i, first[i], again[i])
		}
	}
	if len(first[0]) != 10 {
		t.Errorf("rodata hex is %d chars, want 10 (5 bytes): %q", len(first[0]), first[0])
	}
	// The resolved buffer's length is DERIVED from the instruction sizes rather
	// than restated: a movabs is 10 bytes and a `mov edi, imm32` is 5 (`bf` plus a
	// 4-byte immediate). Writing that as a literal is how this slice first
	// asserted 52 chars instead of 60 -- and a length expectation that is a
	// guess dressed as a requirement is worse than none.
	derived := (rodSites*10 + 2*5) * 2
	if len(first[2]) != derived {
		t.Errorf("resolved code hex is %d chars, want %d (%d bytes = %d x movabs(10) + "+
			"2 x mov edi imm32(5)): %q", len(first[2]), derived, derived/2, rodSites, first[2])
	}
}

// TestPhase151A9a_NoGoFallback keeps the contract every Step 8 slice holds: the
// command delegates to kcc with no Go encoder standing in for it. A Go
// implementation producing the same bytes would pass this gate while proving
// nothing about kcc, which is the exact dishonesty increment 151 exists to end.
func TestPhase151A9a_NoGoFallback(t *testing.T) {
	src, err := os.ReadFile(filepath.Join(repoRoot(t), "pkg", "cli", "native_encode.go"))
	if err != nil {
		t.Fatalf("read native_encode.go: %v", err)
	}
	if !bytes.Contains(src, []byte(`return kccSubcommand(w, "native-value-rodata")`)) {
		t.Error("KCCNativeValueRodataCommand does not delegate to kccSubcommand")
	}
}
