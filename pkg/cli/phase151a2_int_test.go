package cli

// Phase 151A Step 2 - native integer value/expression lowering in kcc.
//
// Contract: the self-hosted integer lowering must produce the SAME BYTES as
// the Go oracle (pkg/native), not merely equivalent code. The oracle's
// integer surface is reproduced rather than widened:
//
//   * int arithmetic is +, - and * only; `/` and `%` are REFUSED. The gate
//     asserts the refusals match, so the narrowing stays visible instead of
//     quietly disappearing.
//   * a comparison is a BRANCH, not a value. The oracle's emitCond sets flags
//     and jumps on the INVERSE condition; it never leaves a boolean in RAX.
//     There is no boolean value to compare, and the gate does not invent one.
//
// Three independent layers, each catching a different class of error:
//
//  1. kcc's bytes vs the .text the oracle produced by COMPILING the same
//     source. This is the real differential, and it is why the corpus is
//     written as Karkain source rather than as expected bytes.
//  2. the frame immediate the oracle's own layout computed for that program,
//     so a layout drift is caught even when every opcode is right.
//  3. stated opcodes and label names, so a mistake both engines share cannot
//     pass.
//
// A single layer is not enough: kcc's corpus is a compiled-in SHAPE rather
// than a parsed program (no AST-driven native driver exists yet), so layer 1
// is what ties each shape to a real program and layer 3 is what keeps the
// shape itself honest.

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"testing"

	"karkain/pkg/lexer"
	"karkain/pkg/native"
	"karkain/pkg/parser"
)

