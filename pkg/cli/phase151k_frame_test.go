package cli

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"karkain/pkg/native"
)

// NAT-FRAME -- the native frame layout against the Go oracle's.
//
// The recorded discrepancy was that a native-generated image and the oracle's
// image differed in the FRAME-SIZE immediate for programs that index an array.
// The diagnosis was two defects, and NEITHER of them was the rounding rule:
//
//  1. `natIncReg` built its REX byte by hand and set the REX.**R** bit
//     (0x04) where the register's extension lives in REX.**B** (0x01). So
//     `inc r8` decoded as `inc rax`: the map helper's entry counter was
//     incremented in the WRONG register and the count it then stored was the
//     un-incremented value. A map's entry count was silently wrong.
//
//  2. The map-STAGE slot was never reserved. The oracle's `b.usesMap` is a
//     PROGRAM-WIDE flag, and its `isMapIndex` predicate is "an index
//     expression whose target is a VARIABLE" -- which an ARRAY access
//     satisfies. So the oracle reserves the 8-byte map-stage slot in every
//     frame of a program that indexes anything, and the driver reserved it in
//     none.
//
// The ROUNDING RULE ITSELF was never different: both sides round the frame up
// to the next multiple of 16 on Windows (`if next % 16 != 0 { next += 8 }`) and
// leave it alone on Linux. This gate pins that, so the claim is measured rather
// than inherited.
//
// A THIRD, smaller difference remains and is ISOLATED here rather than
// normalised away: the driver and the oracle intern the runtime-error
// reporter's strings in a different ORDER, so two absolute rodata addresses
// inside the reporter differ (measured: 16 and 39 bytes). The frames and the
// image SIZES agree; the two addresses do not. See the "what remains" section.

// natFrameCases are the frame-layout boundary cases.
//
// Each entry is a different .kark source. The frame-size immediate is read
// out of the GENERATED IMAGE (the `sub rsp, imm32` in main's prologue), not
// computed here, and it is compared with the ORACLE's image of the same
// source.
var natFrameCases = []struct {
	name string
	src  string
}{
	{
		// ZERO locals: the frame is the scratch area plus the argument spill.
		name: "zero_locals",
		src:  "func main() {\n    print(42)\n}\n",
	},
	{
		// A locals size that is NOT a multiple of 16.
		name: "one_local",
		src:  "func main() {\n    let a = 1\n    print(a)\n}\n",
	},
	{
		name: "three_locals",
		src:  "func main() {\n    let a = 1\n    let b = 2\n    let c = 3\n    print(a + b + c)\n}\n",
	},
	{
		// A locals size that IS a multiple of 16 (four int locals), the case
		// the "strictly greater" claim was about.
		name: "four_locals_aligned",
		src:  "func main() {\n    let a = 1\n    let b = 2\n    let c = 3\n    let d = 4\n    print(a + b + c + d)\n}\n",
	},
	{
		// Aligned size plus one.
		name: "five_locals_aligned_plus_one",
		src:  "func main() {\n    let a = 1\n    let b = 2\n    let c = 3\n    let d = 4\n    let e = 5\n    print(a + b + c + d + e)\n}\n",
	},
	{
		// An array: the element area lives in the frame, and an array access
		// is a MAP USE to the oracle, so the map-stage slot is reserved too.
		name: "array_one_element",
		src:  "func main() {\n    let a = [3]\n    print(a[0])\n}\n",
	},
	{
		name: "array_three_elements",
		src:  "func main() {\n    let a = [3, 5, 7]\n    print(a[0])\n}\n",
	},
	{
		// A call: the argument spill area is part of every frame.
		name: "call_with_argument",
		src:  "func f(n int) int {\n    return n + 1\n}\nfunc main() {\n    let a = 1\n    print(f(a))\n}\n",
	},
	{
		// Nested calls: the extras area is sized by the hungriest call site.
		name: "nested_calls",
		src:  "func g(n int) int {\n    return n + 1\n}\nfunc f(n int) int {\n    return g(n) + g(n)\n}\nfunc main() {\n    let a = 1\n    print(f(a))\n}\n",
	},
	{
		// A variable index and a nested index, so the index path is exercised
		// in its two shapes.
		name: "variable_and_nested_index",
		src:  "func main() {\n    let a = [3, 5, 7]\n    let i = 1\n    print(a[i])\n    print(a[a[0] - 3])\n}\n",
	},
}

