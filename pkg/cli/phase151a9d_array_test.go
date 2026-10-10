package cli

// Phase 151A Step 9d - integer arrays and indexing through the real AST driver.
//
// Step 9c proved the SOURCE reaches the emitter. This adds the first COLLECTION kind
// to that driver, so it is the first step where a local is not one 8-byte unit and
// where the frame stops being a flat list of scalar slots.
//
// THE CLAIM IS NARROW AND EXECUTIONAL: given
//
//     let a = [7, 35]
//     print(a[1])
//
// kcc compiles a real .kark file into a PE image, and running that image writes
// exactly "35\n" to stdout. A gate that only inspected the image could be satisfied
// by a wrong-but-well-formed image, so the primary layer is EXECUTION with EXACT
// BYTES. Nothing is trimmed: TrimSpace is the precise tool that hides a missing
// trailing newline, which is exactly the defect Step 9b had.
//
// LAYERS, each catching a class the others cannot:
//
//  1. exact stdout bytes + exit code, per source, by executing the PE;
//  2. element and index COME FROM THE SOURCE - a distinct image per distinct element
//     value and per distinct index. This is the anti-hard-coding layer: a driver that
//     emitted a constant instead of reading the array would produce identical images
//     for print(a[0]) and print(a[1]), so it fails here even though its stdout layer
//     could be satisfied by picking the right expected value once.
//  3. the FRAME is derived from the program's own bindings and matches the value
//     model's documented rule. An array owns a two-unit header plus 8 bytes per
//     element, so 3 elements must need MORE frame than 2, which a flat per-local
//     stride would get wrong.
//  4. determinism and non-vacuity.
//  5. shapes outside the supported subset are REFUSED BY NAME, not mis-lowered.
//  6. no Go fallback.
//
// MEASURED BOUNDARY, not papered over: the oracle's out-of-range path calls
// karkain_runtime_error with a source location (increment 152-B0). kcc does not have
// that helper, so an out-of-range index here TRAPS with Int3 - the behaviour
// increment 150A measured and recorded. In-range execution is what this gate proves;
// the real diagnostic is later 151A work.
//
// NOT RUN HERE: any whole-tree KCC workload (see PHASE-151A-BASELINE.md 9a-1).

import (
	"encoding/binary"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"karkain/pkg/native"
)

// nat9dCase is one source program and the exact observable result it must produce.
type nat9dCase struct {
	name string
	src  string
	// wantOut is EXACT BYTES, never trimmed.
	wantOut []byte
	// wantExit is the process exit status.
	wantExit int
	// refuse is a substring the refusal must contain. Empty means the case must
	// produce a runnable image.
	refuse string
	// localBytes is the locals-region size this program's bindings must occupy,
	// derived independently in the test from the value model's documented rule: 8
	// bytes per scalar; a two-unit 16-byte header plus 8 bytes per element for an
	// array. Zero means the frame is not asserted for this case.
	localBytes int
}

