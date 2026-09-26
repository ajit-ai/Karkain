package native

import (
	"bytes"
	"fmt"
)

// ELF64 writer, Phase 145.
//
// Layout (static, position-dependent, libc-free):
//
//	[ELF header 64][program header 56][.text][.rodata]
//
// A single PT_LOAD segment maps the whole file at BaseAddr with R+X
// permissions (reads are permitted; there is no separate writable segment
// in v1 — all mutation happens on the stack). No section headers, no
// interpreter, no dynamic section: the kernel loads it and jumps to
// _start. This is the complete static-link story for programs that only
// need raw syscalls.

// BaseAddr is the conventional x86-64 load address.
const BaseAddr = 0x400000

// headerSize + phSize locate .text (and therefore the entry point).
const (
	elfHeaderSize  = 64
	progHeaderSize = 56
)

// Link combines .text, .rodata and an optional writable data segment into a
// complete executable image. textStart is the file offset of .text; the
// header count follows from whether `data` is present, so a program with no
// heap gets exactly the pre-150B single-LOAD image (byte-for-byte).
// entryLabel is resolved by the caller to a file offset before calling.
//
// Phase 150B: `data` is the heap arena, carried by a second R+W PT_LOAD. The
// R+X segment stops at the arena so the two mappings never overlap, the
// arena is placed on a page boundary, and p_offset ≡ p_vaddr (mod page) —
// the ELF loader's alignment requirement.
func Link(text, rodata, data []byte, textOffset, entryOffset int) ([]byte, error) {
	phnum := 1
	if len(data) > 0 {
		phnum = 2
	}
	if textOffset != elfHeaderSize+progHeaderSize*phnum {
		return nil, fmt.Errorf("native backend: unsupported text offset %d (phnum %d)", textOffset, phnum)
	}
	// Arena placement: page-aligned file offset, so the vaddr has the same
	// page residue as the offset.
	dataOff := peAlignUp(textOffset+len(text)+len(rodata), 0x1000)
	bodyEnd := dataOff
	if len(data) == 0 {
		dataOff = 0
		bodyEnd = textOffset + len(text) + len(rodata)
	}
	total := bodyEnd
	out := make([]byte, 0, total+len(data))

	eh := make([]byte, elfHeaderSize)
	eh[0], eh[1], eh[2], eh[3] = 0x7F, 'E', 'L', 'F'
	eh[4], eh[5], eh[6] = 2, 1, 1 // 64-bit, little-endian, version
	// eh[7] osabi = 0 (SysV); eh[8] abiversion = 0
	put16(eh[16:], 2)                            // type: EXEC
	put16(eh[18:], 62)                           // machine: x86-64
	put32(eh[20:], 1)                            // version
	put64(eh[24:], uint64(BaseAddr+entryOffset)) // entry
	put64(eh[32:], elfHeaderSize)                // phoff
	put64(eh[40:], 0)                            // shoff: none
	put32(eh[48:], 0)                            // flags
	put16(eh[52:], elfHeaderSize)                // ehsize
	put16(eh[54:], progHeaderSize)               // phentsize
	put16(eh[56:], uint16(phnum))
	put16(eh[58:], 0) // shentsize
	put16(eh[60:], 0) // shnum
	put16(eh[62:], 0) // shstrndx
	out = append(out, eh...)

	ph := make([]byte, progHeaderSize)
	put32(ph[0:], 1)                 // type: LOAD
	put32(ph[4:], 5)                 // flags: R+X
	put64(ph[8:], 0)                 // offset
	put64(ph[16:], BaseAddr)         // vaddr
	put64(ph[24:], BaseAddr)         // paddr
	put64(ph[32:], uint64(bodyEnd))  // filesz
	put64(ph[40:], uint64(bodyEnd))  // memsz
	put64(ph[48:], 0x1000)           // align
	out = append(out, ph...)

	if len(data) > 0 {
		dp := make([]byte, progHeaderSize)
		put32(dp[0:], 1)                           // type: LOAD
		put32(dp[4:], 6)                           // flags: R+W (the arena)
		put64(dp[8:], uint64(dataOff))             // offset
		put64(dp[16:], uint64(BaseAddr+dataOff))  // vaddr
		put64(dp[24:], uint64(BaseAddr+dataOff))  // paddr
		put64(dp[32:], uint64(len(data)))          // filesz
		put64(dp[40:], uint64(len(data)))          // memsz
		put64(dp[48:], 0x1000)                     // align
		out = append(out, dp...)
	}

	out = append(out, text...)
	out = append(out, rodata...)
	if len(data) > 0 {
		// Zero-fill the alignment gap between .rodata and the arena.
		out = append(out, make([]byte, dataOff-len(out))...)
		out = append(out, data...)
	}
	return out, nil
}

