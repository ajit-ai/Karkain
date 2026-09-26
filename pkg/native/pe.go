package native

import (
	"bytes"
	"fmt"
)

// PE (Portable Executable) writer, Phase 149.
//
// Layout (x86-64 Windows, kernel32-hosted):
//
//	[DOS 64][pad to 0x80][PE sig 4][COFF 20][Optional PE32+ 240]
//	[2 section headers 80][pad to 0x200][.text code+rodata][pad][.idata]
//
// Two sections: `.text` (R-X) holds code+rodata, `.idata` (R/W) holds
// the import table for kernel32.dll (ExitProcess, GetStdHandle,
// WriteFile). .idata MUST be writable: the loader resolves the import
// addresses by writing them into the IAT slots at load time — a
// read-only .idata fails the load (Phase-149 diagnosis via objdump +
// ERROR_BAD_EXE_FORMAT). Windows has no stable user-mode syscall ABI, so exit and
// output go through these imports; user functions keep the System V
// convention internally and speak Win64 only at the three boundary
// sequences in program.go. IAT slots resolve through the existing
// imm64-patch machinery (`movabs` + `call rax` — no RIP-relative
// encoding needed). TimeDateStamp is 0 for determinism.

// PEBaseAddr is the conventional Windows x64 image base.
const PEBaseAddr = 0x140000000

const (
	peFileAlign  = 0x200
	peSectAlign  = 0x1000
	peSigOff     = 0x80
	peCoffSize   = 20
	peOptSize    = 240
	peSectSize   = 40
	peTextRVA    = 0x1000
	peIdataRVA   = 0x2000
	peRelocRVA   = 0x3000
	peNumImports = 3
	peNumSections = 3
)

// peTextOff locates .text: headers (0x80+4+20+240+80 = 472) padded to
// the 0x200 file alignment.
const peTextOff = 0x200

// IAT slot indices for the kernel32 imports (program.go references these).
const (
	IATExitProcess  = 0
	IATGetStdHandle = 1
	IATWriteFile    = 2
)

var peImportNames = [peNumImports]string{"ExitProcess", "GetStdHandle", "WriteFile"}

func peAlignUp(v, align int) int {
	return (v + align - 1) &^ (align - 1)
}

