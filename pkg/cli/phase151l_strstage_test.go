package cli

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"karkain/pkg/native"
)

// NAT-STR-STAGE -- the native string staging area: its SCOPE and its COVERAGE.
//
// The recorded defect was a difference of exactly 2560 bytes -- the string
// staging area, 8 * 5 * maxBinDepth -- between the two engines' frame sizes,
// with two separate causes:
//
//  1. SCOPE. The oracle's staging area is reserved from a set of PROGRAM-WIDE
//     flags and its layout reserves the area in EVERY function:
//
//     pkg/native/program.go:4454
//       if b.usesConcat || b.usesStrEq || b.usesStrSlice || b.usesPush || b.usesStrField {
//           b.strTemp = next
//           next += 8*5*maxBinDepth
//       }
//
//     The driver reserved it PER FUNCTION, so a callee in a program whose main
//     did any string work had a frame 2560 bytes short. Measured on the Linux
//     ELF target, because the Windows 16-byte frame rounding quantises an
//     8-byte difference away and cannot see a 2560-byte one:
//
//       program                     driver frames   oracle frames
//       helper + concat in main     [608, 3168]     [3168, 3168]
//       helper + push in main       [608, 3208]     [3168, 3208]
//
//  2. COVERAGE. The oracle's flag covers FIVE operations. The driver's covered
//     two (a concat site and a push count). A string FIELD ASSIGNMENT -- the
//     oracle's fifth -- reserved no area at all:
//
//       string field assignment     [640]           [3200]
//
// The correction is the ALLOCATION, and only the allocation: the flag is
// computed once, over the whole program, and threaded into every function's
// layout -- the same shape as the map-stage flag that NAT-FRAME established.
// The driver's own string-field write does NOT stage through the area (it
// stores the (pointer, length) pair directly), so the FRAME matches without
// the CODE matching: the oracle's staging implementation is not duplicated.
//
// WHAT IS NOT CLAIMED. The two engines' images are NOT byte-identical, and
// this gate does not assert that they are:
//
//   - the oracle's string-field write stages the pair through the area and the
//     driver's does not, so the field-assignment image is 25 bytes SHORTER on
//     the driver (measured: 443 against 468) and its rodata starts at a
//     different offset;
//   - the reporter's rodata interning order still differs, as NAT-FRAME
//     recorded.
//
// The frames, the function counts and the container layout are what this gate
// asserts, and those are the things the correction is about.

// natStrStageBytes is the size of the string staging area: 8 * 5 * maxBinDepth.
//
// It is stated here as ARITHMETIC over the same constants the driver uses
// (natStrTempBytes) rather than as a literal, so a reader can see what it is.
const natStrStageBytes = 8 * 5 * 64

// natStrStageCase is one differential case.
//
// wantStage says whether the program performs a string OPERATION -- as opposed
// to merely containing a string. The oracle's five flags are about operations,
// so a program that binds a string and prints it reserves nothing.
//
// probe is a DISTINCTIVE literal of the program, used to record where its
// string data lands in the image. It is distinctive on purpose: a one-letter
// literal would match bytes of the code itself, and the offsets recorded would
// be noise.
type natStrStageCase struct {
	name string
	src  string
	// wantStage says whether every frame in the program reserves the area.
	wantStage bool
	// wantFuncs is the number of functions in the program, and therefore the
	// number of frame-size immediates its image carries.
	wantFuncs int
	// wantOut is the program's stdout, EXACTLY, on the PE that runs on this
	// host.
	wantOut string
	// probe is the literal whose file offsets are recorded. Empty means the
	// program has no string data.
	probe string
}

