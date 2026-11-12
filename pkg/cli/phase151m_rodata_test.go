package cli

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"karkain/pkg/native"
)

// NAT-RODATA -- the reporter's read-only data: what is interned, in what
// order, at what offset.
//
// WHAT THIS SLICE FOUND, and it is not what the record said.
//
// The NAT-FRAME record described a residual of TWO bytes in an indexing
// program's image and attributed it to the ORDER in which the reporter's
// strings are interned: "the oracle emits the reporter helper before the user
// function, so its strings are interned first; the driver appends them to the
// rodata list after the program's own literals".
//
// MEASURED, that reading is WRONG for the program the residual was measured
// on. The rodata layout of that program was BYTE-IDENTICAL between the two
// engines -- "runtime error: " at the same offset, " at " at the same offset,
// "array index out of range" at the same offset -- and the two differing bytes
// were the array-bounds call site's two ABSOLUTE ADDRESSES, which pointed at
// DIFFERENT BYTES:
//
//   the oracle's  ->  "array index out of range" and the file name
//   the driver's  ->  offsets 6 and 7 of the section, which are the middle of
//                     "runtime error: "
//
// The cause was not an ordering difference at all. The driver computed the
// call site's two rodata references as LIST POSITIONS (2 + len(literals) + 4
// and + 5) and used them as RODATA OFFSETS. A position counts ENTRIES and an
// offset counts BYTES, and the two differ as soon as any earlier string is
// longer than one byte -- which every one of them is. The address difference
// (16 and 39 bytes) was the difference between the two wrong numbers and the
// two right ones, and the record read it as an offset difference.
//
// So the out-of-range diagnostic pointed at the WRONG TEXT, and that is a
// correctness defect, not a layout one.
//
// The D5 baseline had already stated the intended design, and the driver
// violated both halves of it (PHASE-D5-BASELINE.md section 8.3):
//
//   "The reporter's strings must be appended in the order the oracle emits
//    them (helpers first, then the call site), and their offsets read back
//    OUT of natRodataSection's offset array."
//
// Both are now true.
//
// WHAT IS STILL DIFFERENT, and is stated rather than hidden: the oracle
// interns the call site's two strings AT THE FIRST INDEX SITE, so a program
// that binds a string literal AFTER an index site interleaves them
// differently. The driver groups them after the literals. That is a LAYOUT
// difference with no correctness consequence -- the diagnostic's text is
// identical in both engines -- and closing it would mean re-deriving the
// interning order from the emitter's walk, which is a different blast radius.

// natRodataFixture is one program and the string data it puts into the
// image's read-only section.
type natRodataFixture struct {
	name string
	src  string
	// reporter says whether the program can raise a diagnostic, and so
	// whether the runtime-error reporter's strings are in the image at all.
	reporter bool
	// literals are the program's own string literals, in source order.
	literals []string
	// identical says whether the driver's image is expected to be BYTE
	// identical to the oracle's. Every fixture except the interleaving one
	// is: the oracle interns the call site's two strings at the first index
	// site, so a program that binds a literal AFTER an index site puts them
	// in a different place.
	identical bool
	// oracleOrder is the interning order the ORACLE produces, when it differs
	// from the driver's. Nil means the two agree.
	oracleOrder []string
}

