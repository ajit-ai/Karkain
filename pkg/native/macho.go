package native

import (
	"bytes"
	"fmt"
)

// Mach-O 64-bit writer, Phase 149.
//
// Layout (x86-64, dyld-hosted, raw syscalls):
//
//	[header 32][LC_SEGMENT_64 72][LC_DYLD_INFO_ONLY 48][LC_LOAD_DYLINKER 28][LC_MAIN 24][.text][.rodata]
//
// One __TEXT segment (nsects=0: pure mapping, no section table to get
// wrong) covers the whole file at MachoBase with R+X. LC_LOAD_DYLINKER
// names /usr/lib/dyld and LC_MAIN carries the entry file offset; the
// code uses raw `syscall` with macOS class-shifted numbers, which works
// in a dyld-hosted process. LC_DYLD_INFO_ONLY is zeroed (best effort per
// tiny-demo lore).
//
// HONEST LIMIT: no Intel-mac runner exists to execute this (GitHub macOS
// legs are arm64), so the writer is proven structurally only
// (ParseMachO + golden header pins). First execution on a real Mac may
// still indict the dyld-info shape — that would be a 149 follow-up, not
// a silent acceptance.

// MachoBase is the conventional x86-64 macOS load address.
const MachoBase = 0x100000000

const (
	machoHeaderSize = 32
	segCmdSize      = 72
	dyldInfoSize    = 48
	dyldLoaderSize  = 32 // 12 + 20 ("/usr/lib/dyld\0" padded to 8)
	mainCmdSize     = 24
)

// machoTextOff locates .text (and therefore the entry point).
var machoTextOff = machoHeaderSize + segCmdSize + dyldInfoSize + dyldLoaderSize + mainCmdSize // 204

// LinkMachO combines .text and .rodata into a complete Mach-O image.
// textStart is the file offset of .text (always machoTextOff here);
// entryLabel is resolved by the caller to a file offset before calling.
func LinkMachO(text, rodata []byte, textOffset, entryOffset int) ([]byte, error) {
	if textOffset != machoTextOff {
		return nil, fmt.Errorf("native backend: unsupported mach-o text offset %d", textOffset)
	}
	total := textOffset + len(text) + len(rodata)
	out := make([]byte, 0, total)

	hdr := make([]byte, machoHeaderSize)
	put32(hdr[0:], 0xFEEDFACF) // magic (LE bytes CF FA ED FE)
	put32(hdr[4:], 0x01000007) // cputype: x86-64
	put32(hdr[8:], 3)          // cpusubtype: all
	put32(hdr[12:], 2)         // filetype: EXECUTE
	put32(hdr[16:], 4)         // ncmds
	put32(hdr[20:], uint32(segCmdSize+dyldInfoSize+dyldLoaderSize+mainCmdSize))
	put32(hdr[24:], 0x85) // flags: NOUNDEFS|DYLDLINK|TWOLEVEL
	put32(hdr[28:], 0)    // reserved
	out = append(out, hdr...)

	seg := make([]byte, segCmdSize)
	put32(seg[0:], 0x19) // LC_SEGMENT_64
	put32(seg[4:], segCmdSize)
	copy(seg[8:24], "__TEXT")
	put64(seg[24:], MachoBase) // vmaddr
	put64(seg[32:], uint64(total))
	put64(seg[40:], 0) // fileoff
	put64(seg[48:], uint64(total))
	put32(seg[56:], 5) // maxprot: R+X
	put32(seg[60:], 5) // initprot: R+X
	put32(seg[64:], 0) // nsects
	put32(seg[68:], 0) // flags
	out = append(out, seg...)

	di := make([]byte, dyldInfoSize)
	put32(di[0:], 0x80000022) // LC_DYLD_INFO_ONLY (zeroed: no rebase/bind info)
	put32(di[4:], dyldInfoSize)
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
	put64(mc[8:], uint64(entryOffset)) // entryoff: file offset (already absolute)
	put64(mc[16:], 0)                  // stacksize
	out = append(out, mc...)

	out = append(out, text...)
	out = append(out, rodata...)
	return out, nil
}

// ParseMachO checks the structural shape of a linked image (magic,
// cputype, EXECUTE type, four load commands, LC_MAIN entry inside the
// file). It is the assertion helper where no execution host exists.
func ParseMachO(img []byte) (entry uint64, textOff int, err error) {
	if len(img) < machoTextOff {
		return 0, 0, fmt.Errorf("image too small (%d bytes)", len(img))
	}
	if !bytes.Equal(img[0:4], []byte{0xCF, 0xFA, 0xED, 0xFE}) {
		return 0, 0, fmt.Errorf("bad Mach-O magic")
	}
	if get32(img[4:]) != 0x01000007 {
		return 0, 0, fmt.Errorf("not x86-64 (cputype)")
	}
	if get32(img[12:]) != 2 {
		return 0, 0, fmt.Errorf("not an executable (filetype != EXECUTE)")
	}
	ncmds := get32(img[16:])
	if ncmds != 4 {
		return 0, 0, fmt.Errorf("want 4 load commands, got %d", ncmds)
	}
	off := machoHeaderSize
	var entryOff uint64
	foundMain := false
	for i := uint32(0); i < ncmds; i++ {
		if off+8 > len(img) {
			return 0, 0, fmt.Errorf("load command %d out of range", i)
		}
		cmd := get32(img[off:])
		size := get32(img[off+4:])
		if cmd == 0x80000028 { // LC_MAIN
			if size != mainCmdSize || off+24 > len(img) {
				return 0, 0, fmt.Errorf("bad LC_MAIN")
			}
			entryOff = get64(img[off+8:])
			foundMain = true
		}
		if size < 8 || size%8 != 0 {
			return 0, 0, fmt.Errorf("bad load command size %d", size)
		}
		off += int(size)
	}
	if !foundMain {
		return 0, 0, fmt.Errorf("no LC_MAIN")
	}
	if entryOff < uint64(machoTextOff) || entryOff >= uint64(len(img)) {
		return 0, 0, fmt.Errorf("entry file offset %#x outside image", entryOff)
	}
	return MachoBase + entryOff, machoTextOff, nil
}
