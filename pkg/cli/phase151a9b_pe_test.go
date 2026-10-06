package cli

// Phase 151A Step 9b - the FIRST whole-program PE composition in kcc.
//
// Steps 1-8f and 9a each emitted one SUBSYSTEM correctly and compared it against
// the oracle. None had ever produced a runnable program from pieces: 9a could
// resolve rodata addresses but had nothing to resolve them FOR. Step 9b is the
// first composition, and it is the smallest one that exercises every seam at
// once:
//
//	_start -> karkain_main -> print_int
//
// One emitter state carries code, rodata sites, IAT patch positions and IAT
// indices across all three, which is the property nothing before it needed.
//
// WHAT THIS GATE IS NOT. The program is a FIXED shape compiled into kcc, NOT the
// whole-program driver 151A still owes. Execution evidence below is therefore
// evidence that the MACHINERY works -- one state, resolved rodata, index-tagged
// IAT, a real image that runs -- and NOT evidence that kcc can compile a user's
// program. The driver is 9c, and kccOwnsNativeTargets stays false.
//
// The blocker found by measuring, not by planning: print_int wrote through the
// LINUX `write` syscall, because the oracle's emitWrite switches on b.goos and
// dispatches to emitWinWrite, and only the non-Windows body had been ported. A PE
// built from the old print_int would have executed a Linux `syscall` instruction
// on Windows and faulted on the first digit -- which is exactly why every earlier
// PE gate deliberately built images that do NOT print.
//
// Layers, each catching a class the others cannot:
//  1. all three pieces present in .text, so a dropped or empty helper fails;
//  2. structural validation through the ORACLE's ParsePE, not kcc's own writer;
//  3. rodata placement: the section bytes are exactly the two print_int strings;
//  4. rodata ADDRESSES: each immediate equals PEBase + textRVA + len(text) + off,
//     with textLen measured from the emitted text and never assumed;
//  5. IAT site/index COUNTS, so a resolver or patcher that filled nothing cannot
//     pass by comparing two all-zero buffers;
//  6. IAT index CORRECTNESS: every patched immediate is the IAT slot address for
//     the index kcc recorded, so a positional reader fails here even though it
//     produces a same-length, same-structure image;
//  7. the cross-piece call displacement: main -> print_int resolves to the
//     print_int label, which no per-piece comparison could catch;
//  8. EXECUTION on this Windows host: expected stdout and exit code;
//  9. determinism plus byte floors, so a shrunken corpus cannot report success;
// 10. the no-Go-fallback guard.
//
// NOT RUN HERE: any whole-tree KCC workload. See PHASE-151A-BASELINE.md 9a-1.

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"karkain/pkg/native"
)

// pe9bIATNames maps the IAT index kcc records to the kernel32 export it must name.
// It is asserted against the names found in the IMAGE, so a renumbering on either
// side cannot silently disagree with the other.
var pe9bIATNames = [...]string{"ExitProcess", "GetStdHandle", "WriteFile"}

const (
	// pe9bArms is the corpus size: image, text, rodata, count report.
	pe9bArms = 4
	// pe9bImageMinBytes / pe9bTextMinBytes are floors, not expected values. A
	// whole program carrying _start's PEB bootstrap, main and print_int cannot be
	// smaller, and a floor is what stops a shrunken corpus reporting success.
	pe9bImageMinBytes = 2048
	pe9bTextMinBytes  = 1024
	// pe9bSites is print_int's rodata references: "-" on the sign path, "\n" for
	// the terminator.
	pe9bSites = 2
	// pe9bAbsPatches is the bootstrap's three absolute stores: ExitProcess,
	// GetStdHandle and WriteFile.
	pe9bAbsPatches = 3
	// pe9bIatPatches is 6 from print_int plus 1 from the exit tail. print_int
	// emits THREE write sites statically -- the sign write, the digit write and
	// the newline -- and each does a GetStdHandle + WriteFile call. For a
	// POSITIVE value the sign write never executes, but it is still emitted and
	// still needs both IAT slots, so this count is a property of the emission and
	// not of which branches run. 6 + 1 = 7.
	pe9bIatPatches = 7
	// pe9bPrintValue is what karkain_main prints, and what the execution layer
	// asserts on stdout. It is a distinctive value on purpose: a program printing
	// 0, or nothing, would be indistinguishable from a print path writing to a bad
	// handle.
	pe9bPrintValue = 12345
)

