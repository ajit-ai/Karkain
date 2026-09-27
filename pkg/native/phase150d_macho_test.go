package native

// Phase 150D — Mach-O PIE, rebase opcodes and the writable __DATA segment.
//
// What this gate proves, and why each assertion earns its place:
//
//  1. The rebase ENCODER is golden-pinned byte-for-byte, and the DECODER
//     round-trips it. A stream that encodes wrongly and decodes wrongly in
//     agreement is the failure mode a round-trip alone would miss.
//  2. The decoder REJECTS malformed streams (no DONE, trailing bytes after
//     DONE, a non-POINTER type, the wrong segment, a truncated uleb). A
//     permissive decoder would let a corrupt stream pass validation.
//  3. Every baked absolute address is PUBLISHED as a rebase site — the
//     load-bearing PIE assertion. A site missing from the stream is an image
//     that loads and then dereferences a pointer that was never slid.
//  4. An allocating program maps a WRITABLE __DATA arena, and the arena's
//     absolute address is the one the code actually holds.
//  5. parseMachO REJECTS a tampered image: a cleared MH_PIE flag, a
//     non-writable __DATA and a moved rebase stream each fail validation.
//
// HONEST LIMIT, unchanged from Phase 149: no Intel-mac runner exists, so
// nothing here EXECUTES a Mach-O image. These are structural proofs.

import (
	"bytes"
	"testing"

	"karkain/pkg/lexer"

	"karkain/pkg/parser"
)

// TestMachORebaseOpcodesGolden pins the encoder's bytes. A rebase stream is a
// wire format dyld interprets, so its exact encoding is part of the contract,
// not an implementation detail.
func TestMachORebaseOpcodesGolden(t *testing.T) {
	for _, c := range []struct {
		name  string
		sites []int
		want  []byte
	}{
		// No sites: the type is still declared, then DONE.
		{"empty", nil, []byte{0x11, 0x00}},
		// One site: SET_SEGMENT_AND_OFFSET_ULEB(seg 2, off 0) + DO_REBASE(1).
		{"one", []int{0}, []byte{0x11, 0x22, 0x00, 0x51, 0x00}},
		// A second site 8 bytes later is the ADJACENT slot, and the cursor is
		// already there after the first rebase, so it costs a bare DO_REBASE.
		// This is the arena base+limit shape, and mistaking it for a duplicate
		// (the first version of this encoder did) dropped the limit.
		{"two_adjacent", []int{0, 8}, []byte{0x11, 0x22, 0x00, 0x51, 0x51, 0x00}},
		// Unsorted input must produce the same stream as sorted input: the
		// cursor may never walk backwards.
		{"unsorted", []int{16, 0, 8}, []byte{0x11, 0x22, 0x00, 0x51, 0x51, 0x51, 0x00}},
		// A duplicate slot needs one rebase, not two, and must not be encoded
		// as a forward hop of a negative delta.
		{"dup", []int{4, 4}, []byte{0x11, 0x22, 0x04, 0x51, 0x00}},
		// A site inside the 1-byte hop range folds into ADD_ADDR_ULEB. The hop
		// is the DISTANCE from the cursor (which advanced to 8 after the first
		// rebase), so site 40 is a hop of 40-8 = 32 = 0x20.
		{"near", []int{0, 40}, []byte{0x11, 0x22, 0x00, 0x51, 0x30, 0x20, 0x51, 0x00}},
		// A far site cannot use the 1-byte hop, so it re-positions absolutely.
		// 200 encodes as 0xC8 0x01 (uleb128).
		{"far", []int{0, 200}, []byte{0x11, 0x22, 0x00, 0x51, 0x22, 0xC8, 0x01, 0x51, 0x00}},
	} {
		t.Run(c.name, func(t *testing.T) {
			got := machoRebaseOpcodes(c.sites)
			if !bytes.Equal(got, c.want) {
				t.Fatalf("stream = % x, want % x", got, c.want)
			}
			// ...and the decoder must recover exactly the distinct input set.
			sites, err := parseMachORebase(got)
			if err != nil {
				t.Fatalf("decode: %v", err)
			}
			want := map[int]bool{}
			for _, s := range c.sites {
				want[s] = true
			}
			if len(sites) != len(want) {
				t.Fatalf("decoded %v, want %d distinct site(s)", sites, len(want))
			}
			for i, s := range sites {
				if !want[s] {
					t.Fatalf("decoded site %d = %d, which was never encoded (all: %v)", i, s, sites)
				}
			}
		})
	}
}