// natFrameImmediate returns the frame size a PE or ELF reserves in main's
// prologue: the `sub rsp, imm32` (REX.W 81 /5) at the function's entry.
//
// The helpers are emitted first and they use the SHORT form (`48 83 EC`, a
// signed byte), so the first long form found is main's frame.
func natFrameImmediate(t *testing.T, img []byte) int32 {
	t.Helper()
	for i := 0; i+7 <= len(img); i++ {
		if img[i] == 0x48 && img[i+1] == 0x81 && img[i+2] == 0xEC {
			return int32(binary.LittleEndian.Uint32(img[i+3:]))
		}
	}
	t.Fatalf("no frame prologue (48 81 EC) found in the %d-byte image", len(img))
	return 0
}

// natFrameBuild builds one source with each engine for the given target and
// returns the two images.
func natFrameBuild(t *testing.T, karkain, src, target string) (kccImg, goImg []byte) {
	t.Helper()
	kccRes := nativeBuildWithEngine(t, karkain, src, target, "kcc", "frame-kcc.img")
	if kccRes.ExitCode != 0 || !kccRes.WroteImage {
		t.Fatalf("kcc produced no image (exit %d):\n%s", kccRes.ExitCode, kccRes.Output)
	}
	goRes := nativeBuildWithEngine(t, karkain, src, target, "go", "frame-go.img")
	if goRes.ExitCode != 0 || !goRes.WroteImage {
		t.Fatalf("the Go oracle produced no image (exit %d):\n%s", goRes.ExitCode, goRes.Output)
	}
	return kccRes.Image, goRes.Image
}

// TestPhaseNATFrame_TheFrameImmediateMatchesTheOracle is the core claim.
//
// Every boundary case is built with BOTH engines for the SAME target, and the
// frame-size immediate is read out of each image. The two must be EQUAL.
//
// The target matters and the reason is worth stating: on WINDOWS both engines
// round the frame up to the next multiple of 16, which QUANTISES an 8-byte
// difference away -- a program whose frame was 8 bytes short looked identical
// on Windows and differed on Linux. So the Linux case is the one that can see
// a frame delta at all, and BOTH are asserted.
func TestPhaseNATFrame_TheFrameImmediateMatchesTheOracle(t *testing.T) {
	karkain := phase130Karkain(t)
	for _, c := range natFrameCases {
		c := c
		for _, target := range []string{NativeLinuxTarget, NativeWindowsTarget} {
			target := target
			t.Run(c.name+"/"+target, func(t *testing.T) {
				kccImg, goImg := natFrameBuild(t, karkain, c.src, target)
				kccFrame := natFrameImmediate(t, kccImg)
				goFrame := natFrameImmediate(t, goImg)
				if kccFrame != goFrame {
					t.Errorf("the frame-size immediate differs: native 0x%x (%d), oracle 0x%x (%d).\n"+
						"  source:\n%s\n"+
						"  The oracle's map-stage slot is reserved in every frame of a program "+
						"that indexes anything (its `isMapIndex` predicate accepts any "+
						"variable-target index), and the driver must reserve it too.",
						uint32(kccFrame), kccFrame, uint32(goFrame), goFrame, c.src)
				}
			})
		}
	}
}

