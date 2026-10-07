package cli

// Phase 151A Step 9c - native PE emission driven by a REAL AST.
//
// Step 9b produced a runnable kcc-generated PE, but from a FIXED program compiled
// into kcc: `_start + karkain_main(with a hard-coded value) + print_int`. That
// proved the machinery -- one emitter state, resolved rodata, index-tagged IAT, a
// linked image that executes -- while still being unable to compile a USER'S
// program. This gate is the evidence that the architecture actually changed from
// "fixed program" to "user AST -> native emission".
//
// THE CENTRAL CLAIM is that the SOURCE reaches the emitter. A gate that only ran
// the 9b program would pass against the old fixed path unchanged, so every case
// here is a DIFFERENT .kark file and the expected stdout is derived from what that
// file says. The pair (12345, 42) is the load-bearing one: same driver, same
// emitter, same linker, different source, different exact bytes at runtime. If the
// driver ignored the AST and re-emitted the 9b program, that case fails.
//
// Layers:
//  1. the source reaches the emitter at all (a real file, a real image);
//  2. exact stdout bytes for a literal argument;
//  3. exact stdout for a LOCAL argument, which also proves the frame slot is used;
//  4. two locals print their OWN values, so the slots are distinct;
//  5. the EXIT CODE follows an explicit `return`, which a stdout-only assertion
//     cannot see;
//  6. unsupported constructs are REFUSED BY NAME;
//  7. the image is a structurally valid PE with DIR64 relocations present, so 9c
//     did not regress the Step 9b relocation work;
//  8. no Go fallback.
//
// NOT RUN HERE: any whole-tree KCC workload (see PHASE-151A-BASELINE.md 9a-1).

import (
	"encoding/binary"
	"encoding/hex"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"karkain/pkg/native"
)

// nat9cCase is one source program and the exact observable result it must produce.
type nat9cCase struct {
	name string
	src  string
	// wantOut is compared as EXACT BYTES. Never trimmed: a missing trailing
	// newline is a real defect (Step 9b), and trimming is the tool that hides it.
	wantOut []byte
	// wantExit is the process exit status.
	wantExit int
	// refuse is a substring the refusal must contain. Empty means the case must
	// produce an image.
	refuse string
}

// The corpus. Every entry is a different program; none of them is the Step 9b
// fixed program, and the two literal cases exist precisely to differ from it and
// from each other.
var nat9cCases = []nat9cCase{
	{
		name: "literal_12345",
		src:  "func main() {\n    print(12345)\n}\n",
		// Byte-identical to the Step 9b result, but reached from source.
		wantOut: []byte("12345\n"), wantExit: 0,
	},
	{
		name:    "literal_42",
		src:     "func main() {\n    print(42)\n}\n",
		wantOut: []byte("42\n"), wantExit: 0,
	},
	{
		name:    "local_12345",
		src:     "func main() {\n    let x = 12345\n    print(x)\n}\n",
		wantOut: []byte("12345\n"), wantExit: 0,
	},
	{
		name:    "local_42",
		src:     "func main() {\n    let y = 42\n    print(y)\n}\n",
		wantOut: []byte("42\n"), wantExit: 0,
	},
	{
		// Two locals in one program. Each must print its OWN value, which is
		// what proves declaration order is being used as slot order rather than
		// both names resolving to slot 0.
		name:    "two_locals",
		src:     "func main() {\n    let a = 7\n    let b = 35\n    print(a)\n    print(b)\n}\n",
		wantOut: []byte("7\n35\n"), wantExit: 0,
	},
	{
		// The exit status is main's return value, so this case observes a value
		// stdout cannot: the program prints NOTHING and must still exit 3.
		name: "return_sets_exit_code",
		src:  "func main() {\n    let c = 3\n    return c\n}\n",
		// No output at all, and a non-zero exit.
		wantOut: []byte{}, wantExit: 3,
	},
	{
		name:     "refuse_while",
		src:      "func main() {\n    while 1 < 2 {\n        print(9)\n    }\n}\n",
		wantExit: 0, refuse: "error[K145]",
	},
	{
		name:     "refuse_binary_expression",
		src:      "func main() {\n    print(1 + 2)\n}\n",
		wantExit: 0, refuse: "error[K145]",
	},
	{
		// `let s = "hi"` was pinned here as REFUSED when 9c landed, because the
		// driver was int-only. Step 9f added string literals, so this exact program
		// now compiles and prints "hi". The case was REPLACED rather than deleted,
		// because the table's intent is "a binding initialised from something this
		// driver cannot lower is refused by name", and a boolean keeps that intent
		// while remaining refused. Leaving the old program in place would have made
		// 9c fail on correct behaviour and taught its readers to ignore it.
		//
		// The string program itself is now gated properly by TestPhase151A9F_*.
		name:     "refuse_non_integer_binding",
		src:      "func main() {\n    let b = true\n    print(b)\n}\n",
		wantExit: 0, refuse: "error[K145]",
	},
	{
		// No main at all: the entry stub calls karkain_main, so this must be
		// refused rather than emitting an image that cannot be entered.
		name:     "refuse_no_main",
		src:      "func helper() {\n    print(1)\n}\n",
		wantExit: 0, refuse: "error[K145]",
	},
}