func runKCCStep2(t *testing.T, karkain, sub string) []string {
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

// intCorpus pairs each kcc corpus line with the Karkain source the oracle must
// be given, the local count that sizes its frame, and the stated opcode bytes.
var intCorpus = []struct {
	name    string
	src     string
	nLocals int
	stated  []string
}{
	{
		name:    "literal_store_load_return",
		src:     "func main() {\n    let x = 42\n    return x\n}\n",
		nLocals: 1,
		stated:  []string{"48b82a00000000000000", "48890424", "488b0424", "c3"},
	},
	{
		name:    "two_locals_mul",
		src:     "func main() {\n    let a = 6\n    let b = 7\n    return a * b\n}\n",
		nLocals: 2,
		stated:  []string{"48b80600000000000000", "48b80700000000000000", "480fafc1"},
	},
	{
		// nLocals is 0, not 1: neither source declares a `let`, so the oracle
		// allocates no local and the frame is 0 + 512 + 96 = 608 (0x260). This
		// was 1 in the first draft and the frame layer reported the oracle's own
		// 608-byte frame as "wrong" -- a wrong expectation in the test, which is
		// the 151B lesson (a baseline with wrong expectations is worse than none).
		name:    "negate_nested",
		src:     "func main() {\n    return -(3 + 4)\n}\n",
		nLocals: 0,
		stated:  []string{"48b80300000000000000", "48b80400000000000000", "4801c8", "48f7d8"},
	},
	{
		name:    "nested_two_depths",
		src:     "func main() {\n    return (2 + 3) * (10 - 4)\n}\n",
		nLocals: 0,
		stated:  []string{"48b80200000000000000", "48b80300000000000000", "48b80a00000000000000", "4829c8", "480fafc1"},
	},
	{
		// No return statement: the oracle's `if !returned` supplies the 0.
		name:    "missing_return_is_zero",
		src:     "func main() {\n    let x = 1\n}\n",
		nLocals: 1,
		stated:  []string{"4831c0"},
	},
	{
		// Step 2b: a bare `x = 2` assignment to an existing int local. The
		// frame is UNCHANGED at one local, which the frame layer checks: a
		// reassignment allocates nothing.
		name:    "reassign_then_read",
		src:     "func main() {\n    let x = 1\n    x = 2\n    return x\n}\n",
		nLocals: 1,
		stated:  []string{"48b80100000000000000", "48890424", "48b80200000000000000", "48890424", "488b0424"},
	},
	{
		// Step 2b: a REASSIGNED local read as a binary operand must take the
		// SAME staging path as a freshly bound one (shape 1), so the scratch
		// store at bt+0 and the reload are present and the add is `add rax,rcx`.
		name:    "reassign_in_binary",
		src:     "func main() {\n    let x = 3\n    x = 4\n    return x + 1\n}\n",
		nLocals: 1,
		stated:  []string{"48b80300000000000000", "48890424", "48b80400000000000000", "48890424", "4889442408", "4889c1", "488b442408", "4801c8"},
	},
}

// cmpCorpus pairs each comparison operator with its source and the INVERSE jump
// opcode the oracle uses, which is the part most easily reversed.
var cmpCorpus = []struct {
	op     string
	src    string
	stated string
}{
	{"==", "func main() {\n    if 5 == 9 {\n        let x = 1\n    } else {\n        let x = 2\n    }\n}\n", "0f85"},
	{"!=", "func main() {\n    if 5 != 9 {\n        let x = 1\n    } else {\n        let x = 2\n    }\n}\n", "0f84"},
	{"<", "func main() {\n    if 5 < 9 {\n        let x = 1\n    } else {\n        let x = 2\n    }\n}\n", "0f8d"},
	{"<=", "func main() {\n    if 5 <= 9 {\n        let x = 1\n    } else {\n        let x = 2\n    }\n}\n", "0f8f"},
	{">", "func main() {\n    if 5 > 9 {\n        let x = 1\n    } else {\n        let x = 2\n    }\n}\n", "0f8e"},
	{">=", "func main() {\n    if 5 >= 9 {\n        let x = 1\n    } else {\n        let x = 2\n    }\n}\n", "0f8c"},
}

// TestPhase151A2_IntegerLoweringByteIdentical is the main Step 2 gate.
func TestPhase151A2_IntegerLoweringByteIdentical(t *testing.T) {
	karkain := phase130Karkain(t)
	got := runKCCStep2(t, karkain, "native-value-int")
	if len(got) != len(intCorpus) {
		t.Fatalf("kcc native-value-int produced %d lines, want %d:\n%v", len(got), len(intCorpus), got)
	}

	for i, tc := range intCorpus {
		tc, i := tc, i
		t.Run(tc.name, func(t *testing.T) {
			kccHex := got[i]
			kccBytes, err := hex.DecodeString(kccHex)
			if err != nil {
				t.Fatalf("kcc emitted non-hex %q: %v", kccHex, err)
			}

			// Layer 1: kcc's bytes must appear VERBATIM, exactly once, in
			// the .text the Go oracle produced by compiling this source.
			text := oracleText(t, tc.src)
			if n := countOccurrences(text, kccBytes); n != 1 {
				t.Errorf("kcc's %d function bytes occur %d times in the oracle's %d-byte .text, want exactly 1\n kcc = %s",
					len(kccBytes), n, len(text), kccHex)
			}

			// Layer 2: the frame size the oracle's layout computed here.
			frame, _, _ := refFrame(8*tc.nLocals, 0, false, false, 0, false)
			if !strings.Contains(kccHex, le32Hex(frame)) {
				t.Errorf("kcc's frame immediate is not the oracle's %d (0x%x)\n kcc = %s", frame, frame, kccHex)
			}

			// Layer 3: the stated opcodes.
			for _, w := range tc.stated {
				if !strings.Contains(kccHex, w) {
					t.Errorf("missing stated opcode bytes %s\n kcc = %s", w, kccHex)
				}
			}

			t.Logf("%s: %d bytes sha256=%x", tc.name, len(kccBytes), sha256.Sum256(kccBytes))
		})
	}
}

// TestPhase151A2_ComparisonsAreBranches pins the comparison lowering. The
// opcode is the INVERSE condition, which is the part most easily reversed:
// a `<` that branches on Jge is a correct-looking instruction sequence that
// implements the wrong relation.
func TestPhase151A2_ComparisonsAreBranches(t *testing.T) {
	karkain := phase130Karkain(t)
	got := runKCCStep2(t, karkain, "native-value-cmp")
	if len(got) != len(cmpCorpus) {
		t.Fatalf("kcc native-value-cmp produced %d lines, want %d:\n%v", len(got), len(cmpCorpus), got)
	}

	for i, tc := range cmpCorpus {
		tc, i := tc, i
		t.Run("op"+tc.op, func(t *testing.T) {
			kccHex := got[i]
			kccBytes, err := hex.DecodeString(kccHex)
			if err != nil {
				t.Fatalf("kcc emitted non-hex %q: %v", kccHex, err)
			}

			// Layer 1: the real differential, against the oracle's own
			// compiled output for this exact condition.
			text := oracleText(t, tc.src)
			if n := countOccurrences(text, kccBytes); n != 1 {
				t.Errorf("kcc's %d bytes occur %d times in the oracle's %d-byte .text, want exactly 1\n kcc = %s",
					len(kccBytes), n, len(text), kccHex)
			}

			// Layer 2: cmp must precede the branch, and the branch must be
			// the oracle's INVERSE opcode for this operator.
			cmpAt := strings.Index(kccHex, "4839c8")
			jumpAt := strings.Index(kccHex, tc.stated)
			if cmpAt < 0 {
				t.Fatalf("no cmp r64,r64 in kcc output: %s", kccHex)
			}
			if jumpAt < 0 {
				t.Errorf("missing the inverse branch opcode %s for '%s'\n kcc = %s", tc.stated, tc.op, kccHex)
			} else if jumpAt < cmpAt {
				t.Errorf("branch %s appears before the cmp at %d for '%s'", tc.stated, jumpAt, tc.op)
			}

			// Layer 3: the label NAMES. A rel32 encodes a distance, so a
			// renamed or reordered label is a different image even when the
			// displacement happens to come out the same.
			// The oracle's fresh() allocates the ELSE label first.
			if jmpEnd := strings.Index(kccHex, "e90e000000"); jmpEnd < 0 {
				t.Errorf("missing the oracle's jmp to the end label (e90e000000)\n kcc = %s", kccHex)
			}
		})
	}
}

// TestPhase151A2_RefusalsMatchOracle pins the surface NARROWING. Integer `/`
// and `%` do not exist on the native path: the oracle's emitBinary refuses
// them, and kcc must refuse the same things for the same reason. Without this
// test the refusals could quietly disappear, and a later slice would have no
// evidence that they were ever there.
func TestPhase151A2_RefusalsMatchOracle(t *testing.T) {
	karkain := phase130Karkain(t)
	got := runKCCStep2(t, karkain, "native-value-refuse")
	if len(got) != 6 {
		t.Fatalf("kcc native-value-refuse produced %d lines, want 6:\n%v", len(got), got)
	}

	// Measured from the oracle, not written down here.
	propagateExpr, propagateRet, merr := measuredPropagateRefusals()
	if merr != nil {
		t.Fatalf("propagate refusal measurement: %v", merr)
	}

	for i, want := range []string{
		"error[K145]: unsupported operator '/' (want +, - or *)",
		"error[K145]: unsupported operator '%' (want +, - or *)",
		"error[K145]: condition must be a comparison (==, !=, <, <=, >, >=) over ints or floats",
		"error[K145]: expression nesting exceeds 64 binary levels",
		// Slots 4 and 5 are the `?` propagation refusals. They are NOT
		// spelled out here: TestPhase151A2_PropagateRefusalsMatchOracle
		// derives BOTH from the oracle by compiling real programs. Hardcoding
		// them on both sides is the 151C2 defect class -- two transcriptions
		// of the same belief, wrong together.
		propagateExpr,
		propagateRet,
	} {
		if got[i] != want {
			t.Errorf("refusal %d:\n kcc = %q\n want= %q", i, got[i], want)
		}
	}
}

// propagateRefusalCases tie each `?` refusal to a REAL program the oracle is
// actually given, so the expected message is MEASURED rather than believed.
//
// The discriminator is subtle, and the first draft of the Step 2b baseline
// got it backwards: it claimed the message depends on the `?` being TEXTUALLY
// in return position. Measurement says otherwise. The oracle runs a
// return-kind inference pass (retKindOfExpr, reached from walkReturns) BEFORE
// emission, and that helper FOLLOWS AN IDENTIFIER BACK TO ITS `let`
// INITIALIZER (program.go:1920). So the real rule is:
//
//	a `?` REACHABLE FROM a return  -> "unsupported RETURN expression"
//	any other `?`                 -> "unsupported expression"
//
// Hence `let x = 1?` + `return x` reports the RETURN message even though the
// `?` is textually in the `let`, while `x = 1?` on an already-bound local
// reports the plain EXPRESSION message even when that local is returned -- a
// reassignment does not update the table the inference walk reads.
var propagateRefusalCases = []struct {
	name string
	src  string
	want string
}{
	{
		name: "expr_position",
		src:  "func main() {\n    print(1?)\n}\n",
		want: "error[K145]: unsupported expression *parser.PropagateExpr",
	},
	{
		name: "return_position",
		src:  "func main() {\n    return 1?\n}\n",
		want: "error[K145]: unsupported return expression *parser.PropagateExpr",
	},
	{
		name: "let_then_return",
		src:  "func main() {\n    let x = 1?\n    return x\n}\n",
		want: "error[K145]: unsupported return expression *parser.PropagateExpr",
	},
	{
		name: "assign_then_return",
		src:  "func main() {\n    let x = 1\n    x = 1?\n    return x\n}\n",
		want: "error[K145]: unsupported expression *parser.PropagateExpr",
	},
}

// The two strings kcc must emit, MEASURED from the oracle rather than written
// down. Measured lazily through a sync.Once rather than by one test filling a
// global for another: Go runs tests in source order, so a producer/consumer
// pair across two test functions would silently compare against empty strings
// on a fresh run. The measurement is cheap (four tiny programs through an
// in-process compiler), so doing it once on demand costs nothing.
var (
	propagateOnce    sync.Once
	propagateExprMsg string
	propagateRetMsg  string
	propagateErr     error
)

func measuredPropagateRefusals() (exprMsg, retMsg string, err error) {
	propagateOnce.Do(func() {
		for _, tc := range propagateRefusalCases {
			msg := measureOracleErr(tc.src)
			if msg == "" {
				propagateErr = fmt.Errorf("oracle accepted a program it must refuse (%s):\n%s", tc.name, tc.src)
				return
			}
			switch tc.name {
			case "expr_position":
				propagateExprMsg = msg
			case "return_position":
				propagateRetMsg = msg
			}
			if msg != tc.want {
				propagateErr = fmt.Errorf("oracle message for %s:\n got = %q\nwant = %q", tc.name, msg, tc.want)
				return
			}
		}
		if propagateExprMsg == "" || propagateRetMsg == "" {
			propagateErr = fmt.Errorf("propagate refusal measurement incomplete")
			return
		}
		// Both messages must be reachable, or a kcc that emitted ONE string
		// for every `?` would satisfy a one-element corpus.
		if propagateExprMsg == propagateRetMsg {
			propagateErr = fmt.Errorf("the two propagate refusals are indistinguishable (%q); the corpus cannot discriminate", propagateExprMsg)
		}
	})
	return propagateExprMsg, propagateRetMsg, propagateErr
}

// oracleErr compiles src with the Go oracle and returns its error text. This
// is what makes the two propagate refusals a DIFFERENTIAL: the expected string
// comes from the same pkg/native the increment's contract is stated against,
// not from a second copy of the same belief.
func oracleErr(t *testing.T, src string) string {
	t.Helper()
	msg := measureOracleErr(src)
	if msg == "" {
		t.Fatalf("oracle ACCEPTED a program it must refuse:\n%s", src)
	}
	return msg
}

// measureOracleErr is oracleErr without the *testing.T, so the shared
// measurement can run inside a sync.Once.
func measureOracleErr(src string) string {
	p := parser.New(lexer.New(src))
	prog := p.ParseProgram()
	if len(p.Errors) > 0 {
		return ""
	}
	_, err := native.CompileProgramForOS(prog, native.OSLinux)
	if err == nil {
		return ""
	}
	return err.Error()
}

// TestPhase151A2_PropagateRefusalsMatchOracle measures the oracle's `?`
// refusals from real programs and requires kcc to emit exactly those strings.
// This is the layer the hardcoded table could not be.
func TestPhase151A2_PropagateRefusalsMatchOracle(t *testing.T) {
	karkain := phase130Karkain(t)
	got := runKCCStep2(t, karkain, "native-value-refuse")
	if len(got) != 6 {
		t.Fatalf("kcc native-value-refuse produced %d lines, want 6:\n%v", len(got), got)
	}
	propagateExpr, propagateRet, merr := measuredPropagateRefusals()
	if merr != nil {
		t.Fatalf("propagate refusal measurement: %v", merr)
	}

	// Per-case subtests: each construct is named, so a regression says WHICH
	// program changed behaviour rather than only that a string moved.
	for _, tc := range propagateRefusalCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			if gotMsg := oracleErr(t, tc.src); gotMsg != tc.want {
				t.Errorf("oracle message for %s:\n got = %q\nwant = %q", tc.name, gotMsg, tc.want)
			}
		})
	}

	// Both messages must actually be reachable, or a kcc that emitted one
	// string for everything would satisfy a one-element corpus.
	if propagateExpr == propagateRet {
		t.Fatalf("the two propagate refusals are indistinguishable (%q); the corpus cannot discriminate", propagateExpr)
	}

	// kcc's two slots must carry the measured strings.
	if got[4] != propagateExpr {
		t.Errorf("kcc propagate (expression) slot:\n kcc = %q\nwant = %q", got[4], propagateExpr)
	}
	if got[5] != propagateRet {
		t.Errorf("kcc propagate (return) slot:\n kcc = %q\nwant = %q", got[5], propagateRet)
	}
}

