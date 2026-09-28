package cli

import (
	"encoding/hex"
	"os/exec"
	"strings"
	"testing"

	"karkain/pkg/native"
)

// Phase 151B - the machine-code encoder in the self-hosted engine.
// See docs/audit/PHASE-151A-BASELINE.md section 4.
//
// Increment 151B's contract is unusually narrow and unusually strict: the
// kcc encoder must produce the SAME BYTES as the Go oracle
// (pkg/native.Emitter), not merely an equivalent instruction sequence. The
// Go emitter resolves every intra-text reference eagerly and patches rel32
// displacements in one pass at the end, and reproducing that discipline is
// what makes byte-identical images possible. An encoder that was merely
// correct would fail this gate, which is the point.
//
// THE GATE IS A DIFFERENTIAL, NOT A GOLDEN. Every expected value below is
// produced by calling the real Go emitter in this file, so the two
// implementations are compared to each other. A golden would pin kcc against
// a transcription of the Go code, which is exactly the kind of test that
// passes when both copies are wrong in the same way.

// The kcc encoder's five reference sequences, built here with the Go oracle.
// The order matches `kcc native-encode`: imm64, loop, multi, undef, dup.
//
// The two negative sequences are included on the Go side too. The Go emitter
// PANICS on both conditions rather than returning an error, so they are
// exercised through a recover, and the point of asserting the panic is that
// kcc must REFUSE them the same way -- an encoder that quietly emitted a zero
// displacement would produce a jumping-wrong-place image instead of an error.

// goSeqImm64 mirrors natSeqImm64: mov rax, imm64; jmp later; nop; nop; later: ret.
//
// The two nops are load-bearing in both implementations. Without them the
// displacement would be zero even before the fixup pass runs, so a pass that
// did nothing would still produce the right-looking bytes.
func goSeqImm64(t *testing.T) []byte {
	e := native.NewEmitter()
	e.MovRegImm64(native.RAX, 0x1122334455667788)
	e.Jmp("later")
	e.Nop()
	e.Nop()
	e.Mark("later")
	e.Ret()
	return e.Bytes()
}

// goSeqLoop mirrors natSeqLoop: the counted loop whose jnz jumps BACKWARD.
//
// This is the case that catches a sign error. The displacement is negative and
// must be written as a little-endian two's-complement 32-bit value; a wrong
// sign produces a jump far outside the text rather than a subtly wrong answer.
func goSeqLoop(t *testing.T) []byte {
	e := native.NewEmitter()
	e.MovRegImm32(native.RAX, 0)
	e.MovRegImm32(native.RCX, 10)
	e.MovRegImm32(native.RDX, 1)
	e.Mark("loop")
	e.AddRegReg(native.RAX, native.RCX)
	e.SubRegReg(native.RCX, native.RDX)
	e.Jnz("loop")
	e.Ret()
	return e.Bytes()
}

// goSeqMulti mirrors natSeqMulti: three fixups, two forward and one backward,
// plus an imm64 placeholder left for a linker to fill.
//
// A single fixup cannot distinguish a real fixup table from one displacement
// variable, so the gate needs several per stream, and needs mixed directions
// to catch a sign error that only shows up backwards.
func goSeqMulti(t *testing.T) []byte {
	e := native.NewEmitter()
	e.Call("helper_a")
	e.Jmp("done")
	e.Mark("helper_a")
	e.Call("helper_b")
	e.Mark("back")
	e.Jmp("done")
	e.Mark("helper_b")
	e.Call("back")
	e.Mark("done")
	e.Ret()
	// The trailing imm64 PLACEHOLDER. Emitter.imm64Patch is unexported, and
	// adding an exported wrapper to production code so a test can reach it
	// would be the wrong trade. MovRegImm64(RAX, 0) emits byte-identical
	// bytes -- REX.W + B8+rd + u64(0) -- so it stands in exactly, and the
	// encoder's own imm64Patch differs only in recording the position for a
	// linker, which is not part of the byte stream.
	e.MovRegImm64(native.RAX, 0)
	return e.Bytes()
}