// natStrStageCases is the corpus.
//
// The matrix is the one the brief names: one function with one string, one
// function with several strings, several functions with independent strings,
// several functions with the same string, nested calls with strings, strings
// alongside arrays and integer expressions, no strings at all, and a mixed
// program.
var natStrStageCases = []natStrStageCase{
	{
		// ONE FUNCTION, ONE STRING, and no string OPERATION. Neither engine
		// reserves the area: the oracle's five flags are about operations, and
		// binding a string and printing it is not one of them. This case is
		// what keeps the correction from over-reserving.
		name:      "one_string_no_operation",
		src:       "func main() {\n    let s = \"hi\"\n    print(s)\n}\n",
		wantStage: false, wantFuncs: 1, wantOut: "hi\n", probe: "hi",
	},
	{
		// ONE FUNCTION, SEVERAL STRINGS, one operation: a concatenation.
		name:      "one_function_several_strings",
		src:       "func main() {\n    let a = \"alpha\"\n    let b = \"beta\"\n    print(a + b)\n}\n",
		wantStage: true, wantFuncs: 1, wantOut: "alphabeta\n", probe: "alpha",
	},
	{
		// SEVERAL FUNCTIONS, INDEPENDENT STRINGS. The helper does no string
		// work and main's string is not an operation, so NEITHER function
		// reserves the area.
		name:      "several_functions_independent_strings",
		src:       "func helper() int {\n    return 7\n}\nfunc main() {\n    let s = \"solo\"\n    print(s)\n    print(helper())\n}\n",
		wantStage: false, wantFuncs: 2, wantOut: "solo\n7\n", probe: "solo",
	},
	{
		// SEVERAL FUNCTIONS, THE SAME STRING, one operation in main. The area
		// is reserved in BOTH functions -- this is the SCOPE case -- and the
		// literal is interned ONCE, so its offset list has one entry.
		name:      "several_functions_same_string",
		src:       "func helper() int {\n    return 7\n}\nfunc main() {\n    let a = \"twin\"\n    let b = \"twin\"\n    print(a + b)\n    print(helper())\n}\n",
		wantStage: true, wantFuncs: 2, wantOut: "twintwin\n7\n", probe: "twin",
	},
	{
		// NESTED CALLS with a string operation in main.
		name:      "nested_calls_with_strings",
		src:       "func g(n int) int {\n    return n + 1\n}\nfunc f(n int) int {\n    return g(n) + g(n)\n}\nfunc main() {\n    let a = \"nest\"\n    print(a + \"tail\")\n    print(f(1))\n}\n",
		wantStage: true, wantFuncs: 3, wantOut: "nesttail\n4\n", probe: "nest",
	},
	{
		// STRINGS ALONGSIDE ARRAYS AND INTEGER EXPRESSIONS. The array access
		// is a map use to the oracle, so this case carries the map-stage unit
		// as well -- and the string stage must not displace it.
		name:      "strings_with_array_and_integer_expressions",
		src:       "func main() {\n    let v = [3, 5, 7]\n    let s = \"mixe\"\n    print(s + \"dxy\")\n    print(v[1])\n    print(1 + 2)\n}\n",
		wantStage: true, wantFuncs: 1, wantOut: "mixedxy\n5\n3\n", probe: "mixe",
	},
	{
		// NO STRINGS AT ALL.
		name:      "no_strings",
		src:       "func main() {\n    print(42)\n}\n",
		wantStage: false, wantFuncs: 1, wantOut: "42\n",
	},
	{
		// MIXED: a push, a concatenation and an array index in one program.
		name:      "mixed_push_concat_and_array",
		src:       "func main() {\n    let a = [1]\n    let b = push(a, 2)\n    let s = \"combo\"\n    print(s + \"zap\")\n    print(b[1])\n}\n",
		wantStage: true, wantFuncs: 1, wantOut: "combozap\n2\n", probe: "combo",
	},
	{
		// A STRING FIELD ASSIGNMENT: the oracle's FIFTH operation, and the
		// one the driver did not cover at all.
		name:      "string_field_assignment",
		src:       "type S struct { label string; n int }\nfunc main() {\n    let s = S { label: \"a\", n: 1 }\n    s.label = \"changed\"\n    print(s.label)\n}\n",
		wantStage: true, wantFuncs: 1, wantOut: "changed\n", probe: "changed",
	},
	{
		// A push() on its own: the oracle's FOURTH operation.
		name:      "push_only",
		src:       "func main() {\n    let a = [1]\n    let b = push(a, 2)\n    print(b[1])\n}\n",
		wantStage: true, wantFuncs: 1, wantOut: "2\n",
	},
	{
		// MAIN IS NOT THE LAST FUNCTION.
		//
		// The program-wide flag is an AGGREGATION over the functions, and an
		// aggregation that keeps only the LAST function's answer would miss a
		// string operation in `main` when `main` is not last. This case is
		// what catches that, and it is the only shape in the corpus where
		// the string work is in a function other than the last one.
		name:      "main_is_not_the_last_function",
		src:       "func main() {\n    let a = \"first\"\n    print(a + \"second\")\n}\nfunc helper() int {\n    return 7\n}\n",
		wantStage: true, wantFuncs: 2, wantOut: "firstsecond\n", probe: "first",
	},
}