// LinkPE combines .text (emitter output) and .rodata into a complete PE
// image, resolving rodata patches against the .text base and IAT patches
// (b.ipatches) against the .idata import slots. textOffset must equal
// peTextOff; entryOffset is the _start file offset (same convention as
// Link: textOff + label).
//
// ASLR note (Phase-149 diagnosis): the host loader rebases the image
// despite the absent relocation table (live-process proof: image base
// 0x7FF6… instead of the linked 0x140000000, every absolute movabs
// stale, instant AV). A .reloc section is therefore emitted from the
// movabs-site lists (rodata + IAT patches): type-10 (DIR64) entries per
// 4KB page, so DYNAMIC_BASE stays honest on any host or policy.
func LinkPE(b *Builder, text, rodata []byte, textOffset, entryOffset int) ([]byte, error) {
	if textOffset != peTextOff {
		return nil, fmt.Errorf("native backend: unsupported pe text offset %d", textOffset)
	}
	codeLen := len(text) + len(rodata)
	textRaw := peAlignUp(codeLen, peFileAlign)
	idata := buildIdata()
	idataFileOff := peTextOff + textRaw
	// Phase 150B: the heap arena rides inside .idata, which is already
	// R/W (0xC0000040) and proven writable at load time. No fourth section,
	// no extra import, and the import/IAT data directories keep pointing at
	// the import structures only — the loader never sees the arena.
	arenaLen := 0
	if b.heap != nil {
		arenaLen = len(b.heap)
	}
	idataTotal := len(idata) + arenaLen
	idataRaw := peAlignUp(idataTotal, peFileAlign)
	reloc, relocBlocks := buildReloc(b)
	relocFileOff := idataFileOff + idataRaw
	relocRaw := peAlignUp(len(reloc), peFileAlign)
	sizeOfImage := peAlignUp(peRelocRVA+len(reloc), peSectAlign)

	out := make([]byte, 0, idataFileOff+idataRaw)

	dos := make([]byte, 64)
	dos[0], dos[1] = 'M', 'Z'
	put32(dos[0x3C:], peSigOff)
	out = append(out, dos...)
	out = append(out, make([]byte, peSigOff-64)...)
	out = append(out, 'P', 'E', 0, 0)

	coff := make([]byte, peCoffSize)
	put16(coff[0:], 0x8664) // x86-64
	put16(coff[2:], peNumSections)
	put32(coff[4:], 0)      // TimeDateStamp: 0 (deterministic)
	put16(coff[16:], peOptSize)
	put16(coff[18:], 0x22) // EXECUTABLE_IMAGE | LARGE_ADDRESS_AWARE
	out = append(out, coff...)

	opt := make([]byte, peOptSize)
	put16(opt[0:], 0x20B) // PE32+
	put32(opt[4:], uint32(peAlignUp(codeLen, peFileAlign)))
	put32(opt[8:], uint32(idataRaw+relocRaw))
	put32(opt[16:], uint32(peTextRVA+(entryOffset-textOffset))) // entry RVA
	put32(opt[20:], peTextRVA)                                  // BaseOfCode
	put64(opt[24:], PEBaseAddr)
	put32(opt[32:], peSectAlign)
	put32(opt[36:], peFileAlign)
	put32(opt[56:], uint32(sizeOfImage))
	put32(opt[60:], uint32(peTextOff)) // SizeOfHeaders
	put16(opt[68:], 3)                 // CONSOLE subsystem
	// Phase-149 diagnosis: MajorSubsystemVersion (opt+48) of 0 fails the
	// load with ERROR_BAD_EXE_FORMAT on Windows 11 (bisected: OS version
	// 0 is fine, subsystem 0 is not). 6.0 is the honest minimum.
	put16(opt[48:], 6)     // MajorSubsystemVersion
	put16(opt[70:], 0x160) // HIGH_ENTROPY_VA|DYNAMIC_BASE|NX_COMPAT
	put64(opt[72:], 0x100000)          // stack reserve
	put64(opt[80:], 0x1000)            // stack commit
	put64(opt[88:], 0x100000)          // heap reserve
	put64(opt[96:], 0x1000)            // heap commit
	put32(opt[108:], 16) // NumberOfRvaAndSizes
	put32(opt[120:], uint32(peIdataRVA))
	put32(opt[124:], 40) // import table (entry 1): RVA + IDT size
	put32(opt[152:], uint32(peRelocRVA))
	put32(opt[156:], uint32(relocBlocks)) // base relocation table (entry 5)
	// Phase-149 diagnosis: some loader paths consult the IAT directory
	// (entry 12); gcc sets it, so we do too (IAT RVA + slots size).
	put32(opt[208:], uint32(peIdataRVA+peIATOff))
	put32(opt[212:], uint32(peNumImports*8))
	out = append(out, opt...)

	sect := func(name string, vsize, rva, raw, off int, chars uint32) {
		s := make([]byte, peSectSize)
		copy(s, name)
		put32(s[8:], uint32(vsize))
		put32(s[12:], uint32(rva))
		put32(s[16:], uint32(raw))
		put32(s[20:], uint32(off))
		put32(s[36:], chars)
		out = append(out, s...)
	}
	sect(".text", codeLen, peTextRVA, textRaw, peTextOff, 0x60000020)
	sect(".idata", idataTotal, peIdataRVA, idataRaw, idataFileOff, 0xC0000040)
	sect(".reloc", relocBlocks, peRelocRVA, relocRaw, relocFileOff, 0x42000040)
	if len(out) > peTextOff {
		return nil, fmt.Errorf("native backend: headers overflow .text start (%d > %d)", len(out), peTextOff)
	}
	out = append(out, make([]byte, peTextOff-len(out))...)

	textBase := uint64(PEBaseAddr + peTextRVA)
	roBase := textBase + uint64(len(text))
	img := append(out, text...)
	img = append(img, rodata...)
	img = append(img, make([]byte, textRaw-codeLen)...)
	for _, p := range b.patches {
		v := roBase + p.roOff
		img[peTextOff+p.pos] = byte(v)
		img[peTextOff+p.pos+1] = byte(v >> 8)
		img[peTextOff+p.pos+2] = byte(v >> 16)
		img[peTextOff+p.pos+3] = byte(v >> 24)
		img[peTextOff+p.pos+4] = byte(v >> 32)
		img[peTextOff+p.pos+5] = byte(v >> 40)
		img[peTextOff+p.pos+6] = byte(v >> 48)
		img[peTextOff+p.pos+7] = byte(v >> 56)
	}
	idataBase := uint64(PEBaseAddr + peIdataRVA)
	for _, p := range b.ipatches {
		v := idataBase + uint64(peIATOff+p.index*8)
		img[peTextOff+p.pos] = byte(v)
		img[peTextOff+p.pos+1] = byte(v >> 8)
		img[peTextOff+p.pos+2] = byte(v >> 16)
		img[peTextOff+p.pos+3] = byte(v >> 24)
		img[peTextOff+p.pos+4] = byte(v >> 32)
		img[peTextOff+p.pos+5] = byte(v >> 40)
		img[peTextOff+p.pos+6] = byte(v >> 48)
		img[peTextOff+p.pos+7] = byte(v >> 56)
	}
	// Phase 149 bootstrap publishes resolve to the same IAT slots.
	for _, p := range b.apatches {
		v := idataBase + uint64(peIATOff+p.index*8)
		img[peTextOff+p.pos] = byte(v)
		img[peTextOff+p.pos+1] = byte(v >> 8)
		img[peTextOff+p.pos+2] = byte(v >> 16)
		img[peTextOff+p.pos+3] = byte(v >> 24)
		img[peTextOff+p.pos+4] = byte(v >> 32)
		img[peTextOff+p.pos+5] = byte(v >> 40)
		img[peTextOff+p.pos+6] = byte(v >> 48)
		img[peTextOff+p.pos+7] = byte(v >> 56)
	}
	// Phase 150B: arena addresses are position-dependent exactly like
	// rodata pointers, so they join the DIR64 fixup list.
	for _, p := range b.hpatches {
		v := idataBase + uint64(len(idata)) + p.roOff
		img[peTextOff+p.pos] = byte(v)
		img[peTextOff+p.pos+1] = byte(v >> 8)
		img[peTextOff+p.pos+2] = byte(v >> 16)
		img[peTextOff+p.pos+3] = byte(v >> 24)
		img[peTextOff+p.pos+4] = byte(v >> 32)
		img[peTextOff+p.pos+5] = byte(v >> 40)
		img[peTextOff+p.pos+6] = byte(v >> 48)
		img[peTextOff+p.pos+7] = byte(v >> 56)
	}
	img = append(img, idata...)
	if arenaLen > 0 {
		img = append(img, b.heap...)
	}
	img = append(img, make([]byte, idataRaw-idataTotal)...)
	img = append(img, reloc...)
	img = append(img, make([]byte, relocRaw-len(reloc))...)
	return img, nil
}

