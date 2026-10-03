package cli

// Phase 151A - the `_start` entry stub and the Linux exit tail.
//
// This is a STRUCTURAL slice. It emits the entry sequence and nothing else:
// there is no `main` body, no `print`, no arena and no value model, so the
// corpus is not a runnable program. NO EXECUTION EVIDENCE IS CLAIMED. What is
// claimed is byte parity with pkg/native on the one sequence this slice owns.
//
// Deliberately ABSENT, each separately recorded as open 151A work, and each
// out of scope here: the Windows 16-byte alignment fix, the PEB bootstrap, the
// Windows exit/IAT tail, and the macOS exit number (0x2000001). Emitting any
// of them would be inventing scope rather than porting it.
//
// FOUR INDEPENDENT LAYERS, following the 151B / 151A Step 1-3 methodology:
//
//  1. kcc's bytes vs the REAL pkg/native Emitter building the same sequence.
//     A differential, not a golden: a golden pins kcc against a transcription
//     of the Go code and passes when both are wrong together.
//  2. the rel32 displacement derived from first principles in this file.
//  3. opcodes stated from the Intel SDM.
//  4. the exit tail must occur verbatim in the .text of a REAL oracle image
//     produced by compiling a reference program -- which ties the sequence to
//     pkg/native's actual _start emission rather than to a hand-built one.
//
// Layer 4 compares only the bytes AFTER the call, and that exclusion is
// deliberate rather than convenient: a rel32 encodes a displacement whose value
// depends on where the compiler placed karkain_main in that program, so kcc's
// corpus displacement (10, because the corpus puts the label immediately after
// the tail) must NOT equal a real image's. The bytes after the call are
// layout-independent and must match exactly.

import (
	"bytes"
	"encoding/hex"
	"testing"

	"karkain/pkg/lexer"
	"karkain/pkg/native"
	"karkain/pkg/parser"
)