// natStrStageFrames returns EVERY frame-size immediate in an image, in the
// order the functions appear in .text (source order).
//
// It is the NAT-FRAME gate's own helper (natFrameAllFrames in
// pkg/cli/phase151k_frame_test.go): the helpers are emitted first and use the
// SHORT form (`48 83 EC`, a signed byte), so every long form (`48 81 EC`)
// belongs to a user function. That is what makes the list a list of
// PER-FUNCTION frames rather than a single number, and the list's LENGTH is
// therefore the function count.
func natStrStageFrames(t *testing.T, img []byte) []int32 {
	t.Helper()
	return natFrameAllFrames(t, img)
}

// natStrStageBuildRaw builds one source with one engine and returns the
// result WITHOUT failing when the engine refuses the program.
//
// The refusal is a first-class outcome here: several of the cases in this
// gate are about constructs the driver refuses, and a helper that failed the
// test on a refusal could not assert one.
func natStrStageBuildRaw(t *testing.T, karkain, src, target, engine string) nativeBuildResult {
	t.Helper()
	return nativeBuildWithEngine(t, karkain, src, target, engine, "strstage.img")
}

// natStrStageBuild builds one source with BOTH engines and returns the two
// images. A refusal by either engine fails the test.
func natStrStageBuild(t *testing.T, karkain, src, target string) (kccImg, goImg []byte) {
	t.Helper()
	kccRes := natStrStageBuildRaw(t, karkain, src, target, "kcc")
	if kccRes.ExitCode != 0 || !kccRes.WroteImage {
		t.Fatalf("kcc produced no image (exit %d):\n%s", kccRes.ExitCode, kccRes.Output)
	}
	goRes := natStrStageBuildRaw(t, karkain, src, target, "go")
	if goRes.ExitCode != 0 || !goRes.WroteImage {
		t.Fatalf("the Go oracle produced no image (exit %d):\n%s", goRes.ExitCode, goRes.Output)
	}
	return kccRes.Image, goRes.Image
}

// natStrStageBuildOracle builds one source with the ORACLE only.
func natStrStageBuildOracle(t *testing.T, karkain, src, target string) []byte {
	t.Helper()
	goRes := natStrStageBuildRaw(t, karkain, src, target, "go")
	if goRes.ExitCode != 0 || !goRes.WroteImage {
		t.Fatalf("the Go oracle produced no image (exit %d):\n%s", goRes.ExitCode, goRes.Output)
	}
	return goRes.Image
}

// natStrStageLayout describes an ELF image's SECTION BOUNDARIES: the number of
// program headers and the .text offset.
//
// It is read with the ORACLE's own parser (native.Parse), not with a check
// written here: a structural check written in the gate would share the
// assumptions of the code under test.
//
// The ENTRY POINT is deliberately NOT part of it. The entry sits at the END of
// .text (Phase D1), so it moves with .text's size -- and the field-assignment
// case's .text IS a different size, because the oracle's string-field write
// stages through the staging area and the driver's does not. The entry and the
// file length are recorded by the differential instead of being asserted
// equal.
func natStrStageLayout(t *testing.T, img []byte) string {
	t.Helper()
	_, textOff, err := native.Parse(img)
	if err != nil {
		t.Fatalf("the image is not a structurally valid ELF: %v", err)
	}
	phnum := (textOff - 64) / 56
	return fmt.Sprintf("phnum=%d textOff=%d", phnum, textOff)
}

// natStrStageEntry returns an image's entry point and file length, for the
// differential record.
func natStrStageEntry(t *testing.T, img []byte) string {
	t.Helper()
	entry, _, err := native.Parse(img)
	if err != nil {
		t.Fatalf("the image is not a structurally valid ELF: %v", err)
	}
	return fmt.Sprintf("entry=%#x len=%d", entry, len(img))
}

// natStrStageDiffBytes counts the bytes in which two images differ, and the
// length of the shorter one.
func natStrStageDiffBytes(a, b []byte) (diff, shorter int) {
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	for i := 0; i < n; i++ {
		if a[i] != b[i] {
			diff++
		}
	}
	return diff, n
}

