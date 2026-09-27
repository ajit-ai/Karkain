package native

import (
	"bytes"
	"fmt"
	"sort"
)

// Mach-O 64-bit writer, Phase 149. Phase 150D adds the three things a
// position-independent executable needs: the PIE flag, real rebase opcodes,
// and a writable __DATA segment for the heap arena.
//
// Layout (x86-64, dyld-hosted, raw syscalls):
//
//	[header 32]
//	[LC_SEGMENT_64 __PAGEZERO 72]
//	[LC_SEGMENT_64 __TEXT     72]
//	[LC_SEGMENT_64 __DATA     72]   (only when the program allocates)
//	[LC_SEGMENT_64 __LINKEDIT 72]
//	[LC_DYLD_INFO_ONLY        48]
//	[LC_LOAD_DYLINKER         32]
//	[LC_MAIN                  24]
//	[.text][.rodata]
//	[zero pad to page][arena, as __DATA]
//	[zero pad to page][rebase opcodes, as __LINKEDIT]
//
// __PAGEZERO plus __TEXT at 0x100000000 is the conventional macOS shape, and
// it is what lets dyld slide the image anywhere: MH_PIE says "this is
// position-independent" and the rebase opcodes are the contract that makes
// that true. The backend bakes absolute addresses into the instruction stream
// as imm64 operands (every .rodata pointer and every arena base), and those
// sites are not PC-relative, so an unslide image would silently break every
// one of them. Each site is therefore published in the rebase stream: dyld
// adds the slide to the 8-byte pointer there at load time. A site that is NOT
// published is a latent miscompile, which is why LinkMachO derives the stream
// from the very same patch lists the addresses were resolved from instead of a
// second, separately maintained inventory.
//
// Section tables stay empty (nsects=0) on purpose: pure mappings have no
// section table to get wrong, and the arena rides in __DATA's filesize.
//
// HONEST LIMIT: no Intel-mac runner exists to execute this (GitHub macOS legs
// are arm64), so the writer is proven structurally only — parseMachO decodes
// the rebase stream and checks it against the image, and the unit goldens pin
// the bytes. First execution on a real Mac is still unproven.

// MachoBase is the conventional x86-64 macOS load address (__TEXT's vmaddr).
//
// It is a uint64 because it does NOT fit in an int on a 32-bit target: 0x100000000
// overflows a 32-bit int, and `pkg/native` is compiled for GOARCH=arm and 386 as
// part of the release build matrix. Keep every use of it in 64-bit arithmetic —
// use machoAddr() to turn a file offset into a vmaddr rather than adding to this
// constant directly.
const MachoBase uint64 = 0x100000000

// machoAddr converts a file offset to its Mach-O vmaddr: the image is mapped
// 1:1 from file offset 0 at MachoBase, so the vmaddr is simply the offset added
// to the base. Doing the addition in uint64 here is what keeps the writer
// buildable on 32-bit hosts.
func machoAddr(fileOff int) uint64 { return MachoBase + uint64(fileOff) }

// MachoPageZeroSize is __PAGEZERO's vmsize: the 4 GiB hole below __TEXT that
// makes a null dereference a guaranteed fault instead of a wild read.
const MachoPageZeroSize = 0x100000000

// machoPageSize is the segment alignment Mach-O requires: a segment's fileoff
// and vmaddr must share the same residue modulo the page size.
const machoPageSize = 0x1000

const (
	machoHeaderSize = 32
	segCmdSize      = 72
	dyldInfoSize    = 48
	dyldLoaderSize  = 32 // 12 + 20 ("/usr/lib/dyld\0" padded so cmdsize % 8 == 0)
	mainCmdSize     = 24
)

// Segment indices as the rebase opcodes see them: 1-based over the
// LC_SEGMENT_64 commands in load-command order. __PAGEZERO comes first, so the
// code — where every rebase site lives — is segment 2, not 1.
const (
	machoSegPageZero = 1
	machoSegText     = 2
	machoProtNone    = 0
	machoProtRead    = 1
	machoProtRW      = 3
	machoProtRX      = 5
)