// natRodataFixtures is the corpus.
//
// The matrix is the one the brief names: a program with no reporter rodata,
// repeated identical reporter strings (two index sites -- the reporter's
// strings are interned ONCE), distinct reporter strings, the same literal
// bound twice, several functions, mixed string/array/integer programs, and
// two different DISCOVERY ORDERS (a literal bound before the index site and
// one bound after it).
var natRodataFixtures = []natRodataFixture{
	{
		name:      "ints_only_no_reporter",
		src:       "func main() {\n    print(42)\n}\n",
		reporter:  false,
		literals:  []string{},
		identical: true,
	},
	{
		name:      "string_no_reporter",
		src:       "func main() {\n    let s = \"hello\"\n    print(s)\n}\n",
		reporter:  false,
		literals:  []string{"hello"},
		identical: true,
	},
	{
		name:      "array_index_no_literals",
		src:       "func main() {\n    let a = [3, 5, 7]\n    print(a[0])\n}\n",
		reporter:  true,
		literals:  []string{},
		identical: true,
	},
	{
		// TWO index sites: the reporter's four strings and the call site's two
		// are each interned ONCE, so the second site references the same
		// bytes as the first.
		name:      "repeated_reporter_strings",
		src:       "func main() {\n    let a = [3, 5, 7]\n    print(a[0])\n    print(a[1])\n}\n",
		reporter:  true,
		literals:  []string{},
		identical: true,
	},
	{
		name:      "array_index_with_literal",
		src:       "func main() {\n    let s = \"hello\"\n    let a = [3, 5, 7]\n    print(s)\n    print(a[0])\n}\n",
		reporter:  true,
		literals:  []string{"hello"},
		identical: true,
	},
	{
		name:      "array_index_two_literals",
		src:       "func main() {\n    let s = \"alpha\"\n    let t = \"beta\"\n    let a = [3, 5, 7]\n    print(s)\n    print(t)\n    print(a[0])\n}\n",
		reporter:  true,
		literals:  []string{"alpha", "beta"},
		identical: true,
	},
	{
		// The SAME literal bound twice: interned once, and both references
		// resolve to the same copy.
		name:      "repeated_program_literal",
		src:       "func main() {\n    let s = \"hello\"\n    let a = [3, 5, 7]\n    print(s)\n    print(s)\n    print(a[0])\n}\n",
		reporter:  true,
		literals:  []string{"hello"},
		identical: true,
	},
	{
		// SEVERAL FUNCTIONS: the helper does no string work at all, and the
		// program still interns the reporter's strings once, for the whole
		// program.
		name:      "several_functions",
		src:       "func helper() int {\n    return 7\n}\nfunc main() {\n    let a = [1]\n    print(a[0])\n    print(helper())\n}\n",
		reporter:  true,
		literals:  []string{},
		identical: true,
	},
	{
		// A DIFFERENT DISCOVERY ORDER: the literal is bound AFTER the index
		// site, so the oracle interns the call site's two strings BEFORE it.
		// This is the one known remaining difference.
		name:      "literal_after_the_index_site",
		src:       "func main() {\n    let a = [3, 5, 7]\n    print(a[0])\n    let s = \"hello\"\n    print(s)\n}\n",
		reporter:  true,
		literals:  []string{"hello"},
		identical: false,
		// The oracle interns the call site's two strings AT THE FIRST INDEX
		// SITE, which is before the literal bound after it.
		oracleOrder: []string{"-", "\n", "runtime error: ", " at ", ":",
			"array index out of range", "probe.kark", "hello"},
	},
}

// natRodataReporterStrings are the runtime-error reporter's own four strings,
// in the order the reporter helper emits them (pkg/native/program.go:3732,
// 3744, 3756, 3785).
var natRodataReporterStrings = []string{"runtime error: ", " at ", ":", "\n"}

// natRodataCallSiteStrings are the two strings the array-bounds call site
// references: the message and the source file's base name.
var natRodataCallSiteStrings = []string{"array index out of range", "probe.kark"}

// natRodataPrintIntStrings are print_int's own two strings, which are the
// first entries of every image's read-only section.
var natRodataPrintIntStrings = []string{"-", "\n"}

// natRodataKnownStrings returns every string a fixture's section can hold, in
// no particular order: print_int's two, the reporter's four and the call
// site's two (when the fixture has a reporter), and the program's literals.
func natRodataKnownStrings(f natRodataFixture) []string {
	out := append([]string{}, natRodataPrintIntStrings...)
	if f.reporter {
		out = append(out, natRodataReporterStrings...)
		out = append(out, natRodataCallSiteStrings...)
	}
	out = append(out, f.literals...)
	return out
}

// natRodataDistinct returns the DISTINCT strings of a fixture's section, in
// the order they are first interned -- which is the order they appear in the
// section.
//
// The dedup is by CONTENT, so a string that appears twice in the program
// occupies ONE entry.
func natRodataDistinct(f natRodataFixture) []string {
	var out []string
	seen := map[string]bool{}
	add := func(s string) {
		if seen[s] {
			return
		}
		seen[s] = true
		out = append(out, s)
	}
	for _, s := range natRodataPrintIntStrings {
		add(s)
	}
	if f.reporter {
		for _, s := range natRodataReporterStrings {
			add(s)
		}
	}
	for _, s := range f.literals {
		add(s)
	}
	if f.reporter {
		for _, s := range natRodataCallSiteStrings {
			add(s)
		}
	}
	return out
}