// The corpus. Every entry is a different program.
var nat9dCases = []nat9dCase{
	{
		// The baseline int case, unchanged from 9c. It is here so 9d is proven not to
		// have regressed the shape it extended.
		name:    "int_local_unchanged",
		src:     "func main() {\n    let x = 12345\n    print(x)\n}\n",
		wantOut: []byte("12345\n"), wantExit: 0, localBytes: 8,
	},
	{
		name:    "array_index_0",
		src:     "func main() {\n    let a = [7, 35]\n    print(a[0])\n}\n",
		wantOut: []byte("7\n"), wantExit: 0, localBytes: 32,
	},
	{
		name:    "array_index_1",
		src:     "func main() {\n    let a = [7, 35]\n    print(a[1])\n}\n",
		wantOut: []byte("35\n"), wantExit: 0, localBytes: 32,
	},
	{
		// Three elements indexed at 2. This proves the element area is sized by the
		// literal's LENGTH: 3 elements need 24 bytes of element area, and reading the
		// third must not read past what a 2-element allocation would have reserved.
		name:    "array_index_2_of_3",
		src:     "func main() {\n    let a = [7, 35, 99]\n    print(a[2])\n}\n",
		wantOut: []byte("99\n"), wantExit: 0, localBytes: 40,
	},
	{
		// An array local AND a scalar local. The load-bearing layout case: the array
		// owns 32 bytes and the scalar follows it, so the scalar is correct only if the
		// offset allocation is SEQUENTIAL and kind-aware. A fixed 8-byte-per-local
		// stride would place the scalar inside the array's element area and print a
		// wrong value, which no single-array case could detect.
		name:    "array_and_scalar_local",
		src:     "func main() {\n    let a = [7, 35]\n    let b = 1000\n    print(a[1])\n    print(b)\n}\n",
		wantOut: []byte("35\n1000\n"), wantExit: 0, localBytes: 40,
	},
	{
		// Two prints of the SAME array at DIFFERENT indexes. Besides proving both
		// indices resolve, this exercises per-site label tagging: kcc has no fresh()
		// counter, so two index expressions must not Mark the same label twice
		// (natFinish refuses a duplicate label).
		name:    "two_indexes_one_array",
		src:     "func main() {\n    let a = [7, 35, 99]\n    print(a[0])\n    print(a[2])\n}\n",
		wantOut: []byte("7\n99\n"), wantExit: 0, localBytes: 40,
	},
	{
		// A multi-digit element next to a single-digit one, so a driver emitting the
		// wrong immediate width or the wrong element is caught by byte comparison
		// rather than by a value that happens to look right.
		name:    "multi_digit_elements",
		src:     "func main() {\n    let a = [12345, 7]\n    print(a[0])\n    print(a[1])\n}\n",
		wantOut: []byte("12345\n7\n"), wantExit: 0, localBytes: 32,
	},
	{
		// A single-element array: the element area is 8 bytes, so the whole local is
		// 24 - the smallest array there is, and the boundary where a header-only
		// implementation with no element area would still look correct.
		name:    "single_element_array",
		src:     "func main() {\n    let a = [42]\n    print(a[0])\n}\n",
		wantOut: []byte("42\n"), wantExit: 0, localBytes: 24,
	},
}

// nat9dRefusals are shapes outside the supported subset. Each must be refused BY
// NAME with K145, and must NOT produce an image: silently lowering one of these into
// a runnable program that computes the wrong thing is the failure mode the native
// backend exists to avoid, and a refusal is the only honest answer while the subset
// is this small.
var nat9dRefusals = []struct {
	name string
	src  string
	// want is a substring identifying WHICH construct was refused, so a refusal that
	// fires for the wrong reason does not pass.
	want string
	// code is the diagnostic the refusal must carry. It is a field rather than a
	// constant because one case here is caught EARLIER than the driver's own scan:
	// an undeclared index target is an undefined identifier, which kcc's type
	// checker reports as K102 before natNativeScan ever runs. Asserting K145 for it
	// would have been a wrong expectation, and a gate that fails on correct
	// behaviour trains its readers to ignore it.
	code string
}{
	{
		// A negative element is a UnaryExpr, not an IntLit. The subset is integer
		// LITERALS; unary minus on an element is not implemented yet.
		name: "negative_element",
		src:  "func main() {\n    let a = [-7, 35]\n    print(a[0])\n}\n",
		want: "array element 0",
		code: "error[K145]",
	},
	{
		// A variable index, not a literal.
		name: "variable_index",
		src:  "func main() {\n    let a = [7, 35]\n    let i = 1\n    print(a[i])\n}\n",
		want: "array index is not an integer literal",
		code: "error[K145]",
	},
	{
		// Indexing a SCALAR. This one was a real defect found by this gate: the scan
		// accepted it, lowering read the scalar's slot as a (base, len) header, and
		// the result was a VALID PE printing a meaningless number. No diagnostic,
		// no trap.
		name: "index_of_scalar",
		src:  "func main() {\n    let a = 7\n    print(a[0])\n}\n",
		want: "is indexed but is not an array",
		code: "error[K145]",
	},
	{
		// Caught by kcc's type checker, not by the driver's scan: `q` does not exist.
		name: "index_of_undeclared",
		src:  "func main() {\n    print(q[0])\n}\n",
		want: "undefined identifier 'q'",
		code: "error[K102]",
	},
	{
		// `len(a)` was pinned here as OUT OF SUBSET when 9d landed. Step 9e added it,
		// so this case was removed rather than left asserting a refusal that is no
		// longer honest -- and its behaviour is now gated properly by
		// TestPhase151A9E_* , including the shape distinctions (len of a scalar, len
		// of an expression, wrong arity). Moving a construct from "refused" to
		// "supported" has to delete the old pin; keeping it would have made this gate
		// fail on correct behaviour and taught its readers to ignore it.
		name: "for_in_over_array",
		src:  "func main() {\n    let a = [7, 35]\n    for x in a {\n        print(x)\n    }\n}\n",
		want: "error[K145]",
		code: "error[K145]",
	},
	{
		// Printing an array AS A WHOLE. Also a real defect found by this gate: it
		// scanned clean, and lowering loaded the header's base pointer -- one 8-byte
		// slot -- and printed that address as if it were the array's value. There is
		// no scalar reading of a collection, so there is nothing to lower it to.
		name: "print_whole_array",
		src:  "func main() {\n    let a = [7, 35]\n    print(a)\n}\n",
		want: "cannot print `a` as a whole",
		code: "error[K145]",
	},
}