const (
	machoFlagNoUndefs = 0x1
	machoFlagDyldLink = 0x4
	machoFlagTwoLevel = 0x80
	machoFlagPIE      = 0x200000
)

// machoTextOffset returns the file offset of .text, which follows every load
// command. It is a function rather than a constant because the header grows by
// one 72-byte segment command when the program needs the writable __DATA — the
// same discipline the ELF writer uses for its program-header count.
func machoTextOffset(hasData bool) int {
	segs := 3 // __PAGEZERO + __TEXT + __LINKEDIT
	if hasData {
		segs = 4
	}
	return machoHeaderSize + segs*segCmdSize + dyldInfoSize + dyldLoaderSize + mainCmdSize
}

// MachoDataOffset returns the page-aligned file offset of the writable __DATA
// segment holding the arena, for an image whose mapped body ends at bodyEnd.
// The caller resolves the arena's absolute address from this and LinkMachO
// places the segment here, so the two agree by construction instead of by two
// hand-kept copies of the same arithmetic.
func MachoDataOffset(bodyEnd int) int { return peAlignUp(bodyEnd, machoPageSize) }

// Rebase opcode set (dyld's rebase opcodes, as carried by
// LC_DYLD_INFO_ONLY). A stream is a tiny program over a cursor that walks the
// image 8 bytes at a time, rebasing every pointer-sized slot it visits.
const (
	rebaseOpDone                    = 0x00
	rebaseOpSetTypeImm              = 0x10
	rebaseOpSetSegmentAndOffsetULEB = 0x20
	rebaseOpAddAddrULEB             = 0x30
	rebaseOpDoRebaseImmTimes        = 0x50
	rebaseTypePointer               = 1
	rebasePointerSize               = 8
)

// machoRebaseOpcodes encodes the rebase stream for the given .text offsets of
// the pointer sites. Sites are sorted, so the cursor never walks backwards
// (dyld requires a monotone walk), duplicates are dropped, and runs of nearby
// sites fold into a short hop rather than a fresh SET_SEGMENT_AND_OFFSET.
//
// Every site is a POINTER rebase: the slot already holds the link-time
// (unslid) address, and dyld adds the slide to it at load time.
//
// Two positions are tracked separately, and conflating them is a real logic
// error this function was written to avoid: `cur` is the CURSOR, i.e. the
// address the stream will visit next, which dyld advances by 8 after every
// rebase; `last` is the last slot actually emitted. A site sitting exactly at
// the cursor is the ADJACENT 8-byte slot, not a duplicate — so folding the two
// into one variable silently DROPS such a site, and a negative delta (what a
// genuine duplicate looks like) falls through the range check and encodes as a
// bogus forward hop of a truncated value. The golden table in the 150D gate
// caught both.
//
// In the current lowering the d==0 case is defensive rather than live: every
// baked address is an imm64 inside a 10-byte `movabs`, so distinct sites are at
// least 10 bytes apart and the cursor always lands past the previous one. The
// guard stays because that is a property of today's emission, not of this
// encoding, and a future tighter instruction would otherwise lose a pointer.
func machoRebaseOpcodes(sites []int) []byte {
	sorted := append([]int(nil), sites...)
	sort.Ints(sorted)
	out := []byte{rebaseOpSetTypeImm | rebaseTypePointer}
	last, cur := -1, -1
	for _, s := range sorted {
		if s == last {
			continue // the same slot twice needs one rebase, not two
		}
		switch d := s - cur; {
		case cur < 0:
			// No cursor yet: the first site must be positioned absolutely.
			out = append(out, rebaseOpSetSegmentAndOffsetULEB|machoSegText)
			out = appendUleb(out, uint64(s))
		case d == 0:
			// The cursor already points here; do not reposition.
		case d > 0 && d < 0x80:
			// A short forward hop: cheaper than re-positioning, and the form
			// dyld's own encoder prefers.
			out = append(out, rebaseOpAddAddrULEB, byte(d))
		default:
			out = append(out, rebaseOpSetSegmentAndOffsetULEB|machoSegText)
			out = appendUleb(out, uint64(s))
		}
		out = append(out, rebaseOpDoRebaseImmTimes|1)
		last, cur = s, s+rebasePointerSize
	}
	return append(out, rebaseOpDone)
}