// TestPhaseNATFrame_TheMapStageSlotIsReserved pins the SECOND defect.
//
// The oracle reserves an 8-byte map-stage slot in every frame of a program
// that indexes anything. A program that indexes NOTHING must not reserve it --
// reserving it unconditionally would grow every frame that does not need it
// and would break the byte-identity of every image that does not use maps.
//
// So the assertion is a DIFFERENCE: a program with an index has a frame 8
// bytes LARGER than the same program without one, and both match the oracle.
func TestPhaseNATFrame_TheMapStageSlotIsReserved(t *testing.T) {
	karkain := phase130Karkain(t)

	// The two programs differ ONLY by the index: the same array, the same
	// locals, once indexed and once not. Comparing an indexing program with a
	// program that has no array would measure the array's element area too.
	withIndex := "func main() {\n    let a = [3, 5, 7]\n    print(a[0])\n}\n"
	withoutIndex := "func main() {\n    let a = [3, 5, 7]\n    print(3)\n}\n"

	idxImg, _ := natFrameBuild(t, karkain, withIndex, NativeLinuxTarget)
	noIdxImg, _ := natFrameBuild(t, karkain, withoutIndex, NativeLinuxTarget)

	idxFrame := natFrameImmediate(t, idxImg)
	noIdxFrame := natFrameImmediate(t, noIdxImg)
	if idxFrame != noIdxFrame+8 {
		t.Errorf("the map-stage slot is not reserved for an indexing program: "+
			"frame with an index 0x%x (%d), without 0x%x (%d), want a difference of 8.\n"+
			"  The oracle reserves it because its map-usage predicate accepts any "+
			"variable-target index, an array access included.",
			uint32(idxFrame), idxFrame, uint32(noIdxFrame), noIdxFrame)
	}
}

// TestPhaseNATFrame_TheIncUsesTheBExtBit pins the FIRST defect, at the byte
// level.
//
// `inc r8` must be REX.W | REX.B (0x49) followed by FF /0 with the rm field
// naming R8. The driver used to set REX.R (0x04) instead, which decodes as
// `inc rax` -- the map helper's entry counter was incremented in the wrong
// register.
//
// The assertion is on the ENCODING, not on a program's behaviour: a wrong
// increment can be invisible in a program's output (the value it corrupts is
// overwritten), and the corruption it causes -- a map's entry count -- has no
// gate at all today.
func TestPhaseNATFrame_TheIncUsesTheBExtBit(t *testing.T) {
	karkain := phase130Karkain(t)
	// A program with an index carries the map helpers, and the map SET helper
	// is the one that increments the entry counter.
	src := "func main() {\n    let a = [3, 5, 7]\n    print(a[0])\n}\n"
	kccImg, goImg := natFrameBuild(t, karkain, src, NativeLinuxTarget)

	kccInc := natFrameFindIncR8(t, kccImg)
	goInc := natFrameFindIncR8(t, goImg)
	if natFrameHex(kccInc) != natFrameHex(goInc) {
		t.Errorf("the `inc r8` encoding differs: native %s, oracle %s.\n"+
			"  The register's extension lives in REX.B (0x01); REX.R (0x04) names the "+
			"ModRM reg field, which for FF /0 is the opcode extension and not a "+
			"register. Setting R instead of B decodes `inc r8` as `inc rax`.",
			natFrameHex(kccInc), natFrameHex(goInc))
	}
	if kccInc[0] != 0x49 {
		t.Errorf("the `inc r8` REX byte is %s, want 49 (REX.W | REX.B).\n"+
			"  4c would be REX.W | REX.R | REX.X, which names RAX.",
			natFrameHex(kccInc))
	}
}

// natFrameFindIncR8 finds the three bytes of an `inc r8` (REX.W|REX.B, FF /0,
// rm=0) in an image.
func natFrameFindIncR8(t *testing.T, img []byte) []byte {
	t.Helper()
	for i := 0; i+3 <= len(img); i++ {
		if img[i] == 0x49 && img[i+1] == 0xFF && img[i+2] == 0xC0 {
			return img[i : i+3]
		}
	}
	t.Fatalf("no `inc r8` (49 FF C0) found in the %d-byte image", len(img))
	return nil
}

// natFrameHex renders bytes as hex for a diagnostic.
func natFrameHex(b []byte) string {
	var sb strings.Builder
	for _, x := range b {
		sb.WriteString(hexByte(x))
	}
	return sb.String()
}

// hexByte renders one byte as two hex digits.
func hexByte(x byte) string {
	const digits = "0123456789abcdef"
	return string([]byte{digits[x>>4], digits[x&0x0f]})
}

