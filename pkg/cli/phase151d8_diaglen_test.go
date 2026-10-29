package cli

import (
	"encoding/binary"
	"testing"
)

// Phase D8: the array-bounds diagnostic length must equal the real length of
// the string handed to the reporter.
//
// THE DEFECT THIS COVERS. natArrayIndex emitted the literal 25 into RSI for the
// length of "array index out of range", which is 24 characters. The reporter
// therefore received a length one byte greater than the string it was given.
// The oracle derives the same value with len("array index out of range")
// (pkg/native/program.go:6324), so this was a kcc-only divergence with no
// behavioural test able to see it: the diagnostic is written to a stream the
// Phase-152-B0 GetStdHandle limitation leaves empty for PE images on this host,
// and the D5 size-delta gate counts bytes rather than reading immediates.
//
// WHY THIS IS NOT A WHOLE-FILE SNAPSHOT. A byte snapshot would pin every
// incidental byte and would keep failing for unrelated reasons, which is how
// gates get deleted. This test locates ONE instruction by decoding it, and
// asserts on its immediate operand.
//
// HOW THE INSTRUCTION IS FOUND. The call sequence emitted for the failure arm
// is:
//
//	movabs $0x400408,%rdi ; kind ptr      (10 bytes)
//	mov    $0x18,%esi     ; kind LENGTH   (5 bytes)  <- the operand under test
//	movabs $0x40040f,%rdx ; file ptr      (10 bytes)
//	mov    $0xa,%ecx      ; file length   (5 bytes)
//	mov    $0x2,%r8d      ; line          (6 bytes)
//	call   karkain_runtime_error
//
// Locating a bare `BE` anywhere in the file is NOT sufficient, and the first
// draft of this test proved it: `0xBE` occurs inside a `movabs` immediate (a
// rodata address), so a naive scan decoded 0x00400408 and reported a length of
// 4195336. The scan is therefore anchored to the FULL argument sequence -- the
// `movabs` kind pointer is 10 bytes (49 BF .. .. .. .. .. .. .. ..), then
// `BE imm32` at a fixed +10 offset. A `movabs` whose operand lies inside
// .rodata is 10 bytes ending in a 5-byte displacement, so requiring the
// following 5 bytes to decode to a small length distinguishes the two.
//
// The test still reports the OBSERVED immediate, so a mutation to 25 produces a
// diagnosis ("24 expected, 25 observed") rather than a bare "not found".

const (
	d8KindString = "array index out of range"
	// d8FileLength is the length of the interned source basename, which lands in
	// the NEXT immediate (RCX). Stating it makes the sequence recognisable and
	// confirms the scan found the real call site rather than an incidental match.
	d8FileLength = 10 // len("probe.kark")
)

func TestPhaseD8_ArrayBoundsDiagnosticLengthMatchesString(t *testing.T) {
	karkain := phase130Karkain(t)
	src := "func main() {\n    let a = [3, 5, 7]\n    print(a[1])\n}\n"

	res := nativeBuildWithEngine(t, karkain, src, NativeLinuxTarget, "kcc", "d8len.elf")
	if res.ExitCode != 0 || len(res.Image) == 0 {
		t.Fatalf("kcc produced no image (exit %d):\n%s", res.ExitCode, res.Output)
	}
	img := res.Image
	want := uint32(len(d8KindString))

	got, ok := d8FindBoundsLengthImm(img)
	if !ok {
		t.Fatalf("the bounds-diagnostic argument sequence was not found in the kcc "+
			"image (expected a movabs of the kind pointer into RDI followed by a "+
			"mov of its length into ESI). Expected length %d for %q. "+
			"The image is %d bytes.", want, d8KindString, len(img))
	}

	if got != want {
		t.Errorf("the bounds-diagnostic length immediate is %d, want %d.\n"+
			"  emitted encoding: % X\n"+
			"  wanted encoding: % X\n"+
			"  The string handed to the reporter is %q, which is %d bytes. RSI carries "+
			"that length, so a mismatch makes the reporter read %d bytes for a "+
			"%d-byte string.\n"+
			"  (Phase D8: this was the literal 25 for a 24-character string.)",
			got, want, d8MovEsiBytes(got), d8MovEsiBytes(want), d8KindString, want,
			got, want)
	}
}

// d8FindBoundsLengthImm locates the RSI length operand of the bounds reporter.
//
// It anchors on the 10-byte `movabs $kind,%rdi` (49 BF + 8 immediate bytes) and
// reads the `BE`+imm32 that must follow at +10, then requires the argument after
// it to be the file length -- which is what distinguishes the real call site from
// a `0xBE` byte occurring inside some other immediate.
func d8FindBoundsLengthImm(img []byte) (uint32, bool) {
	// Anchor on the SECOND operand and walk forward, which is both simpler and
	// immune to the rodata-address false positive.
	//
	// The sequence, verified by disassembly of the emitted image:
	//
	//	48 bf <8>   movabs $file,%rdi     10 bytes
	//	be   <4>    mov    $len,%esi       5 bytes   <- the operand under test
	//	48 ba <8>   movabs $source,%rdx   10 bytes
	//	b9   <4>    mov    $filelen,%ecx  5 bytes
	//	41 b8 <4>   mov    $line,%r8d     6 bytes
	//	e8   <4>    call                   5 bytes
	//
	// NOTE the kind pointer is emitted with `48 BF`, not `49 BF`: the oracle's
	// rodataRef uses REX.W|B (49 BF) while natRodataRefOff emits 48 BF. A first
	// draft of this test searched for 49 and therefore matched nothing -- the
	// test was wrong about the emitter, not the emitter being wrong.
	for i := 0; i+41 <= len(img); i++ {
		if img[i] != 0x48 || img[i+1] != 0xBF {
			continue
		}
		kindPtr := binary.LittleEndian.Uint64(img[i+2 : i+10])
		if kindPtr < 0x400000 || kindPtr > 0x600000 {
			continue
		}
		j := i + 10
		if img[j] != 0xBE {
			continue
		}
		length := binary.LittleEndian.Uint32(img[j+1 : j+5])
		if length > 4096 {
			continue
		}
		k := j + 5
		if img[k] != 0x48 || img[k+1] != 0xBA {
			continue
		}
		if img[k+10] != 0xB9 {
			continue
		}
		fileLen := binary.LittleEndian.Uint32(img[k+11 : k+15])
		if fileLen != d8FileLength {
			continue
		}
		if img[k+15] != 0x41 || img[k+16] != 0xB8 {
			continue
		}
		if img[k+21] != 0xE8 {
			continue
		}
		return length, true
	}
	return 0, false
}

// d8MovEsiBytes renders an immediate as it appears in the image.
func d8MovEsiBytes(v uint32) []byte {
	return []byte{0xBE, byte(v), byte(v >> 8), byte(v >> 16), byte(v >> 24)}
}

// d8EsiImmBytes renders an observed immediate the way it appears in the image,
// so a failure message shows the bytes rather than a bare number.
func d8EsiImmBytes(v uint32) []byte {
	return []byte{0xBE, byte(v), byte(v >> 8), byte(v >> 16), byte(v >> 24)}
}