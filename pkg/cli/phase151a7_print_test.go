package cli

// Phase 151A Step 7 - int `print`.
//
// This is the first 151A surface that makes a real program's OUTPUT
// observable. Every earlier slice could compute a value and could enter and
// exit an image, but a kcc-compiled program could not say anything, so
// whole-image parity against the Go engine had nothing to compare.
//
// NO EXECUTION EVIDENCE IS CLAIMED. The corpus is the helper, not a program:
// it has no `main`, no entry stub and no resolved rodata, so it cannot be run.
// What is claimed is byte parity with pkg/native on the sequences this slice
// owns, plus the placeholder shape for the rodata references.
//
// FOUR INDEPENDENT LAYERS, following the 151B / 151A Step 1-6 methodology:
//
//  1. kcc's bytes vs the REAL pkg/native Emitter building the same sequence.
//     A differential, not a golden: a golden pins kcc against a transcription
//     of the Go code and passes when both are wrong together.
//  2. arithmetic derived from the encodings in this file (the digit-loop
//     displacement, and the buffer arithmetic the helper performs).
//  3. opcodes stated from the Intel SDM, independently of both emitters.
//  4. the helper must occur in the .text of a REAL oracle image produced by
//     compiling a reference program that prints - which ties these bytes to
//     pkg/native's actual emission rather than to a hand-built sequence.
//
// ONE DELIBERATE DIFFERENCE FROM STEP 4, which this file's doc comment has to
// be honest about: `print` contains UNRESOLVED absolute-address placeholders
// (`movabs rsi, <rodata>`), because a rodata address is only known at link
// time. The oracle resolves them to real addresses, so layer 1 compares
// STRUCTURE for the whole helper - same opcodes, same rel32 displacements,
// same length - rather than raw bytes, and layer 1b pins the placeholder bytes
// exactly. Asserting raw equality here would be asserting that kcc invents an
// address, which is precisely what it must not do.

import (
	"bytes"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"

	"karkain/pkg/lexer"
	"karkain/pkg/native"
	"karkain/pkg/parser"
)

func decode151A7(t *testing.T, h string) []byte {
	t.Helper()
	b, err := hex.DecodeString(h)
	if err != nil {
		t.Fatalf("corpus line %q is not hex: %v", h, err)
	}
	return b
}

// goPrint151A7 builds the digit loop with the REAL Go emitter, in the same
// order kcc emits it.
//
// Only the digit loop is built this way, and that is a consequence of what the
// oracle exports: every method the loop needs (Cqo, DivReg, DecReg, StoreMem8,
// TestRegReg, Jnz) is exported, while the rodata reference goes through the
// UNEXPORTED imm64Patch. A hand-built whole-helper sequence was therefore not
// available from outside pkg/native, and inventing an exported wrapper purely
// to enable one would widen the production API for a test's convenience. The
// whole helper is instead compared against the oracle's REAL emission, which is
// stronger evidence anyway - see oraclePrintInt151A7.
func goPrint151A7() []byte {
	e := native.NewEmitter()
	e.Mark("print_int_pos")
	e.MovRegImm32(native.RCX, 10)
	e.Mark("print_int_loop")
	e.Cqo()
	e.DivReg(native.RCX)
	e.AddRegImm32(native.RDX, '0')
	e.DecReg(native.RSI)
	e.StoreMem8(native.RSI, native.RDX)
	e.TestRegReg(native.RAX, native.RAX)
	e.Jnz("print_int_loop")
	return e.Bytes()
}