// parseMachORebase decodes a rebase stream and returns the .text offsets it
// rebases. This is the evidence half of the PIE claim: the stream is decoded
// and checked rather than assumed, so one that names the wrong segment,
// truncates, or forgets to terminate is a test failure instead of an image dyld
// would reject at load time.
func parseMachORebase(stream []byte) ([]int, error) {
	var sites []int
	addr, i := 0, 0
	for i < len(stream) {
		op := stream[i]
		i++
		code, imm := op&0xF0, int(op&0x0F)
		readULEB := func() (int, error) {
			v, n, err := readUleb(stream[i:])
			if err != nil {
				return 0, err
			}
			i += n
			return v, nil
		}
		switch code {
		case rebaseOpDone:
			if i != len(stream) {
				return nil, fmt.Errorf("rebase stream has %d byte(s) after DONE", len(stream)-i)
			}
			return sites, nil
		case rebaseOpSetTypeImm:
			if imm != rebaseTypePointer {
				return nil, fmt.Errorf("rebase type %d is not POINTER", imm)
			}
		case rebaseOpSetSegmentAndOffsetULEB:
			if imm != machoSegText {
				return nil, fmt.Errorf("rebase names segment %d; this writer only rebases __TEXT (segment %d)", imm, machoSegText)
			}
			v, err := readULEB()
			if err != nil {
				return nil, fmt.Errorf("rebase offset: %w", err)
			}
			addr = v
		case rebaseOpAddAddrULEB:
			v, err := readULEB()
			if err != nil {
				return nil, fmt.Errorf("rebase add-addr: %w", err)
			}
			addr += v
		case rebaseOpDoRebaseImmTimes:
			if imm == 0 {
				return nil, fmt.Errorf("DO_REBASE_IMM_TIMES 0 rebases nothing")
			}
			for k := 0; k < imm; k++ {
				sites = append(sites, addr)
				addr += rebasePointerSize
			}
		default:
			return nil, fmt.Errorf("unsupported rebase opcode %#x", op)
		}
	}
	return nil, fmt.Errorf("rebase stream ended without DONE")
}

func appendUleb(dst []byte, v uint64) []byte {
	for {
		b := byte(v & 0x7F)
		v >>= 7
		if v != 0 {
			b |= 0x80
		}
		dst = append(dst, b)
		if v == 0 {
			return dst
		}
	}
}

func readUleb(b []byte) (int, int, error) {
	v, shift := 0, 0
	for i := 0; i < len(b); i++ {
		if shift > 21 {
			return 0, 0, fmt.Errorf("uleb128 too long")
		}
		v |= int(b[i]&0x7F) << shift
		if b[i]&0x80 == 0 {
			return v, i + 1, nil
		}
		shift += 7
	}
	return 0, 0, fmt.Errorf("truncated uleb128")
}



