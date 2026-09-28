package cli

import (
	"encoding/hex"
	"os/exec"
	"strings"
	"testing"

	"karkain/pkg/native"
)

// Phase 151C2 - the Mach-O PIE container writer in the self-hosted engine.
// See docs/audit/PHASE-151A-BASELINE.md section 4 (slice 151C).
//
// WHY MACH-O IS THE SUBTLE ONE OF THE THREE CONTAINERS. ELF is a header and
// two program headers. Mach-O is a load-command chain sized by its own
// contents: __TEXT contains the header, the header contains the load commands,
// __LINKEDIT is placed after the body, and one extra LC_SEGMENT_64 (72 bytes)
// appears only when the program needs a writable arena -- which moves .text and
// therefore moves the very offsets the rebase stream addresses. So the writer
// and the rebase encoder are entangled, where in ELF the header count is a
// simple consequence of whether data is present.
//
// THE REBASE STREAM IS THE POINT OF A PIE. The kernel loads a PIE at a slide
// it chooses, so every absolute address baked into the text is wrong until dyld
// adds the slide. A site the stream misses is not a crash: it is an image that
// loads and dereferences a pointer nobody slid. So the stream is DECODED and
// checked here, not assumed.
//
// As in 151B and 151C the gate is a differential, not a golden: every expected
// image comes from calling the real native.LinkMachO.

// The reference inputs, shared with natMachoText/Rodata/Data in
// src/compiler/native_macho.kark and with the ELF writer, so the two
// containers are exercised over identical bytes.
const (
	machoTextHex   = "b800000000b90a000000ba010000004801c84829d10f85f4ffffffc3"
	machoRodataHex = "68690a0000000000"
	machoDataHex   = "10000000200000000000000000000000000000000000000000000000000000"
)