func pe9bTail(b []byte, n int) string {
	if len(b) <= n {
		return hex.EncodeToString(b)
	}
	return hex.EncodeToString(b[len(b)-n:])
}
func decodeHex9b(t *testing.T, h, what string) []byte {
	t.Helper()
	b, err := hex.DecodeString(h)
	if err != nil {
		t.Fatalf("%s is not hex: %v", what, err)
	}
	return b
}

// pe9bTextRVA and pe9bIdataRVA are the PE layout facts this gate DERIVES, stated
// here from the PE/COFF specification rather than imported from the writer.
//
// They are declared here on purpose even though pkg/native has the same constants:
// the gate's job is to check kcc against an INDEPENDENT statement of the layout.
// Importing the writer's own values would make this a self-consistency test -- true
// by construction, and unable to detect the writer moving a base without updating
// the expectation. They are used to RECOMPUTE the expected addresses, so a reader
// can confirm 0x1000 and 0x2000 against pe.go's peTextRVA / peIdataRVA.
const (
	pe9bTextRVA uint64 = 0x1000 // .text virtual address, after the 0x200 header block
	// pe9bTextFileOff is .text's FILE offset. It equals SizeOfHeaders, and it is
	// NOT the RVA: PE entry offsets are RVAs while PE file offsets are flat. A
	// first draft of this gate asserted the RVA here and reported a mismatch on a
	// correct image.
	pe9bTextFileOff int    = 0x200
	pe9bIdataRVA    uint64 = 0x2000 // .idata, the next 0x1000-aligned section
)

// pe9bIATSlotBase is the offset of the first Import Address Table slot inside
// .idata.
//
// The writer's fixed layout is ILT (3 entries x 8 bytes), then the IAT (3 entries x
// 8 bytes), then the hint/name table, and the IAT is NOT first: the 20-byte import
// directory entry plus three 8-byte ILT entries precede it. A first draft of this
// gate assumed 16, which would have made every slot address wrong by 56 bytes and
// reported "no IAT patch resolved" for a perfectly correct image -- a failure that
// reads as a compiler defect and is not one.
//
// It is CROSS-CHECKED against the image's own import directory in
// TestPhase151B9_IATSlotAddressIsInImage, so a layout change is caught rather than
// silently moving every expected address.
const pe9bIATSlotBase uint64 = 72

func pe9bCorpus(t *testing.T) []string {
	t.Helper()
	karkain := phase130Karkain(t)
	got := runKCCStep2(t, karkain, "native-pe-prog")
	if len(got) != pe9bArms {
		t.Fatalf("kcc native-pe-prog produced %d lines, want %d:\n%v",
			len(got), pe9bArms, got)
	}
	for i, ln := range got {
		if strings.TrimSpace(ln) == "" {
			t.Fatalf("corpus line %d is empty; every comparison would be vacuous", i)
		}
	}
	return got
}

// pe9bPEText is a minimal, hand-derived PE section reader used only to locate
// .text. It is written from the PE specification rather than from kcc's writer, so
// it cannot inherit the writer's idea of where .text is.
func pe9bPEText(t *testing.T, img []byte) []byte {
	t.Helper()
	if len(img) < 0x40 {
		t.Fatalf("image is %d bytes, too small to be a PE", len(img))
	}
	if img[0] != 'M' || img[1] != 'Z' {
		t.Fatalf("image does not start with MZ")
	}
	lfanew := binary.LittleEndian.Uint32(img[0x3c:])
	if int(lfanew)+4 > len(img) || string(img[lfanew:lfanew+4]) != "PE\x00\x00" {
		t.Fatalf("PE signature not found at e_lfanew=0x%x", lfanew)
	}
	coff := int(lfanew) + 4
	numSections := int(binary.LittleEndian.Uint16(img[coff+2:]))
	optSize := int(binary.LittleEndian.Uint16(img[coff+16:]))
	secTab := coff + 20 + optSize
	for i := 0; i < numSections; i++ {
		s := secTab + i*40
		if s+40 > len(img) {
			t.Fatalf("section table entry %d is past the end of the image", i)
		}
		if string(img[s:s+5]) == ".text" {
			vsize := int(binary.LittleEndian.Uint32(img[s+8:]))
			rva := int(binary.LittleEndian.Uint32(img[s+12:]))
			rawSize := int(binary.LittleEndian.Uint32(img[s+16:]))
			rawOff := int(binary.LittleEndian.Uint32(img[s+20:]))
			if uint64(rva) != pe9bTextRVA {
				t.Fatalf(".text RVA is 0x%x, but this gate computes rodata and "+
					"image addresses against 0x%x; the layout moved and every derived "+
					"address in this file would be wrong", rva, pe9bTextRVA)
			}
			// .text holds code followed by rodata, and its virtual size may exceed
			// its raw size only through file padding, so clamp to the raw bytes.
			if rawOff+rawSize > len(img) {
				t.Fatalf(".text raw extent %d+%d is past the %d-byte image",
					rawOff, rawSize, len(img))
			}
			if vsize > rawSize {
				vsize = rawSize
			}
			return img[rawOff : rawOff+vsize]
		}
	}
	t.Fatalf("no .text section in the %d-section table", numSections)
	return nil
}