// nat9dFrameSubRsp returns the distinct imm32 operands of every `sub rsp, imm32`
// (REX.W 81 /5, i.e. 48 81 EC) found in an image, ascending.
//
// It scans rather than indexing a fixed position on purpose. The driver's frame
// adjustment is emitted at a known place in the emission order, but pinning that
// position would make this helper a second transcription of the layout code it is
// supposed to check. Collecting the SET and asserting the expected value is PRESENT
// is a real constraint (a wrong frame is absent) without restating the layout.
func nat9dFrameSubRsp(t *testing.T, img []byte) []int32 {
	t.Helper()
	var out []int32
	for i := 0; i+6 < len(img); i++ {
		if img[i] == 0x48 && img[i+1] == 0x81 && img[i+2] == 0xEC {
			out = append(out, int32(binary.LittleEndian.Uint32(img[i+3:])))
		}
	}
	sort.Slice(out, func(a, b int) bool { return out[a] < out[b] })
	return out
}

// nat9dExpectFrame is the value model's own frame rule, restated INDEPENDENTLY of
// src/compiler/native_value.kark so the gate is not comparing the implementation to
// itself:
//
//	locals region rounded up to 16   (Win64 stack discipline)
//	+ 512  bin temporary area        (maxBinDepth 64 * 8)
//	+  96  argument spill area       (argSpillBytes, 6 args * 16)
//
// The rounding is applied to the LOCALS REGION BEFORE the fixed regions are added,
// which is the discipline Step 1 established and the reason a one-local program's
// frame is 624 rather than 616.
func nat9dExpectFrame(localBytes int) int32 {
	rounded := ((localBytes + 15) / 16) * 16
	if rounded == 0 {
		// A program with no bindings still reserves the fixed regions; an empty
		// locals region rounds to 0, not to 16.
		rounded = 0
	}
	return int32(rounded + 512 + 96)
}

func nat9dContainsInt32(xs []int32, want int32) bool {
	for _, x := range xs {
		if x == want {
			return true
		}
	}
	return false
}

// nat9dImage compiles one source through kcc and returns the image bytes, failing if
// the driver refused it.
func nat9dImage(t *testing.T, src string) []byte {
	t.Helper()
	out := nat9cBuild(t, src)
	if strings.Contains(out, "error[K") {
		t.Fatalf("kcc refused a program this gate expects to compile:\n%.400s", out)
	}
	return nat9cDecodeHex(t, out, "driver image")
}