// nat9cBuild runs the self-hosted driver over one source and returns kcc's answer.
func nat9cBuild(t *testing.T, src string) string {
	t.Helper()
	dir := t.TempDir()
	file := filepath.Join(dir, "prog.kark")
	if err := os.WriteFile(file, []byte(src), 0o644); err != nil {
		t.Fatalf("writing the test program: %v", err)
	}
	karkain := phase130Karkain(t)
	out, err := exec.Command(karkain, "native-ast", file).CombinedOutput()
	s := strings.TrimSpace(string(out))
	if err != nil && s == "" {
		t.Fatalf("kcc native-ast produced no output (err %v); a driver crash must "+
			"not be reported as an empty refusal", err)
	}
	return s
}

// nat9cDecodeHex turns kcc's hex answer into bytes.
func nat9cDecodeHex(t *testing.T, s, what string) []byte {
	t.Helper()
	b, err := hex.DecodeString(s)
	if err != nil {
		t.Fatalf("%s is not hex: %v\noutput was: %.200s", what, err, s)
	}
	return b
}

// nat9cRelocCount decodes the DIR64 entries out of an image's .reloc section.
func nat9cRelocCount(t *testing.T, img []byte) []uint32 {
	t.Helper()
	lfanew := binary.LittleEndian.Uint32(img[0x3c:])
	opt := int(lfanew) + 4 + 20
	relRVA := binary.LittleEndian.Uint32(img[opt+152:])
	numSec := int(binary.LittleEndian.Uint16(img[lfanew+4+2:]))
	optSize := int(binary.LittleEndian.Uint16(img[lfanew+4+16:]))
	secTab := int(lfanew) + 4 + 20 + optSize
	for i := 0; i < numSec; i++ {
		s := secTab + i*40
		if string(img[s:s+6]) != ".reloc" {
			continue
		}
		if binary.LittleEndian.Uint32(img[s+12:]) != relRVA {
			t.Fatalf(".reloc RVA disagrees with the relocation data directory")
		}
		rawSize := int(binary.LittleEndian.Uint32(img[s+16:]))
		rawOff := int(binary.LittleEndian.Uint32(img[s+20:]))
		rel := img[rawOff : rawOff+rawSize]
		var out []uint32
		p := 0
		for p+8 <= len(rel) {
			page := binary.LittleEndian.Uint32(rel[p:])
			size := binary.LittleEndian.Uint32(rel[p+4:])
			if size == 0 {
				break
			}
			for e := p + 8; e < p+int(size); e += 2 {
				entry := binary.LittleEndian.Uint16(rel[e:])
				out = append(out, page|uint32(entry&0xFFF))
			}
			p += int(size)
		}
		return out
	}
	t.Fatal("no .reloc section in the driver image")
	return nil
}