// TestPhaseNATFrame_TheImagesAgreeOnEverythingButTheReporterStrings isolates
// what REMAINS.
//
// After the frame and the REX fixes, an indexing program's images differ in
// exactly TWO bytes, and both are absolute rodata addresses inside the
// runtime-error reporter.
//
// The cause is the ORDER in which the reporter's strings are interned: the
// oracle emits the reporter HELPER before the user function, so its strings are
// interned first; the driver appends them to the rodata list AFTER the
// program's own literals. The offsets therefore differ, and the two absolute
// addresses that carry them differ.
//
// This gate asserts the residual DIFFERENCE rather than hiding it: the frame
// immediates are equal, the image sizes are equal, and exactly two bytes
// differ -- all three are stated, so the next increment has a measured target.
func TestPhaseNATFrame_TheImagesAgreeOnEverythingButTheReporterStrings(t *testing.T) {
	karkain := phase130Karkain(t)
	src := "func main() {\n    let a = [3, 5, 7]\n    print(a[0])\n}\n"
	kccImg, goImg := natFrameBuild(t, karkain, src, NativeLinuxTarget)

	if len(kccImg) != len(goImg) {
		t.Errorf("the image sizes differ: native %d, oracle %d", len(kccImg), len(goImg))
	}
	if natFrameImmediate(t, kccImg) != natFrameImmediate(t, goImg) {
		t.Errorf("the frame immediates differ: native 0x%x, oracle 0x%x",
			uint32(natFrameImmediate(t, kccImg)), uint32(natFrameImmediate(t, goImg)))
	}
	diff := 0
	n := len(kccImg)
	if len(goImg) < n {
		n = len(goImg)
	}
	for i := 0; i < n; i++ {
		if kccImg[i] != goImg[i] {
			diff++
		}
	}
	if diff != 2 {
		t.Errorf("the images differ in %d bytes, want 2.\n"+
			"  The two are the runtime-error reporter's absolute rodata addresses: the "+
			"driver and the oracle intern the reporter's strings in a different ORDER "+
			"(the oracle emits the reporter helper before the user function, so its "+
			"strings are interned first; the driver appends them after the program's "+
			"own literals). Everything else -- the frame, the code, the sizes -- agrees.",
			diff)
	}
}

// TestPhaseNATFrame_TheStringStageIsStillDivergent pins the THIRD divergence
// of the same class, and this gate is where it is recorded.
//
// NAT-FRAME found TWO of them and fixed both: the map-stage slot was reserved
// per-function instead of program-wide, and `inc r8` decoded as `inc rax`. A
// third remains, and it is the STRING stage:
//
//   - The oracle's string-stage flag is PROGRAM-WIDE and covers FIVE
//     operations (usesConcat || usesStrEq || usesStrSlice || usesPush ||
//     usesStrField), computed by scanValueUsage over the whole program.
//   - The driver's flag is PER FUNCTION and covers only concat sites and push
//     counts.
//
// So a program whose string work happens in one function gets a DIFFERENT
// frame in another function, and a program that assigns a string to a field
// gets no string stage at all.
//
// MEASURED (windows PE): `s.label = "changed"` gives the driver a 640-byte
// frame and the oracle a 3200-byte one -- a difference of exactly 2560 bytes,
// which is the string stage (8 * 5 * maxBinDepth = 8 * 5 * 64).
//
// This is asserted as a DIFFERENCE, deliberately, for the same reason the
// two-byte reporter residual is: a known gap that is measured and stated
// cannot be forgotten, and a gate that silently ignored it could not be
// noticed when it closed.
//
// It is NOT a NAT-FRAME defect. NAT-FRAME is the frame LAYOUT; this is a
// string-stage POLICY, and it wants its own increment.
func TestPhaseNATFrame_TheStringStageIsStillDivergent(t *testing.T) {
	karkain := phase130Karkain(t)

	// A string assigned to a field. The oracle's scanStructUsage sets
	// usesStrField, which reserves the string stage.
	src := "type S struct { label string; n int }\nfunc main() {\n" +
		"    let s = S { label: \"a\", n: 1 }\n" +
		"    s.label = \"changed\"\n" +
		"    print(s.label)\n}\n"

	kccImg, goImg := natFrameBuild(t, karkain, src, NativeWindowsTarget)
	kccFrame := natFrameImmediate(t, kccImg)
	goFrame := natFrameImmediate(t, goImg)

	if kccFrame == goFrame {
		t.Errorf("the string-stage frames now agree (0x%x). That is progress: move this "+
			"case into TestPhaseNATFrame_TheFrameImmediateMatchesTheOracle so it is "+
			"covered by the parity assertion rather than merely noted here.",
			uint32(kccFrame))
		return
	}
	// The difference is EXACTLY the string stage, which is what makes this a
	// string-stage difference and not something else that happens to be wrong.
	const strStage = 8 * 5 * 64
	if goFrame-kccFrame != strStage {
		t.Errorf("the frame difference is %d bytes, want %d (the string stage).\n"+
			"  A DIFFERENT difference means something else moved, and this pin is no "+
			"  longer describing the real state.",
			goFrame-kccFrame, strStage)
	}
	t.Logf("known divergence: the string stage is reserved program-wide by the oracle "+
		"and per-function by the driver: native 0x%x, oracle 0x%x, difference %d bytes",
		uint32(kccFrame), uint32(goFrame), goFrame-kccFrame)
}