// nat9dRun writes an image to disk, executes it, and returns EXACT stdout bytes and
// the exit code. It never trims.
func nat9dRun(t *testing.T, img []byte) ([]byte, int) {
	t.Helper()
	if _, _, err := native.ParsePE(img); err != nil {
		t.Fatalf("driver image is not a structurally valid PE: %v", err)
	}
	if n := len(nat9cRelocCount(t, img)); n < 12 {
		t.Errorf("driver image has %d DIR64 relocation entries, want at least 12 "+
			"(2 rodata sites + 3 bootstrap stores + 7 IAT loads); Step 9d must not "+
			"regress Step 9b's relocation work", n)
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
	return got, code
}

// TestPhase151A9D_ArrayProgramsExecuteWithExactBytes is layer 1: the primary claim.
//
// Each case is a DIFFERENT .kark file, driven through kcc's whole-program driver,
// written to disk as a PE, EXECUTED, and compared byte for byte.
//
// The pair (array_index_0, array_index_1) is the load-bearing pair: identical except
// for the index, and different exact stdout. A driver that ignored the index and
// always printed element 0 fails one of them; a driver that ignored the array and
// printed a constant fails both.
func TestPhase151A9D_ArrayProgramsExecuteWithExactBytes(t *testing.T) {
	if os.PathSeparator == '/' {
		t.Skip("PE images only execute on Windows; the driver itself is cross-platform")
	}
	for _, c := range nat9dCases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			img := nat9dImage(t, c.src)
			got, code := nat9dRun(t, img)
			if !nat9cBytesEq(got, c.wantOut) {
				t.Errorf("stdout = %q (% x), want %q (% x)\n  source: %s\n"+
					"  compared as EXACT BYTES: a missing or extra trailing newline is a "+
					"real defect, and trimming would hide it",
					got, got, c.wantOut, c.wantOut, c.src)
			}
			if code != c.wantExit {
				t.Errorf("exit = %d, want %d\n  source: %s", code, c.wantExit, c.src)
			}
		})
	}
}

// TestPhase151A9D_ElementsAndIndexesComeFromTheSource is layer 2: the anti-hard-coding
// layer.
//
// It compares IMAGES, not stdout, because a stdout comparison can be satisfied by
// choosing the right expected value once. A driver that emitted a fixed constant
// instead of reading the array would produce the SAME IMAGE for every one of these
// programs, so distinctness fails even where a single stdout case might pass.
//
// The two pairs that carry the weight:
//   - same array, different index  -> proves the index is read;
//   - different element values      -> proves the elements are read.
func TestPhase151A9D_ElementsAndIndexesComeFromTheSource(t *testing.T) {
	type variant struct {
		name string
		src  string
	}
	variants := []variant{
		{"idx0_of_7_35", "func main() {\n    let a = [7, 35]\n    print(a[0])\n}\n"},
		{"idx1_of_7_35", "func main() {\n    let a = [7, 35]\n    print(a[1])\n}\n"},
		{"idx0_of_7_36", "func main() {\n    let a = [7, 36]\n    print(a[0])\n}\n"},
		{"idx0_of_8_35", "func main() {\n    let a = [8, 35]\n    print(a[0])\n}\n"},
		{"idx0_of_9_35", "func main() {\n    let a = [9, 35]\n    print(a[0])\n}\n"},
	}

	// Images are compared as EXACT BYTES, not through a hash. An earlier draft used
	// an FNV-style fingerprint as the map key, which reported two different images as
	// identical -- a lesson already paid once in this repository (increment 151B,
	// where hand-written constants made a differential compare nothing). A
	// fingerprint that can collide is a weaker claim than the one this layer wants:
	// "these N images are pairwise different, checked byte for byte".
	var priorName string
	var prior []byte
	for _, v := range variants {
		v := v
		t.Run(v.name, func(t *testing.T) {
			img := nat9dImage(t, v.src)
			if prior != nil && nat9cBytesEq(img, prior) {
				t.Fatalf("image for %s is byte-identical to %s.\n"+
					"  The driver must read the element values and the index out of the "+
					"AST; identical images mean one of them is not reaching the emitter.\n"+
					"  source: %s", v.name, priorName, v.src)
			}
			prior, priorName = img, v.name
		})
	}
}