// natRodataSection returns the read-only section's file range in an ELF
// image.
//
// The ELF carries no section headers (e_shoff is 0), so the range is derived
// from the container's own layout (pkg/native/elf.go:102-108): the section
// follows .text and is the last thing in the file, unless the program has an
// arena -- a second, R+W segment -- in which case the section ends where the
// zero-filled alignment gap begins.
//
// The START is derived from the interned SIZE, which is the sum of the
// distinct entries' lengths, and is then CROSS-CHECKED against the section's
// own bytes by natRodataEntries: a wrong start makes the walk below fail
// loudly rather than silently describe nonsense.
func natRodataSection(t *testing.T, img []byte, f natRodataFixture) (int, int) {
	t.Helper()
	end := len(img)
	// A second program header means an arena, and the arena's file offset is
	// the second header's p_offset (ELF64: e_phoff + 56 + 8).
	if phnum := int(binary.LittleEndian.Uint16(img[56:])); phnum == 2 {
		arenaOff := int(binary.LittleEndian.Uint64(img[64+56+8:]))
		// Walk back over the zero-filled alignment gap. The section's last
		// byte is never zero -- every entry is printable text or a newline.
		end = arenaOff
		for end > 0 && img[end-1] == 0 {
			end--
		}
	}
	size := 0
	for _, s := range natRodataDistinct(f) {
		size += len(s)
	}
	start := end - size
	if start < 0 {
		t.Fatalf("the read-only section does not fit: it would start at %d", start)
	}
	return start, end
}

// natRodataEntry is one interned entry of the read-only section.
type natRodataEntry struct {
	// text is the entry's content.
	text string
	// offset is the entry's offset WITHIN the section.
	offset int
	// fileOffset is the entry's offset in the image.
	fileOffset int
}

// natRodataEntries reads the section's entries, in the order they were
// interned.
//
// The order is the section's byte order, because the offsets are assigned at
// first use and the section is built in that order. Each entry is matched by
// its bytes, and the walk must consume the section EXACTLY -- so a wrong
// section boundary or an unexpected entry fails loudly instead of producing a
// plausible-looking list.
func natRodataEntries(t *testing.T, img []byte, f natRodataFixture) []natRodataEntry {
	t.Helper()
	start, end := natRodataSection(t, img, f)
	known := natRodataKnownStrings(f)
	var out []natRodataEntry
	p := start
	for p < end {
		matched := ""
		// LONGEST match first, so a literal that happens to contain a
		// shorter known entry cannot be mis-read as the shorter one.
		for _, k := range known {
			if len(k) == 0 {
				continue
			}
			if p+len(k) <= end && string(img[p:p+len(k)]) == k {
				if len(k) > len(matched) {
					matched = k
				}
			}
		}
		if matched == "" {
			t.Fatalf("the read-only section at %d..%d holds bytes that are not any known "+
				"entry (% x at %d); the section boundary is wrong or the section holds an "+
				"entry this gate does not know about", start, end, img[p:min(p+16, end)], p)
		}
		out = append(out, natRodataEntry{
			text:       matched,
			offset:     p - start,
			fileOffset: p,
		})
		p = p + len(matched)
	}
	return out
}

// natRodataReferences returns the absolute addresses the image bakes into its
// code, decoded from the `movabs r64, imm64` form (REX.W + B8+rd + imm64).
//
// It is how the image says "this string lives THERE", so it is the direct
// measurement of what the code points at.
func natRodataReferences(t *testing.T, img []byte) []uint64 {
	t.Helper()
	var out []uint64
	for i := 0; i+10 <= len(img); i++ {
		if img[i] != 0x48 {
			continue
		}
		op := img[i+1]
		if op < 0xB8 || op > 0xBF {
			continue
		}
		out = append(out, binary.LittleEndian.Uint64(img[i+2:i+10]))
	}
	return out
}

// natRodataPointsAt reports whether the image references the given address.
func natRodataPointsAt(refs []uint64, addr uint64) bool {
	for _, r := range refs {
		if r == addr {
			return true
		}
	}
	return false
}

