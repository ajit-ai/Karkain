package cli

// Phase 151A Step 3 - native CONTROL-FLOW lowering in kcc.
//
// Contract: the self-hosted loop lowering must produce the SAME BYTES as the
// Go oracle (pkg/native), not merely equivalent control flow. The oracle's
// surface is reproduced rather than widened:
//
//   * while, C-style for, break and continue are the four supported forms.
//   * operands are INT ONLY. No float, array, string, map or record operand
//     reaches a loop, and none of those lowerings exists yet.
//   * integer arithmetic in a loop body is `+` only, reusing the Step 2
//     primitive. Nothing new is introduced here.
//
// FOUR INDEPENDENT LAYERS, each catching a different class of error:
//
//  1. kcc's bytes vs the .text the oracle produced by COMPILING the same
//     source. This is the real differential, and it is why the corpus is
//     written as Karkain source rather than as expected bytes.
//  2. the frame immediate, derived independently in THIS file from first
//     principles, so a layout wrong in the same way on both sides is caught.
//  3. opcodes stated from the Intel SDM, so a mistake BOTH engines share
//     cannot pass.
//  4. LABEL NAMES AND ALLOCATION ORDER, pinned as data.
//
// Layer 4 exists because of an x86 property that makes the other three
// insufficient alone: a rel32 encodes only a DISPLACEMENT. Renumbering a label
// can leave every displacement identical -- the byte stream is then
// legitimately equal while the label discipline is wrong. The names are the
// only layer that distinguishes those two situations, which is why
// `native-value-loop-labels` is a separate surface.
//
// A single layer is not enough either: kcc's corpus is a compiled-in SHAPE
// rather than a parsed program (no AST-driven native driver exists yet), so
// layer 1 ties each shape to a real program and layers 3-4 keep the shape
// itself honest.

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"strings"
	"testing"

	"karkain/pkg/lexer"
	"karkain/pkg/native"
	"karkain/pkg/parser"
)