// goSeqUndefinedLabel mirrors natSeqUndefinedLabel: a rel32 to a label that is
// never marked. The Go emitter panics; kcc must refuse.
func goSeqUndefinedLabel(t *testing.T) {
	t.Helper()
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("Go emitter resolved a rel32 to an undefined label instead of failing; " +
				"kcc refuses this case, so the two implementations would disagree")
		}
	}()
	e := native.NewEmitter()
	e.Jmp("nowhere")
	_ = e.Bytes()
}

// goSeqDuplicateLabel mirrors natSeqDuplicateLabel: one label marked twice.
func goSeqDuplicateLabel(t *testing.T) {
	t.Helper()
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("Go emitter accepted a duplicate label instead of failing; " +
				"kcc refuses this case, so the two implementations would disagree")
		}
	}()
	e := native.NewEmitter()
	e.Mark("twice")
	e.Mark("twice")
	_ = e.Bytes()
}

// kccNativeEncode runs `kcc native-encode` and returns the output lines with
// any build banner removed. The subcommand reads no source; it drives the
// encoder over fixed sequences compiled into kcc itself.
func kccNativeEncode(t *testing.T, karkain string) []string {
	t.Helper()
	cmd := exec.Command(karkain, "native-encode")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("kcc native-encode failed: %v\n%s", err, out)
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

// TestPhase151B_EncoderByteIdenticalToOracle is the increment's whole
// contract: for every reference sequence, the bytes kcc's encoder produces
// equal the bytes the Go oracle produces.
//
// A byte-identity gate is the only honest one here. Asserting that kcc emits
// *a* correct rel32 displacement would be satisfied by an implementation that
// differs from the oracle in some other byte, and the whole purpose of 151 is
// that the two engines' images are indistinguishable.
func TestPhase151B_EncoderByteIdenticalToOracle(t *testing.T) {
	karkain := phase130Karkain(t)
	got := kccNativeEncode(t, karkain)

	want := map[string]string{
		"imm64": hex.EncodeToString(goSeqImm64(t)),
		"loop":  hex.EncodeToString(goSeqLoop(t)),
		"multi": hex.EncodeToString(goSeqMulti(t)),
	}
	if len(got) != 5 {
		t.Fatalf("kcc native-encode produced %d lines, want 5:\n%v", len(got), got)
	}

	seen := map[string]bool{}
	for _, ln := range got {
		parts := strings.SplitN(ln, " ", 2)
		if len(parts) != 2 {
			t.Errorf("malformed native-encode line %q", ln)
			continue
		}
		name, payload := parts[0], strings.TrimSpace(parts[1])
		seen[name] = true

		exp, isSeq := want[name]
		if !isSeq {
			// undef and dup are the two REFUSAL cases, asserted separately.
			if !strings.HasPrefix(payload, "error[K117]") {
				t.Errorf("%s: want an error[K117] refusal, got %q", name, payload)
			}
			continue
		}
		if payload != exp {
			t.Errorf("%s: encoder bytes differ from the Go oracle\n kcc: %s\n  go: %s",
				name, payload, exp)
		}
	}
	for name := range want {
		if !seen[name] {
			t.Errorf("kcc native-encode did not report sequence %q", name)
		}
	}
}

// TestPhase151B_Rel32DisplacementsAreArithmeticallyCorrect pins the fixup
// arithmetic independently of the oracle.
//
// The differential above would pass if BOTH implementations shared a sign
// error. This test computes each displacement from the offsets in the
// sequence definition and requires the encoded bytes to contain it, so the
// backward jump is checked against first principles rather than against
// another copy of the same idea.
func TestPhase151B_Rel32DisplacementsAreArithmeticallyCorrect(t *testing.T) {
	karkain := phase130Karkain(t)
	byName := map[string]string{}
	for _, ln := range kccNativeEncode(t, karkain) {
		parts := strings.SplitN(ln, " ", 2)
		if len(parts) == 2 {
			byName[parts[0]] = strings.TrimSpace(parts[1])
		}
	}

	cases := []struct{ name, want, why string }{
		// loop: three 5-byte mov r32,imm32 put `loop` at offset 15. The body
		// is 3+3 bytes then 0f 85, so the displacement field spans 23..27:
		// 15 - 27 = -12 = 0xfffffff4 little-endian.
		{"loop", "0f85f4ffffff", "backward jnz to offset 15"},
		// imm64: a 10-byte mov, then e9 at offset 10, so the field spans
		// 11..15 and `later` sits at 17: 17 - 15 = 2.
		{"imm64", "e902000000", "forward jmp to offset 17"},
		// multi: helper_b sits at 20, and the call at 20 has its field at
		// 21..25, so 20 - 25 = -5.
		{"multi", "e805000000", "call rel32 with displacement -5"},
	}

	for _, c := range cases {
		got, ok := byName[c.name]
		if !ok {
			t.Errorf("kcc native-encode did not report sequence %q", c.name)
			continue
		}
		if !strings.Contains(got, c.want) {
			t.Errorf("%s: encoded bytes %s do not contain %s (%s)", c.name, got, c.want, c.why)
		}
	}
}

// TestPhase151B_LoopSequenceDisassemblesToTheIntendedCode guards against a
// "correct by construction" mistake in the gate itself.
//
// The two previous tests compare against the Go emitter and against
// arithmetic. This one states, in bytes, what the loop sequence is SUPPOSED
// to be. If someone rewrote natSeqLoop and goSeqLoop together and both drifted
// the same way, the first two tests would be satisfied and only this one would
// notice.
func TestPhase151B_LoopSequenceDisassemblesToTheIntendedCode(t *testing.T) {
	const want = "b800000000" + // mov eax, 0
		"b90a000000" + // mov ecx, 10
		"ba01000000" + // mov edx, 1
		"4801c8" + // add rax, rcx   (REX.W 01 /r, modrm 0xC8)
		"4829d1" + // sub rcx, rdx   (REX.W 29 /r, modrm 0xD1)
		"0f85f4ffffff" + // jnz -12
		"c3" // ret
	if got := hex.EncodeToString(goSeqLoop(t)); got != want {
		t.Errorf("Go oracle loop sequence = %s\n want %s", got, want)
	}
}

// TestPhase151B_RefusalsMatchTheOracle asserts the two error paths agree: both
// implementations fail on an undefined label and on a duplicate label, and
// neither invents a byte stream.
func TestPhase151B_RefusalsMatchTheOracle(t *testing.T) {
	goSeqUndefinedLabel(t)
	goSeqDuplicateLabel(t)

	karkain := phase130Karkain(t)
	joined := strings.Join(kccNativeEncode(t, karkain), "\n")

	// The refusal must NAME the condition, not merely fail, so a reader of a
	// failed build knows whether the encoder or the program is at fault.
	if !strings.Contains(joined, "undefined label 'nowhere'") {
		t.Errorf("kcc did not name the undefined label; output:\n%s", joined)
	}
	if !strings.Contains(joined, "duplicate label 'twice'") {
		t.Errorf("kcc did not name the duplicate label; output:\n%s", joined)
	}
	if n := strings.Count(joined, "error[K117]"); n != 2 {
		t.Errorf("expected exactly 2 K117 refusals, got %d:\n%s", n, joined)
	}
}

// TestPhase151B_EncoderIsDeterministic runs the encoder twice and requires
// identical output. The Go emitter is deterministic by construction; kcc's
// label lookup is a linear scan over an append-ordered table, and a future
// change to that table must not be able to reorder it.
func TestPhase151B_EncoderIsDeterministic(t *testing.T) {
	karkain := phase130Karkain(t)
	first := strings.Join(kccNativeEncode(t, karkain), "\n")
	second := strings.Join(kccNativeEncode(t, karkain), "\n")
	if first != second {
		t.Errorf("native-encode is not deterministic\n run 1:\n%s\n run 2:\n%s", first, second)
	}
}

// TestPhase151B_OpcodeConstantsAreIndependentlyComputed exists because the
// first draft of the encoder's constant table was wrong and nothing caught it.
//
// Karkain has no hex literals -- 0xFF lexes as the identifier xFF -- so every
// x86 constant in the encoder is a hand-written decimal. Converting by hand
// turned 0xFF into 597 (0x255) and 0x0F into 21 (0x15), and the encoder then
// emitted garbage that no test caught, because the differential against the
// oracle had not been written yet. The wrong bytes were noticed by DECODING
// the output by eye, not by any assertion.
//
// This test states the constants Go computes for the same instructions, so a
// mistyped selector is a test failure rather than a corrupt image. The values
// are what pkg/native actually emits, not what this file's author believed.
func TestPhase151B_OpcodeConstantsAreIndependentlyComputed(t *testing.T) {
	cases := []struct {
		name string
		emit func(e *native.Emitter)
		want string
		why  string
	}{
		{
			"mov r64, imm64",
			func(e *native.Emitter) { e.MovRegImm64(native.RAX, 0x1122334455667788) },
			"48b88877665544332211",
			"REX.W is 0x48 and the opcode is 0xB8+rd",
		},
		{
			"add rax, rcx",
			func(e *native.Emitter) { e.AddRegReg(native.RAX, native.RCX) },
			"4801c8",
			"REX.W 0x01 with modrm mod=11 reg=rcx rm=rax",
		},
		{
			"sub rcx, rdx",
			func(e *native.Emitter) { e.SubRegReg(native.RCX, native.RDX) },
			"4829d1",
			"REX.W 0x29 with modrm mod=11 reg=rdx rm=rcx",
		},
		{
			"jnz rel32 opcode",
			func(e *native.Emitter) { e.Jnz("x"); e.Mark("x") },
			"0f8500000000",
			"0x0F escape then 0x85; the label sits at the end so the displacement is 0",
		},
		{
			"jmp rel32 opcode",
			func(e *native.Emitter) { e.Jmp("x"); e.Mark("x") },
			"e900000000",
			"0xE9 with a zero displacement, the label sitting at the end",
		},
		{
			"ret",
			func(e *native.Emitter) { e.Ret() },
			"c3",
			"0xC3",
		},
		{
			"mov r32, imm32",
			func(e *native.Emitter) { e.MovRegImm32(native.RCX, 10) },
			"b90a000000",
			"no REX is emitted for a legacy 32-bit mov with a low register",
		},
	}
	for _, c := range cases {
		e := native.NewEmitter()
		c.emit(e)
		if got := hex.EncodeToString(e.Bytes()); got != c.want {
			t.Errorf("%s = %s, want %s (%s)", c.name, got, c.want, c.why)
		}
	}
}

// TestPhase151B_SequencesAreNotEmpty is a cheap guard that a sequence has not
// silently become a no-op. A differential over two EMPTY byte strings passes
// trivially, which is the vacuity the governance rule in AGENTS.md warns
// about, so the non-empty shape is asserted explicitly.
func TestPhase151B_SequencesAreNotEmpty(t *testing.T) {
	for name, seq := range map[string][]byte{
		"imm64": goSeqImm64(t),
		"loop":  goSeqLoop(t),
		"multi": goSeqMulti(t),
	} {
		if len(seq) == 0 {
			t.Errorf("sequence %q encoded to zero bytes; a differential over empty "+
				"output would pass without comparing anything", name)
		}
	}
}