// LinkMachO combines .text, .rodata, an optional writable .data arena and the
// rebase opcode stream into a complete PIE image.
//
// rebaseSites are the .text offsets of the imm64 slots that hold absolute
// addresses; each is published in the rebase stream so dyld slides it. Those
// slots must already hold link-time addresses (MachoBase-relative), which is
// what the caller resolves them to.
//
// textOffset must equal machoTextOffset(len(data) > 0): the header carries a
// __DATA segment command only when there is something writable to map.
func LinkMachO(text, rodata, data []byte, textOffset, entryOffset int, rebaseSites []int) ([]byte, error) {
	hasData := len(data) > 0
	if textOffset != machoTextOffset(hasData) {
		return nil, fmt.Errorf("native backend: unsupported mach-o text offset %d (want %d)", textOffset, machoTextOffset(hasData))
	}
	bodyEnd := textOffset + len(text) + len(rodata)
	// __DATA and __LINKEDIT each start on a page boundary, so neither can
	// overlap the mapping before it and both keep fileoff == vmaddr mod page.
	dataOff, linkEditOff := 0, MachoDataOffset(bodyEnd)
	if hasData {
		dataOff = linkEditOff
		linkEditOff = MachoDataOffset(dataOff + len(data))
	}
	// Rebase opcodes address a segment-relative offset, and __TEXT is mapped
	// from file offset 0, so a segment-relative offset is a plain file offset:
	// the caller's .text-relative sites become file offsets by adding the
	// header size. Doing the translation here (rather than in the caller) keeps
	// the caller's contract "these are offsets into .text" and keeps the two
	// from drifting apart.
	segs := make([]int, len(rebaseSites))
	for i, s := range rebaseSites {
		segs[i] = s + textOffset
	}
	rebase := machoRebaseOpcodes(segs)
	out := make([]byte, 0, linkEditOff+len(rebase))

	nsegs := 3 // __PAGEZERO + __TEXT + __LINKEDIT
	if hasData {
		nsegs = 4
	}

	hdr := make([]byte, machoHeaderSize)
	put32(hdr[0:], 0xFEEDFACF) // magic (LE bytes CF FA ED FE)
	put32(hdr[4:], 0x01000007) // cputype: x86-64
	put32(hdr[8:], 3)          // cpusubtype: all
	put32(hdr[12:], 2)         // filetype: EXECUTE
	put32(hdr[16:], uint32(nsegs)+3)
	put32(hdr[20:], uint32(nsegs*segCmdSize+dyldInfoSize+dyldLoaderSize+mainCmdSize))
	put32(hdr[24:], machoFlagNoUndefs|machoFlagDyldLink|machoFlagTwoLevel|machoFlagPIE)
	put32(hdr[28:], 0) // reserved
	out = append(out, hdr...)

	// __PAGEZERO: an unmapped hole from 0 to MachoBase. It is what gives a PIE
	// its conventional preferred address while still guaranteeing that address
	// 0 is never valid.
	out = append(out, machoSegCmd("__PAGEZERO", 0, MachoPageZeroSize, 0, 0, machoProtNone, machoProtNone)...)
	// __TEXT: header + .text + .rodata, R+X, mapped 1:1 from the file start.
	out = append(out, machoSegCmd("__TEXT", MachoBase, uint64(bodyEnd), 0, uint64(bodyEnd), machoProtRX, machoProtRX)...)
	if hasData {
		out = append(out, machoSegCmd("__DATA", machoAddr(dataOff), uint64(len(data)), uint64(dataOff), uint64(len(data)), machoProtRW, machoProtRW)...)
	}
	out = append(out, machoSegCmd("__LINKEDIT", machoAddr(linkEditOff), uint64(len(rebase)), uint64(linkEditOff), uint64(len(rebase)), machoProtRead, machoProtRead)...)

	// LC_DYLD_INFO_ONLY: the rebase opcodes are the only content that matters
	// here. bind/weak_bind/lazy_bind/export stay zero — this image imports
	// nothing (raw syscalls; the loader is named by LC_LOAD_DYLINKER) and
	// exports nothing.
	di := make([]byte, dyldInfoSize)
	put32(di[0:], 0x80000022) // LC_DYLD_INFO_ONLY
	put32(di[4:], dyldInfoSize)
	put32(di[8:], uint32(linkEditOff))  // rebase_off (file offset)
	put32(di[12:], uint32(len(rebase))) // rebase_size
	out = append(out, di...)

	ld := make([]byte, dyldLoaderSize)
	put32(ld[0:], 0xC) // LC_LOAD_DYLINKER
	put32(ld[4:], dyldLoaderSize)
	put32(ld[8:], 12) // name offset
	copy(ld[12:], "/usr/lib/dyld\x00")
	out = append(out, ld...)

	mc := make([]byte, mainCmdSize)
	put32(mc[0:], 0x80000028) // LC_MAIN
	put32(mc[4:], mainCmdSize)
	put64(mc[8:], uint64(entryOffset)) // entryoff: a file offset, so slide-independent
	put64(mc[16:], 0)                  // stacksize
	out = append(out, mc...)

	out = append(out, text...)
	out = append(out, rodata...)
	if hasData {
		out = append(out, make([]byte, dataOff-len(out))...) // zero-fill the page gap
		out = append(out, data...)
	}
	out = append(out, make([]byte, linkEditOff-len(out))...) // zero-fill the page gap
	out = append(out, rebase...)
	return out, nil
}

