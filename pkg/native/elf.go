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

// Link combines .text and .rodata into a complete executable image.
// textStart is the file offset of .text (always headerSize+phSize here);
// entryLabel is resolved by the caller to a file offset before calling.
func Link(text, rodata []byte, textOffset, entryOffset int) ([]byte, error) {
	if textOffset != elfHeaderSize+progHeaderSize {
		return nil, fmt.Errorf("native backend: unsupported text offset %d", textOffset)
	}
	total := textOffset + len(text) + len(rodata)
	out := make([]byte, 0, total)

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
	put16(eh[56:], 1)                            // phnum
	put16(eh[58:], 0)                            // shentsize
	put16(eh[60:], 0)                            // shnum
	put16(eh[62:], 0)                            // shstrndx
	out = append(out, eh...)

	ph := make([]byte, progHeaderSize)
	put32(ph[0:], 1)              // type: LOAD
	put32(ph[4:], 5)              // flags: R+X
	put64(ph[8:], 0)              // offset
	put64(ph[16:], BaseAddr)      // vaddr
	put64(ph[24:], BaseAddr)      // paddr
	put64(ph[32:], uint64(total)) // filesz
	put64(ph[40:], uint64(total)) // memsz
	put64(ph[48:], 0x1000)        // align
	out = append(out, ph...)

	out = append(out, text...)
	out = append(out, rodata...)
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
// machine, exactly one LOAD, entry inside .text). It is the assertion
// helper for tests on hosts that cannot execute ELF.
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
	if get16(img[56:]) != 1 {
		return 0, 0, fmt.Errorf("want exactly one program header")
	}
	if get32(img[elfHeaderSize:]) != 1 {
		return 0, 0, fmt.Errorf("the program header is not LOAD")
	}
	entry = get64(img[24:])
	textOff = elfHeaderSize + progHeaderSize
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