// .idata layout (offsets relative to the section start): IDT 40B @0
// (one entry + null), ILT 32B @40 (3 RVAs + null), IAT 32B @72,
// Hint/Name blobs @104 (16, 16 and 12 bytes), "kernel32.dll" @148.
const (
	peIDTOff = 0
	peILTOff = 40
	peIATOff = 72
	peHNOff  = 104
)

func buildIdata() []byte {
	hnSizes := [peNumImports]int{16, 16, 12}
	hnOff := peHNOff
	hnRVAs := [peNumImports]int{}
	for i := range peImportNames {
		hnRVAs[i] = peIdataRVA + hnOff
		hnOff += hnSizes[i]
	}
	dllOff := hnOff
	idata := make([]byte, dllOff+13)

	put32(idata[peIDTOff+0:], uint32(peIdataRVA+peILTOff))  // OriginalFirstThunk
	put32(idata[peIDTOff+12:], uint32(peIdataRVA+dllOff))   // Name
	put32(idata[peIDTOff+16:], uint32(peIdataRVA+peIATOff)) // FirstThunk

	for i := 0; i < peNumImports; i++ {
		put64(idata[peILTOff+i*8:], uint64(hnRVAs[i]))
		put64(idata[peIATOff+i*8:], uint64(hnRVAs[i])) // loader overwrites with addresses
	}
	off := peHNOff
	for i := range peImportNames {
		put16(idata[off:], 0) // hint
		copy(idata[off+2:], peImportNames[i])
		off += hnSizes[i]
	}
	copy(idata[dllOff:], "kernel32.dll\x00")
	return idata
}

