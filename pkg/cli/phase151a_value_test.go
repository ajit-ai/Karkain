package cli

// Phase 151A Step 1 — the native value/frame foundation in kcc.
//
// Contract: the self-hosted engine's frame layout and local-slot addressing
// must agree with the Go oracle (pkg/native/program.go), not merely behave
// similarly, because increment 151's premise is byte-identical images.
//
// THE GATE HAS INDEPENDENT LAYERS, following the 151B/151C/151C2/151C3
// discipline. A single-layer differential is not enough here, for two
// specific reasons:
//
//  1. A frame layout that is wrong in the SAME way on both sides still
//     produces matching bytes. So the layout NUMBERS are compared too, not
//     only the bytes they produced, and those numbers are derived in this
//     file from the layout rules rather than read out of either engine.
//
//  2. The oracle's program bytes sit inside a full image that also contains
//     its helper routines and its entry tail, neither of which exists yet on
//     the kcc side. So the oracle's contribution is its .text, and kcc's bytes
//     must appear there verbatim, exactly once, at the right length.
//
// The differential reconstructs the same bytes by calling the real
// pkg/native.Emitter, which is what makes this a comparison of two
// implementations rather than two invocations of one.

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"testing"

	"karkain/pkg/lexer"
	"karkain/pkg/native"
	"karkain/pkg/parser"
)

// ---- independent reference values -------------------------------------
//
// Derived here from the layout rules in pkg/native/program.go, deliberately
// WITHOUT calling the oracle, so a mistake the oracle and kcc share cannot
// cancel out.

const (
	refArgSpillBytes = 96                     // 6 args x (ptr+len), always reserved
	refMaxBinDepth   = 64                     // sizes binTemp: 8 bytes per depth
	refBinTempBytes  = 8 * refMaxBinDepth     //
	refStrTempBytes  = 8 * 5 * refMaxBinDepth // five units per depth
	refMapStageBytes = 8
)

// refFrame reproduces the oracle's layout arithmetic for the inputs Step 1
// uses: locals, then binTemp, then the two conditional regions, then extras,
// then the Windows-only 16-byte rounding, then the fixed spill.
func refFrame(localBytes, recArgBytes int, usesStr, usesMap bool, maxExtra int, isWindows bool) (frame, strTemp, mapStage int) {
	next := localBytes + recArgBytes + refBinTempBytes
	if usesStr {
		strTemp = next
		next += refStrTempBytes
	}
	if usesMap {
		mapStage = next
		next += refMapStageBytes
	}
	next += maxExtra * 8
	if isWindows && next%16 != 0 {
		next += 8
	}
	return next + refArgSpillBytes, strTemp, mapStage
}

// refFrameOf is the frame size for a reference program with nLocals int
// locals and nothing else.
func refFrameOf(nLocals int) int {
	frame, _, _ := refFrame(8*nLocals, 0, false, false, 0, false)
	return frame
}

// refMainBytes builds the expected machine code for a reference program from
// the real Go Emitter, mirroring natValueMain. kcc's layout feeds kcc's
// encoder and this feeds the oracle's; the two byte streams must match.
func refMainBytes(t *testing.T, nLocals int) []byte {
	t.Helper()
	frame := refFrameOf(nLocals)
	e := native.NewEmitter()
	e.SubRegImm32(native.RSP, int32(frame))
	e.MovRegImm64(native.RAX, 42)
	e.StoreStack(native.RAX, 0)
	if nLocals == 2 {
		e.LoadStack(native.RAX, 0)
		e.StoreStack(native.RAX, 8) // slot 1 = 8 * 1 * kindUnits(int)
	}
	e.XorRegReg(native.RAX)
	e.AddRegImm32(native.RSP, int32(frame))
	e.Ret()
	return e.Bytes()
}