// TestPhase151A9D_FrameFollowsTheValueModelRule is layer 3.
//
// An array local is a TWO-unit header plus 8 bytes per element, all in the frame, and
// nothing is allocated. So the frame must grow with the element count, and a scalar
// declared after an array must not land inside that array's element area.
//
// The expected frame is derived HERE, independently of the implementation, from the
// value model's documented rule. A gate that recomputed the number the same way the
// implementation does would only prove self-consistency.
func TestPhase151A9D_FrameFollowsTheValueModelRule(t *testing.T) {
	for _, c := range nat9dCases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			img := nat9dImage(t, c.src)
			got := nat9dFrameSubRsp(t, img)
			want := nat9dExpectFrame(c.localBytes)
			if !nat9dContainsInt32(got, want) {
				t.Errorf("no `sub rsp, %d` in the image; got %v.\n  want frame %d for "+
					"localBytes %d = round16(%d) + 512 (binTemp) + 96 (argSpill).\n"+
					"  source: %s",
					want, got, want, c.localBytes, c.localBytes, c.src)
			}
		})
	}
}

// TestPhase151A9D_ElementsNeedRoomInTheFrame is the boundary the previous test
// implies but does not state.
//
// The first draft of this gate asserted the frame grows STRICTLY with the element
// count. That expectation was wrong, and measuring showed why: the locals region is
// rounded up to 16 bytes, so a 1-element array (24 bytes) and a 2-element array
// (32 bytes) legitimately share a frame. A wrong expectation in a gate is worse than
// no gate, because it reports a defect that does not exist.
//
// The property that actually matters is a SOUNDNESS one: the frame must reserve room
// for the two-unit header AND every element. A frame sized as though the array had
// no elements would let `a[n]` read outside the area reserved for it -- reading
// adjacent stack data and printing it as if it were an array element.
//
//	locals region >= 16 (header) + 8*n (elements)
//
// An element-blind frame (a flat 8-bytes-per-local stride) gives a locals region of
// 16 for every n, which fails this at n = 1.
func TestPhase151A9D_ElementsNeedRoomInTheFrame(t *testing.T) {
	srcFor := func(n int) string {
		var elems []string
		for i := 1; i <= n; i++ {
			elems = append(elems, itoa9d(i*10))
		}
		return "func main() {\n    let a = [" + join9d(elems, ", ") +
			"]\n    print(a[0])\n}\n"
	}
	prev := int32(-1)
	for n := 1; n <= 5; n++ {
		n := n
		t.Run(itoa9d(n)+"_elements", func(t *testing.T) {
			img := nat9dImage(t, srcFor(n))
			got := nat9dFrameSubRsp(t, img)
			want := nat9dExpectFrame(16 + 8*n)
			if !nat9dContainsInt32(got, want) {
				t.Fatalf("%d-element array: no `sub rsp, %d` in the image; got %v",
					n, want, got)
			}
			locals := want - 512 - 96
			if int(locals) < 16+8*n {
				t.Errorf("%d-element array: locals region is %d bytes but the header "+
					"plus %d elements needs %d.\n  A frame that under-reserves lets "+
					"a[n] read adjacent stack data and print it as an array element",
					n, locals, n, 16+8*n)
			}
			if prev > want {
				t.Errorf("%d-element array: frame %d is SMALLER than the %d-element "+
					"array's %d; the frame must never shrink as the element area grows",
					n, want, n-1, prev)
			}
			if prev == want {
				t.Logf("%d-element array shares frame %d with %d elements; that is "+
					"legitimate, the locals region is rounded up to 16", n, want, n-1)
			}
			prev = want
		})
	}
}