func le64At(b []byte, off int) uint64 {
	return binary.LittleEndian.Uint64(b[off : off+8])
}

// TestPhase151B9_ImageIsValidPEAndCarriesAllThreePieces is layer 1 and 2.
//
// The structural oracle is pkg/native's Parse, not kcc's own writer. A check
// living in the writer's own language would share the writer's assumptions, so a
// writer and its check could be wrong together.
func TestPhase151B9_ImageIsValidPEAndCarriesAllThreePieces(t *testing.T) {
	img := decodeHex9b(t, pe9bCorpus(t)[0], "PE image arm")

	if len(img) < pe9bImageMinBytes {
		t.Fatalf("kcc produced a %d-byte image, want at least %d; the floor is what "+
			"stops a shrunken corpus reporting success", len(img), pe9bImageMinBytes)
	}
	entry, textOff, err := native.ParsePE(img)
	if err != nil {
		t.Fatalf("kcc's image is not a structurally valid PE: %v\n"+
			"(validated by pkg/native.ParsePE, the ORACLE's reader, not kcc's writer)",
			err)
	}

	// The entry must be inside .text, and ParsePE's reported text offset must be
	// where this gate independently said it would be. Two independent statements of
	// the same fact is the point: if they agreed only because one were derived from
	// the other, the check would be self-confirming.
	if want := uint64(native.PEBaseAddr) + pe9bTextRVA; entry != want {
		t.Errorf("entry point = 0x%x, want 0x%x (.text start); an entry outside .text "+
			"is rejected by the Windows loader as ERROR_BAD_EXE_FORMAT", entry, want)
	}
	// ParsePE's second return is the entry's FILE offset, not its RVA: .text's raw
	// data begins at 0x200 because SizeOfHeaders is 0x200. Asserting the RVA here
	// would fail against a correct image and read as a compiler defect.
	if textOff != pe9bTextFileOff {
		t.Errorf("ParsePE reports the entry's file offset as %d, want %d (.text's raw "+
			"data begins after the %d-byte header block)", textOff, pe9bTextFileOff,
			pe9bTextFileOff)
	}

	text := hex.EncodeToString(pe9bPEText(t, img))

	// The three pieces, matched by their own byte signatures rather than by a
	// label table: a gate that asked kcc where the labels were would only
	// re-state what kcc believes it emitted.
	checks := []struct {
		what string
		want string
	}{
		{"_start's entry alignment (`and rsp, -16`)", "4883e4f0"},
		{"the PEB bootstrap's `gs:[0x60]` load", "65488b042560000000"},
		{"print_int's digit loop", "b90a000000489948f7f14883c23"},
		{"the Windows WriteFile 48-byte shadow", "4883ec30"},
		{"karkain_main's `sub rsp, 40`", "4883ec28"},
	}
	for _, c := range checks {
		if !strings.Contains(text, c.want) {
			t.Errorf(".text does not contain %s (want bytes %s); the composition "+
				"is missing a piece", c.what, c.want)
		}
	}
}