// le32Hex renders v as the four little-endian bytes an x86 imm32/disp32
// occupies.
//
// This is exactly the mistake the first draft of this file made: it formatted
// the value as a hex STRING and zero-padded it on the left, producing
// 00000268 where the instruction stream needs 68020000. An immediate is not a
// number written out, it is four bytes in memory order, and a stated-bytes
// layer that renders it the other way round is worse than no layer at all.
func le32Hex(v int) string {
	out := ""
	for i := 0; i < 4; i++ {
		b := (v >> (8 * i)) & 0xFF
		out += fmt.Sprintf("%02x", b)
	}
	return out
}

// statedMainHex writes the expected bytes out longhand, from the Intel SDM
// and the oracle's own source, with only the frame immediate substituted.
// This is the layer that catches a mistake BOTH engines share: a golden typed
// once and a differential can both agree on a wrong answer, but a byte
// sequence spelled out from the specification cannot drift with the code.
//
//	sub rsp, imm32      48 81 ec <imm32>        (REX.W, 81, modrm 11 101 100)
//	mov rax, 42         48 b8 2a 00 00 00 00 00 00 00   (REX.W, B8+rd, imm64)
//	mov [rsp], rax      48 89 04 24            (REX.W, 89, modrm 00 000 100, SIB 24)
//	mov rax, [rsp]      48 8b 04 24            (only in the two-local case)
//	mov [rsp+8], rax    48 89 44 24 08         (REX.W, 89, modrm 01 000 100, SIB 24, disp8=8)
//	xor rax, rax        48 31 c0               (REX.W, 31, modrm 11 000 000)
//	add rsp, imm32      48 81 c4 <imm32>       (REX.W, 81, modrm 11 000 100)
//	ret                 c3
func statedMainHex(nLocals int) string {
	imm := le32Hex(refFrameOf(nLocals))
	s := "4881ec" + imm // sub rsp, frame
	s += "48b82a00000000000000"
	s += "48890424"
	if nLocals == 2 {
		s += "488b0424"
		s += "4889442408"
	}
	s += "4831c0"
	s += "4881c4" + imm // add rsp, frame
	s += "c3"
	return s
}

// oracleText compiles a reference program with the Go oracle and returns its
// .text, so kcc's bytes can be located in real oracle output.
func oracleText(t *testing.T, src string) []byte {
	t.Helper()
	l := lexer.New(src)
	p := parser.New(l)
	prog := p.ParseProgram()
	if len(p.Errors) > 0 {
		t.Fatalf("oracle source parse: %v", p.Errors)
	}
	img, err := native.CompileProgramForOS(prog, native.OSLinux)
	if err != nil {
		t.Fatalf("oracle compile: %v", err)
	}
	_, textOff, perr := native.Parse(img)
	if perr != nil {
		t.Fatalf("oracle image parse: %v", perr)
	}
	return img[textOff:]
}

func countOccurrences(hay, needle []byte) int {
	if len(needle) == 0 {
		return -1
	}
	n := 0
	for i := 0; i+len(needle) <= len(hay); i++ {
		if bytes.Equal(hay[i:i+len(needle)], needle) {
			n++
			i += len(needle) - 1
		}
	}
	return n
}

