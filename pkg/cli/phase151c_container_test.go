package cli

import (
	"encoding/hex"
	"os/exec"
	"strings"
	"testing"

	"karkain/pkg/native"
)

// Phase 151C - the ELF64 container writer in the self-hosted engine.
// See docs/audit/PHASE-151A-BASELINE.md section 4 (slice 151C).
//
// WHY THE CONTAINER CAN LAND BEFORE THE VALUE MODEL. native.Link takes
// already-encoded bytes -- (text, rodata, data) plus two offsets -- and knows
// nothing about functions, arrays or strings. The container is therefore pure
// serialisation, and it can be proven byte-identical to the Go oracle using
// FIXED INPUT BYTES with no value model anywhere in the picture. That is what
// makes 151C reachable while 151A is still open.
//
// As in 151B, the gate is a DIFFERENTIAL, not a golden: every expected value
// comes from calling the real pkg/native.Link here, so the two writers are
// compared to each other. A golden would pin kcc against a transcription of the
// Go code and pass when both copies are wrong the same way.

// The reference inputs, shared byte for byte with natELFText/natELFRodata/
// natELFData in src/compiler/native_elf.kark.
//
// The .text is the 151B loop sequence, i.e. real machine code whose rel32 was
// itself proven byte-identical to the Go encoder's output. Using a real body
// rather than filler means the container is exercised over bytes that have
// already been through a differential.
const (
	elfTextHex   = "b800000000b90a000000ba010000004801c84829d10f85f4ffffffc3"
	elfRodataHex = "68690a0000000000"
	elfDataHex   = "10000000200000000000000000000000000000000000000000000000000000"
)

// elfEntryInText is where the entry lands WITHIN .text: the 151B loop
// sequence's `loop` label, 15 bytes in.
//
// It is added to the text offset to form the value passed to Link. That is not
// a detail: native.Link takes the entry as a FILE offset, because it writes
// e_entry as BaseAddr+entryOffset and the loader compares that against
// BaseAddr+textOffset. A .text-RELATIVE value therefore lands before .text
// and is rejected as "entry outside image" -- which is exactly how the first
// draft of this gate failed. The writer was correct; the test input was not.
// The failure message names the writer, so the mistake is easy to make and
// worth stating in advance.
const elfEntryInText = 15

func mustHex(t *testing.T, s string) []byte {
	t.Helper()
	b, err := hex.DecodeString(s)
	if err != nil {
		t.Fatalf("bad hex fixture: %v", err)
	}
	return b
}

func kccNativeELF(t *testing.T, karkain string) []string {
	t.Helper()
	cmd := exec.Command(karkain, "native-elf")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("kcc native-elf failed: %v\n%s", err, out)
	}
	var lines []string
	for _, ln := range strings.Split(strings.TrimRight(string(out), "\n"), "\n") {
		ln = strings.TrimRight(ln, "\r")
		if strings.HasPrefix(ln, "[ok]") || strings.HasPrefix(ln, "[kcc]") {
			continue
		}
		if strings.TrimSpace(ln) == "" {
			continue
		}
		lines = append(lines, ln)
	}
	return lines
}

// TestPhase151C_ELFImagesByteIdenticalToOracle is the increment's contract:
// for every reference case, the image kcc's ELF writer produces equals the
// image pkg/native's Link produces.
func TestPhase151C_ELFImagesByteIdenticalToOracle(t *testing.T) {
	karkain := phase130Karkain(t)
	got := kccNativeELF(t, karkain)

	text := mustHex(t, elfTextHex)
	rodata := mustHex(t, elfRodataHex)
	data := mustHex(t, elfDataHex)

	// The Go side of each case, mirroring natELFCorpus exactly. The header
	// count follows from whether the arena is present, so .text starts after
	// one 56-byte program header without an arena and after two with one.
	type ref struct {
		name                   string
		text, rod, dat         []byte
		textOffset, entryOffst int
	}
	refs := []ref{
		{"text", text, nil, nil, 64 + 56, 64 + 56 + elfEntryInText},
		{"arena", text, rodata, data, 64 + 56*2, 64 + 56*2 + elfEntryInText},
		{"rodata", text, rodata, nil, 64 + 56, 64 + 56 + elfEntryInText},
	}

	if len(got) != len(refs)+1 {
		t.Fatalf("kcc native-elf produced %d lines, want %d:\n%v", len(got), len(refs)+1, got)
	}

	byName := map[string]string{}
	for _, ln := range got {
		parts := strings.SplitN(ln, " ", 2)
		if len(parts) != 2 {
			t.Errorf("malformed native-elf line %q", ln)
			continue
		}
		byName[parts[0]] = strings.TrimSpace(parts[1])
	}

	for _, r := range refs {
		img, err := native.Link(r.text, r.rod, r.dat, r.textOffset, r.entryOffst)
		if err != nil {
			t.Fatalf("Go oracle refused case %s: %v", r.name, err)
		}
		want := hex.EncodeToString(img)
		k, ok := byName[r.name]
		if !ok {
			t.Errorf("kcc native-elf did not report case %q", r.name)
			continue
		}
		if k != want {
			t.Errorf("case %s: image differs from the Go oracle (%d bytes vs %d)\n kcc: %s\n  go: %s",
				r.name, len(k)/2, len(want)/2, k, want)
		}
	}
}