// natStrStageLiteralOffsets returns the file offsets at which a literal's
// bytes appear in an image.
//
// An empty literal matches everywhere, so it is refused: a program with no
// string data has no offsets to record, and a list of every offset in the
// image would be noise.
func natStrStageLiteralOffsets(img []byte, literal string) []int {
	if literal == "" {
		return nil
	}
	pat := []byte(literal)
	var out []int
	for i := 0; i+len(pat) <= len(img); i++ {
		if bytes.Equal(img[i:i+len(pat)], pat) {
			out = append(out, i)
		}
	}
	return out
}

// TestPhaseNATStrStage_TheFramesMatchTheOracle is the differential.
//
// Every case is built with BOTH engines for the LINUX target, the frame-size
// immediates are read out of each image, and the two lists must be EQUAL --
// element for element, so the SCOPE is asserted as well as the total.
//
// The Linux target is the one that can see this at all: the Windows frame is
// rounded up to a multiple of 16, which quantises an 8-byte difference away
// and cannot see a 2560-byte one. The Windows target is checked separately,
// in the behaviour test.
func TestPhaseNATStrStage_TheFramesMatchTheOracle(t *testing.T) {
	karkain := phase130Karkain(t)
	for _, c := range natStrStageCases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			kccImg, goImg := natStrStageBuild(t, karkain, c.src, NativeLinuxTarget)
			kccFrames := natStrStageFrames(t, kccImg)
			goFrames := natStrStageFrames(t, goImg)

			if len(kccFrames) != c.wantFuncs {
				t.Errorf("the native image carries %d frame immediates, want %d (one per function).\n"+
					"  source:\n%s", len(kccFrames), c.wantFuncs, c.src)
			}
			if len(goFrames) != c.wantFuncs {
				t.Errorf("the oracle image carries %d frame immediates, want %d (one per function).\n"+
					"  source:\n%s", len(goFrames), c.wantFuncs, c.src)
			}
			if len(kccFrames) != len(goFrames) {
				t.Fatalf("the two images carry %d and %d frames; they cannot be compared.\n"+
					"  source:\n%s", len(kccFrames), len(goFrames), c.src)
			}
			for i := range kccFrames {
				if kccFrames[i] != goFrames[i] {
					t.Errorf("frame %d differs: native 0x%x (%d), oracle 0x%x (%d), difference %d bytes.\n"+
						"  The staging area is %d bytes and is reserved in EVERY function of a program "+
						"that performs a string operation (pkg/native/program.go:4454).\n"+
						"  source:\n%s",
						i, uint32(kccFrames[i]), kccFrames[i], uint32(goFrames[i]), goFrames[i],
						goFrames[i]-kccFrames[i], natStrStageBytes, c.src)
				}
			}
		})
	}
}

// TestPhaseNATStrStage_TheStageIsReservedInEveryFunction is the SCOPE claim,
// stated as a difference rather than as an equality.
//
// The same helper appears in two programs that differ ONLY by main doing a
// string operation. Because the oracle's flag is program-wide, the HELPER's
// frame grows by exactly the staging area -- even though the helper never
// touches a string. A per-function flag would leave the helper's frame
// unchanged, which is what this assertion rules out.
func TestPhaseNATStrStage_TheStageIsReservedInEveryFunction(t *testing.T) {
	karkain := phase130Karkain(t)

	withConcat := "func helper() int {\n    return 7\n}\nfunc main() {\n" +
		"    let a = \"x\"\n    print(a + \"y\")\n    print(helper())\n}\n"
	withoutConcat := "func helper() int {\n    return 7\n}\nfunc main() {\n" +
		"    let a = \"x\"\n    print(a)\n    print(helper())\n}\n"

	withImg, _ := natStrStageBuild(t, karkain, withConcat, NativeLinuxTarget)
	withoutImg, _ := natStrStageBuild(t, karkain, withoutConcat, NativeLinuxTarget)

	withFrames := natStrStageFrames(t, withImg)
	withoutFrames := natStrStageFrames(t, withoutImg)
	if len(withFrames) != 2 || len(withoutFrames) != 2 {
		t.Fatalf("expected two frames per image (helper, main), got %d and %d",
			len(withFrames), len(withoutFrames))
	}
	for i := range withFrames {
		diff := withFrames[i] - withoutFrames[i]
		if diff != natStrStageBytes {
			t.Errorf("frame %d grew by %d bytes when main gained a concatenation, want %d.\n"+
				"  The oracle reserves the staging area in EVERY function of a program that "+
				"performs a string operation, so the helper -- which never touches a string -- "+
				"grows too.\n"+
				"  with a concat:    %v\n  without a concat: %v",
				i, diff, natStrStageBytes, withFrames, withoutFrames)
		}
	}
}