func runKCCValueLines(t *testing.T, karkain, sub string) []string {
	t.Helper()
	cmd := exec.Command(karkain, sub)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("kcc %s failed: %v\n%s", sub, err, out)
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

// ---- gates ------------------------------------------------------------

// TestPhase151A_ValueFrameByteIdentity is the core gate: kcc's machine code
// for the two reference programs must equal the oracle's, byte for byte.
func TestPhase151A_ValueFrameByteIdentity(t *testing.T) {
	karkain := phase130Karkain(t)
	got := runKCCValueLines(t, karkain, "native-value")
	if len(got) != 2 {
		t.Fatalf("kcc native-value produced %d lines, want 2:\n%v", len(got), got)
	}

	cases := []struct {
		name    string
		nLocals int
		src     string
	}{
		{"one", 1, "func main() {\n    let x = 42\n}\n"},
		{"two", 2, "func main() {\n    let x = 42\n    let y = x\n}\n"},
	}

	for i, tc := range cases {
		tc, i := tc, i
		t.Run(tc.name, func(t *testing.T) {
			kccHex := got[i]
			kccBytes, err := hex.DecodeString(kccHex)
			if err != nil {
				t.Fatalf("kcc emitted non-hex %q: %v", kccHex, err)
			}

			// Layer 1: differential against the oracle's own encoder.
			want := refMainBytes(t, tc.nLocals)
			if hex.EncodeToString(want) != kccHex {
				t.Errorf("byte identity vs oracle encoder:\n kcc = %s\n want= %s",
					kccHex, hex.EncodeToString(want))
			}

			// Layer 2: kcc's bytes must appear VERBATIM, exactly once, in the
			// image the Go oracle produced for the same source. This is what
			// rules out a shared transcription error in refMainBytes above:
			// the oracle's bytes come from compiling the program, not from us.
			text := oracleText(t, tc.src)
			if n := countOccurrences(text, kccBytes); n != 1 {
				t.Errorf("kcc's %d function bytes occur %d times in the oracle's %d-byte .text, want exactly 1",
					len(kccBytes), n, len(text))
			}

			// Layer 3: the stated bytes.
			stated := statedMainHex(tc.nLocals)
			if stated != kccHex {
				t.Errorf("stated-bytes check:\n kcc = %s\n want= %s", kccHex, stated)
			}

			t.Logf("%s: %d bytes sha256=%x", tc.name, len(kccBytes), sha256.Sum256(kccBytes))
		})
	}
}

// TestPhase151A_FrameLayoutNumbers pins the layout arithmetic itself.
//
// Matching bytes cannot catch a layout error the two sides share, so these
// offsets are compared against values derived in this file.
func TestPhase151A_FrameLayoutNumbers(t *testing.T) {
	karkain := phase130Karkain(t)
	got := runKCCValueLines(t, karkain, "native-value-layout")
	if len(got) != 5 {
		t.Fatalf("kcc native-value-layout produced %d lines, want 5:\n%v", len(got), got)
	}
	itoa := strconv.Itoa

	f1 := refFrameOf(1)
	if want := "int1 units=1 frame=" + itoa(f1) + " slot0=0"; got[0] != want {
		t.Errorf("one int local:\n kcc = %q\n want= %q", got[0], want)
	}

	f2 := refFrameOf(2)
	if want := "int2 units=1 frame=" + itoa(f2) + " slot0=0 slot1=8"; got[1] != want {
		t.Errorf("two int locals:\n kcc = %q\n want= %q", got[1], want)
	}

	// The unit widths, i.e. the oracle's kindUnits table. string and array are
	// two units (ptr+len); everything else is one.
	if want := "units int=1 string=2 float=1 array=2 struct=1 map=1"; got[2] != want {
		t.Errorf("kind units:\n kcc = %q\n want= %q", got[2], want)
	}

	// The conditional regions. An int program must reserve NEITHER strTemp nor
	// mapStage: reserving them unconditionally would grow every int image and
	// break byte identity with the oracle.
	_, sNone, mNone := refFrame(8, 0, false, false, 0, false)
	fStr, sStr, _ := refFrame(8, 0, true, false, 0, false)
	fMap, _, mMap := refFrame(8, 0, false, true, 0, false)
	want4 := "cond int_strTemp=" + itoa(sNone) + " int_mapStage=" + itoa(mNone) +
		" str_strTemp=" + itoa(sStr) + " str_frame=" + itoa(fStr) +
		" map_mapStage=" + itoa(mMap) + " map_frame=" + itoa(fMap)
	if got[3] != want4 {
		t.Errorf("conditional regions:\n kcc = %q\n want= %q", got[3], want4)
	}

	// Windows rounds the frame to 16; Linux does not. With maxExtra 2 the
	// unsized total is 8+512+16 = 536, which is 8 mod 16, so Windows adds one
	// unit (632 -> 640) and Linux does not. This is the check that would catch
	// an unconditional rounding, which would silently change every Linux image.
	fWin, _, _ := refFrame(8, 0, false, false, 2, true)
	fLin, _, _ := refFrame(8, 0, false, false, 2, false)
	if fWin == fLin {
		t.Fatalf("test bug: the Windows/Linux rounding case does not differ (%d)", fWin)
	}
	if want := "round win_frame=" + itoa(fWin) + " linux_frame=" + itoa(fLin); got[4] != want {
		t.Errorf("Windows 16-byte rounding:\n kcc = %q\n want= %q", got[4], want)
	}
}

// TestPhase151A_NewEncoderPrimitivesMatchOracle is a three-way, per-primitive
// differential for the seven encoder opcodes this slice added to
// native_emit.kark (memRsp / LoadStack / StoreStack / AddRegImm32 /
// SubRegImm32 / XorRegReg / Ret).
//
// The composite gate above already exercises all of them, but a composite
// mismatch does not say WHICH primitive is wrong. Three sources must agree:
// kcc's encoder, the Go oracle's own Emitter method, and the byte sequence
// stated from the Intel SDM. Two agreeing is not enough — that is the shared-
// error case the whole gate design is built to catch.
func TestPhase151A_NewEncoderPrimitivesMatchOracle(t *testing.T) {
	karkain := phase130Karkain(t)
	got := runKCCValueLines(t, karkain, "native-value-prims")
	if len(got) != 11 {
		t.Fatalf("kcc native-value-prims produced %d lines, want 11:\n%v", len(got), got)
	}

	cases := []struct {
		name string
		emit func(e *native.Emitter)
		ref  string
	}{
		{"store0", func(e *native.Emitter) { e.StoreStack(native.RAX, 0) }, "48890424"},
		{"load0", func(e *native.Emitter) { e.LoadStack(native.RAX, 0) }, "488b0424"},
		{"load8", func(e *native.Emitter) { e.LoadStack(native.RAX, 8) }, "488b442408"},
		{"store8", func(e *native.Emitter) { e.StoreStack(native.RAX, 8) }, "4889442408"},
		// 616 is the real frame size of the one-local reference program, so
		// the wide-displacement path is exercised with a value that actually
		// occurs. 616 does not fit in a signed byte, so the displacement is
		// disp32: mod=10 makes the ModRM 0x84, not the disp8 0x44.
		{"storebig", func(e *native.Emitter) { e.StoreStack(native.RAX, 616) }, "48898424" + le32Hex(616)},
		{"subbig", func(e *native.Emitter) { e.SubRegImm32(native.RSP, 616) }, "4881ec68020000"},
		{"addbig", func(e *native.Emitter) { e.AddRegImm32(native.RSP, 616) }, "4881c468020000"},
		// 16 fits in a signed byte, so these are the short 83 /digit ib form.
		{"subsmall", func(e *native.Emitter) { e.SubRegImm32(native.RSP, 16) }, "4883ec10"},
		{"addsmall", func(e *native.Emitter) { e.AddRegImm32(native.RSP, 16) }, "4883c410"},
		{"xor", func(e *native.Emitter) { e.XorRegReg(native.RAX) }, "4831c0"},
		{"ret", func(e *native.Emitter) { e.Ret() }, "c3"},
	}

	for i, tc := range cases {
		tc, i := tc, i
		t.Run(tc.name, func(t *testing.T) {
			kccHex := got[i]

			e := native.NewEmitter()
			tc.emit(e)
			oracleHex := hex.EncodeToString(e.Bytes())

			if oracleHex != tc.ref {
				t.Errorf("the ORACLE disagrees with the stated bytes, so the stated bytes are wrong:\n oracle= %s\n stated= %s", oracleHex, tc.ref)
			}
			if kccHex != oracleHex {
				t.Errorf("kcc differs from the oracle:\n kcc   = %s\n oracle= %s", kccHex, oracleHex)
			}
			if kccHex != tc.ref {
				t.Errorf("kcc differs from the stated bytes:\n kcc    = %s\n stated = %s", kccHex, tc.ref)
			}
		})
	}
}