// TestPhase151C_ELFRefusalMatchesTheOracle pins the one condition the writer
// can refuse: an unsupported .text offset. It must be REFUSED, not silently
// rounded into a wrong layout -- a wrong header count would make every
// subsequent offset wrong while still producing a structurally plausible image.
func TestPhase151C_ELFRefusalMatchesTheOracle(t *testing.T) {
	karkain := phase130Karkain(t)
	var kccOut string
	for _, ln := range kccNativeELF(t, karkain) {
		if strings.HasPrefix(ln, "refuse ") {
			kccOut = strings.TrimSpace(strings.TrimPrefix(ln, "refuse "))
		}
	}
	if kccOut == "" {
		t.Fatal("kcc native-elf did not report the refusal case")
	}
	if !strings.HasPrefix(kccOut, "error[K117]") {
		t.Errorf("want an error[K117] refusal, got %q", kccOut)
	}
	// The message must name the offset it rejected, so a reader knows which
	// caller computed it wrong.
	if !strings.Contains(kccOut, "999") {
		t.Errorf("refusal does not name the offending offset: %q", kccOut)
	}

	// The Go oracle must refuse the same input, for the same reason.
	if _, err := native.Link(mustHex(t, elfTextHex), nil, nil, 999, 0); err == nil {
		t.Error("Go oracle accepted an unsupported text offset; the two writers would disagree")
	}
}

// TestPhase151C_ELFStructuralParse validates kcc's images with the ORACLE's
// parser, not with kcc's own.
//
// This matters: a structural check written in the same language as the writer
// would share its assumptions. pkg/native.Parse is the Go implementation that
// already validates increments 145-150's images, so a kcc image that parses
// under it is a real, independent confirmation that the container is well
// formed, not merely self-consistent.
func TestPhase151C_ELFStructuralParse(t *testing.T) {
	karkain := phase130Karkain(t)
	for _, ln := range kccNativeELF(t, karkain) {
		parts := strings.SplitN(ln, " ", 2)
		if len(parts) != 2 {
			continue
		}
		name, payload := parts[0], strings.TrimSpace(parts[1])
		if name == "refuse" {
			continue
		}
		img, err := hex.DecodeString(payload)
		if err != nil {
			t.Errorf("case %s: kcc emitted non-hex: %v", name, err)
			continue
		}
		// The header count follows the arena, so .text starts after one
		// program header without one and after two with it.
		wantText := 64 + 56
		if name == "arena" {
			wantText = 64 + 56*2
		}
		entry, textOff, err := native.Parse(img)
		if err != nil {
			t.Errorf("case %s: kcc image failed the oracle's structural parse: %v", name, err)
			continue
		}
		if textOff != wantText {
			t.Errorf("case %s: text offset = %d, want %d", name, textOff, wantText)
		}
		wantEntry := uint64(0x400000 + wantText + elfEntryInText)
		if entry != wantEntry {
			t.Errorf("case %s: entry = %#x, want %#x", name, entry, wantEntry)
		}
	}
}