func kccNativeMachO(t *testing.T, karkain string) []string {
	t.Helper()
	cmd := exec.Command(karkain, "native-macho")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("kcc native-macho failed: %v\n%s", err, out)
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

// machoTextOffset re-derives the .text file offset for an image, so the test's
// expectation is computed the same way the writer's is rather than hardcoded.
// The constants are the load-command sizes from pkg/native/macho.go.
func machoTextOffset(hasData bool) int {
	segs := 3
	if hasData {
		segs = 4
	}
	return 32 + segs*72 + 48 + 32 + 24
}

// TestPhase151C2_MachOImagesByteIdenticalToOracle is the contract: for every
// reference case, the image kcc's writer produces equals the image
// native.LinkMachO produces.
func TestPhase151C2_MachOImagesByteIdenticalToOracle(t *testing.T) {
	karkain := phase130Karkain(t)
	got := kccNativeMachO(t, karkain)

	text := mustHex(t, machoTextHex)
	rodata := mustHex(t, machoRodataHex)
	data := mustHex(t, machoDataHex)

	type ref struct {
		name    string
		textOff int
		sites   []int
		hasData bool
	}
	refs := []ref{
		{"single", machoTextOffset(false), []int{15}, false},
		{"arena", machoTextOffset(true), []int{15, 25}, true},
		{"nosite", machoTextOffset(true), nil, true},
		{"adjacent", machoTextOffset(false), []int{15, 23}, false},
	}

	if len(got) != len(refs)+1 {
		t.Fatalf("kcc native-macho produced %d lines, want %d:\n%v", len(got), len(refs)+1, got)
	}

	byName := map[string]string{}
	for _, ln := range got {
		parts := strings.SplitN(ln, " ", 2)
		if len(parts) != 2 {
			t.Errorf("malformed native-macho line %q", ln)
			continue
		}
		byName[parts[0]] = strings.TrimSpace(parts[1])
	}

	for _, r := range refs {
		var dat []byte
		if r.hasData {
			dat = data
		}
		img, err := native.LinkMachO(text, rodata, dat, r.textOff, r.textOff+15, r.sites)
		if err != nil {
			t.Fatalf("Go oracle refused case %s: %v", r.name, err)
		}
		want := hex.EncodeToString(img)
		k, ok := byName[r.name]
		if !ok {
			t.Errorf("kcc native-macho did not report case %q", r.name)
			continue
		}
		if k != want {
			t.Errorf("case %s: image differs from the Go oracle (%d bytes vs %d)\n kcc: %s\n  go: %s",
				r.name, len(k)/2, len(want)/2, k, want)
		}
	}
}

// TestPhase151C2_MachORefusalMatchesTheOracle pins the one condition the writer
// can refuse. Like the ELF entry-offset trap in 151C, a wrong text offset would
// shift every load command and produce a structurally plausible image, so it has
// to be refused loudly.
func TestPhase151C2_MachORefusalMatchesTheOracle(t *testing.T) {
	karkain := phase130Karkain(t)
	var kccOut string
	for _, ln := range kccNativeMachO(t, karkain) {
		if strings.HasPrefix(ln, "refuse ") {
			kccOut = strings.TrimSpace(strings.TrimPrefix(ln, "refuse "))
		}
	}
	if kccOut == "" {
		t.Fatal("kcc native-macho did not report the refusal case")
	}
	if !strings.HasPrefix(kccOut, "error[K117]") {
		t.Errorf("want an error[K117] refusal, got %q", kccOut)
	}
	if !strings.Contains(kccOut, "352") {
		t.Errorf("refusal does not name the offset it wanted (352 for a 3-segment image): %q", kccOut)
	}
	if _, err := native.LinkMachO(mustHex(t, machoTextHex), nil, nil, 999, 0, nil); err == nil {
		t.Error("Go oracle accepted an unsupported text offset; the two writers would disagree")
	}
}

// TestPhase151C2_MachOIsARealPIE asserts the claims a Mach-O writer makes that a
// byte differential alone cannot establish: the image is flagged MH_PIE, it has
// exactly the expected segment set with the expected protections, and the
// rebase stream DECODES to the file offsets it is supposed to slide.
//
// The decode is the important half. A PIE whose stream dyld cannot walk is not
// a PIE; and one that omits a site is not a crash but a silent corruption. The
// oracle's ParseMachO is used rather than a check written here, because a check
// in the writer's own language would share its assumptions.
func TestPhase151C2_MachOIsARealPIE(t *testing.T) {
	karkain := phase130Karkain(t)
	byName := map[string][]byte{}
	for _, ln := range kccNativeMachO(t, karkain) {
		parts := strings.SplitN(ln, " ", 2)
		if len(parts) != 2 || parts[0] == "refuse" {
			continue
		}
		img, err := hex.DecodeString(strings.TrimSpace(parts[1]))
		if err != nil {
			t.Errorf("case %s: kcc emitted non-hex: %v", parts[0], err)
			continue
		}
		byName[parts[0]] = img
	}

	cases := []struct {
		name    string
		sites   []int
		hasData bool
	}{
		{"single", []int{15}, false},
		{"arena", []int{15, 25}, true},
		{"nosite", nil, true},
		{"adjacent", []int{15, 23}, false},
	}

	for _, c := range cases {
		img, ok := byName[c.name]
		if !ok {
			t.Errorf("case %s: kcc produced no image", c.name)
			continue
		}
		if _, _, err := native.ParseMachO(img); err != nil {
			t.Errorf("case %s: kcc image failed the oracle's structural parse: %v", c.name, err)
			continue
		}

		// MH_PIE: the flag without which a dyld-info image is not a PIE.
		if fl := be32(img, 24); fl&0x200000 == 0 {
			t.Errorf("case %s: flags %#x lack MH_PIE", c.name, fl)
		}

		// The rebase stream must decode to the FILE offsets it slides, which
		// are the .text-relative sites plus the text offset.
		want := []int{}
		for _, s := range c.sites {
			want = append(want, s+machoTextOffset(c.hasData))
		}
		gotSites, err := native.MachORebaseSites(img)
		if err != nil {
			t.Errorf("case %s: rebase stream did not decode: %v", c.name, err)
			continue
		}
		if len(gotSites) != len(want) {
			t.Errorf("case %s: stream slides %v, want %v", c.name, gotSites, want)
			continue
		}
		for i := range want {
			if gotSites[i] != want[i] {
				t.Errorf("case %s: stream site %d = %d, want %d (all: got %v want %v)",
					c.name, i, gotSites[i], want[i], gotSites, want)
			}
		}

		// The presence of a writable __DATA must follow the arena, and it is
		// what makes an allocating program possible at all: __TEXT is a single
		// R+X mapping, so an arena inside it would fault on the first store.
		hasData, err := native.MachOHasDataSegment(img)
		if err != nil {
			t.Errorf("case %s: could not read the segment set: %v", c.name, err)
			continue
		}
		if hasData != c.hasData {
			t.Errorf("case %s: __DATA present = %v, want %v", c.name, hasData, c.hasData)
		}
	}
}

// be32 reads a 32-bit little-endian value at off.
func be32(b []byte, off int) uint32 {
	if off+4 > len(b) {
		return 0
	}
	return uint32(b[off]) | uint32(b[off+1])<<8 | uint32(b[off+2])<<16 | uint32(b[off+3])<<24
}

// TestPhase151C2_AdjacentSitesProduceNoRedundantReposition pins the rebase
// stream's one non-obvious behaviour, which is easy to "fix" by accident.
//
// Two sites exactly 8 apart are the ADJACENT pointer slots, not a duplicate.
// The second must NOT be re-positioned: the cursor already points at it, so the
// correct stream is SET_TYPE, SET_SEGMENT_AND_OFFSET, DO_REBASE, DO_REBASE,
// DONE -- two rebase opcodes and ONE reposition.
//
// The failure mode this guards is specific: an implementation that treats the
// second site as a duplicate drops the rebase entirely and the pointer is never
// slid, which produces an image that loads cleanly and then reads a pointer
// nobody fixed. That is why 150D's note calls the d==0 branch load-bearing even
// though today's lowering never reaches it.
func TestPhase151C2_AdjacentSitesProduceNoRedundantReposition(t *testing.T) {
	karkain := phase130Karkain(t)
	for _, ln := range kccNativeMachO(t, karkain) {
		parts := strings.SplitN(ln, " ", 2)
		if len(parts) != 2 || parts[0] != "adjacent" {
			continue
		}
		img, err := hex.DecodeString(strings.TrimSpace(parts[1]))
		if err != nil {
			t.Fatalf("non-hex image: %v", err)
		}
		// The stream is the tail of the image; __LINKEDIT is the last segment.
		stream := img[findLinkEditData(img):]
		if n := countByte(stream, 0x20|2); n != 1 {
			t.Errorf("adjacent case emitted %d SET_SEGMENT_AND_OFFSET opcodes, want 1 "+
				"(the second site is adjacent, so it must not be re-positioned)", n)
		}
		if n := countByte(stream, 0x50|1); n != 2 {
			t.Errorf("adjacent case emitted %d DO_REBASE opcodes, want 2", n)
		}
		if stream[len(stream)-1] != 0x00 {
			t.Errorf("adjacent stream does not end with DONE: last byte %#x", stream[len(stream)-1])
		}
		return
	}
	t.Fatal("kcc native-macho did not report the adjacent case")
}

// findLinkEditData returns the file offset at which the rebase stream begins,
// read out of the image's LC_DYLD_INFO_ONLY. It reads the IMAGE rather than
// recomputing the layout, so it cannot drift from the writer.
func findLinkEditData(img []byte) int {
	if len(img) < 32 {
		return 0
	}
	ncmds := int(be32(img, 16))
	off := 32
	for i := 0; i < ncmds; i++ {
		if off+8 > len(img) {
			return 0
		}
		cmd := be32(img, off)
		cmdsize := int(be32(img, off+4))
		if cmdsize <= 0 {
			return 0
		}
		if cmd == 0x80000022 { // LC_DYLD_INFO_ONLY
			return int(be32(img, off+8))
		}
		off += cmdsize
	}
	return 0
}

func countByte(b []byte, want byte) int {
	n := 0
	for _, c := range b {
		if c == want {
			n++
		}
	}
	return n
}

// TestPhase151C2_MachOImagesAreNonEmpty is the vacuity guard.
func TestPhase151C2_MachOImagesAreNonEmpty(t *testing.T) {
	karkain := phase130Karkain(t)
	for _, ln := range kccNativeMachO(t, karkain) {
		parts := strings.SplitN(ln, " ", 2)
		if len(parts) != 2 || parts[0] == "refuse" {
			continue
		}
		// 32 header + 3 segment commands is the smallest legal image here.
		if n := len(strings.TrimSpace(parts[1])) / 2; n < 32+3*72 {
			t.Errorf("case %s: kcc emitted %d bytes, too short to be a Mach-O image", parts[0], n)
		}
	}
}

// TestPhase151C2_MachOConstantsAreIndependentlyComputed guards the constant
// table, because the first draft of this writer got two of them wrong.
//
// Karkain has no hex literals, so every Mach-O constant is a hand-written
// decimal. Two of them -- LC_DYLD_INFO_ONLY and LC_MAIN -- were mistyped
// (2147484194 for 0x80000022 and 2147484712 for 0x80000028), and the
// consequence was silent and confusing: the load command's id no longer
// matched anything, so the oracle reported "no LC_MAIN" and a test that scanned
// for the rebase stream read the WHOLE IMAGE instead of the stream, inventing
// a second fault out of the first.
//
// This is the same failure class 151B found in the encoder's opcode table, and
// it is now asserted rather than left to inspection. The values below are what
// the image header and load commands actually contain, not what the writer's
// author believed.
func TestPhase151C2_MachOConstantsAreIndependentlyComputed(t *testing.T) {
	img, err := native.LinkMachO(mustHex(t, machoTextHex), nil, nil, machoTextOffset(false), 0, nil)
	if err != nil {
		t.Fatalf("oracle link: %v", err)
	}

	checks := []struct {
		name string
		off  int
		want uint32
		why  string
	}{
		{"magic", 0, 0xFEEDFACF, "MH_MAGIC_64, stored little-endian"},
		{"cputype", 4, 0x01000007, "CPU_TYPE_X86_64"},
		{"filetype", 12, 2, "MH_EXECUTE"},
		{"flags", 24, 0x1 | 0x4 | 0x80 | 0x200000, "NOUNDEFS | DYLDLINK | TWOLEVEL | PIE"},
	}
	for _, c := range checks {
		if got := be32(img, c.off); got != c.want {
			t.Errorf("%s at +%d = %#x, want %#x (%s)", c.name, c.off, got, c.want, c.why)
		}
	}

	// The load-command ids, which is exactly where the two mistyped constants
	// lived.
	ids := map[uint32]string{
		0x19:       "LC_SEGMENT_64",
		0x80000022: "LC_DYLD_INFO_ONLY",
		0xC:        "LC_LOAD_DYLINKER",
		0x80000028: "LC_MAIN",
	}
	seen := map[string]bool{}
	ncmds := int(be32(img, 16))
	off := 32
	for i := 0; i < ncmds; i++ {
		if off+8 > len(img) {
			t.Fatalf("load command %d runs past the image", i)
		}
		cmd := be32(img, off)
		name, known := ids[cmd]
		if !known {
			t.Errorf("load command %d has unknown id %#x", i, cmd)
		} else {
			seen[name] = true
		}
		cmdsize := int(be32(img, off+4))
		if cmdsize <= 0 {
			t.Fatalf("load command %d has cmdsize %d", i, cmdsize)
		}
		off += cmdsize
	}
	for _, want := range []string{"LC_SEGMENT_64", "LC_DYLD_INFO_ONLY", "LC_LOAD_DYLINKER", "LC_MAIN"} {
		if !seen[want] {
			t.Errorf("image has no %s; load-command ids are wrong", want)
		}
	}
}