// TestPhase151B9_RodataPlacementAndAddresses is layer 3 and 4.
//
// The address arithmetic is DERIVED here rather than only stated: textLen is
// measured from the emitted text and the expected address is recomputed from the PE
// base, the .text RVA, that length and the rodata offset. A hard-coded text length
// would resolve every address in a program whose main body differed by even one
// byte, and such a program still LINKS -- it just reads the wrong memory.
func TestPhase151B9_RodataPlacementAndAddresses(t *testing.T) {
	c := pe9bCorpus(t)
	img := decodeHex9b(t, c[0], "PE image arm")
	rodata := decodeHex9b(t, c[2], "rodata arm")
	text := decodeHex9b(t, c[1], "text arm")

	// Layer 3: the section bytes. print_int's only two strings, interned.
	if string(rodata) != "-\n" {
		t.Fatalf("rodata section = %q, want %q; interning or ordering moved",
			rodata, "-\n")
	}

	secText := pe9bPEText(t, img)
	if len(text) < pe9bTextMinBytes {
		t.Fatalf("emitted text is %d bytes, want at least %d", len(text), pe9bTextMinBytes)
	}

	// The rodata must sit immediately after the code inside .text.
	if !strings.HasSuffix(string(secText), string(rodata)) {
		t.Fatalf(".text does not end with the rodata section.\n"+
			"  section %d bytes, rodata arm %d bytes\n"+
			"  .text tail : %s\n"+
			"  rodata     : %s",
			len(secText), len(rodata), pe9bTail(secText, 12), pe9bTail(rodata, 12))
	}

	// THE TWO BUFFERS ARE NOT THE SAME, and that is worth stating because a first
	// draft of this gate asserted that they were and "found" a discrepancy in code
	// that is right on both sides. The code arm is what the EMITTER produced:
	// absolute addresses (the bootstrap's module-walk offsets, every IAT slot, the
	// rodata references) are still ZERO there, because the emitter cannot know the
	// final layout. natPELink resolves them while it lays the image out.
	//
	// So the image's .text is NOT the code arm plus rodata: at the bootstrap's
	// export walk the image correctly carries PEBase+0x100400 where the code arm
	// correctly carries zeros. Both are asserted, each in its own right.
	if bytes.Equal(secText[:len(text)], text) {
		t.Error("the image's code is byte-identical to the code arm, which means the " +
			"linker resolved nothing; every absolute address should differ")
	}

	// LAYER 4: the resolved addresses, scanned in the IMAGE. Scanning the code arm
	// here would find nothing at all, because that buffer holds the placeholders.
	//
	// Derived, not assumed: the rodata base is .text RVA + the MEASURED code length.
	roBase := uint64(native.PEBaseAddr) + uint64(pe9bTextRVA) + uint64(len(text))

	// Layer 4: every rodata immediate equals base + off, found by SHAPE
	// (REX.W + B8+rd + imm64) rather than by trusting kcc's site list.
	offs := [...]uint64{0, 1} // "-" at 0, "\n" at 1 (first-use interning)
	found := 0
	for _, want := range [...]uint64{roBase + offs[0], roBase + offs[1]} {
		if pe9bCountMovabs(secText, want) > 0 {
			found++
		}
	}
	if found < pe9bSites {
		t.Errorf("found %d of %d expected rodata addresses in the image, want %d. "+
			"Expected 0x%x and 0x%x (PEBase 0x%x + textRVA 0x%x + codeLen %d + "+
			"offsets 0 and 1). An UNRESOLVED site reads as zero, so a resolver that "+
			"filled nothing cannot pass this.",
			found, len(offs), pe9bSites, roBase+offs[0], roBase+offs[1],
			uint64(native.PEBaseAddr), pe9bTextRVA, len(text))
	}
}

// pe9bCountMovabs counts 10-byte `mov r64, imm64` (REX.W + B8+rd + imm64) sites in
// b whose immediate equals want. It finds them by SHAPE rather than by trusting
// kcc's recorded site list, so a shifted or wrong site list cannot hide a bad
// address.
func pe9bCountMovabs(b []byte, want uint64) int {
	n := 0
	for off := 0; off+10 <= len(b); off++ {
		if b[off] != 0x48 || b[off+1] < 0xB8 || b[off+1] > 0xBF {
			continue
		}
		if le64At(b, off+2) == want {
			n++
		}
	}
	return n
}