func put16(b []byte, v uint16) {
	b[0] = byte(v)
	b[1] = byte(v >> 8)
}

func put32(b []byte, v uint32) {
	b[0] = byte(v)
	b[1] = byte(v >> 8)
	b[2] = byte(v >> 16)
	b[3] = byte(v >> 24)
}

func put64(b []byte, v uint64) {
	put32(b, uint32(v))
	put32(b[4:], uint32(v>>32))
}

// Parse checks the structural shape of a linked image (magic, class,
// machine, one or two LOAD headers, entry inside .text). It is the assertion
// helper for tests on hosts that cannot execute ELF.
//
// Phase 150B: a program with a heap carries a second, R+W PT_LOAD for the
// arena, so phnum is 1 or 2 and .text starts after both headers. A heapless
// program still has exactly one, so its layout and returned textOff are
// unchanged — which is what keeps the increment-149 byte-identity pins honest.
func Parse(img []byte) (entry uint64, textOff int, err error) {
	if len(img) < elfHeaderSize+progHeaderSize {
		return 0, 0, fmt.Errorf("image too small (%d bytes)", len(img))
	}
	if !bytes.Equal(img[0:4], []byte{0x7F, 'E', 'L', 'F'}) {
		return 0, 0, fmt.Errorf("bad ELF magic")
	}
	if img[4] != 2 || img[5] != 1 {
		return 0, 0, fmt.Errorf("not a 64-bit little-endian object")
	}
	if get16(img[16:]) != 2 {
		return 0, 0, fmt.Errorf("not an executable (type != EXEC)")
	}
	if get16(img[18:]) != 62 {
		return 0, 0, fmt.Errorf("not x86-64 (machine != 62)")
	}
	phnum := int(get16(img[56:]))
	if phnum != 1 && phnum != 2 {
		return 0, 0, fmt.Errorf("want 1 or 2 program headers (got %d)", phnum)
	}
	if get32(img[elfHeaderSize:]) != 1 {
		return 0, 0, fmt.Errorf("the program header is not LOAD")
	}
	if phnum == 2 {
		// The second header is the heap arena: LOAD and R+W. A R+X arena
		// would fault on the first bump-cursor store.
		second := elfHeaderSize + progHeaderSize
		if get32(img[second:]) != 1 {
			return 0, 0, fmt.Errorf("the second program header is not LOAD")
		}
		if fl := get32(img[second+4:]); fl != 6 {
			return 0, 0, fmt.Errorf("heap segment flags are %#x, want R+W (6)", fl)
		}
	}
	entry = get64(img[24:])
	textOff = elfHeaderSize + progHeaderSize*phnum
	if entry < BaseAddr+uint64(textOff) || entry >= BaseAddr+uint64(len(img)) {
		return 0, 0, fmt.Errorf("entry %#x outside image", entry)
	}
	return entry, textOff, nil
}

func get16(b []byte) uint16 { return uint16(b[0]) | uint16(b[1])<<8 }
func get32(b []byte) uint32 {
	return uint32(b[0]) | uint32(b[1])<<8 | uint32(b[2])<<16 | uint32(b[3])<<24
}
func get64(b []byte) uint64 { return uint64(get32(b)) | uint64(get32(b[4:]))<<32 }