// oracleText151A4 compiles a Karkain source with the Go oracle and returns the
// image's .text, so layer 4 searches real compiler output rather than a
// synthetic buffer.
func oracleText151A4(t *testing.T, src string) []byte {
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

func decode151A4(t *testing.T, h string) []byte {
	t.Helper()
	b, err := hex.DecodeString(h)
	if err != nil {
		t.Fatalf("corpus line %q is not hex: %v", h, err)
	}
	return b
}

// goStart151A4 builds the corpus sequence with the REAL Go emitter: the same
// three shapes kcc emits, in the same order.
func goStart151A4(which int) []byte {
	e := native.NewEmitter()
	switch which {
	case 0: // full stub: call, mov rdi,rax, mov eax,60, syscall
		e.Mark("_start")
		e.Call("karkain_main")
		e.MovRegReg(native.RDI, native.RAX)
		e.MovRegImm32(native.RAX, 60)
		e.Syscall()
		e.Mark("karkain_main")
	case 1: // the syscall primitive alone
		e.Syscall()
	case 2: // the exit tail without its call
		e.Mark("_start")
		e.MovRegReg(native.RDI, native.RAX)
		e.MovRegImm32(native.RAX, 60)
		e.Syscall()
	default:
		panic("bad which")
	}
	return e.Bytes()
}

// TestPhase151A4_StartStubByteIdenticalToOracle is this slice's whole contract:
// the bytes kcc's `_start` emission produces equal the bytes the Go oracle's
// emitter produces, for all three corpus shapes.
func TestPhase151A4_StartStubByteIdenticalToOracle(t *testing.T) {
	karkain := phase130Karkain(t)
	got := runKCCStep2(t, karkain, "native-value-start")
	if len(got) != 3 {
		t.Fatalf("kcc native-value-start produced %d lines, want 3:\n%v", len(got), got)
	}
	for which := 0; which < 3; which++ {
		g := decode151A4(t, got[which])
		w := goStart151A4(which)
		if !bytes.Equal(g, w) {
			t.Errorf("corpus %d: kcc=% x\n            oracle=% x", which, g, w)
		}
	}
}

// TestPhase151A4_Rel32DisplacementIsArithmeticallyCorrect derives the call's
// displacement from first principles, independently of both implementations.
//
// This layer exists because the corpus displacement is a property of THIS
// corpus rather than of a real image: karkain_main is placed immediately after
// the tail so the sequence is self-contained, which is what lets 151B's rel32
// fixup resolve a genuine forward reference. Reading the displacement off the
// hex would read a number correct here and meaningless elsewhere.
func TestPhase151A4_Rel32DisplacementIsArithmeticallyCorrect(t *testing.T) {
	karkain := phase130Karkain(t)
	got := runKCCStep2(t, karkain, "native-value-start")
	stub := decode151A4(t, got[0])

	// Lengths derived from the encodings, not counted from the output:
	//   call rel32                    5  (E8 + rel32)
	//   mov rdi, rax  REX.W 89 /r     3
	//   mov eax, 60   prefix-less B8  5  (imm32; zero-extends into rax)
	//   syscall                        2  (0F 05)
	const (
		callBytes  = 5
		movRRBytes = 3
		movImmByte = 5
		syscallLen = 2
	)
	wantLen := callBytes + movRRBytes + movImmByte + syscallLen
	if len(stub) != wantLen {
		t.Fatalf("stub length = %d, want %d (% x)", len(stub), wantLen, stub)
	}
	if stub[0] != 0xE8 {
		t.Fatalf("first byte = %#x, want 0xE8 (call rel32)", stub[0])
	}
	// rel32 = offset(karkain_main) - offset(next instruction). The label sits at
	// the end of the tail, and the next instruction begins 5 bytes after the
	// call's own start.
	wantDisp := int32(wantLen - callBytes)
	gotDisp := int32(uint32(stub[1]) | uint32(stub[2])<<8 | uint32(stub[3])<<16 | uint32(stub[4])<<24)
	if gotDisp != wantDisp {
		t.Errorf("call displacement = %d, want %d", gotDisp, wantDisp)
	}
}

// TestPhase151A4_ExitTailMatchesBytesStatedFromTheIntelSDM checks each
// instruction against bytes stated here, so a mistake BOTH implementations
// share still fails.
func TestPhase151A4_ExitTailMatchesBytesStatedFromTheIntelSDM(t *testing.T) {
	karkain := phase130Karkain(t)
	got := runKCCStep2(t, karkain, "native-value-start")

	// SYSCALL: 0F 05 (Intel SDM Vol. 2B), no REX, no ModRM.
	if g := decode151A4(t, got[1]); !bytes.Equal(g, []byte{0x0F, 0x05}) {
		t.Errorf("syscall = % x, want 0f 05", g)
	}

	// MOV rdi, rax: REX.W (48) + 89 /r, reg = RAX (0), rm = RDI (7) -> ModRM 0xC7.
	// MOV eax, 60:   B8+rd (rd = RAX = 0) + imm32 0x0000003C, 5 bytes, NO REX --
	//   the prefix-less form zero-extends into the full 64-bit register, which is
	//   what pkg/native.MovRegImm32 emits after the Phase-147 correction.
	// SYSCALL:      0F 05.
	want := []byte{0x48, 0x89, 0xC7, 0xB8, 0x3C, 0x00, 0x00, 0x00, 0x0F, 0x05}
	if g := decode151A4(t, got[2]); !bytes.Equal(g, want) {
		t.Errorf("exit tail = % x, want % x", g, want)
	}
}

// TestPhase151A4_ExitTailAppearsInARealOracleImage ties the slice to
// pkg/native's actual _start emission rather than to a hand-built sequence.
//
// The corpus proves kcc and the Go emitter agree in isolation. This proves the
// sequence is the one the compiler really emits, by compiling a reference
// program and requiring kcc's exit-tail bytes to occur verbatim in its .text.
func TestPhase151A4_ExitTailAppearsInARealOracleImage(t *testing.T) {
	karkain := phase130Karkain(t)
	got := runKCCStep2(t, karkain, "native-value-start")
	tail := decode151A4(t, got[2])

	text := oracleText151A4(t, "func main() {\n\tlet x = 1\n\tprint(x + 1)\n}\n")
	idx := bytes.Index(text, tail)
	if idx < 0 {
		t.Fatalf("kcc's exit tail % x does not occur in the oracle .text (%d bytes); "+
			"either pkg/native's _start emission changed, or this layer's premise is wrong",
			tail, len(text))
	}
}

// TestPhase151A4_CorpusIsNonVacuousAndDeterministic guards the two failure modes
// a hex-comparison gate is otherwise blind to: an empty corpus compares equal to
// nothing, and a non-deterministic corpus would report parity that does not
// exist.
func TestPhase151A4_CorpusIsNonVacuousAndDeterministic(t *testing.T) {
	karkain := phase130Karkain(t)
	first := runKCCStep2(t, karkain, "native-value-start")
	for i, h := range first {
		if len(h) == 0 {
			t.Fatalf("corpus line %d is empty; an empty comparison is not evidence", i)
		}
	}
	for run := 0; run < 2; run++ {
		again := runKCCStep2(t, karkain, "native-value-start")
		for i := range first {
			if again[i] != first[i] {
				t.Fatalf("run %d line %d differs:\n  %s\n  %s", run+1, i, first[i], again[i])
			}
		}
	}
}