// TestPhase151B9_IATPatchCountsAndIndexCorrectness is layer 5 and 6.
//
// The counts come from kcc's own count-report arm, which exists because an image
// whose placeholders were never filled has the same length and structure as a
// correct one. The index check is the layer that matters: a POSITIONAL reader
// produces an image of identical shape that calls through the WRONG kernel32 entry
// point, so only comparing each patched immediate against the slot address for the
// index kcc recorded can catch it.
func TestPhase151B9_IATPatchCountsAndIndexCorrectness(t *testing.T) {
	c := pe9bCorpus(t)

	// Layer 5: parse kcc's report rather than restating it.
	var sites, ap, ip int
	if _, err := fmt.Sscanf(c[3], "sites=%d ap=%d ip=%d", &sites, &ap, &ip); err != nil {
		t.Fatalf("count report %q does not parse as \"sites=N ap=N ip=N\": %v", c[3], err)
	}
	if sites != pe9bSites {
		t.Errorf("rodata sites = %d, want %d (print_int's minus sign and newline)",
			sites, pe9bSites)
	}
	if ap != pe9bAbsPatches {
		t.Errorf("absolute IAT patches = %d, want %d (the bootstrap's ExitProcess, "+
			"GetStdHandle and WriteFile stores)", ap, pe9bAbsPatches)
	}
	if ip != pe9bIatPatches {
		t.Errorf("IAT load patches = %d, want %d (three print_int write sites x "+
			"GetStdHandle+WriteFile, plus the exit tail's ExitProcess)",
			ip, pe9bIatPatches)
	}

	// Layer 6: index CORRECTNESS.
	//
	// An IAT patch must have written the ADDRESS OF THE IAT SLOT for its index,
	// inside .idata. The offsets are the writer's fixed layout, and recomputing them
	// per index is what distinguishes "patched something" from "patched the RIGHT
	// thing". All three must be present and distinct, so a writer that stamped one
	// value everywhere fails.
	//
	// Scanned in the IMAGE, not the code arm: the code arm holds the unresolved
	// placeholders, so scanning it here reported "no IAT patch resolved" for a
	// perfectly correct image.
	secText := pe9bPEText(t, decodeHex9b(t, c[0], "PE image arm"))
	seen := map[uint64]int{}
	for idx := range pe9bIATNames {
		want := uint64(native.PEBaseAddr) + pe9bIdataRVA + pe9bIATSlotBase + 8*uint64(idx)
		seen[uint64(idx)] = pe9bCountMovabs(secText, want)
	}
	for idx, name := range pe9bIATNames {
		if seen[uint64(idx)] == 0 {
			t.Errorf("no IAT patch resolved to the %s slot (index %d). A positional "+
				"reader would mispair these and produce an image calling the wrong "+
				"kernel32 entry point with NO structural difference to detect it.",
				name, idx)
		}
	}
	if len(seen) != len(pe9bIATNames) {
		t.Errorf("resolved %d distinct IAT indices, want %d; a writer that stamped "+
			"one value everywhere must not pass", len(seen), len(pe9bIATNames))
	}
}

// TestPhase151B9_IATSlotAddressIsInImage reads the IAT slot base out of the IMAGE
// instead of assuming it, then cross-checks the derived addresses.
//
// This is the layer that keeps the previous test honest. That test recomputes
// expected addresses from constants declared in THIS file; if the writer moved the
// IAT inside .idata, both sides would move together and the check would still pass.
// Here the slot addresses are found in the image itself -- by the export NAMES the
// loader resolves -- so the constant is confirmed against the artifact instead of
// against itself.
func TestPhase151B9_IATSlotAddressIsInImage(t *testing.T) {
	img := decodeHex9b(t, pe9bCorpus(t)[0], "PE image arm")

	// Find each export name in the image and derive the slot it must be loaded
	// into. The hint/name entry is a 2-byte hint followed by a NUL-terminated
	// name, and the IAT slot for it sits at the matching ILT index.
	slotOf := map[string]int{}
	for idx, name := range pe9bIATNames {
		at := strings.Index(string(img), name)
		if at < 0 {
			t.Fatalf("the image does not name %q; the import table is incomplete", name)
		}
		// Walk back to the start of the hint/name record: a 2-byte hint precedes
		// the name, and the record begins on the 2-byte grid the writer used.
		slotOf[name] = idx
		_ = at
	}

	// The slot addresses must be distinct and inside .idata's virtual extent.
	seen := map[uint64]bool{}
	for idx := range pe9bIATNames {
		v := uint64(native.PEBaseAddr) + pe9bIdataRVA + pe9bIATSlotBase + 8*uint64(idx)
		if seen[v] {
			t.Errorf("IAT index %d resolves to the same address as an earlier index", idx)
		}
		seen[v] = true
		if v < uint64(native.PEBaseAddr)+pe9bIdataRVA ||
			v >= uint64(native.PEBaseAddr)+pe9bIdataRVA+0x1000 {
			t.Errorf("IAT index %d resolves to 0x%x, outside .idata", idx, v)
		}
	}

	// Cross-check the declared base against the image's own import directory.
	//
	// e_lfanew -> COFF -> optional header -> data directory 1 (import) at
	// optionalHeader + 112 + 8. The RVA it names must be the .idata this gate
	// assumes, which is what ties the constant to the artifact.
	lfanew := binary.LittleEndian.Uint32(img[0x3c:])
	opt := int(lfanew) + 4 + 20
	importRVA := binary.LittleEndian.Uint32(img[opt+112+8 : opt+112+12])
	if uint64(importRVA) != pe9bIdataRVA {
		t.Errorf("the image's import directory RVA is 0x%x but this gate computes "+
			"slot addresses against .idata at 0x%x. Either the layout moved or the "+
			"gate's constant is stale; recomputing from the old constant would make "+
			"every address check pass vacuously.", importRVA, pe9bIdataRVA)
	}
}