// oraclePrintInt151A7 returns the oracle's REAL print_int bytes out of the
// image's .text.
//
// Locating the function without labels (labels do not survive into the image)
// is done from two anchors rather than by a length:
//
//   - the OPENING is `sub rsp, 64` (REX.W 83 /5 40), asserted to occur once;
//   - the TAIL ANCHOR is `add rsp, 64` immediately followed by the newline
//     write's `movabs rsi` (REX.W 83 /4 40, then 48 BE), asserted to occur once.
//
// The anchor had to be this specific rather than `mov eax,1; syscall; ret`.
// That shorter pattern is NOT unique: print_str ends the same way, and a first
// draft of this function used it. The ambiguity guard below did catch the
// duplicate - the guard working as intended - but the honest fix is an anchor
// only print_int can produce, and `add rsp, 64` followed by a rodata reference
// is exactly that: only print_int both tears down its frame and then writes out
// of rodata.
//
// After the anchor the remainder is a FIXED sequence: 8 unresolved immediate
// bytes, then the two writes and the ret. That is checked rather than assumed,
// so a changed tail fails here instead of being silently truncated.
func oraclePrintInt151A7(t *testing.T, text []byte) []byte {
	t.Helper()
	open := []byte{0x48, 0x83, 0xEC, 0x40}               // sub rsp, 64
	anchor := []byte{0x48, 0x83, 0xC4, 0x40, 0x48, 0xBE} // add rsp, 64; movabs rsi
	const imm64 = 8
	tail := []byte{
		0xBA, 0x01, 0x00, 0x00, 0x00, // mov edx, 1
		0xB8, 0x01, 0x00, 0x00, 0x00, // mov eax, 1
		0x0F, 0x05, //                   syscall
		0xC3, //                         ret
	}

	start := bytes.Index(text, open)
	if start < 0 {
		t.Fatalf("oracle .text has no `sub rsp, 64`; print_int was not emitted " +
			"(is the reference program actually printing?)")
	}
	if bytes.Index(text[start+1:], open) >= 0 {
		t.Fatalf("oracle .text has more than one `sub rsp, 64`; the print_int " +
			"span this layer extracts would be ambiguous")
	}

	rel := bytes.Index(text[start:], anchor)
	if rel < 0 {
		t.Fatal("oracle .text has no `add rsp, 64` + `movabs rsi` after " +
			"`sub rsp, 64`; print_int's tail changed and this layer's premise is wrong")
	}
	if bytes.Index(text[start+rel+1:], anchor) >= 0 {
		t.Fatal("oracle .text has more than one tail anchor; the print_int " +
			"span this layer extracts would be ambiguous")
	}

	// The anchor's last byte is the 0x48 REX of `movabs rsi`; the 8-byte
	// immediate follows it, and then the fixed tail must be present verbatim.
	tailAt := start + rel + len(anchor) + imm64
	if tailAt+len(tail) > len(text) {
		t.Fatal("oracle print_int ends before its newline write; extraction premise is wrong")
	}
	if !bytes.Equal(text[tailAt:tailAt+len(tail)], tail) {
		t.Fatalf("oracle print_int tail = % x, want % x", text[tailAt:tailAt+len(tail)], tail)
	}
	return text[start : tailAt+len(tail)]
}