// natRodataBuild builds one fixture with both engines for the given target.
func natRodataBuild(t *testing.T, karkain, src, target string) (kccRes, goRes nativeBuildResult) {
	t.Helper()
	kccRes = nativeBuildWithEngine(t, karkain, src, target, "kcc", "rodata-kcc.img")
	if kccRes.ExitCode != 0 || !kccRes.WroteImage {
		t.Fatalf("kcc produced no image (exit %d):\n%s", kccRes.ExitCode, kccRes.Output)
	}
	goRes = nativeBuildWithEngine(t, karkain, src, target, "go", "rodata-go.img")
	if goRes.ExitCode != 0 || !goRes.WroteImage {
		t.Fatalf("the Go oracle produced no image (exit %d):\n%s", goRes.ExitCode, goRes.Output)
	}
	return kccRes, goRes
}

// TestPhaseNATRodata_TheImagesAgree is the differential.
//
// Every fixture is built with BOTH engines for the LINUX target, and the two
// images must be BYTE-IDENTICAL -- except the one fixture whose discovery
// order the oracle interleaves, which is asserted to DIFFER.
//
// The Linux target is primary because it is the layout-sensitive one: the
// Windows target's frame rounding and its RVA mapping hide nothing here but
// the ELF is where the section boundaries are measurable.
func TestPhaseNATRodata_TheImagesAgree(t *testing.T) {
	karkain := phase130Karkain(t)
	for _, f := range natRodataFixtures {
		f := f
		t.Run(f.name, func(t *testing.T) {
			kccRes, goRes := natRodataBuild(t, karkain, f.src, NativeLinuxTarget)
			diff, shorter := natStrStageDiffBytes(kccRes.Image, goRes.Image)
			identical := len(kccRes.Image) == len(goRes.Image) && diff == 0

			if f.identical && !identical {
				t.Errorf("the two engines emitted DIFFERENT images (%d vs %d bytes, %d "+
					"differing bytes of %d).\n  source:\n%s",
					len(kccRes.Image), len(goRes.Image), diff, shorter, f.src)
			}
			if !f.identical && identical {
				t.Errorf("this fixture is expected to differ -- the oracle interns the call "+
					"site's two strings at the first index site -- and the two images are "+
					"now identical, which means the difference closed and the fixture "+
					"belongs in the identical group.\n  source:\n%s", f.src)
			}
			t.Logf("%s: kcc=%d bytes go=%d bytes, %d differing bytes, identical=%v",
				f.name, len(kccRes.Image), len(goRes.Image), diff, identical)
		})
	}
}

// TestPhaseNATRodata_TheInterningOrderIsTheOracles is the ORDER claim.
//
// The section's entries are read out of each engine's image, in the order the
// bytes appear, and the two lists must be equal: the same entries, at the
// same offsets, in the same order.
//
// And the order must be the one the D5 baseline states: print_int's two
// strings, then the reporter HELPER's four (the helper is emitted before the
// user function), then the program's literals, then the call site's two.
func TestPhaseNATRodata_TheInterningOrderIsTheOracles(t *testing.T) {
	karkain := phase130Karkain(t)
	for _, f := range natRodataFixtures {
		f := f
		t.Run(f.name, func(t *testing.T) {
			kccRes, goRes := natRodataBuild(t, karkain, f.src, NativeLinuxTarget)
			kccEntries := natRodataEntries(t, kccRes.Image, f)
			goEntries := natRodataEntries(t, goRes.Image, f)

			if len(kccEntries) != len(goEntries) {
				t.Fatalf("the two sections hold %d and %d entries:\n  native: %v\n  oracle: %v",
					len(kccEntries), len(goEntries), natRodataEntryText(kccEntries), natRodataEntryText(goEntries))
			}

			// The ORACLE's order: the D5 rule's, except for the one fixture
			// whose discovery order interleaves the call site's two strings
			// at the first index site.
			wantOracle := natRodataDistinctBeforeCallSite(f)
			if f.oracleOrder != nil {
				wantOracle = f.oracleOrder
			}
			gotOracle := natRodataEntryNames(goEntries)
			if strings.Join(gotOracle, "|") != strings.Join(wantOracle, "|") {
				t.Errorf("the ORACLE's interning order is %v, want %v", gotOracle, wantOracle)
			}

			// The DRIVER's order: the D5 rule's -- print_int's two, the
			// reporter HELPER's four (the helper is emitted before the user
			// function), the program's literals, and the call site's two.
			want := natRodataDistinctBeforeCallSite(f)
			got := natRodataEntryNames(kccEntries)
			if strings.Join(got, "|") != strings.Join(want, "|") {
				t.Errorf("the driver's interning order is %v, want %v.\n"+
					"  The reporter HELPER's strings come before the program's literals "+
					"(the helper is emitted before the user function), and the call site's "+
					"two come last.",
					got, want)
			}

			// And the two agree, except where the oracle's discovery order
			// interleaves.
			if f.identical {
				for i := range kccEntries {
					if kccEntries[i].text != goEntries[i].text || kccEntries[i].offset != goEntries[i].offset {
						t.Errorf("entry %d differs: native %q at offset %d, oracle %q at offset %d\n"+
							"  native: %v\n  oracle: %v",
							i, kccEntries[i].text, kccEntries[i].offset,
							goEntries[i].text, goEntries[i].offset,
							natRodataEntryText(kccEntries), natRodataEntryText(goEntries))
					}
				}
			} else {
				same := len(kccEntries) == len(goEntries)
				if same {
					for i := range kccEntries {
						if kccEntries[i].text != goEntries[i].text || kccEntries[i].offset != goEntries[i].offset {
							same = false
						}
					}
				}
				if same {
					t.Errorf("this fixture is expected to DIFFER -- the oracle interns the "+
						"call site's two strings at the first index site -- and the two "+
						"orders are identical, which means the difference closed and the "+
						"fixture belongs in the identical group.\n  native: %v\n  oracle: %v",
						natRodataEntryText(kccEntries), natRodataEntryText(goEntries))
				}
			}
		})
	}
}

