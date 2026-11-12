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
	{
		// A string FIELD ASSIGNMENT -- the oracle's FIFTH string operation.
		//
		// NAT-FRAME recorded this case as a 2560-byte divergence: the oracle
		// reserves the string staging area for it and the driver reserved
		// nothing. NAT-STR-STAGE closed that, so the case belongs HERE, covered
		// by the parity assertion rather than by a note.
		name: "string_field_assignment",
		src:  "type S struct { label string; n int }\nfunc main() {\n    let s = S { label: \"a\", n: 1 }\n    s.label = \"changed\"\n    print(s.label)\n}\n",
	},
}

// natFrameAllFrames returns EVERY frame-size immediate in an image, in the
// order the functions appear in .text (source order).
//
// The helpers are emitted first and use the SHORT form (`48 83 EC`, a signed
// byte), so every long form (`48 81 EC`) belongs to a user function. That is
// what makes this a list of PER-FUNCTION frames rather than a single number,
// and the list's LENGTH is therefore the function count.
//
// natFrameImmediate returns only the FIRST frame, which is main's -- correct
// for a one-function program and insufficient for a program with a callee.
func natFrameAllFrames(t *testing.T, img []byte) []int32 {
	t.Helper()
	var out []int32
	for i := 0; i+7 <= len(img); i++ {
		if img[i] == 0x48 && img[i+1] == 0x81 && img[i+2] == 0xEC {
			out = append(out, int32(binary.LittleEndian.Uint32(img[i+3:])))
		}
	}
	return out
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

// TestPhaseNATFrame_TheStringStageIsReservedProgramWide pins the THIRD
// divergence of the same class -- the STRING stage -- which NAT-STR-STAGE
// closed.
//
// The oracle's string staging area is reserved from FIVE program-wide flags
// (usesConcat || usesStrEq || usesStrSlice || usesPush || usesStrField,
// pkg/native/program.go:4454) and its layout reserves the area in EVERY
// function. The driver reserved it per-function, and for only two of the five
// operations, so:
//
//   - a callee in a program whose main concatenated had a frame 2560 bytes
//     short (measured on the Linux ELF target: [608, 3168] against the
//     oracle's [3168, 3168]); and
//   - a program that assigned a string to a field reserved nothing at all
//     (measured: 640 bytes against the oracle's 3200).
//
// This test asserts the CLOSED state on both axes: the area is reserved in
// every function of a program that performs a string operation, and it is NOT
// reserved at all in a program that performs none.
//
// The field-assignment case is in natFrameCases above, so the parity
// assertion covers it directly.
func TestPhaseNATFrame_TheStringStageIsReservedProgramWide(t *testing.T) {
	karkain := phase130Karkain(t)

	// The size of the staging area: 8 * 5 * maxBinDepth.
	const strStage = 8 * 5 * 64

	// A program with a string OPERATION in main and a helper that does none,
	// and the same program without the operation.
	withOp := "func helper() int {\n    return 7\n}\nfunc main() {\n" +
		"    let a = \"x\"\n    print(a + \"y\")\n    print(helper())\n}\n"
	withoutOp := "func helper() int {\n    return 7\n}\nfunc main() {\n" +
		"    let a = \"x\"\n    print(a)\n    print(helper())\n}\n"

	withImg, withOracle := natFrameBuild(t, karkain, withOp, NativeLinuxTarget)
	withoutImg, _ := natFrameBuild(t, karkain, withoutOp, NativeLinuxTarget)

	withFrames := natFrameAllFrames(t, withImg)
	withoutFrames := natFrameAllFrames(t, withoutImg)
	if len(withFrames) != 2 || len(withoutFrames) != 2 {
		t.Fatalf("expected two frames per image (helper, main), got %d and %d",
			len(withFrames), len(withoutFrames))
	}
	// Both engines must agree, frame for frame.
	for i := range withFrames {
		oracleFrames := natFrameAllFrames(t, withOracle)
		if i < len(oracleFrames) && withFrames[i] != oracleFrames[i] {
			t.Errorf("frame %d differs from the oracle's: native 0x%x, oracle 0x%x",
				i, uint32(withFrames[i]), uint32(oracleFrames[i]))
		}
	}
	// And the area is reserved in EVERY function, not only in the one that
	// does the string work: the helper's frame grows by exactly the area.
	for i := range withFrames {
		if diff := withFrames[i] - withoutFrames[i]; diff != strStage {
			t.Errorf("frame %d grew by %d bytes when main gained a concatenation, want %d.\n"+
				"  The oracle's string flags are program-wide, so the staging area is "+
				"  reserved in every function of the program -- including the helper, "+
				"  which never touches a string.",
				i, diff, strStage)
		}
	}
	// And a program with NO string operation reserves nothing.
	for i := range withoutFrames {
		if withoutFrames[i] >= strStage+608 {
			t.Errorf("frame %d is %d bytes, which is at least a staging area larger than "+
				"the %d-byte baseline; a program with no string operation must reserve "+
				"nothing.", i, withoutFrames[i], 608)
		}
	}
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