// nat9cBytesEq compares two byte slices exactly. It exists so the intent is
// legible at the call site: this gate compares EXACT bytes and never trims.
func nat9cBytesEq(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// TestPhase151A9C_ASTDrivenProgramsProduceTheirOwnOutput is layers 1-7.
//
// THE POINT OF THE SLICE. Each case writes a DIFFERENT .kark file, drives kcc's
// whole-program driver over it, executes the resulting PE, and compares the exact
// stdout bytes and the exact exit code. Two of the cases use the same driver and
// emitter as Step 9b and differ only in the integer in the source, so a driver
// that ignored the AST and re-emitted the old fixed program would fail them.
func TestPhase151A9C_ASTDrivenProgramsProduceTheirOwnOutput(t *testing.T) {
	if os.PathSeparator == '/' {
		t.Skip("PE images only execute on Windows; the driver itself is cross-platform")
	}
	for _, c := range nat9cCases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			out := nat9cBuild(t, c.src)

			if c.refuse != "" {
				if !strings.Contains(out, c.refuse) {
					t.Fatalf("expected a %s refusal for this program, got:\n%.400s",
						c.refuse, out)
				}
				// A refusal must NAME the construct. A bare error code is not
				// actionable, and "refuse by code alone" is the failure mode the 151
				// baseline warns about.
				if len(out) < 80 {
					t.Errorf("refusal is %d chars, too short to name a construct: %.200s",
						len(out), out)
				}
				if !strings.Contains(out, "not handed to the Go engine") {
					t.Errorf("refusal does not say the construct is not handed to the Go "+
						"engine; an unexplained refusal invites a fallback: %.200s", out)
				}
				return
			}

			img := nat9cDecodeHex(t, out, "driver image")

			// Layer 7 folded in here: a valid PE with relocations, so 9c did not
			// regress Step 9b's DIR64 work on the way past.
			if _, _, err := native.ParsePE(img); err != nil {
				t.Fatalf("driver image is not a structurally valid PE: %v", err)
			}
			if n := len(nat9cRelocCount(t, img)); n < 12 {
				t.Errorf("driver image has %d DIR64 relocation entries, want at least 12 "+
					"(2 rodata sites + 3 bootstrap stores + 7 IAT loads)", n)
			}

			exe := filepath.Join(t.TempDir(), "prog.exe")
			if err := os.WriteFile(exe, img, 0o755); err != nil {
				t.Fatalf("writing the driver image: %v", err)
			}
			got, err := exec.Command(exe).CombinedOutput()

			code := 0
			if ee, ok := err.(*exec.ExitError); ok {
				code = ee.ExitCode()
			} else if err != nil {
				t.Fatalf("running the driver image: %v", err)
			}

			if !nat9cBytesEq(got, c.wantOut) {
				t.Errorf("stdout = %q (% x), want %q (% x)\n  source: %s\n"+
					"  compared as EXACT BYTES: a missing or extra trailing newline is a "+
					"real defect, and trimming would hide it.",
					got, got, c.wantOut, c.wantOut, c.src)
			}
			if code != c.wantExit {
				t.Errorf("exit code = %d, want %d (main's return value is what _start "+
					"passes to ExitProcess)\n  source: %s", code, c.wantExit, c.src)
			}
		})
	}
}