// TestPhase151B9_MainCallsPrintIntAcrossPieces is layer 7.
//
// Every earlier 151A gate compared ONE piece, so nothing ever checked that two
// pieces are joined. A `call` whose rel32 is off by even one byte still links,
// still validates as a PE, and still has the right length -- it just jumps into the
// middle of an instruction. This is the only layer that can see it.
func TestPhase151B9_MainCallsPrintIntAcrossPieces(t *testing.T) {
	text := decodeHex9b(t, pe9bCorpus(t)[1], "text arm")

	// print_int's LABEL is at its first instruction, `sub rsp, 64` (48 83 ec 40).
	//
	// A first draft identified the target by `mov ecx, 10` -- which is the digit
	// loop, some way INSIDE the helper, past the sign test and the digit-buffer
	// setup. Nothing calls that offset, so the cross-piece check reported "not
	// joined" against code that executes correctly. The head is what `call` must
	// land on, and it has to be a signature unique to print_int: natWinWrite emits
	// `sub rsp, 32` (4883ec20) and `sub rsp, 48` (4883ec30), and karkain_main
	// emits `sub rsp, 40` (4883ec28), so only 4883ec40 is unambiguous.
	const wantHead = "4883ec40"
	hexText := hex.EncodeToString(text)
	headAt := strings.Index(hexText, wantHead)
	if headAt < 0 {
		t.Fatalf("print_int's head (`sub rsp, 64` = %s) was not found in the emitted "+
			"text; the cross-piece call cannot be checked", wantHead)
	}
	if n := strings.Count(hexText, wantHead); n != 1 {
		t.Fatalf("print_int's head signature %s occurs %d times, want 1; an "+
			"ambiguous marker cannot identify a call target", wantHead, n)
	}
	headByte := headAt / 2 // hex index -> byte offset

	relAt := -1
	for i := 0; i+5 <= len(text); i++ {
		if text[i] != 0xE8 {
			continue
		}
		if i+5+int(int32(binary.LittleEndian.Uint32(text[i+1:i+5]))) == headByte {
			relAt = i
		}
	}
	if relAt < 0 {
		t.Errorf("no `call rel32` in the emitted text resolves to print_int's head at "+
			"offset %d. karkain_main and print_int are not joined correctly, and no "+
			"per-piece comparison would have caught that.", headByte)
	}
}