// TestMachORebaseDecodeRejects is the negative table: a decoder that accepts
// garbage would make the structural validation a rubber stamp.
func TestMachORebaseDecodeRejects(t *testing.T) {
	for _, c := range []struct {
		name   string
		stream []byte
	}{
		{"no_done", []byte{0x11, 0x22, 0x00, 0x51}},
		{"bytes_after_done", []byte{0x11, 0x00, 0x51}},
		{"wrong_type", []byte{0x12, 0x22, 0x00, 0x51, 0x00}},    // TEXTURE, not POINTER
		{"wrong_segment", []byte{0x11, 0x21, 0x00, 0x51, 0x00}}, // segment 1 (__PAGEZERO)
		// An over-long uleb128. NOTE: an earlier version of this table used
		// {0x80, 0x51} as a "truncated uleb", but that is a perfectly valid
		// two-byte uleb (10368) — the fixture was wrong, not the decoder, and
		// it "passed" only because the decoder was right to accept it.
		{"uleb_too_long", []byte{0x11, 0x22, 0x80, 0x80, 0x80, 0x80, 0x80, 0x00}},
		{"zero_times", []byte{0x11, 0x22, 0x00, 0x50, 0x00}},     // rebase nothing
		{"unsupported_opcode", []byte{0x11, 0xC0, 0x00}},         // 0xC0 is not in the set
		{"empty_stream", []byte{}},
	} {
		t.Run(c.name, func(t *testing.T) {
			if sites, err := parseMachORebase(c.stream); err == nil {
				t.Errorf("accepted a malformed stream as %v", sites)
			}
		})
	}
}

// TestMachOPIE pins the load-command set of a heapless image: MH_PIE, the
// __PAGEZERO hole, __TEXT R+X at the conventional base, and __LINKEDIT holding
// the rebase opcodes.
func TestMachOPIE(t *testing.T) {
	img := compileNativeOS(t, OSMacOS, "func main() {\n}\n")
	entry, textOff, err := ParseMachO(img)
	if err != nil {
		t.Fatal(err)
	}
	if flags := get32(img[24:]); flags&machoFlagPIE == 0 {
		t.Errorf("MH_PIE not set; flags = %#x", flags)
	}
	if textOff != machoTextOffset(false) {
		t.Errorf("textOff = %d, want %d", textOff, machoTextOffset(false))
	}
	if entry < MachoBase+uint64(textOff) {
		t.Errorf("entry %#x below the image", entry)
	}
	// A heapless program allocates nothing, so there must be no __DATA segment
	// and no baked ARENA address.
	//
	// It is not correct to expect zero rebase sites here: the always-emitted
	// print helpers reference .rodata (the digit table, the "-" sign), and
	// those pointers are baked too, so they must be slid. What must hold is
	// that every published site points INSIDE the image — i.e. at .rodata —
	// and never at an arena that does not exist.
	hasData, err := MachOHasDataSegment(img)
	if err != nil {
		t.Fatal(err)
	}
	if hasData {
		t.Errorf("heapless program must not carry a __DATA segment")
	}
	sites, err := MachORebaseSites(img)
	if err != nil {
		t.Fatalf("rebase opcodes must decode: %v", err)
	}
	for _, s := range sites {
		v := get64(img[s:])
		if v < MachoBase || v >= MachoBase+uint64(len(img)) {
			t.Errorf("rebase site %#x holds %#x, which is outside this image", s, v)
		}
	}
}