// TestPhaseNATStrStage_TheCoverageIsTheOracleFiveOperations is the COVERAGE
// claim.
//
// Three of the oracle's five operations are lowered by the driver today --
// a concatenation, a push() and a string field assignment -- and each must
// reserve the area. The other two, a string equality and a string slice, are
// REFUSED by the driver by name, so the driver never emits an image for them;
// the ORACLE's frame is measured for them anyway, because the coverage claim
// is about the oracle's contract and not about what the driver happens to
// lower today.
func TestPhaseNATStrStage_TheCoverageIsTheOracleFiveOperations(t *testing.T) {
	karkain := phase130Karkain(t)

	// Each entry is an operation and the program that performs it, together
	// with a twin that performs no string operation at all. The twin has the
	// SAME bindings, so the only difference between the two frames is the
	// staging area.
	type opCase struct {
		name string
		op   string
		twin string
		// refused says whether the driver refuses the operation by name.
		refused bool
	}
	ops := []opCase{
		{
			name: "concatenation",
			op:   "func main() {\n    let a = \"x\"\n    print(a + \"y\")\n}\n",
			twin: "func main() {\n    let a = \"x\"\n    print(a)\n}\n",
		},
		{
			name: "push",
			op:   "func main() {\n    let a = [1]\n    let b = push(a, 2)\n    print(b[1])\n}\n",
			// The twin has the SAME bindings, which is the point: `b` is a
			// two-unit value in both -- a PUSHED array carries no element area
			// (the plan gives it zero elements, pkg/native/program.go's scanLets
			// does the same), and a string local is two units as well. A
			// one-element ARRAY literal would be eight bytes LARGER, and the
			// frames would then differ by 2552 rather than by the staging
			// area alone.
			twin: "func main() {\n    let a = [1]\n    let b = \"zz\"\n    print(a[0])\n}\n",
		},
		{
			name: "string_field_assignment",
			op: "type S struct { label string; n int }\nfunc main() {\n" +
				"    let s = S { label: \"a\", n: 1 }\n    s.label = \"changed\"\n    print(s.label)\n}\n",
			twin: "type S struct { label string; n int }\nfunc main() {\n" +
				"    let s = S { label: \"a\", n: 1 }\n    print(s.label)\n}\n",
		},
		{
			name: "string_equality",
			// A string equality is a CONDITION, not a value: the oracle's
			// native backend refuses `==` in a value position outright
			// ("unsupported operator '=='"), and the driver's does too. So the
			// operation is measured in the shape both engines accept -- a
			// comparison in a loop condition -- which is also the shape the
			// oracle's scanStringViewOps looks for.
			op:   "func main() {\n    let s = \"x\"\n    while (s == \"y\") { print(1) }\n}\n",
			twin: "func main() {\n    let s = \"x\"\n    print(s)\n}\n",
			// The driver refuses a string comparison: its condition scanner
			// requires an int operand.
			refused: true,
		},
		{
			name: "string_slice",
			op:   "func main() {\n    let s = \"abc\"\n    print(s[0:2])\n}\n",
			twin: "func main() {\n    let s = \"abc\"\n    print(s)\n}\n",
			// The driver refuses a slice: a slice expression is neither an
			// integer literal nor a local.
			refused: true,
		},
	}

	for _, oc := range ops {
		oc := oc
		t.Run(oc.name, func(t *testing.T) {
			// The ORACLE's frames are measured for EVERY operation, refused
			// ones included: the coverage claim is the oracle's contract.
			goOp := natStrStageBuildOracle(t, karkain, oc.op, NativeLinuxTarget)
			goTwin := natStrStageBuildOracle(t, karkain, oc.twin, NativeLinuxTarget)
			opFrames := natStrStageFrames(t, goOp)
			twinFrames := natStrStageFrames(t, goTwin)
			if len(opFrames) != 1 || len(twinFrames) != 1 {
				t.Fatalf("expected one frame per oracle image, got %d and %d",
					len(opFrames), len(twinFrames))
			}
			if opFrames[0]-twinFrames[0] != natStrStageBytes {
				t.Errorf("the oracle reserves %d bytes for %s, want %d.\n"+
					"  The oracle's five flags are usesConcat, usesStrEq, usesStrSlice, "+
					"usesPush and usesStrField (pkg/native/program.go:4454); each of them "+
					"reserves the staging area.\n"+
					"  operation: %d, twin: %d",
					opFrames[0]-twinFrames[0], oc.name, natStrStageBytes,
					opFrames[0], twinFrames[0])
			}

			// And the driver: the three it lowers must reserve it too, and
			// the two it refuses must refuse BY NAME, with no image.
			kccRes := natStrStageBuildRaw(t, karkain, oc.op, NativeLinuxTarget, "kcc")
			if oc.refused {
				if kccRes.WroteImage {
					t.Errorf("the driver produced an image for %s; it refuses that operation "+
						"by name, so an image means the refusal was bypassed.\n  output:\n%s",
						oc.name, kccRes.Output)
				}
				if !strings.Contains(kccRes.Output, "error[K145]") {
					t.Errorf("the driver did not refuse %s by name.\n  output:\n%s",
						oc.name, kccRes.Output)
				}
				return
			}
			if !kccRes.WroteImage {
				t.Fatalf("the driver refused %s, which it lowers:\n%s", oc.name, kccRes.Output)
			}
			kccOp := natStrStageFrames(t, kccRes.Image)
			kccTwinRes := natStrStageBuildRaw(t, karkain, oc.twin, NativeLinuxTarget, "kcc")
			if !kccTwinRes.WroteImage {
				t.Fatalf("the driver refused the twin of %s:\n%s", oc.name, kccTwinRes.Output)
			}
			kccTwin := natStrStageFrames(t, kccTwinRes.Image)
			if len(kccOp) != 1 || len(kccTwin) != 1 {
				t.Fatalf("expected one frame per driver image, got %d and %d",
					len(kccOp), len(kccTwin))
			}
			if kccOp[0]-kccTwin[0] != natStrStageBytes {
				t.Errorf("the driver reserves %d bytes for %s, want %d.\n"+
					"  operation: %d, twin: %d",
					kccOp[0]-kccTwin[0], oc.name, natStrStageBytes, kccOp[0], kccTwin[0])
			}
		})
	}
}