func machoSegCmd(name string, vmaddr, vmsize, fileoff, filesize uint64, maxprot, initprot uint32) []byte {
	seg := make([]byte, segCmdSize)
	put32(seg[0:], 0x19) // LC_SEGMENT_64
	put32(seg[4:], segCmdSize)
	copy(seg[8:24], name)
	put64(seg[24:], vmaddr)
	put64(seg[32:], vmsize)
	put64(seg[40:], fileoff)
	put64(seg[48:], filesize)
	put32(seg[56:], maxprot)
	put32(seg[60:], initprot)
	put32(seg[64:], 0) // nsects
	put32(seg[68:], 0) // flags
	return seg
}


// machoSeg is one decoded LC_SEGMENT_64.
type machoSeg struct {
	vmaddr, vmsize, fileoff, filesize uint64
	maxprot, initprot                 uint32
}

// machoInfo is a decoded image: its load-command set, its entry, and the rebase
// stream decoded back into the file offsets it slides.
type machoInfo struct {
	entry   uint64
	textOff int
	bodyEnd int
	hasData bool
	dataOff int
	segs    map[string]machoSeg
	sites   []int
}

// parseMachO validates a linked image's shape and decodes its rebase stream.
//
// It is deliberately strict. Every structural claim this writer makes is
// checked back here — MH_PIE, the __PAGEZERO/__TEXT/__DATA/__LINKEDIT set and
// their protections, the page-aligned segment placement, and the rebase stream
// — so an image dyld would reject is a test failure rather than a claim in a
// comment. It is also the assertion helper where no execution host exists.
func parseMachO(img []byte) (machoInfo, error) {
	var info machoInfo
	info.segs = map[string]machoSeg{}
	if len(img) < machoTextOffset(false) {
		return info, fmt.Errorf("image too small (%d bytes)", len(img))
	}
	if !bytes.Equal(img[0:4], []byte{0xCF, 0xFA, 0xED, 0xFE}) {
		return info, fmt.Errorf("bad Mach-O magic")
	}
	if get32(img[4:]) != 0x01000007 {
		return info, fmt.Errorf("not x86-64 (cputype)")
	}
	if get32(img[12:]) != 2 {
		return info, fmt.Errorf("not an executable (filetype != EXECUTE)")
	}
	// Without MH_PIE the image is position-dependent: it would only load at
	// MachoBase, and on a modern system that address is not guaranteed to be
	// available at all. Refusing to bless that shape keeps the writer's "every
	// baked address is rebased" claim checkable.
	if get32(img[24:])&machoFlagPIE == 0 {
		return info, fmt.Errorf("MH_PIE is not set (position-dependent image)")
	}
	ncmds := int(get32(img[16:]))
	off := machoHeaderSize
	var entryOff uint64
	var rebaseOff, rebaseLen int
	foundMain, foundDyldInfo := false, false
	for i := 0; i < ncmds; i++ {
		if off+8 > len(img) {
			return info, fmt.Errorf("load command %d out of range", i)
		}
		cmd, size := get32(img[off:]), int(get32(img[off+4:]))
		if size < 8 || size%8 != 0 || off+size > len(img) {
			return info, fmt.Errorf("load command %d has bad size %d", i, size)
		}
		switch cmd {
		case 0x19: // LC_SEGMENT_64
			if size != segCmdSize {
				return info, fmt.Errorf("LC_SEGMENT_64 %d is %d bytes, want %d", i, size, segCmdSize)
			}
			name := string(bytes.TrimRight(img[off+8:off+24], "\x00"))
			if get32(img[off+64:]) != 0 {
				return info, fmt.Errorf("segment %s carries a section table; this writer emits none", name)
			}
			info.segs[name] = machoSeg{
				vmaddr: get64(img[off+24:]), vmsize: get64(img[off+32:]),
				fileoff: get64(img[off+40:]), filesize: get64(img[off+48:]),
				maxprot: get32(img[off+56:]), initprot: get32(img[off+60:]),
			}
		case 0x80000028: // LC_MAIN
			if size != mainCmdSize {
				return info, fmt.Errorf("bad LC_MAIN size %d", size)
			}
			entryOff = get64(img[off+8:])
			foundMain = true
		case 0x80000022: // LC_DYLD_INFO_ONLY
			if size != dyldInfoSize {
				return info, fmt.Errorf("bad LC_DYLD_INFO_ONLY size %d", size)
			}
			rebaseOff, rebaseLen = int(get32(img[off+8:])), int(get32(img[off+12:]))
			if get32(img[off+16:]) != 0 || get32(img[off+20:]) != 0 {
				return info, fmt.Errorf("bind opcodes present; this image imports nothing")
			}
			foundDyldInfo = true
		}
		off += size
	}
	if off != machoHeaderSize+int(get32(img[20:])) {
		return info, fmt.Errorf("sizeofcmds (%d) does not match the %d load command bytes", get32(img[20:]), off-machoHeaderSize)
	}
	if !foundMain {
		return info, fmt.Errorf("no LC_MAIN")
	}
	if !foundDyldInfo {
		return info, fmt.Errorf("no LC_DYLD_INFO_ONLY (a PIE needs its rebase opcodes)")
	}
	pz, ok := info.segs["__PAGEZERO"]
	if !ok {
		return info, fmt.Errorf("no __PAGEZERO segment")
	}
	if pz.vmaddr != 0 || pz.vmsize != MachoPageZeroSize || pz.filesize != 0 || pz.initprot != machoProtNone {
		return info, fmt.Errorf("__PAGEZERO is not the 4 GiB unmapped hole at 0")
	}
	text, ok := info.segs["__TEXT"]
	if !ok {
		return info, fmt.Errorf("no __TEXT segment")
	}
	if text.fileoff != 0 || text.vmaddr != MachoBase || text.initprot != machoProtRX {
		return info, fmt.Errorf("__TEXT is not the R+X mapping of the file at %#x", MachoBase)
	}
	le, ok := info.segs["__LINKEDIT"]
	if !ok {
		return info, fmt.Errorf("no __LINKEDIT segment for the rebase opcodes")
	}
	if d, hasData := info.segs["__DATA"]; hasData {
		info.hasData = true
		info.dataOff = int(d.fileoff)
		// A non-writable __DATA would fault on the arena's first bump-cursor
		// store, and a zero-length one would map nothing at all.
		if d.initprot != machoProtRW {
			return info, fmt.Errorf("__DATA initprot is %d, want R+W (%d)", d.initprot, machoProtRW)
		}
		if d.filesize == 0 || d.vmsize != d.filesize {
			return info, fmt.Errorf("__DATA is empty or has BSS (vmsize %d, filesize %d)", d.vmsize, d.filesize)
		}
	}
	return machoCheckLayout(img, info, text, le, entryOff, rebaseOff, rebaseLen)
}
// machoCheckLayout finishes the structural validation: the page-aligned
// segment placement, the entry, and the rebase stream.
func machoCheckLayout(img []byte, info machoInfo, text, le machoSeg, entryOff uint64, rebaseOff, rebaseLen int) (machoInfo, error) {
	info.textOff = machoTextOffset(info.hasData)
	info.bodyEnd = int(text.filesize)
	if info.bodyEnd <= info.textOff {
		return info, fmt.Errorf("__TEXT filesize %d does not cover a %d-byte header", text.filesize, info.textOff)
	}
	// Page-aligned placement is what makes the image loadable at all: fileoff
	// and vmaddr must agree modulo the page size, and no segment may overlap
	// the one before it. Checking the exact offsets pins the whole layout.
	wantLinkEdit := MachoDataOffset(info.bodyEnd)
	if info.hasData {
		if info.dataOff != wantLinkEdit {
			return info, fmt.Errorf("__DATA fileoff %d, want the page-aligned %d", info.dataOff, wantLinkEdit)
		}
		wantLinkEdit = MachoDataOffset(info.dataOff + int(info.segs["__DATA"].filesize))
	}
	if int(le.fileoff) != wantLinkEdit {
		return info, fmt.Errorf("__LINKEDIT fileoff %d, want the page-aligned %d", le.fileoff, wantLinkEdit)
	}
	if le.vmsize != le.filesize {
		return info, fmt.Errorf("__LINKEDIT vmsize %d != filesize %d", le.vmsize, le.filesize)
	}
	if entryOff < uint64(info.textOff) || entryOff >= uint64(info.bodyEnd) {
		return info, fmt.Errorf("entry file offset %#x outside .text", entryOff)
	}
	// The rebase stream must live exactly where __LINKEDIT says it does, and be
	// a complete, well-formed stream: dyld reads these bytes to decide which
	// pointers to slide, so a truncated or misdirected stream is an image that
	// loads with silently unrelocated addresses.
	if rebaseOff != int(le.fileoff) || rebaseLen != int(le.filesize) {
		return info, fmt.Errorf("rebase opcodes at %d+%d are not the __LINKEDIT contents at %d+%d", rebaseOff, rebaseLen, le.fileoff, le.filesize)
	}
	if rebaseOff < 0 || rebaseLen < 1 || rebaseOff+rebaseLen > len(img) {
		return info, fmt.Errorf("rebase opcodes (offset %d, size %d) fall outside the image", rebaseOff, rebaseLen)
	}
	sites, err := parseMachORebase(img[rebaseOff : rebaseOff+rebaseLen])
	if err != nil {
		return info, err
	}
	for _, s := range sites {
		if s < info.textOff || s+rebasePointerSize > info.bodyEnd {
			return info, fmt.Errorf("rebase site %#x is outside .text [%#x,%#x)", s, info.textOff, info.bodyEnd)
		}
	}
	info.sites = sites
	info.entry = MachoBase + entryOff
	return info, nil
}

// ParseMachO checks the structural shape of a linked image and returns the
// link-time entry address and the file offset of .text.
func ParseMachO(img []byte) (entry uint64, textOff int, err error) {
	info, err := parseMachO(img)
	if err != nil {
		return 0, 0, err
	}
	return info.entry, info.textOff, nil
}

// MachORebaseSites returns the file offsets inside .text that the image's
// rebase opcodes slide, i.e. every baked-in absolute address. It is exported so
// a gate can assert the published set is exactly the set the lowering patched:
// a site missing from the stream is a PIE image that would run with unrelocated
// pointers.
func MachORebaseSites(img []byte) ([]int, error) {
	info, err := parseMachO(img)
	if err != nil {
		return nil, err
	}
	return info.sites, nil
}

// MachOHasDataSegment reports whether the image maps a writable __DATA segment
// (the heap arena). Its absence is why an allocating program used to be refused
// on this container.
func MachOHasDataSegment(img []byte) (bool, error) {
	info, err := parseMachO(img)
	if err != nil {
		return false, err
	}
	return info.hasData, nil
}