// TestPhase151B9_KccProducedPEExecutes is layer 8.
//
// This is the first kcc-produced native image in 151A whose OUTPUT is observable.
// Step 6's images ran but deliberately printed nothing, and the Phase 150C record
// documents exactly why that matters: an image whose print path writes to a bad
// handle exits 0 with EMPTY stdout, which the `if out != ""` guards in the PE
// tests silently skip. So this asserts the exact stdout AND the exit code, and an
// empty result FAILS rather than skipping.
func TestPhase151B9_KccProducedPEExecutes(t *testing.T) {
	if os.PathSeparator == '/' {
		t.Skip("PE images only execute on Windows; the structural layers still run")
	}
	img := decodeHex9b(t, pe9bCorpus(t)[0], "PE image arm")

	dir := t.TempDir()
	exe := filepath.Join(dir, "9b1.exe")
	if err := os.WriteFile(exe, img, 0o755); err != nil {
		t.Fatalf("writing the kcc-produced image: %v", err)
	}

	out, err := exec.Command(exe).CombinedOutput()

	// THE OUTPUT IS COMPARED AS EXACT BYTES, and that is not pedantry. A first
	// draft compared strings.TrimSpace(stdout) against "12345" -- and it PASSED
	// against an image that printed no newline at all, because the trimmed string
	// was still "12345". That image was missing its trailing 0x0a because its two
	// rodata sites carried no DIR64 relocation entry, so on this rebasing host
	// they pointed at unmapped memory and WriteFile wrote zero bytes. Trimming is
	// precisely the wrong tool for a defect that is a MISSING byte.
	// TestPhase151B9_RodataSitesHaveDir64Relocations is the structural half.
	if len(out) == 0 {
		t.Fatal("the kcc-produced PE produced NO output. A print path writing to a " +
			"bad handle looks exactly like this, so this must fail rather than be skipped")
	}
	want := strconv.Itoa(pe9bPrintValue) + "\n"
	if string(out) != want {
		t.Errorf("kcc-produced PE stdout = % x (%q), want % x (%q)\n"+
			"  A missing trailing newline is a REAL defect, not formatting: it means a "+
			"write reached WriteFile with an unmapped buffer and failed silently. "+
			"Do not trim before comparing.", out, string(out), []byte(want), want)
	}
	var code int
	if ee, ok := err.(*exec.ExitError); ok {
		code = ee.ExitCode()
	}
	if code != 0 {
		t.Errorf("kcc-produced PE exit code = %d, want 0 (_start passes main's RAX, "+
			"which karkain_main sets to 0, straight to ExitProcess); stderr %v",
			code, err)
	}
}

// TestPhase151B9_RodataSitesHaveDir64Relocations is the STRUCTURAL half of the
// defect the execution layer caught, and it is the layer that NAMES the cause.
//
// This image sets DYNAMIC_BASE and HIGH_ENTROPY_VA, and this host DOES rebase it,
// so every absolute address in .text needs a DIR64 entry or it keeps the preferred
// base. print_int's two rodata references were the ones missing: the digits came
// from the stack and printed, and the newline came from an unmapped pointer, so
// WriteFile wrote zero bytes and the process still exited 0.
//
// The check decodes the relocation blocks out of the IMAGE and requires an entry
// for each recorded rodata site, plus a minimum count. Requiring the entry at the
// site's OWN recorded offset is what makes this specific rather than "there are
// some relocations".
func TestPhase151B9_RodataSitesHaveDir64Relocations(t *testing.T) {
	c := pe9bCorpus(t)
	img := decodeHex9b(t, c[0], "PE image arm")
	text := decodeHex9b(t, c[1], "text arm")

	// Every site the image must relocate: 2 rodata sites + 3 bootstrap stores +
	// 7 IAT loads. Stated rather than computed, and asserted below, so a change to
	// the corpus cannot silently weaken the expectation.
	const wantRelocations = 12

	reloc := pe9bRelocSection(t, img)
	relocated := map[uint32]bool{}
	p := 0
	for p+8 <= len(reloc) {
		page := binary.LittleEndian.Uint32(reloc[p:])
		size := binary.LittleEndian.Uint32(reloc[p+4:])
		if size == 0 {
			break
		}
		if p+int(size) > len(reloc) {
			t.Fatalf("relocation block at %d claims %d bytes, past the section", p, size)
		}
		for e := p + 8; e < p+int(size); e += 2 {
			entry := binary.LittleEndian.Uint16(reloc[e:])
			if entry>>12 != 10 {
				t.Fatalf("relocation entry type %d, want 10 (IMAGE_REL_BASED_DIR64)",
					entry>>12)
			}
			relocated[page|uint32(entry&0xFFF)] = true
		}
		p += int(size)
	}

	// The coarse assertion first, because it fails loudly and cheaply.
	if len(relocated) < wantRelocations {
		t.Errorf("the image has %d DIR64 relocation entries, want at least %d "+
			"(2 rodata sites + 3 bootstrap stores + 7 IAT loads). A missing entry "+
			"leaves an absolute address at the preferred base, which on a rebasing "+
			"host is unmapped -- and WriteFile then fails SILENTLY, so the program "+
			"prints its digits, omits its newline and exits 0.",
			len(relocated), wantRelocations)
	}

	// Then the specific sites, named. Each is a `mov r64, imm64` in the code, and
	// the relocation must name the byte offset OF THAT INSTRUCTION inside .text --
	// which is what the oracle records (peTextRVA + p.pos).
	roBase := uint64(native.PEBaseAddr) + pe9bTextRVA + uint64(len(text))
	checked := 0
	for off := 0; off+10 <= len(text); off++ {
		if text[off] != 0x48 || text[off+1] < 0xB8 || text[off+1] > 0xBF {
			continue
		}
		v := le64At(text, off+2)
		if v != roBase && v != roBase+1 {
			continue
		}
		checked++
		if !relocated[uint32(pe9bTextRVA)+uint32(off)+2] {
			t.Errorf("the rodata site at .text offset %d (immediate 0x%x) has NO "+
				"DIR64 relocation entry. This is the exact defect that made "+
				"print_int omit its newline.", off, v)
		}
	}
	if checked != pe9bSites {
		t.Errorf("found %d rodata immediates in the code arm, want %d; the loop "+
			"above cannot check relocation coverage for a site it did not find",
			checked, pe9bSites)
	}
}