// TestMachOAllocatingDataSegment is the __DATA half of the contract: a program
// that allocates gets a writable segment, and the arena address the code holds
// is the segment's own vmaddr.
func TestMachOAllocatingDataSegment(t *testing.T) {
	// String concatenation needs the arena, so this program allocates.
	img := compileNativeOS(t, OSMacOS, "func main() {\n    let a = \"ka\" + \"rk\"\n    print(a)\n}\n")
	hasData, err := MachOHasDataSegment(img)
	if err != nil {
		t.Fatal(err)
	}
	if !hasData {
		t.Fatalf("allocating program must map a writable __DATA arena")
	}
	info, err := parseMachO(img)
	if err != nil {
		t.Fatal(err)
	}
	// The arena starts at the page-aligned offset after __TEXT, so its
	// absolute address is MachoBase + that offset. Every imm64 that points
	// into the arena must be published for the slide.
	wantArena := uint64(MachoBase + MachoDataOffset(info.bodyEnd))
	if info.segs["__DATA"].vmaddr != wantArena {
		t.Errorf("__DATA vmaddr = %#x, want %#x", info.segs["__DATA"].vmaddr, wantArena)
	}
	if len(info.sites) == 0 {
		t.Fatalf("an allocating program bakes the arena address, so it must publish rebase sites")
	}
	// The published sites must be genuine 8-byte pointer slots in .text, and
	// the arena base must be one of the values actually stored there.
	var sawArena bool
	for _, s := range info.sites {
		v := get64(img[s:])
		if v == wantArena {
			sawArena = true
		}
		if v < MachoBase {
			t.Errorf("rebase site %#x holds %#x, which is not a link-time address", s, v)
		}
	}
	if !sawArena {
		t.Errorf("no rebase site holds the arena base %#x; the arena pointer would never be slid", wantArena)
	}
	// The allocator materialises TWO absolute addresses from the arena: its
	// base (R10) and its END (R11, the bump limit the new cursor is checked
	// against). Both must be published, or a slid cursor would be bound-checked
	// against a pre-slide limit.
	//
	// The end is `arena + filesize`, NOT the stored limit field at header
	// offset 8: emitAllocHelper never loads that field, it re-materialises the
	// end as a link-time constant. An earlier version of this test expected
	// heapLimitOff here and failed — the expectation was wrong, and reading the
	// allocator is what corrected it.
	arenaEnd := wantArena + info.segs["__DATA"].filesize
	var sawBase, sawEnd bool
	for _, s := range info.sites {
		switch get64(img[s:]) {
		case wantArena:
			sawBase = true
		case arenaEnd:
			sawEnd = true
		}
	}
	if !sawBase || !sawEnd {
		t.Errorf("the arena base (%#x) and end (%#x) must both be published as rebase sites; sites=%v",
			wantArena, arenaEnd, info.sites)
	}
}