// step3Corpus pairs each kcc loop shape with the Karkain source the oracle must
// be given, the number of int locals that sizes its frame, the label names the
// oracle allocates (in allocation order), and opcodes stated from the Intel
// SDM.
//
// The sources are the ten reference programs; nLocals is the number of `let`
// declarations a reader can count in the source, INCLUDING a `let` inside a
// loop body, because the oracle allocates a frame slot for it.
var step3Corpus = []struct {
	name    string
	src     string
	nLocals int
	labels  []string
	stated  []string
}{
	{
		name:    "while_simple",
		src:     "func main() {\n    let i = 0\n    while i < 5 {\n        i = i + 1\n    }\n    return i\n}\n",
		nLocals: 1,
		labels:  []string{"while$1", "whileend$2"},
		// sub rsp,imm32 / mov rax,imm64 / mov [rsp],rax / mov rax,[rsp] /
		// mov [rsp+8],rax / mov rax,imm64 / mov rcx,rax / mov rax,[rsp+8] /
		// cmp rax,rcx / jge rel32 / add rax,rcx / mov [rsp],rax / jmp rel32 /
		// mov rax,[rsp] / jmp rel32 / add rsp,imm32 / ret
		stated: []string{"4881ec", "48b8", "48890424", "488b0424", "4889442408", "4889c1", "4839c8", "0f8d", "4801c8", "e9", "4881c4", "c3"},
	},
	{
		// s = s + i : the RIGHT operand is a LOCAL, so the second operand
		// read is mov rax,[rsp] (slot 0) rather than a literal move.
		name:    "while_arith",
		src:     "func main() {\n    let i = 0\n    let s = 0\n    while i < 5 {\n        s = s + i\n        i = i + 1\n    }\n    return s\n}\n",
		nLocals: 2,
		labels:  []string{"while$1", "whileend$2"},
		stated:  []string{"4881ec", "48b8", "48890424", "4889442408", "488b0424", "4889442410", "4889c1", "4839c8", "0f8d", "4801c8", "e9", "c3"},
	},
	{
		// The `if` allocates BOTH labels even with no else branch.
		name:    "while_break",
		src:     "func main() {\n    let i = 0\n    while i < 10 {\n        i = i + 1\n        if i == 5 {\n            break\n        }\n    }\n    return i\n}\n",
		nLocals: 1,
		labels:  []string{"while$1", "whileend$2", "ifelse$3", "ifend$4"},
		// jne rel32 (the inverse of ==) then jmp rel32 for the break and
		// jmp rel32 for emitIf's own unconditional jump to the end label.
		stated: []string{"0f85", "e9", "4889442408", "4801c8", "4839c8"},
	},
	{
		// continue targets the loop HEAD (while$1), not the end label.
		name:    "while_continue",
		src:     "func main() {\n    let i = 0\n    let s = 0\n    while i < 6 {\n        i = i + 1\n        if i == 3 {\n            continue\n        }\n        s = s + i\n    }\n    return s\n}\n",
		nLocals: 2,
		labels:  []string{"while$1", "whileend$2", "ifelse$3", "ifend$4"},
		stated:  []string{"0f85", "e9", "4801c8", "4839c8"},
	},
	{
		// A C-style for allocates THREE labels, post SECOND.
		name:    "for_simple",
		src:     "func main() {\n    let i = 0\n    for (i = 0; i < 5; i = i + 1) {\n    }\n    return i\n}\n",
		nLocals: 1,
		labels:  []string{"for$1", "forpost$2", "forend$3"},
		stated:  []string{"4881ec", "48b8", "488b0424", "4889442408", "4839c8", "0f8d", "4801c8", "e9", "c3"},
	},
	{
		// `let s` is declared FIRST, so s is slot 0 and the for-init's i is
		// slot 1 -- the reverse of while_arith.
		name:    "for_full",
		src:     "func main() {\n    let s = 0\n    for (let i = 0; i < 5; i = i + 1) {\n        s = s + i\n    }\n    return s\n}\n",
		nLocals: 2,
		labels:  []string{"for$1", "forpost$2", "forend$3"},
		stated:  []string{"4881ec", "48b8", "48890424", "4889442408", "488b442408", "4889c1", "4839c8", "0f8d", "4801c8", "e9", "c3"},
	},
	{
		name:    "for_break",
		src:     "func main() {\n    let i = 0\n    for (i = 0; i < 10; i = i + 1) {\n        if i == 5 {\n            break\n        }\n    }\n    return i\n}\n",
		nLocals: 1,
		labels:  []string{"for$1", "forpost$2", "forend$3", "ifelse$4", "ifend$5"},
		stated:  []string{"0f85", "e9", "4839c8", "0f8d", "4801c8"},
	},
	{
		// THE ASYMMETRY CASE: continue targets forpost$2, NOT for$1, because
		// a C-style for's continuation label is its post clause.
		name:    "for_continue",
		src:     "func main() {\n    let s = 0\n    for (let i = 0; i < 6; i = i + 1) {\n        if i == 3 {\n            continue\n        }\n        s = s + i\n    }\n    return s\n}\n",
		nLocals: 2,
		labels:  []string{"for$1", "forpost$2", "forend$3", "ifelse$4", "ifend$5"},
		stated:  []string{"0f85", "e9", "4839c8", "0f8d", "4801c8"},
	},
	{
		// THREE locals: the inner `let j` occupies a frame slot even though it
		// is declared inside the outer body. The shared counter gives the
		// inner loop $3/$4 while the outer keeps $1/$2.
		name:    "nested_while",
		src:     "func main() {\n    let i = 0\n    let t = 0\n    while i < 3 {\n        let j = 0\n        while j < 2 {\n            t = t + 1\n            j = j + 1\n        }\n        i = i + 1\n    }\n    return t\n}\n",
		nLocals: 3,
		labels:  []string{"while$1", "whileend$2", "while$3", "whileend$4"},
		stated:  []string{"4881ec", "48b8", "4889442418", "4839c8", "0f8d", "4801c8", "e9", "c3"},
	},
	{
		// for OUTER, while INNER: pins both shared-counter rules at once.
		name:    "nested_for_while",
		src:     "func main() {\n    let t = 0\n    for (let i = 0; i < 2; i = i + 1) {\n        let j = 0\n        while j < 2 {\n            t = t + 1\n            j = j + 1\n        }\n    }\n    return t\n}\n",
		nLocals: 3,
		labels:  []string{"for$1", "forpost$2", "forend$3", "while$4", "whileend$5"},
		stated:  []string{"4881ec", "48b8", "4889442418", "4839c8", "0f8d", "4801c8", "e9", "c3"},
	},
}