func oracleText151A7(t *testing.T, src string) []byte {
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

func printCorpus151A7(t *testing.T) []string {
	t.Helper()
	karkain := phase130Karkain(t)
	got := runKCCStep2(t, karkain, "native-value-print")
	if len(got) != 3 {
		t.Fatalf("kcc native-value-print produced %d lines, want 3:\n%v", len(got), got)
	}
	return got
}

// TestPhase151A7_DigitLoopByteIdenticalToOracle is the layer-1 differential for
// the digit loop, built with the REAL Go emitter through exported API only.
//
// The loop is chosen as the differential subject precisely because it carries
// no unresolved address, so it can be compared RAW. Everything else about this
// slice's byte claim rests on TestPhase151A7_WholeHelperMatchesOracleExceptRodata.
func TestPhase151A7_DigitLoopByteIdenticalToOracle(t *testing.T) {
	got := printCorpus151A7(t)
	g := decode151A7(t, got[1])
	w := goPrint151A7()
	if !bytes.Equal(g, w) {
		t.Errorf("digit loop: kcc=% x\n              oracle=% x", g, w)
	}
}

// TestPhase151A7_WholeHelperMatchesOracleExceptRodata compares the whole helper
// against the oracle's REAL emission, masking ONLY the two rodata immediates.
//
// The mask is not a weakening of the claim; it is the accurate statement of it.
// Every other byte - every opcode, every ModRM byte, both rel32 displacements,
// the 64-byte frame arithmetic, the sign branch, the buffer reversal and the
// newline write - must be byte-identical to what pkg/native emits for a program
// that actually prints. Only the two 8-byte ABSOLUTE addresses differ, because
// the oracle has a link-time rodata base and kcc does not.
//
// The rel32 displacements ARE expected to match exactly, and that is not luck:
// a rel32 encodes target-minus-next-instruction, both measured from inside the
// function, so the value is invariant under where the function is placed in
// .text. kcc's corpus places the helper at offset 0 of its buffer while the
// oracle's sits after the other helpers, and the displacements still have to
// agree. A gate that masked them would discard the only evidence that kcc laid
// the function out the same shape.
func TestPhase151A7_WholeHelperMatchesOracleExceptRodata(t *testing.T) {
	got := printCorpus151A7(t)
	g := decode151A7(t, got[0])

	text := oracleText151A7(t, "func main() {\n\tlet x = 1\n\tprint(x + 1)\n}\n")
	w := oraclePrintInt151A7(t, text)

	if len(g) != len(w) {
		t.Fatalf("print_int length: kcc=%d oracle=%d\n  kcc=% x\n  oracle=% x", len(g), len(w), g, w)
	}

	// A rodata site is `movabs rsi, imm64`: REX.W (48) + B8+rd with rd = RSI
	// (6 -> 0xBE) + 8 immediate bytes. Mask those 8 bytes and assert there are
	// exactly two sites, so a third cannot hide inside an unmasked region and a
	// changed helper cannot silently reduce the site count.
	maskRodata := func(b []byte, wantSites int, who string) []byte {
		out := bytes.Clone(b)
		const siteLen = 10
		sites := 0
		for i := 0; i+siteLen <= len(out); i++ {
			if out[i] == 0x48 && out[i+1] == 0xBE {
				sites++
				for j := i + 2; j < i+siteLen; j++ {
					out[j] = 0
				}
				i += siteLen - 1
			}
		}
		if sites != wantSites {
			t.Fatalf("%s contains %d movabs-rsi rodata sites, want %d "+
				"(one for the sign, one for the newline)", who, sites, wantSites)
		}
		return out
	}

	gm := maskRodata(g, 2, "kcc print_int")
	wm := maskRodata(w, 2, "oracle print_int")

	if !bytes.Equal(gm, wm) {
		t.Errorf("print_int differs from the oracle outside the rodata immediates:\n"+
			"  kcc   =% x\n  oracle=% x", gm, wm)
	}
}

// TestPhase151A7_RodataPlaceholderIsUnresolved pins the placeholder form on its
// own: a rodata reference must be a zeroed 8-byte immediate plus a RECORDED
// patch position, never a locally invented address.
//
// The recorded-patch half matters as much as the zero: kcc must hand the
// linker a position to fill, and an implementation that emitted `movabs rsi, 0`
// with nothing recorded would look identical here and then produce an image
// printing from address zero.
func TestPhase151A7_RodataPlaceholderIsUnresolved(t *testing.T) {
	got := printCorpus151A7(t)

	// Arm 2 is the bare reference: REX.W + B8+rd, no ModRM, no SIB, no disp.
	want := []byte{0x48, 0xBE, 0, 0, 0, 0, 0, 0, 0, 0}
	if g := decode151A7(t, got[2]); !bytes.Equal(g, want) {
		t.Errorf("bare rodata reference = % x, want % x", g, want)
	}
}

// TestPhase151A7_DigitLoopMatchesBytesStatedFromTheIntelSDM checks the loop
// against bytes stated here, so an error BOTH emitters share still fails.
//
// This is the layer that would catch a transcription mistake in the oracle
// sequence above, which is the one way a differential can be satisfied by two
// wrong copies agreeing.
func TestPhase151A7_DigitLoopMatchesBytesStatedFromTheIntelSDM(t *testing.T) {
	got := printCorpus151A7(t)
	g := decode151A7(t, got[1])

	// Stated from the Intel SDM Vol. 2B, register numbering architectural:
	//   mov rcx, 10   prefix-less B8+rd (rd = RCX = 1) + imm32   5 bytes, no REX
	//   cqo           REX.W (48) + 99                            2
	//   div rcx       REX.W (48) + F7 /6 -> ModRM 0xF1            3
	//   add rdx, '0'  REX.W + 83 /0 + imm8 (0x30)                4
	//   dec rsi       REX.W + FF /1 -> ModRM 0xCE                3
	//   mov [rsi], dl 88 /r with reg = DL (2), rm = RSI (6) -> 0x16, NO REX  2
	//   test rax, rax REX.W + 85 /r -> ModRM 0xC0                3
	//   jnz rel32     0F 85 cd                                   6
	want := []byte{
		0xB9, 0x0A, 0x00, 0x00, 0x00, // mov ecx, 10
		0x48, 0x99, //                   cqo
		0x48, 0xF7, 0xF1, //             div rcx
		0x48, 0x83, 0xC2, 0x30, //       add rdx, 0x30
		0x48, 0xFF, 0xCE, //             dec rsi
		0x88, 0x16, //                   mov [rsi], dl
		0x48, 0x85, 0xC0, //             test rax, rax
		0x0F, 0x85, 0xE9, 0xFF, 0xFF, 0xFF, // jnz back to the loop
	}
	if !bytes.Equal(g, want) {
		t.Errorf("digit loop = % x\n            want % x", g, want)
	}

	// The displacement is derived, not read off the output: the jnz is the last
	// 6 bytes, its next instruction is at len(g), and the loop body it targets
	// begins immediately after `mov ecx, 10`, i.e. at offset 5.
	//
	// The displacement is the FOUR-byte immediate at len-4..len-1, not the last
	// BYTE. A first draft of this check read g[len-1], which is the sign-
	// extension byte (0xff) and reported -1 -- a wrong expectation rather than a
	// wrong emission, since the bytes themselves already matched the SDM layer
	// above. Counting the trailing bytes is exactly the mistake the 151A
	// methodology warns about, so the arithmetic here is written out.
	const loopTop = 5
	disp := int32(uint32(g[len(g)-4]) |
		uint32(g[len(g)-3])<<8 |
		uint32(g[len(g)-2])<<16 |
		uint32(g[len(g)-1])<<24)
	if want := int32(loopTop - len(g)); disp != want {
		t.Errorf("jnz displacement = %d, want %d (loop top at %d, next insn at %d)",
			disp, want, loopTop, len(g))
	}
}

// TestPhase151A7_HelperAppearsInARealOracleImage ties the slice to pkg/native's
// actual print emission rather than to a hand-built sequence.
//
// The corpus proves kcc and the Go emitter agree in isolation. This proves the
// bytes are the ones the compiler really emits, by compiling a reference
// program that prints and requiring kcc's helper to occur verbatim in its
// .text. The DIGIT LOOP is searched for rather than the whole helper, because
// only the loop is layout-independent: the helper contains two rel32
// fixups whose displacements depend on where the oracle placed it.
func TestPhase151A7_HelperAppearsInARealOracleImage(t *testing.T) {
	got := printCorpus151A7(t)
	loop := decode151A7(t, got[1])

	text := oracleText151A7(t, "func main() {\n\tlet x = 1\n\tprint(x + 1)\n}\n")

	if idx := bytes.Index(text, loop); idx < 0 {
		t.Fatalf("kcc's print digit loop % x does not occur in the oracle .text (%d bytes); "+
			"either pkg/native's print emission changed, or this layer's premise is wrong",
			loop, len(text))
	}
}

// TestPhase151A7_CorpusIsNonVacuousAndDeterministic guards the two failure modes
// a hex-comparison gate is otherwise blind to: an empty corpus compares equal to
// nothing, and a non-deterministic corpus would report parity that does not
// exist.
func TestPhase151A7_CorpusIsNonVacuousAndDeterministic(t *testing.T) {
	karkain := phase130Karkain(t)
	first := runKCCStep2(t, karkain, "native-value-print")
	for i, h := range first {
		if len(h) == 0 {
			t.Fatalf("corpus line %d is empty; an empty comparison is not evidence", i)
		}
	}
	for run := 0; run < 2; run++ {
		again := runKCCStep2(t, karkain, "native-value-print")
		for i := range first {
			if again[i] != first[i] {
				t.Fatalf("run %d line %d differs:\n  %s\n  %s", run+1, i, first[i], again[i])
			}
		}
	}
}

// TestPhase151A7_NoGoFallback guards the measurement contract itself: the Go
// side must only RUN kcc, never compute the answer.
//
// Every earlier measurement command in this increment carries this same rule,
// and it is the rule 151 was opened to enforce. A Go-side emitter that produced
// the expected bytes would make every differential above pass while proving
// nothing about kcc.
func TestPhase151A7_NoGoFallback(t *testing.T) {
	src, err := os.ReadFile(filepath.Join(repoRoot(t), "pkg", "cli", "native_encode.go"))
	if err != nil {
		t.Fatalf("read native_encode.go: %v", err)
	}
	if bytes.Contains(src, []byte("func oraclePrint")) ||
		bytes.Contains(src, []byte("func goPrint")) {
		t.Error("pkg/cli/native_encode.go appears to compute the print bytes in Go; " +
			"the measurement surface must only run kcc")
	}
	// The command must delegate to the shared subcommand helper, which is what
	// guarantees it inherits the no-fallback contract.
	if !bytes.Contains(src, []byte(`return kccSubcommand(w, "native-value-print")`)) {
		t.Error("KCCNativeValuePrintCommand does not delegate to kccSubcommand")
	}
}