// natRodataDistinctBeforeCallSite is the driver's order: print_int's two, the
// reporter helper's four, the program's literals, and the call site's two.
//
// The reporter's entries are present only when the program can raise a
// diagnostic, which is the same condition the driver uses to emit the
// reporter at all.
func natRodataDistinctBeforeCallSite(f natRodataFixture) []string {
	var out []string
	seen := map[string]bool{}
	add := func(s string) {
		if seen[s] {
			return
		}
		seen[s] = true
		out = append(out, s)
	}
	for _, s := range natRodataPrintIntStrings {
		add(s)
	}
	if f.reporter {
		for _, s := range natRodataReporterStrings {
			add(s)
		}
	}
	for _, s := range f.literals {
		add(s)
	}
	if f.reporter {
		for _, s := range natRodataCallSiteStrings {
			add(s)
		}
	}
	return out
}

// natRodataEntryText renders a list of entries for a diagnostic.
func natRodataEntryText(entries []natRodataEntry) []string {
	out := make([]string, 0, len(entries))
	for _, e := range entries {
		out = append(out, fmt.Sprintf("%q@%d", e.text, e.offset))
	}
	return out
}

// natRodataEntryNames renders a list of entries' TEXTS, which is the
// interning ORDER without the offsets.
func natRodataEntryNames(entries []natRodataEntry) []string {
	out := make([]string, 0, len(entries))
	for _, e := range entries {
		out = append(out, e.text)
	}
	return out
}

// TestPhaseNATRodata_TheCallSitePointsAtTheRightBytes is the CORRECTNESS
// claim, and it is the one that matters most.
//
// The array-bounds call site bakes two absolute addresses: the message and the
// file name. The driver used to bake the LIST POSITIONS of those two entries
// instead of their interned OFFSETS, so it pointed at the middle of
// "runtime error: " and the diagnostic would have printed "im" and "me error: "
// instead of "array index out of range" and the file name.
//
// The image is asked directly: does it reference the address of the message,
// and the address of the file name?
func TestPhaseNATRodata_TheCallSitePointsAtTheRightBytes(t *testing.T) {
	karkain := phase130Karkain(t)

	// A program that raises a diagnostic and has a string literal, so the
	// section's offsets are not the ones a literal-free program would have.
	src := "func main() {\n    let s = \"hello\"\n    let a = [3, 5, 7]\n    print(s)\n    print(a[0])\n}\n"
	kccRes, _ := natRodataBuild(t, karkain, src, NativeLinuxTarget)

	f := natRodataFixture{reporter: true, literals: []string{"hello"}}
	entries := natRodataEntries(t, kccRes.Image, f)
	start, _ := natRodataSection(t, kccRes.Image, f)
	refs := natRodataReferences(t, kccRes.Image)

	want := map[string]bool{
		"array index out of range": false,
		"probe.kark":               false,
	}
	for _, e := range entries {
		addr := uint64(native.BaseAddr) + uint64(e.fileOffset)
		if _, ok := want[e.text]; ok && natRodataPointsAt(refs, addr) {
			want[e.text] = true
		}
	}
	for text, found := range want {
		if !found {
			t.Errorf("the image does NOT reference the address of %q.\n"+
				"  The array-bounds call site must point at the string itself, not at a "+
				"  LIST POSITION used as an offset.\n"+
				"  the section's entries: %v (starting at file offset %d)",
				text, natRodataEntryText(entries), start)
		}
	}
}