// TestPhaseNATStrStage_TheDifferentialRecord records what the brief asks to
// be recorded, for every case: the total string-stage allocation, the
// allocation's SCOPE, the string offsets, the function count, the image size,
// the container layout, the frame-size immediates and the byte-identity
// result.
//
// It is a RECORD, and it asserts the relations that must hold (the frames and
// the function count agree; the stage is present in every frame exactly when
// the case says it is) while stating the ones that are known to differ (the
// image size and the literal offsets for the field-assignment case, because
// the oracle's field write stages through the area and the driver's does not).
func TestPhaseNATStrStage_TheDifferentialRecord(t *testing.T) {
	karkain := phase130Karkain(t)
	for _, c := range natStrStageCases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			kccImg, goImg := natStrStageBuild(t, karkain, c.src, NativeLinuxTarget)
			kccFrames := natStrStageFrames(t, kccImg)
			goFrames := natStrStageFrames(t, goImg)

			stage := 0
			if c.wantStage {
				stage = natStrStageBytes
			}
			scope := "none"
			if stage != 0 {
				scope = fmt.Sprintf("all %d functions", len(kccFrames))
			}
			diff, shorter := natStrStageDiffBytes(kccImg, goImg)
			identical := len(kccImg) == len(goImg) && diff == 0

			t.Logf("differential %s: stage=%d bytes scope=%s funcs=%d/%d "+
				"size=%d/%d layout=%s|%s entry=%s|%s frames=%v|%v "+
				"literalOffsets(%q)=%v|%v byteIdentity=%v (differing bytes %d of %d)",
				c.name, stage, scope, len(kccFrames), len(goFrames),
				len(kccImg), len(goImg),
				natStrStageLayout(t, kccImg), natStrStageLayout(t, goImg),
				natStrStageEntry(t, kccImg), natStrStageEntry(t, goImg),
				kccFrames, goFrames, c.probe,
				natStrStageLiteralOffsets(kccImg, c.probe),
				natStrStageLiteralOffsets(goImg, c.probe),
				identical, diff, shorter)

			if len(kccFrames) != len(goFrames) {
				t.Errorf("the function counts differ: native %d, oracle %d", len(kccFrames), len(goFrames))
			}
			for i := range kccFrames {
				if kccFrames[i] != goFrames[i] {
					t.Errorf("frame %d differs: native 0x%x, oracle 0x%x", i,
						uint32(kccFrames[i]), uint32(goFrames[i]))
				}
			}
			if natStrStageLayout(t, kccImg) != natStrStageLayout(t, goImg) {
				t.Errorf("the container layout differs:\n  native %s\n  oracle %s",
					natStrStageLayout(t, kccImg), natStrStageLayout(t, goImg))
			}
		})
	}
}