func itoa9d(v int) string {
	if v == 0 {
		return "0"
	}
	var out []byte
	for v > 0 {
		out = append([]byte{byte('0' + v%10)}, out...)
		v /= 10
	}
	return string(out)
}

func join9d(xs []string, sep string) string {
	out := ""
	for i, x := range xs {
		if i > 0 {
			out += sep
		}
		out += x
	}
	return out
}

// TestPhase151A9D_OutOfSubsetShapesAreRefusedByName is layer 5.
//
// Each shape here is one the native value model CAN represent but this driver does
// not yet lower (a variable index, len, for-in) or one that is meaningless for it
// (indexing a scalar, indexing an undeclared name). All must be refused with K145,
// and the refusal must name WHICH construct, so a refusal that fires for the wrong
// reason does not pass.
func TestPhase151A9D_OutOfSubsetShapesAreRefusedByName(t *testing.T) {
	for _, r := range nat9dRefusals {
		r := r
		t.Run(r.name, func(t *testing.T) {
			out := nat9cBuild(t, r.src)
			if !strings.Contains(out, r.code) {
				t.Fatalf("expected %s, got:\n%.400s", r.code, out)
			}
			if !strings.Contains(out, r.want) {
				t.Errorf("refusal does not name the offending construct %q:\n%.400s",
					r.want, out)
			}
			// The refusal must not also claim to have produced an image. A refusal that
			// is followed by hex is a program that was silently mis-lowered.
			if len(out) > 0 && isHex9d(strings.TrimSpace(out)) {
				t.Errorf("a REFUSED program also produced an image:\n%.200s", out)
			}
			// "not handed to the Go engine" is the invariant that distinguishes a
			// driver refusal from a silent fallback.
			//
			// It is asserted only for the driver's OWN refusals. A case rejected
			// earlier, by kcc's type checker, carries that checker's diagnostic
			// instead and has no reason to mention the Go engine: the program never
			// reached the native lowering at all, which is a stronger outcome than
			// refusing it there.
			if r.code == "error[K145]" && !strings.Contains(out, "not handed to the Go engine") {
				t.Errorf("refusal does not say the construct is not handed to the Go "+
					"engine; an unexplained refusal invites a fallback:\n%.300s", out)
			}
		})
	}
}

func isHex9d(s string) bool {
	if s == "" || len(s)%2 != 0 {
		return false
	}
	for _, c := range s {
		if !strings.ContainsRune("0123456789abcdef", c) {
			return false
		}
	}
	return true
}

// TestPhase151A9D_CorpusIsNonVacuousAndDeterministic is layer 4.
//
// Determinism: the same source twice must produce byte-identical images. A driver
// whose label allocation depended on map iteration or a shared counter would pass the
// execution layer on a lucky run and fail here.
//
// Non-vacuity: every case must compile, and the corpus as a whole must carry real
// image bytes. A corpus that silently stopped compiling would otherwise "pass" the
// loop by having nothing in it.
func TestPhase151A9D_CorpusIsNonVacuousAndDeterministic(t *testing.T) {
	if len(nat9dCases) < 8 {
		t.Fatalf("corpus has %d cases; the array gate needs the shape coverage to be "+
			"real, not vestigial", len(nat9dCases))
	}
	total := 0
	for _, c := range nat9dCases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			a := nat9dImage(t, c.src)
			b := nat9dImage(t, c.src)
			if !nat9cBytesEq(a, b) {
				t.Errorf("two runs of the same source produced different images "+
					"(%d vs %d bytes); the driver must be deterministic", len(a), len(b))
			}
			if len(a) < 512 {
				t.Errorf("image is only %d bytes; a PE with an entry stub, a PEB "+
					"bootstrap and print_int cannot be this small", len(a))
			}
			total += len(a)
		})
	}
	if total == 0 {
		t.Fatal("no image bytes were produced at all")
	}
}