// TestPhaseNATRodata_TheDifferentialRecord records what the brief asks to be
// recorded, for every fixture: the reporter's rodata entries, the
// deduplication count, the insertion order, the assigned offsets, the rodata
// size, the section boundaries, the total image size and the byte-identity
// result.
func TestPhaseNATRodata_TheDifferentialRecord(t *testing.T) {
	karkain := phase130Karkain(t)
	for _, f := range natRodataFixtures {
		f := f
		t.Run(f.name, func(t *testing.T) {
			kccRes, goRes := natRodataBuild(t, karkain, f.src, NativeLinuxTarget)
			kccEntries := natRodataEntries(t, kccRes.Image, f)
			kccStart, kccEnd := natRodataSection(t, kccRes.Image, f)
			goStart, goEnd := natRodataSection(t, goRes.Image, f)
			diff, shorter := natStrStageDiffBytes(kccRes.Image, goRes.Image)
			identical := len(kccRes.Image) == len(goRes.Image) && diff == 0

			distinct := len(natRodataDistinct(f))
			t.Logf("differential %s: reporter=%v entries=%d distinct=%d deduped=%d "+
				"insertionOrder=%v offsets=%v rodataSize=%d/%d section=%d..%d/%d..%d "+
				"imageSize=%d/%d byteIdentity=%v (differing bytes %d of %d)",
				f.name, f.reporter, len(kccEntries), distinct, len(kccEntries)-distinct,
				natRodataEntryText(kccEntries),
				natRodataEntryOffsets(kccEntries),
				kccEnd-kccStart, goEnd-goStart, kccStart, kccEnd, goStart, goEnd,
				len(kccRes.Image), len(goRes.Image), identical, diff, shorter)

			if len(kccEntries) != distinct {
				t.Errorf("the section holds %d entries but %d distinct strings are expected; "+
					"the interning must DEDUPLICATE by content.\n  entries: %v",
					len(kccEntries), distinct, natRodataEntryText(kccEntries))
			}
			if kccEnd-kccStart != goEnd-goStart {
				t.Errorf("the read-only section's size differs: native %d, oracle %d",
					kccEnd-kccStart, goEnd-goStart)
			}
		})
	}
}

// natRodataEntryOffsets renders a list of entries' offsets.
func natRodataEntryOffsets(entries []natRodataEntry) []int {
	out := make([]int, 0, len(entries))
	for _, e := range entries {
		out = append(out, e.offset)
	}
	return out
}

// TestPhaseNATRodata_TheOutputIsDeterministic builds the same program twice
// and requires byte-identical images.
//
// The interning is a first-use walk over a fixed list, so it has to be a
// FUNCTION of the program rather than of the order the functions happen to be
// visited.
func TestPhaseNATRodata_TheOutputIsDeterministic(t *testing.T) {
	karkain := phase130Karkain(t)
	for _, f := range natRodataFixtures {
		f := f
		t.Run(f.name, func(t *testing.T) {
			first := natStrStageBuildRaw(t, karkain, f.src, NativeLinuxTarget, "kcc")
			if !first.WroteImage {
				t.Fatalf("the driver refused the program:\n%s", first.Output)
			}
			second := natStrStageBuildRaw(t, karkain, f.src, NativeLinuxTarget, "kcc")
			if !second.WroteImage {
				t.Fatalf("the driver refused the program on the second build:\n%s", second.Output)
			}
			if first.SHA != second.SHA {
				t.Errorf("two builds of the same program differ: %s and %s", first.SHA, second.SHA)
			}
		})
	}
}