// TestPhaseNATStrStage_TheProgramsStillRun is the behavioural half.
//
// A frame that matches the oracle's byte for byte is worth nothing if the
// program stops working, so every case is executed as a PE on this host and
// its stdout compared EXACTLY -- never trimmed, because the Step 9b defect was
// a MISSING byte and trimming is exactly the wrong tool for that.
func TestPhaseNATStrStage_TheProgramsStillRun(t *testing.T) {
	if os.PathSeparator == '/' {
		t.Skip("PE images only execute on Windows")
	}
	karkain := phase130Karkain(t)
	for _, c := range natStrStageCases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			res := nativeBuildWithEngine(t, karkain, c.src, NativeWindowsTarget, "kcc", "strstage.exe")
			if res.ExitCode != 0 || !res.WroteImage {
				t.Fatalf("kcc produced no image (exit %d):\n%s", res.ExitCode, res.Output)
			}
			if _, _, err := native.ParsePE(res.Image); err != nil {
				t.Fatalf("the image is not a valid PE: %v", err)
			}
			out, code := natIRunImage(t, res.Image, "strstage")
			if code != 0 {
				t.Errorf("exit = %d, want 0 (stdout %s)", code, natFrameHex(out))
			}
			if got := string(out); got != c.wantOut {
				t.Errorf("stdout = %q, want %q", got, c.wantOut)
			}
		})
	}
}

// TestPhaseNATStrStage_TheOutputIsDeterministic builds the same program twice
// and requires byte-identical images.
//
// The flag is computed once, over the whole program, and threaded into every
// function's layout, so it has to be a FUNCTION of the program rather than of
// the order the functions happen to be visited.
func TestPhaseNATStrStage_TheOutputIsDeterministic(t *testing.T) {
	karkain := phase130Karkain(t)
	for _, c := range natStrStageCases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			first := natStrStageBuildRaw(t, karkain, c.src, NativeLinuxTarget, "kcc")
			if !first.WroteImage {
				t.Fatalf("the driver refused the program:\n%s", first.Output)
			}
			second := natStrStageBuildRaw(t, karkain, c.src, NativeLinuxTarget, "kcc")
			if !second.WroteImage {
				t.Fatalf("the driver refused the program on the second build:\n%s", second.Output)
			}
			if first.SHA != second.SHA {
				t.Errorf("two builds of the same program differ: %s and %s", first.SHA, second.SHA)
			}
			diff, _ := natStrStageDiffBytes(first.Image, second.Image)
			if diff != 0 {
				t.Errorf("two builds of the same program differ in %d bytes", diff)
			}
		})
	}
}