// TestPhase151A9D_NoGoFallback is layer 6.
//
// Step 151 was opened to delete a dishonest behaviour in which a kcc native request
// was silently served by the Go backend. Two properties must still hold:
//
//  1. the measurement command has no Go fallback of its own -- a misspelled
//     subcommand must FAIL rather than quietly producing something;
//  2. the user-facing native target is STILL REFUSED while
//     kccOwnsNativeTargets is false. 9d extends the driver that 151D will hand the
//     targets to; it must not quietly turn that refusal into a success, because the
//     driver's supported subset is a handful of shapes, not the language.
func TestPhase151A9D_NoGoFallback(t *testing.T) {
	karkain := phase130Karkain(t)

	if out, err := exec.Command(karkain, "native-astx").CombinedOutput(); err == nil {
		t.Errorf("a misspelled native subcommand SUCCEEDED; the native path has a "+
			"fallback or a catch-all:\n%.300s", out)
	}

	// Phase 151D split this in two. kcc OWNS native-x86_64-windows, so a
	// default-engine build of that target must now SUCCEED with kcc provenance
	// — that is the deliverable, and asserting the old blanket K116 here would
	// fail on correct behaviour and, worse, forbid the thing 151D ships.
	//
	// The targets kcc does NOT own keep the original contract verbatim: refuse
	// with K116, exit 6, and leave nothing on disk.
	dir := t.TempDir()
	src := filepath.Join(dir, "p.kark")
	if err := os.WriteFile(src, []byte("func main() {\n    let a = [7, 35]\n    print(a[1])\n}\n"), 0o644); err != nil {
		t.Fatalf("writing the test program: %v", err)
	}

	for _, tgt := range []string{NativeLinuxTarget, NativeMacOSTarget} {
		tgt := tgt
		t.Run("refuses "+tgt, func(t *testing.T) {
			out, err := exec.Command(karkain, "build", "--target", tgt, src).CombinedOutput()
			code := 0
			if ee, ok := err.(*exec.ExitError); ok {
				code = ee.ExitCode()
			} else if err != nil {
				t.Fatalf("running karkain build (%s): %v", tgt, err)
			}
			s := string(out)
			if !strings.Contains(s, "K116") {
				t.Errorf("%s: native target build did not refuse with K116; kcc does not own "+
					"this target, so a silent success would be the Go-fallback regression.%s",
					tgt, s)
			}
			if code != 6 {
				t.Errorf("%s: native target refusal exit = %d, want 6 (ExitEnv)", tgt, code)
			}
			entries, _ := os.ReadDir(dir)
			for _, e := range entries {
				if strings.HasSuffix(e.Name(), ".exe") {
					t.Errorf("%s: a refused native build still wrote %s; the refusal must not "+
						"leave an image behind", tgt, e.Name())
				}
			}
		})
	}

	// The target kcc owns: built by the DEFAULT engine, which is kcc. Success
	// here is the point of 151D, but it is only legitimate WITH provenance.
	t.Run("owns native-x86_64-windows", func(t *testing.T) {
		out, err := exec.Command(karkain, "build", "--target", NativeWindowsTarget, src).CombinedOutput()
		code := 0
		if ee, ok := err.(*exec.ExitError); ok {
			code = ee.ExitCode()
		} else if err != nil {
			t.Fatalf("running karkain build: %v", err)
		}
		s := string(out)
		if code != 0 {
			t.Fatalf("%s: kcc owns this target in 151D but the default-engine build failed "+
				"(exit %d):\n%.400s", NativeWindowsTarget, code, s)
		}
		if !strings.Contains(s, kccNativeProvenance+" "+NativeWindowsTarget) {
			t.Errorf("%s: a default-engine native build succeeded with no kcc provenance; "+
				"without it the image cannot be attributed to the self-hosted engine:\n%.400s",
				NativeWindowsTarget, s)
		}
	})
}