// buildReloc emits the .reloc section body: one DIR64 block per 4KB page
// covering every absolute movabs site (rodata + IAT patches live at
// emitter offsets, i.e. offsets into .text). It returns the file-padded
// bytes and the blocks size for the data directory. The loader adds the
// image delta to each listed address, which is what makes DYNAMIC_BASE
// honest on hosts that rebase despite the absent... now present table.
func buildReloc(b *Builder) (padded []byte, blocksSize int) {
	var rvas []int
	for _, p := range b.patches {
		rvas = append(rvas, peTextRVA+p.pos)
	}
	for _, p := range b.ipatches {
		rvas = append(rvas, peTextRVA+p.pos)
	}
	// Phase 149 bootstrap publishes: absolute store addresses are
	// position-dependent too, so they join the fixup list.
	for _, p := range b.apatches {
		rvas = append(rvas, peTextRVA+p.pos)
	}
	// Phase 150B: arena addresses, for the same reason.
	for _, p := range b.hpatches {
		rvas = append(rvas, peTextRVA+p.pos)
	}
	sortInts(rvas)
	var blocks []byte
	for len(rvas) > 0 {
		page := rvas[0] &^ 0xFFF
		var entries []byte
		var rest []int
		for _, r := range rvas {
			if r&^0xFFF == page {
				v := uint16(10<<12 | (r & 0xFFF))
				entries = append(entries, byte(v), byte(v>>8))
			} else {
				rest = append(rest, r)
			}
		}
		rvas = rest
		// Blocks pad to a DWORD boundary (type-0 ABSOLUTE entry).
		if len(entries)%4 != 0 {
			entries = append(entries, 0, 0)
		}
		hdr := make([]byte, 8)
		put32(hdr[0:], uint32(page))
		put32(hdr[4:], uint32(8+len(entries)))
		blocks = append(blocks, hdr...)
		blocks = append(blocks, entries...)
	}
	padded = append(blocks, make([]byte, peAlignUp(len(blocks), peFileAlign)-len(blocks))...)
	return padded, len(blocks)
}

func sortInts(v []int) {
	for i := 1; i < len(v); i++ {
		for j := i; j > 0 && v[j] < v[j-1]; j-- {
			v[j], v[j-1] = v[j-1], v[j]
		}
	}
}