// oracleTextLoop compiles a reference program with the Go oracle and returns
// its .text. The oracle is the reference; kcc's bytes must appear in it.
func oracleTextLoop(t *testing.T, src string) []byte {
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

func countOccLoop(hay, needle []byte) int {
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

// step3Frame derives the expected frame for a function with nLocals int
// locals: 8 bytes per local, plus binTemp (8 * 64 = 512), no strTemp, no
// mapStage, no extras, no Windows 16-byte rounding, plus the fixed 96-byte
// argument spill. Written out longhand so it is an INDEPENDENT expectation
// rather than a call into the implementation under test.
func step3Frame(nLocals int) int {
	const binTemp = 8 * 64
	const argSpill = 96
	return 8*nLocals + binTemp + argSpill
}

// TestPhase151A3_ControlFlowByteIdentical is the main Step 3 gate: layers 1-3.
func TestPhase151A3_ControlFlowByteIdentical(t *testing.T) {
	karkain := phase130Karkain(t)
	got := runKCCStep2(t, karkain, "native-value-loop")
	if len(got) != len(step3Corpus) {
		t.Fatalf("kcc native-value-loop produced %d lines, want %d:\n%v", len(got), len(step3Corpus), got)
	}

	for i, tc := range step3Corpus {
		tc, i := tc, i
		t.Run(tc.name, func(t *testing.T) {
			kccHex := got[i]
			kccBytes, err := hex.DecodeString(kccHex)
			if err != nil {
				t.Fatalf("kcc emitted non-hex %q: %v", kccHex, err)
			}

			// Layer 1: kcc's bytes must appear VERBATIM, exactly once, in
			// the .text the Go oracle produced by compiling this source.
			text := oracleTextLoop(t, tc.src)
			if n := countOccLoop(text, kccBytes); n != 1 {
				t.Errorf("kcc's %d function bytes occur %d times in the oracle's %d-byte .text, want exactly 1\n kcc = %s",
					len(kccBytes), n, len(text), kccHex)
			}

			// Layer 2: the frame immediate, derived independently above.
			frame := step3Frame(tc.nLocals)
			if !strings.Contains(kccHex, le32Hex(frame)) {
				t.Errorf("kcc's frame immediate is not the derived %d (0x%x)\n kcc = %s", frame, frame, kccHex)
			}

			// Layer 3: opcodes stated from the Intel SDM.
			for _, w := range tc.stated {
				if !strings.Contains(kccHex, w) {
					t.Errorf("missing stated opcode bytes %s\n kcc = %s", w, kccHex)
				}
			}
		})
	}
}

// TestPhase151A3_LabelAllocation is layer 4: the label NAMES and their
// allocation ORDER, pinned as data.
//
// This is the layer the byte differential cannot supply. A rel32 carries only
// a displacement, so a renumbered label can produce a byte-for-byte identical
// stream; only the names distinguish that from a correct implementation.
//
// The suffix check is the part that catches a REAL bug: a per-loop counter
// would restart at $1 for each loop and would still produce the right names in
// a single-loop program, but it fails here on the nested cases, where the
// inner loop must continue the shared sequence at $3/$4 or $4/$5.
func TestPhase151A3_LabelAllocation(t *testing.T) {
	karkain := phase130Karkain(t)
	got := runKCCStep2(t, karkain, "native-value-loop-labels")
	if len(got) != len(step3Corpus) {
		t.Fatalf("kcc native-value-loop-labels produced %d lines, want %d:\n%v", len(got), len(step3Corpus), got)
	}

	for i, tc := range step3Corpus {
		tc, i := tc, i
		t.Run(tc.name, func(t *testing.T) {
			want := strings.Join(tc.labels, " ")
			if got[i] != want {
				t.Errorf("label names/order:\n kcc = %q\nwant = %q", got[i], want)
			}
			fields := strings.Fields(got[i])
			if len(fields) != len(tc.labels) {
				t.Fatalf("kcc reported %d labels, want %d: %q", len(fields), len(tc.labels), got[i])
			}
			// The suffix must be a strictly increasing sequence starting at 1,
			// which is what "ONE shared counter, allocated at loop entry" means.
			for j, name := range fields {
				wantSuffix := fmt.Sprintf("$%d", j+1)
				if !strings.HasSuffix(name, wantSuffix) {
					t.Errorf("label %q should carry counter suffix %q (one shared counter, allocated in entry order); full set %q",
						name, wantSuffix, got[i])
				}
			}
		})
	}
}

// TestPhase151A3_BreakAndContinuePresent states the property that distinguishes
// `break` from `continue`, which the byte differential cannot on its own: the
// four break/continue cases must each carry a loop-label set PLUS the if's own
// label pair, and the if pair must come from the SAME shared counter.
//
// It exists so that a corpus edit which silently drops a break/continue case
// fails loudly here rather than shrinking the corpus unnoticed.
func TestPhase151A3_BreakAndContinuePresent(t *testing.T) {
	karkain := phase130Karkain(t)
	got := runKCCStep2(t, karkain, "native-value-loop-labels")
	if len(got) != len(step3Corpus) {
		t.Fatalf("kcc native-value-loop-labels produced %d lines, want %d", len(got), len(step3Corpus))
	}
	// while_break(2), while_continue(3): 2 loop labels + 2 if labels = 4.
	// for_break(6), for_continue(7):   3 loop labels + 2 if labels = 5.
	// The FIRST draft of this test asserted 5 for all four and index 2 for the
	// if pair; both were wrong, and the oracle settled it -- a while allocates
	// two loop labels and a for three, so the if pair starts at index 2 for a
	// while and index 3 for a for. An expectation written from memory rather
	// than from the oracle is the failure mode this gate exists to catch.
	for _, tc := range []struct {
		idx     int
		wantLen int
		ifIdx   int
	}{
		{2, 4, 2},
		{3, 4, 2},
		{6, 5, 3},
		{7, 5, 3},
	} {
		fields := strings.Fields(got[tc.idx])
		if len(fields) != tc.wantLen {
			t.Errorf("case %d (%s) should report %d labels (loop labels + if pair), got %d: %q",
				tc.idx, step3Corpus[tc.idx].name, tc.wantLen, len(fields), got[tc.idx])
			continue
		}
		if !strings.HasPrefix(fields[tc.ifIdx], "ifelse$") {
			t.Errorf("case %d (%s) label %d should be the if's else label, got %q",
				tc.idx, step3Corpus[tc.idx].name, tc.ifIdx, fields[tc.ifIdx])
		}
	}
}