// TestPhaseNATRodata_TheWindowsPERegression is the PE half.
//
// THE PE IS NOT A BYTE-IDENTITY TARGET, and the reason is measured rather than
// assumed. The Windows entry stub and the loader-independent PEB bootstrap are
// SEPARATE IMPLEMENTATIONS in the two engines -- the driver's `natWinStub`
// against the oracle's `emitWinBootstrap` -- so the PE's .text is not the same
// bytes: measured, the driver's .text virtual size is 0xa9c and the oracle's
// 0xa94, eight bytes apart, and about a thousand bytes of the bootstrap
// differ. That is why the byte-identity differential (TestPhaseD2_) compares
// the LINUX ELF, where both engines share one entry stub.
//
// So the PE is checked for the things it CAN show: that it is a valid PE, that
// the program's string data is interned in it, that each string is interned
// ONCE, and that the program still RUNS.
func TestPhaseNATRodata_TheWindowsPERegression(t *testing.T) {
	if os.PathSeparator == '/' {
		t.Skip("PE images only execute on Windows")
	}
	karkain := phase130Karkain(t)
	for _, f := range natRodataFixtures {
		f := f
		t.Run(f.name, func(t *testing.T) {
			res := nativeBuildWithEngine(t, karkain, f.src, NativeWindowsTarget, "kcc", "rodata.exe")
			if res.ExitCode != 0 || !res.WroteImage {
				t.Fatalf("kcc produced no image (exit %d):\n%s", res.ExitCode, res.Output)
			}
			if _, _, err := native.ParsePE(res.Image); err != nil {
				t.Fatalf("the driver's image is not a valid PE: %v", err)
			}
			// Every string the program interns appears EXACTLY ONCE: the
			// interning deduplicates by content.
			//
			// The ONE-BYTE entries are excluded, and not only because a
			// one-byte search cannot tell them apart: ":" also occurs inside
			// "runtime error: ", so a count of the whole image would see two
			// of them and conclude the interning had failed.
			for _, s := range natRodataDistinct(f) {
				if len(s) < 2 {
					continue
				}
				if n := bytes.Count(res.Image, []byte(s)); n != 1 {
					t.Errorf("the string %q appears %d times in the image, want exactly 1 "+
						"(the interning deduplicates by content)", s, n)
				}
			}
			// The program must still run.
			out, code := natIRunImage(t, res.Image, "rodata")
			if code != 0 {
				t.Errorf("exit = %d, want 0 (stdout %s)", code, natFrameHex(out))
			}
			t.Logf("%s: PE %d bytes, the program ran with stdout %s",
				f.name, len(res.Image), natFrameHex(out))
		})
	}
}

// TestPhaseNATRodata_TheRefusalsAreNamed asserts that the one construct the
// driver cannot lower -- a string in a CALLEE -- is refused by name and
// produces no image.
//
// It is the boundary of the "same strings discovered in different functions"
// case: the oracle interns a callee's literals in the same program-wide walk,
// and the driver refuses them, so the question of their ORDER never arises
// there. The refusal is the honest boundary and it is asserted rather than
// assumed.
func TestPhaseNATRodata_TheRefusalsAreNamed(t *testing.T) {
	karkain := phase130Karkain(t)
	src := "func h() int {\n    print(\"hi\")\n    return 1\n}\nfunc main() {\n" +
		"    let a = [1]\n    print(a[0])\n    print(h())\n}\n"
	res := natStrStageBuildRaw(t, karkain, src, NativeLinuxTarget, "kcc")
	if res.WroteImage {
		t.Errorf("the driver produced an image for a program with a string in a callee; "+
			"it refuses that construct.\n  output:\n%s", res.Output)
	}
	if !strings.Contains(res.Output, "error[K145]") {
		t.Errorf("the driver did not refuse a string in a callee with a K145.\n  output:\n%s",
			res.Output)
	}
	if !strings.Contains(res.Output, "not an integer expression") {
		t.Errorf("the refusal does not name the construct.\n  output:\n%s", res.Output)
	}
}

// TestPhaseNATRodata_GateIsWiredInCI asserts that this gate has a ci.yml step.
func TestPhaseNATRodata_GateIsWiredInCI(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join(repoRoot(t), ".github", "workflows", "ci.yml"))
	if err != nil {
		t.Fatalf("reading ci.yml: %v", err)
	}
	if !strings.Contains(string(raw), "TestPhaseNATRodata_") {
		t.Errorf("ci.yml has no step running this gate's pattern 'TestPhaseNATRodata_'")
	}
}