// TestPhaseNATFrame_BoundsAndCallsStillWork is the behavioural half.
//
// A frame that matches the oracle's byte for byte is worth nothing if the
// program stops working. These cases are the ones the brief names: local
// reads and writes, a call, a nested call, a variable index, a nested index, and
// the bounds-error path.
func TestPhaseNATFrame_BoundsAndCallsStillWork(t *testing.T) {
	if os.PathSeparator == '/' {
		t.Skip("PE images only execute on Windows")
	}
	karkain := phase130Karkain(t)

	// A program whose local is written through an assignment and then read,
	// with a call and a nested call in the middle.
	const ok = "func g(n int) int {\n    return n + 1\n}\nfunc f(n int) int {\n    return g(n) + g(n)\n}\n" +
		"func main() {\n    let a = 1\n    a = f(a)\n    let v = [3, 5, 7]\n    let i = 1\n    print(a)\n    print(v[i])\n    print(v[v[0] - 3])\n}\n"
	res := nativeBuildWithEngine(t, karkain, ok, NativeWindowsTarget, "kcc", "frame-ok.exe")
	if res.ExitCode != 0 || !res.WroteImage {
		t.Fatalf("kcc produced no image (exit %d):\n%s", res.ExitCode, res.Output)
	}
	if _, _, err := native.ParsePE(res.Image); err != nil {
		t.Fatalf("the image is not a valid PE: %v", err)
	}
	out, code := natIRunImage(t, res.Image, "frame_ok")
	if code != 0 {
		t.Errorf("exit = %d, want 0", code)
	}
	// f(1) = g(1)+g(1) = 2+2 = 4; v[1] = 5; v[v[0]-3] = v[0] = 3.
	if got := natFrameHex(out); got != "340a350a330a" {
		t.Errorf("stdout = %s, want 340a350a330a (4, 5, 3)", got)
	}

	// The bounds-error path: an out-of-range index is a runtime error with a
	// non-zero exit and no value printed.
	const oob = "func main() {\n    let v = [3, 5, 7]\n    let i = 3\n    print(v[i])\n}\n"
	oobRes := nativeBuildWithEngine(t, karkain, oob, NativeWindowsTarget, "kcc", "frame-oob.exe")
	if oobRes.ExitCode != 0 || !oobRes.WroteImage {
		t.Fatalf("kcc produced no image for the bounds case (exit %d):\n%s",
			oobRes.ExitCode, oobRes.Output)
	}
	oobOut, oobCode := natIRunImage(t, oobRes.Image, "frame_oob")
	if oobCode == 0 {
		t.Errorf("an out-of-range access exited 0; it must be a runtime error (stdout %s)",
			natFrameHex(oobOut))
	}
	if len(oobOut) != 0 {
		t.Errorf("an out-of-range access printed %s; it must print nothing",
			natFrameHex(oobOut))
	}
}

// TestPhaseNATFrame_GateIsWiredInCI asserts that this gate has a ci.yml step.
func TestPhaseNATFrame_GateIsWiredInCI(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join(repoRoot(t), ".github", "workflows", "ci.yml"))
	if err != nil {
		t.Fatalf("reading ci.yml: %v", err)
	}
	if !strings.Contains(string(raw), "TestPhaseNATFrame_") {
		t.Errorf("ci.yml has no step running this gate's pattern 'TestPhaseNATFrame_'")
	}
}