// TestPhaseNATStrStage_TheOracleNameQuirkIsNotReproduced records the ONE
// remaining difference, and says why it is not reproduced.
//
// The oracle's string inference is NAME-based and program-wide: scanConcatSites
// builds a name table from every `let` binding in the program and repeats the
// walk until it is stable (pkg/native/program.go:1180-1182), so a name bound
// to a string ANYWHERE makes that name a string EVERYWHERE -- shadowing
// included.
//
// So in this program the oracle classifies the helper's `a + a` as a string
// CONCATENATION SITE, reserves the staging area in both functions, and
// allocates an arena. Its own EMISSION does not agree: the same program runs
// and prints 6 for `h(3)`, which is an integer addition (measured: the
// oracle's PE for this program prints "x" and "6").
//
// The driver does not reproduce that, and the reason is not convenience: the
// oracle's SCAN over-approximates its own EMISSION, and copying the
// over-approximation would mean allocating an arena for a program that
// allocates nothing and classifying an integer addition as a string
// concatenation. The driver follows the EMISSION.
//
// The oracle's frames are asserted here so the difference is MEASURED and
// cannot be forgotten.
func TestPhaseNATStrStage_TheOracleNameQuirkIsNotReproduced(t *testing.T) {
	karkain := phase130Karkain(t)

	// The helper's `a` is an int parameter; main binds a STRING to the same
	// name. The oracle's program-wide name table cannot see the difference.
	src := "func h(a int) int {\n    return a + a\n}\nfunc main() {\n" +
		"    let a = \"x\"\n    print(a)\n    print(h(3))\n}\n"

	kccRes := natStrStageBuildRaw(t, karkain, src, NativeLinuxTarget, "kcc")
	if !kccRes.WroteImage {
		t.Fatalf("the driver refused the program:\n%s", kccRes.Output)
	}
	goRes := natStrStageBuildRaw(t, karkain, src, NativeLinuxTarget, "go")
	if !goRes.WroteImage {
		t.Fatalf("the oracle refused the program:\n%s", goRes.Output)
	}

	kccFrames := natStrStageFrames(t, kccRes.Image)
	goFrames := natStrStageFrames(t, goRes.Image)

	// The oracle reserves the staging area in both functions ...
	if len(goFrames) != 2 {
		t.Fatalf("the oracle image carries %d frames, want 2", len(goFrames))
	}
	if goFrames[0]-kccFrames[0] != natStrStageBytes {
		t.Errorf("the oracle reserves %d bytes in the helper, want %d.\n"+
			"  If this test fails, the oracle's name-based inference has changed and the "+
			"  divergence recorded here is no longer the one being described.",
			goFrames[0]-kccFrames[0], natStrStageBytes)
	}
	// ... and the driver reserves nothing, because the program performs no
	// string operation.
	for i, f := range kccFrames {
		if f >= natStrStageBytes+608 {
			t.Errorf("the driver reserved the staging area in frame %d (%d bytes) for a "+
				"program with no string operation.\n"+
				"  The driver's kind inference is the plan's slot kinds, so the helper's "+
				"  `a` is an int and `a + a` is an integer addition.", i, f)
		}
	}
	t.Logf("known divergence: the oracle's program-wide name table reserves %d bytes in "+
		"both frames of a program whose only string is a binding; the driver reserves "+
		"none. native frames %v, oracle frames %v",
		natStrStageBytes, kccFrames, goFrames)
}

// TestPhaseNATStrStage_TheRefusalsAreNamed asserts that the constructs the
// driver does not lower are refused BY NAME and produce NO image.
//
// A refusal that produced an image, or one that named the wrong construct,
// would be a silent gap rather than a diagnostic.
func TestPhaseNATStrStage_TheRefusalsAreNamed(t *testing.T) {
	karkain := phase130Karkain(t)
	cases := []struct {
		name string
		src  string
		want string
	}{
		{
			name: "string_in_a_branch",
			src:  "func main() {\n    let s = \"x\"\n    let c = 1\n    if (c > 0) { print(s) }\n}\n",
			want: "is string",
		},
		{
			name: "string_in_a_loop",
			src:  "func main() {\n    let s = \"x\"\n    let i = 0\n    while (i < 2) { print(s) i = i + 1 }\n}\n",
			want: "is string",
		},
		{
			name: "string_in_a_callee",
			src:  "func h() int {\n    print(\"hi\")\n    return 1\n}\nfunc main() {\n    print(h())\n}\n",
			want: "not an integer expression",
		},
		{
			name: "string_equality",
			src:  "func main() {\n    let s = \"x\"\n    print(s == \"y\")\n}\n",
			want: "not lowered",
		},
		{
			name: "string_slice",
			src:  "func main() {\n    let s = \"abc\"\n    print(s[0:2])\n}\n",
			want: "kind 46",
		},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			res := natStrStageBuildRaw(t, karkain, c.src, NativeLinuxTarget, "kcc")
			if res.WroteImage {
				t.Errorf("the driver produced an image for %s; it refuses that construct.\n"+
					"  output:\n%s", c.name, res.Output)
			}
			if !strings.Contains(res.Output, "error[K145]") {
				t.Errorf("the driver did not refuse %s with a K145.\n  output:\n%s", c.name, res.Output)
			}
			if !strings.Contains(res.Output, c.want) {
				t.Errorf("the refusal for %s does not name the construct (want %q).\n  output:\n%s",
					c.name, c.want, res.Output)
			}
		})
	}
}

// TestPhaseNATStrStage_GateIsWiredInCI asserts that this gate has a ci.yml step.
func TestPhaseNATStrStage_GateIsWiredInCI(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join(repoRoot(t), ".github", "workflows", "ci.yml"))
	if err != nil {
		t.Fatalf("reading ci.yml: %v", err)
	}
	if !strings.Contains(string(raw), "TestPhaseNATStrStage_") {
		t.Errorf("ci.yml has no step running this gate's pattern 'TestPhaseNATStrStage_'")
	}
}