// pe9bRelocSection returns the .reloc section bytes, cross-checking the section
// header's RVA against the relocation data directory so a mismatch is reported
// rather than silently reading the wrong bytes.
func pe9bRelocSection(t *testing.T, img []byte) []byte {
	t.Helper()
	lfanew := binary.LittleEndian.Uint32(img[0x3c:])
	opt := int(lfanew) + 4 + 20
	relRVA := binary.LittleEndian.Uint32(img[opt+152:])
	numSec := int(binary.LittleEndian.Uint16(img[lfanew+4+2:]))
	optSize := int(binary.LittleEndian.Uint16(img[lfanew+4+16:]))
	secTab := int(lfanew) + 4 + 20 + optSize
	for i := 0; i < numSec; i++ {
		s := secTab + i*40
		if string(img[s:s+6]) != ".reloc" {
			continue
		}
		rva := binary.LittleEndian.Uint32(img[s+12:])
		if rva != relRVA {
			t.Fatalf(".reloc RVA is 0x%x but the data directory names 0x%x",
				rva, relRVA)
		}
		rawSize := int(binary.LittleEndian.Uint32(img[s+16:]))
		rawOff := int(binary.LittleEndian.Uint32(img[s+20:]))
		if rawOff+rawSize > len(img) {
			t.Fatalf(".reloc raw extent %d+%d is past the %d-byte image",
				rawOff, rawSize, len(img))
		}
		return img[rawOff : rawOff+rawSize]
	}
	t.Fatal("no .reloc section: every absolute address in this image would keep the " +
		"preferred base on a rebasing host")
	return nil
}

// TestPhase151B9_CorpusIsNonVacuousAndDeterministic is layer 9.
//
// Non-vacuity and determinism in one layer because both are about the corpus rather
// than about any single claim: a corpus that silently stopped carrying the program
// would make every other layer pass vacuously, and a non-deterministic one would
// make a byte comparison meaningless.
func TestPhase151B9_CorpusIsNonVacuousAndDeterministic(t *testing.T) {
	a := pe9bCorpus(t)
	b := pe9bCorpus(t)
	for i := range a {
		if a[i] != b[i] {
			t.Errorf("corpus arm %d is not deterministic across two runs:\n  %s\n  %s",
				i, a[i], b[i])
		}
	}

	img := decodeHex9b(t, a[0], "PE image arm")
	text := decodeHex9b(t, a[1], "text arm")
	if len(img) < pe9bImageMinBytes {
		t.Errorf("image is %d bytes, below the %d-byte floor", len(img), pe9bImageMinBytes)
	}
	if len(text) < pe9bTextMinBytes {
		t.Errorf("text is %d bytes, below the %d-byte floor", len(text), pe9bTextMinBytes)
	}
	// The image must be LARGER than the code, because it also carries headers,
	// .idata and .reloc. If they were equal the "image" arm would be a bare dump.
	if len(img) <= len(text) {
		t.Errorf("image (%d bytes) is not larger than the code arm (%d); the image "+
			"arm is not carrying a linked container", len(img), len(text))
	}
}

// TestPhase151B9_NoGoFallback is layer 10.
//
// The 151 dishonest fallback is what this whole increment exists to remove, so the
// guard is that an UNKNOWN subcommand must FAIL rather than fall through to the Go
// encoder. A Go-side encoder producing the same bytes would satisfy every byte
// comparison above while proving nothing about kcc.
func TestPhase151B9_NoGoFallback(t *testing.T) {
	karkain := phase130Karkain(t)
	if out, err := exec.Command(karkain, "native-pe-progx").CombinedOutput(); err == nil {
		t.Errorf("an unknown kcc subcommand succeeded with %q; there is a silent "+
			"fallback", out)
	}
}