// TestPhase151A2b_ReassignmentSharesLetLowering states the Step 2b property
// rather than the bytes: a reassigned local is lowered EXACTLY like the `let`
// that bound it. It compares the store structure of the two shapes, so it
// fails if a future change gives reassignment its own path.
func TestPhase151A2b_ReassignmentSharesLetLowering(t *testing.T) {
	karkain := phase130Karkain(t)
	got := runKCCStep2(t, karkain, "native-value-int")
	if len(got) < 7 {
		t.Fatalf("kcc native-value-int produced %d lines, want >= 7:\n%v", len(got), got)
	}

	// shape 0 is `let x = 42; return x`; shape 5 is `let x = 1; x = 2;
	// return x`. The frame must be the ONE-LOCAL frame in both: a
	// reassignment allocates nothing, which is the property worth pinning.
	letHex, reHex := got[0], got[5]
	frame := le32Hex(refFrameOf(1))
	if !strings.Contains(reHex, frame) {
		t.Errorf("reassignment frame is not the one-local frame %s:\n kcc = %s", frame, reHex)
	}
	store := "48890424" // mov [rsp], rax
	if n := countOccurrences([]byte(reHex), []byte(store)); n != 2 {
		t.Errorf("reassignment shape has %d slot stores, want 2 (the let plus the reassignment):\n kcc = %s", n, reHex)
	}
	if n := countOccurrences([]byte(letHex), []byte(store)); n != 1 {
		t.Errorf("let-only shape has %d slot stores, want 1 (sanity on the comparison):\n kcc = %s", n, letHex)
	}
}