// TestPhase151C_ELFLayoutFacts pins the two layout decisions the writer makes,
// in the two cases that distinguish them. A differential over the images
// already covers this, but stating the facts is what makes a future change to
// either decision visible in a failure message rather than only as a byte diff.
func TestPhase151C_ELFLayoutFacts(t *testing.T) {
	text := mustHex(t, elfTextHex)

	// No arena: EXACTLY ONE PT_LOAD, and .text immediately after it. This is
	// the pre-150B layout, and preserving it byte for byte is what keeps
	// increment 149's identity pins honest.
	noArena, err := native.Link(text, nil, nil, 64+56, 64+56+elfEntryInText)
	if err != nil {
		t.Fatalf("no-arena link: %v", err)
	}
	if n := countLoadSegments(t, noArena); n != 1 {
		t.Errorf("no-arena image has %d PT_LOAD segments, want 1", n)
	}
	if len(noArena) != 64+56+len(text) {
		t.Errorf("no-arena image is %d bytes, want %d (headers + text, no padding)",
			len(noArena), 64+56+len(text))
	}

	// With an arena: TWO PT_LOADs, and the arena's file OFFSET is page
	// aligned.
	//
	// The assertion is on the OFFSET, not on the total image size. An earlier
	// draft of this test asserted the size was a multiple of 4096, which is
	// simply not what the layout does: the arena is placed at a page-aligned
	// offset and its own length need not be a whole number of pages. The
	// requirement the ELF loader actually imposes is that p_offset and p_vaddr
	// share the same residue modulo the page, so that is what is checked.
	arena := mustHex(t, elfDataHex)
	withArena, err := native.Link(text, mustHex(t, elfRodataHex), arena, 64+56*2, 64+56*2+elfEntryInText)
	if err != nil {
		t.Fatalf("arena link: %v", err)
	}
	if n := countLoadSegments(t, withArena); n != 2 {
		t.Errorf("arena image has %d PT_LOAD segments, want 2", n)
	}
	off := readPh64(t, withArena, 1, 8)    // second PT_LOAD, p_offset
	vaddr := readPh64(t, withArena, 1, 16) // p_vaddr
	if off%4096 != 0 {
		t.Errorf("arena p_offset = %d, want a multiple of the 4096 page", off)
	}
	if (vaddr-0x400000)%4096 != off%4096 {
		t.Errorf("arena p_vaddr residue %d does not match p_offset residue %d; "+
			"the loader requires them equal", (vaddr-0x400000)%4096, off%4096)
	}
	// The R+X segment must stop before the arena so the two mappings cannot
	// overlap.
	if rxFilesz := readPh64(t, withArena, 0, 32); rxFilesz > off {
		t.Errorf("R+X segment runs to %d but the arena starts at %d; they overlap",
			rxFilesz, off)
	}
}

// countLoadSegments reads e_phnum from an ELF64 header and counts the PT_LOAD
// program headers, verifying each one's type field.
func countLoadSegments(t *testing.T, img []byte) int {
	t.Helper()
	if len(img) < 64 {
		t.Fatalf("image too short: %d bytes", len(img))
	}
	phoff := int(uint64(img[32]) | uint64(img[33])<<8 | uint64(img[34])<<16 | uint64(img[35])<<24 |
		uint64(img[36])<<32 | uint64(img[37])<<40 | uint64(img[38])<<48 | uint64(img[39])<<56)
	phentsize := int(uint64(img[54]) | uint64(img[55])<<8)
	phnum := int(uint64(img[56]) | uint64(img[57])<<8)
	n := 0
	for i := 0; i < phnum; i++ {
		off := phoff + i*phentsize
		if off+4 > len(img) {
			t.Fatalf("program header %d runs past the image", i)
		}
		ptype := int(uint64(img[off]) | uint64(img[off+1])<<8 | uint64(img[off+2])<<16 | uint64(img[off+3])<<24)
		if ptype == 1 {
			n++
		}
	}
	return n
}

// TestPhase151C_ELFImagesAreNonEmpty is the vacuity guard. A differential
// over empty or truncated output would pass without comparing anything.
func TestPhase151C_ELFImagesAreNonEmpty(t *testing.T) {
	karkain := phase130Karkain(t)
	for _, ln := range kccNativeELF(t, karkain) {
		parts := strings.SplitN(ln, " ", 2)
		if len(parts) != 2 || parts[0] == "refuse" {
			continue
		}
		if n := len(strings.TrimSpace(parts[1])) / 2; n < 64+56 {
			t.Errorf("case %s: kcc emitted %d bytes, too short to be an ELF image "+
				"(headers alone are %d)", parts[0], n, 64+56)
		}
	}
}

// readPh64 reads a 64-bit little-endian field from the nth program header of an
// ELF64 image. It is written out rather than reusing the oracle's internals so
// the test reads the IMAGE rather than the writer's own variables -- a test
// that consulted the writer for its own expected values would be circular.
func readPh64(t *testing.T, img []byte, n, off int) int {
	t.Helper()
	if len(img) < 64 {
		t.Fatalf("image too short: %d bytes", len(img))
	}
	phoff := 0
	for i := 0; i < 8; i++ {
		phoff |= int(img[32+i]) << (8 * i)
	}
	phentsize := int(img[54]) | int(img[55])<<8
	phnum := int(img[56]) | int(img[57])<<8
	if n >= phnum {
		t.Fatalf("program header %d requested but the image has %d", n, phnum)
	}
	base := phoff + n*phentsize + off
	if base+8 > len(img) {
		t.Fatalf("program header %d field at +%d runs past the image", n, off)
	}
	v := 0
	for i := 0; i < 8; i++ {
		v |= int(img[base+i]) << (8 * i)
	}
	return v
}