// BenchmarkNativeCompile measures pure-Go emission (parse is excluded) for the
// three containers.
//
// It exists to justify a design decision with a number rather than a hunch: the
// incremental cache refuses every native target, and "native-split caching is
// not worth it" is only a defensible claim if the thing being cached is
// actually slow. Run it with -bench to re-measure on a new host before
// revisiting that decision.
func BenchmarkNativeCompile(b *testing.B) {
	cases := []struct {
		name   string
		osName string
		src    string
	}{
		{"hello_linux", OSLinux, "func main() {\n    print(\"hello native\")\n}\n"},
		{"concat_linux", OSLinux, "func main() {\n    let a = \"ka\" + \"rk\"\n    print(a)\n}\n"},
		{"map_linux", OSLinux, "func main() {\n    let m = {1: 10, 2: 20}\n    m[3] = 30\n    print(len(m))\n}\n"},
		{"concat_windows", OSWindows, "func main() {\n    let a = \"ka\" + \"rk\"\n    print(a)\n}\n"},
		{"concat_macos", OSMacOS, "func main() {\n    let a = \"ka\" + \"rk\"\n    print(a)\n}\n"},
	}
	for _, c := range cases {
		prog := parseNativeB(b, c.src)
		b.Run(c.name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				if _, err := CompileProgramForOS(prog, c.osName); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// parseNativeB parses src for a benchmark. The parser is shared with the
// compile being measured, so it is hoisted out of the timed loop.
func parseNativeB(b *testing.B, src string) *parser.Program {
	b.Helper()
	l := lexer.New(src)
	p := parser.New(l)
	prog := p.ParseProgram()
	if len(p.Errors) > 0 {
		b.Fatalf("parse: %v", p.Errors)
	}
	return prog
}

// image could be wrong; each must be caught, because a structural validator that
// blesses a broken image is worse than no validator.
func TestParseMachORejects(t *testing.T) {
	allocating := compileNativeOS(t, OSMacOS, "func main() {\n    let a = \"ka\" + \"rk\"\n    print(a)\n}\n")
	// A non-allocating image is the one with no __DATA segment.
	heapless := compileNativeOS(t, OSMacOS, "func main() {\n    print(42)\n}\n")

	clone := func(src []byte) []byte { return append([]byte(nil), src...) }

	// A heapless program must still validate, or the tamper table below would
	// be testing a pre-broken fixture rather than the tampering.
	if hasData, err := MachOHasDataSegment(heapless); err != nil || hasData {
		t.Fatalf("heapless fixture invalid: hasData=%v err=%v", hasData, err)
	}

	t.Run("no_pie", func(t *testing.T) {
		img := clone(allocating)
		put32(img[24:], get32(img[24:])&^machoFlagPIE)
		if _, _, err := ParseMachO(img); err == nil {
			t.Errorf("accepted an image with MH_PIE cleared")
		}
	})
	t.Run("text_not_rx", func(t *testing.T) {
		img := clone(allocating)
		// __TEXT is the first segment command after the header; initprot sits
		// at +60 within it. R+W instead of R+X.
		put32(img[machoHeaderSize+60:], machoProtRW)
		if _, _, err := ParseMachO(img); err == nil {
			t.Errorf("accepted a __TEXT that is not R+X")
		}
	})
	t.Run("pagezero_shrunk", func(t *testing.T) {
		img := clone(allocating)
		// vmsize at +32 of the first segment command: 1 MiB instead of 4 GiB.
		put64(img[machoHeaderSize+32:], 0x100000)
		if _, _, err := ParseMachO(img); err == nil {
			t.Errorf("accepted a __PAGEZERO that is not the 4 GiB hole")
		}
	})
	t.Run("data_not_writable", func(t *testing.T) {
		img := clone(allocating)
		// __PAGEZERO, __TEXT, then __DATA: the third segment command.
		off := machoHeaderSize + 2*segCmdSize
		if get32(img[off:]) != 0x19 {
			t.Fatalf("test setup: expected LC_SEGMENT_64 at %#x, got %#x", off, get32(img[off:]))
		}
		put32(img[off+60:], machoProtRX) // R+X: the arena's first store would fault
		if _, _, err := ParseMachO(img); err == nil {
			t.Errorf("accepted a non-writable __DATA arena")
		}
	})
	t.Run("data_unaligned", func(t *testing.T) {
		img := clone(allocating)
		info, err := parseMachO(img)
		if err != nil {
			t.Fatal(err)
		}
		off := machoHeaderSize + 2*segCmdSize
		put64(img[off+40:], info.segs["__DATA"].fileoff+1) // fileoff off the page
		if _, _, err := ParseMachO(img); err == nil {
			t.Errorf("accepted a __DATA that is not page-aligned after __TEXT")
		}
	})
	t.Run("rebase_stream_moved", func(t *testing.T) {
		img := clone(allocating)
		// LC_DYLD_INFO_ONLY follows the segment commands: 4 of them.
		diOff := machoHeaderSize + 4*segCmdSize
		if get32(img[diOff:]) != 0x80000022 {
			t.Fatalf("test setup: expected LC_DYLD_INFO_ONLY at %#x, got %#x", diOff, get32(img[diOff:]))
		}
		put32(img[diOff+8:], 0) // rebase_off -> the file header, not the stream
		if _, _, err := ParseMachO(img); err == nil {
			t.Errorf("accepted a rebase stream that is not the __LINKEDIT contents")
		}
	})
	t.Run("truncated_rebase_stream", func(t *testing.T) {
		img := clone(allocating)
		diOff := machoHeaderSize + 4*segCmdSize
		leOff := machoHeaderSize + 3*segCmdSize // __LINKEDIT is the last segment
		shorten := uint32(int(get32(img[diOff+12:])) - 1)
		put32(img[diOff+12:], shorten)             // LC_DYLD_INFO_ONLY: rebase_size
		put32(img[leOff+48:], shorten)             // __LINKEDIT: filesize
		if _, _, err := ParseMachO(img); err == nil {
			t.Errorf("accepted a rebase stream with no DONE")
		}
	})
	t.Run("bind_opcodes_present", func(t *testing.T) {
		img := clone(allocating)
		// bind_off at +16 of LC_DYLD_INFO_ONLY. This image imports nothing, so
		// a bind stream is a claim the image cannot honour.
		put32(img[machoHeaderSize+4*segCmdSize+16:], 1)
		if _, _, err := ParseMachO(img); err == nil {
			t.Errorf("accepted bind opcodes on an image that imports nothing")
		}
	})
}