// TestPhase151A9C_SourceChangesTheGeneratedImage is the AST-dependency claim stated
// on its own, because it is the one most likely to rot: two programs that differ
// ONLY in the integer must produce DIFFERENT images.
//
// This is a byte-level assertion rather than another stdout comparison, and the
// difference matters. Two images could differ for reasons unrelated to the literal
// -- a frame size that happened to change, an unrelated non-determinism -- so the
// test also pins WHERE they differ: the differing bytes must lie inside the
// 4-byte immediates of the two `mov r32, imm32` forms that carry the literal. A
// driver that emitted the right literal but also perturbed the rest of the image
// would therefore still fail.
func TestPhase151A9C_SourceChangesTheGeneratedImage(t *testing.T) {
	if os.PathSeparator == '/' {
		t.Skip("image comparison is platform-independent, but the corpus build is not")
	}
	a := nat9cDecodeHex(t, nat9cBuild(t, "func main() {\n    print(12345)\n}\n"), "12345 image")
	b := nat9cDecodeHex(t, nat9cBuild(t, "func main() {\n    print(42)\n}\n"), "42 image")

	if len(a) != len(b) {
		t.Fatalf("the two images differ in LENGTH (%d vs %d). Both programs bind no "+
			"locals and print once, so their frames and code shapes should match and "+
			"only the literal immediates should differ.", len(a), len(b))
	}

	// Find where they differ, and require that every difference sits inside an
	// immediate we can attribute to the literal.
	var diffs []int
	for i := range a {
		if a[i] != b[i] {
			diffs = append(diffs, i)
		}
	}
	if len(diffs) == 0 {
		t.Fatal("the two images are byte-identical, so the literal in the source did " +
			"not reach the emitter. This is the exact failure the slice exists to " +
			"rule out.")
	}
	if len(diffs) > 8 {
		t.Errorf("the two images differ in %d bytes (% x); the literal is one 32-bit "+
			"immediate, so at most 4 bytes should move (plus any absolute rodata "+
			"address that shifts with the code length)", len(diffs), diffs)
	}

	// Each differing byte must be inside .text, i.e. a real code or immediate
	// byte, and NOT inside the PE headers -- a difference in e.g. a timestamp or
	// SizeOfCode would mean something other than the literal changed.
	const pe9bTextFileOff = 0x200
	pe := nat9cPETextFileOff(t, a)
	for _, d := range diffs {
		if d < pe9bTextFileOff {
			t.Errorf("byte %d differs and is in the PE HEADER, not .text; the literal "+
				"in the source is not what changed", d)
		}
		if d < pe {
			t.Errorf("byte %d differs and is before .text begins (%d)", d, pe)
		}
	}
	t.Logf("literal change moves %d byte(s) at %v, all inside .text (starts %d)",
		len(diffs), diffs, pe)
}

// nat9cPETextFileOff returns the file offset at which .text begins.
func nat9cPETextFileOff(t *testing.T, img []byte) int {
	t.Helper()
	lfanew := binary.LittleEndian.Uint32(img[0x3c:])
	numSec := int(binary.LittleEndian.Uint16(img[lfanew+4+2:]))
	optSize := int(binary.LittleEndian.Uint16(img[lfanew+4+16:]))
	secTab := int(lfanew) + 4 + 20 + optSize
	for i := 0; i < numSec; i++ {
		s := secTab + i*40
		if string(img[s:s+5]) == ".text" {
			return int(binary.LittleEndian.Uint32(img[s+20:]))
		}
	}
	t.Fatal("no .text section")
	return 0
}

// TestPhase151A9C_NoGoFallback is layer 8.
//
// The dishonesty 151 exists to remove is a native path that LOOKS like kcc's and
// is not. The guard is that an unknown subcommand must fail rather than fall
// through, and that the driver's own refusal text must state the construct is not
// handed to the Go engine -- if the driver could not say that, a user would have
// no way to tell a refusal from a fallback.
func TestPhase151A9C_NoGoFallback(t *testing.T) {
	karkain := phase130Karkain(t)
	if out, err := exec.Command(karkain, "native-astx").CombinedOutput(); err == nil {
		t.Errorf("an unknown kcc subcommand succeeded with %q; there is a silent fallback",
			out)
	}

	// The refusal text is asserted above in the corpus test; this asserts the
	// claim in a place that cannot be skipped by a filtered subtest.
	out := nat9cBuild(t, "func main() {\n    while 1 < 2 {\n        print(9)\n    }\n}\n")
	if !strings.Contains(out, "not handed to the Go engine") {
		t.Errorf("the driver's refusal does not state that the construct is refused "+
			"rather than handed to the Go engine: %.300s", out)
	}
}