// ParsePE checks the structural shape of a linked image (MZ magic,
// PE signature, x86-64 COFF with two sections, entry RVA inside .text,
// import directory inside .idata). Assertion helper for tests on hosts
// that cannot execute PE (and the structural pin where they can).
func ParsePE(img []byte) (entry uint64, textOff int, err error) {
	if len(img) < peTextOff {
		return 0, 0, fmt.Errorf("image too small (%d bytes)", len(img))
	}
	if !bytes.Equal(img[0:2], []byte{'M', 'Z'}) {
		return 0, 0, fmt.Errorf("bad DOS magic")
	}
	peOff := int(get32(img[0x3C:]))
	if peOff+6 > len(img) || !bytes.Equal(img[peOff:peOff+4], []byte{'P', 'E', 0, 0}) {
		return 0, 0, fmt.Errorf("bad PE signature")
	}
	coff := peOff + 4
	if get16(img[coff:]) != 0x8664 {
		return 0, 0, fmt.Errorf("not x86-64 (machine)")
	}
	if get16(img[coff+2:]) != peNumSections {
		return 0, 0, fmt.Errorf("want %d sections", peNumSections)
	}
	opt := coff + peCoffSize
	if get16(img[opt:]) != 0x20B {
		return 0, 0, fmt.Errorf("not PE32+")
	}
	entryRVA := uint64(get32(img[opt+16:]))
	importRVA := uint64(get32(img[opt+120:]))
	importSize := get32(img[opt+124:])
	relocRVA := uint64(get32(img[opt+152:]))
	relocSize := get32(img[opt+156:])
	secs := opt + int(get16(img[coff+16:]))
	textRVA, textRaw, textFile := uint64(0), 0, 0
	idataRVA, idataRaw, idataFile := uint64(0), 0, 0
	relocSecRVA, relocSecRaw, relocSecFile := uint64(0), 0, 0
	for i := 0; i < peNumSections; i++ {
		s := secs + i*peSectSize
		if s+peSectSize > len(img) {
			return 0, 0, fmt.Errorf("section %d out of range", i)
		}
		rva := uint64(get32(img[s+12:]))
		switch i {
		case 0:
			textRVA, textRaw, textFile = rva, int(get32(img[s+16:])), int(get32(img[s+20:]))
		case 1:
			idataRVA, idataRaw, idataFile = rva, int(get32(img[s+16:])), int(get32(img[s+20:]))
		default:
			relocSecRVA, relocSecRaw, relocSecFile = rva, int(get32(img[s+16:])), int(get32(img[s+20:]))
		}
	}
	if textRVA != peTextRVA || textFile != peTextOff {
		return 0, 0, fmt.Errorf("unexpected .text layout")
	}
	if idataRVA != peIdataRVA || textFile+textRaw > idataFile || idataFile+idataRaw > relocSecFile {
		return 0, 0, fmt.Errorf("unexpected .idata layout")
	}
	if entryRVA < textRVA {
		return 0, 0, fmt.Errorf("entry %#x outside .text", entryRVA)
	}
	if importRVA != idataRVA || importSize != 40 {
		return 0, 0, fmt.Errorf("bad import directory")
	}
	if relocRVA != peRelocRVA || relocSize == 0 || relocSecRVA != peRelocRVA || relocSecFile+relocSecRaw > len(img) {
		return 0, 0, fmt.Errorf("bad relocation directory")
	}
	// Walk the relocation blocks: sane pages inside the image, type-10
	// (DIR64) entries only, plus zero padding.
	off := relocSecFile
	end := off + int(relocSize)
	imgSize := int(get32(img[opt+56:]))
	for off < end {
		if off+8 > len(img) {
			return 0, 0, fmt.Errorf("reloc block out of range")
		}
		page := get32(img[off:])
		size := int(get32(img[off+4:]))
		if size < 8 || size%4 != 0 || off+size > len(img) {
			return 0, 0, fmt.Errorf("bad reloc block size %d", size)
		}
		if page < peTextRVA || page >= uint32(imgSize) {
			return 0, 0, fmt.Errorf("reloc page %#x outside image", page)
		}
		for e := off + 8; e < off+size; e += 2 {
			typ := get16(img[e:]) >> 12
			if typ != 0 && typ != 10 {
				return 0, 0, fmt.Errorf("reloc entry type %d (want 0 or 10)", typ)
			}
		}
		off += size
	}
	entryOff := textFile + int(entryRVA-textRVA)
	if entryOff >= len(img) {
		return 0, 0, fmt.Errorf("entry outside image")
	}
	return uint64(PEBaseAddr) + entryRVA, textFile, nil
}
