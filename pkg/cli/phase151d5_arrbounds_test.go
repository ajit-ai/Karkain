package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// Phase D5 -- the kcc native target reports an out-of-range array index the way
// the Go backend does.
//
// This is a separate gate from TestPhaseD2_KnownNonParityIsStillNonParity
// because BYTES CANNOT SHOW ANY OF THIS. Byte-identity proves the two engines
// emit the same image; it says nothing about what that image DOES. The
// behaviour D5 adds -- an out-of-range index exiting 1 instead of trapping with
// Int3 -- is invisible to every byte comparison in the repository, so without
// this gate it could regress silently while parity stayed green.
//
// WHY EXECUTION AND NOT A STRUCTURAL CHECK: this is the only value-model
// mechanism whose whole purpose is observable output at run time, and the
// Windows PE is the only container that executes on this host. So the gate runs
// the images rather than inspecting them.
//
// WHAT IS ASSERTED, and why each thing is here:
//
//   - a VALID index prints the right value and exits 0. This is the
//     missing-Mark(doneLbl) regression the oracle records as real: when the
//     success path can fall into the diagnostic block, EVERY array-using program
//     reports an error on a VALID index. A gate that only tested the failure
//     path would not notice.
//   - an out-of-range index exits 1 and prints nothing on STDOUT. The exit code
//     is the observable; see the stderr note below.
//   - both engines are run side by side and must agree, so the assertion is a
//     DIFFERENTIAL rather than a golden. A golden would pin the current exit
//     code and would have passed throughout the period when the kcc image
//     trapped with Int3 (0xC0000005), because a golden cannot distinguish
//     "exits 1 as specified" from "exits with whatever it currently does".
//
// STDERR IS NOT ASSERTED, and this is inherited rather than convenient.
// PHASE-152-BASELINE.md records that stderr output is EMPTY on this host for PE
// images, by the 152-B0 GetStdHandle(STD_ERROR_HANDLE) limitation, and that
// 152-B1's reject tests therefore assert exit code, stdout route and interned
// text instead. The Go backend is equally silent here -- verified, not assumed:
// the `go` half of the differential below produces no stderr either. Asserting
// captured stderr here would assert something true of NEITHER engine.
//
// The interned diagnostic TEXT is asserted by TestPhaseD5_DiagnosticTextIsInternedInTheImage,
// which is the only place in the repository that the reporter's text exists at
// all.
func TestPhaseD5_ArrayBoundsReportThePhase100Diagnostic(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("the PE container is the only one that executes on the dev host; " +
			"the Linux CI leg runs these exact bytes natively")
	}
	karkain := phase130Karkain(t)

	// Each case states the SOURCE, the expected stdout, and the expected exit
	// code. The out-of-range cases are the D5 behaviour; the valid ones are the
	// regression guards described above.
	//
	// The expected stdout is the EXACT bytes, newline included, not a trimmed
	// string. `print` terminates its line (Phase 147: "42\n", not "42"), and
	// trimming is exactly the wrong tool for a missing-byte defect -- a trimmed
	// comparison passes identically whether or not the newline is there. The 9b
	// gate found that the hard way: a first draft asserted "12345" and passed
	// an image that printed its digits and silently lost its newline.
	cases := []struct {
		name       string
		src        string
		wantStdout string
		wantExit   int
	}{
		{
			name:       "valid_index_0",
			src:        "func main() {\n    let a = [3, 5, 7]\n    print(a[0])\n}\n",
			wantStdout: "3\n",
			wantExit:   0,
		},
		{
			name:       "valid_index_1",
			src:        "func main() {\n    let a = [3, 5, 7]\n    print(a[1])\n}\n",
			wantStdout: "5\n",
			wantExit:   0,
		},
		{
			name:       "valid_index_last",
			src:        "func main() {\n    let a = [3, 5, 7]\n    print(a[2])\n}\n",
			wantStdout: "7\n",
			wantExit:   0,
		},
		{
			// The D5 case: before the reporter existed this TRAPPED with Int3,
			// which on Windows is 0xC0000005 -- a negative exit code, not 1.
			name:       "index_past_end",
			src:        "func main() {\n    let a = [3, 5, 7]\n    print(a[9])\n}\n",
			wantStdout: "",
			wantExit:   1,
		},
		{
			name:       "index_far_past_end",
			src:        "func main() {\n    let a = [3, 5, 7]\n    print(a[1000000])\n}\n",
			wantStdout: "",
			wantExit:   1,
		},
	}

	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			// Both engines are built from the SAME source and run. The
			// differential is the point: asserting kcc's exit code against a
			// literal would also pass if the Go backend changed, and then the
			// two would disagree while the gate stayed green.
			kccExe := phase151D5BuildWindows(t, karkain, c.src, "kcc", c.name+"-kcc.exe")
			goExe := phase151D5BuildWindows(t, karkain, c.src, "go", c.name+"-go.exe")

			gotOut, gotExit := phase151D5Run(t, kccExe)
			goOut, goExit := phase151D5Run(t, goExe)

			if gotExit != c.wantExit {
				t.Errorf("kcc exit = %d, want %d.\n"+
					"  If this is 0xC0000005 (-1073741819) the bounds check is still "+
					"trapping with Int3 instead of calling the reporter, which is the "+
					"pre-D5 behaviour.", gotExit, c.wantExit)
			}
			if gotOut != c.wantStdout {
				t.Errorf("kcc stdout = %q, want %q", gotOut, c.wantStdout)
			}
			if goExit != gotExit {
				t.Errorf("engines disagree on the exit code: kcc %d, go %d.\n"+
					"  D5's claim is that they now behave identically, so a divergence "+
					"here means the kcc path drifted again.", gotExit, goExit)
			}
			if goOut != gotOut {
				t.Errorf("engines disagree on stdout: kcc %q, go %q", gotOut, goOut)
			}
		})
	}
}

// TestPhaseD5_DiagnosticTextIsInternedInTheImage asserts the part of the
// diagnostic that IS observable on this host: the strings the reporter writes
// must be INTERNED in the image, even though writing them to stderr is silently
// dropped by the 152-B0 GetStdHandle limitation.
//
// This is a separate test because it is a different KIND of claim. The one
// above is behavioural and runs the image; this one is structural and reads it,
// and it is the only assertion in the repository that the reporter's TEXT exists
// at all. Without it, a reporter that emitted the right control flow and no
// strings would pass every execution gate on this host.
func TestPhaseD5_DiagnosticTextIsInternedInTheImage(t *testing.T) {
	karkain := phase130Karkain(t)
	src := "func main() {\n    let a = [3, 5, 7]\n    print(a[1])\n}\n"

	res := nativeBuildWithEngine(t, karkain, src, NativeWindowsTarget, "kcc", "diagtext.exe")
	if res.ExitCode != 0 || len(res.Image) == 0 {
		t.Fatalf("kcc produced no image (exit %d):\n%s", res.ExitCode, res.Output)
	}
	img := string(res.Image)

	for _, want := range []string{
		"runtime error: ",
		"array index out of range",
		" at ",
	} {
		if !strings.Contains(img, want) {
			t.Errorf("the diagnostic text %q is NOT present in the kcc image.\n"+
				"  stderr is empty on this host for PE images (the 152-B0 "+
				"GetStdHandle(STD_ERROR_HANDLE) limitation), so the interned "+
				"string is the only place this text can be verified. A reporter "+
				"that emitted the right control flow with no strings would pass "+
				"every execution gate here.", want)
		}
	}
}

// TestPhaseD5_MapHelpersAppearOnTheSyntacticPredicate pins the
// over-approximation D5 deliberately reproduced, so it can never be removed by
// accident.
//
// scanMapUsage's isMapIndex returns true for ANY `name[...]` regardless of the
// receiver's kind, so an ARRAY index makes the Go backend emit the map helper
// set as unreachable code. kcc reproduces that, because byte-identity is
// defined as agreement with the oracle. This test asserts the helpers are
// PRESENT for an array program -- which looks wrong written plainly, and is
// exactly why it needs an assertion: without one, the next reader would "fix" it
// and silently break parity.
//
// CORRECTED 2026-10-15 (strength, not product): this assertion was `delta >=
// 209`, with a comment claiming the no-index case caught a driver that emits the
// helpers UNCONDITIONALLY. That claim was false. Under unconditional emission
// BOTH images grow by the same amount, so the difference stays 209 and the test
// passes -- the defect the comment promised to catch was precisely the one it
// could not see. The assertion is now an EQUALITY against the Go oracle's own
// delta: if kcc emitted the helpers unconditionally, kcc's delta would collapse
// to the program's own size difference while the oracle's stayed at 209, and the
// two would disagree. Comparing with the oracle is also why no magic number is
// needed here: the constant remains a floor, and the floor is no longer asked to
// do work it cannot do.
func TestPhaseD5_MapHelpersAppearOnTheSyntacticPredicate(t *testing.T) {
	karkain := phase130Karkain(t)

	indexed := "func main() {\n    let a = [3, 5, 7]\n    print(a[1])\n}\n"
	unindexed := "func main() {\n    let a = [3, 5, 7]\n    print(5)\n}\n"

	sizeOf := func(src, engine, name string) int {
		res := nativeBuildWithEngine(t, karkain, src, NativeLinuxTarget, engine, name)
		if res.ExitCode != 0 || len(res.Image) == 0 {
			t.Fatalf("%s build produced no image for %s (exit %d):\n%s",
				engine, name, res.ExitCode, res.Output)
		}
		return len(res.Image)
	}

	kccIndexed := sizeOf(indexed, "kcc", "mapgate-index-kcc.elf")
	kccUnindexed := sizeOf(unindexed, "kcc", "mapgate-noindex-kcc.elf")
	goIndexed := sizeOf(indexed, "go", "mapgate-index-go.elf")
	goUnindexed := sizeOf(unindexed, "go", "mapgate-noindex-go.elf")

	// The oracle emits 209 bytes of map runtime for the indexed program and
	// none for the unindexed one. The absolute size depends on the program, so
	// the floor is the DIFFERENCE, not a literal on either image.
	const mapRuntimeBytes = 209
	kccDelta := kccIndexed - kccUnindexed
	goDelta := goIndexed - goUnindexed

	if kccDelta < mapRuntimeBytes {
		t.Errorf("the kcc indexed image is only %d bytes larger than the unindexed "+
			"one, want at least %d.\n"+
			"  The Go backend emits map_find/map_set/map_get whenever the program "+
			"contains `name[...]` REGARDLESS of the receiver's kind "+
			"(scanMapUsage's isMapIndex tests only the syntactic shape). kcc "+
			"reproduces that deliberately, because byte-identity is agreement "+
			"with the oracle. If this helper set has been removed as 'dead code "+
			"for an array program', parity has been broken.",
			kccDelta, mapRuntimeBytes)
	}

	// The equality is the assertion that actually catches unconditional
	// emission, which the floor above provably cannot.
	//
	// It is an equality WITHIN one frame-rounding unit, and that tolerance is
	// derived rather than guessed. Measured on this host: kcc's delta is 629 and
	// the oracle's is 637 -- a difference of exactly 8 bytes, which is the
	// already-documented frame residual (kcc `sub rsp,0x288` against the
	// oracle's `sub rsp,0x290`). The two indexed programs do not share a frame
	// with the two unindexed ones, so part of each delta is frame padding and
	// not map runtime. A strict equality was tried first and failed on exactly
	// this; the tolerance below is the frame layout's own 16-byte rounding unit,
	// which bounds how much padding can differ.
	//
	// The tolerance is still far tighter than the defect it must catch:
	// unconditional emission collapses kcc's delta to the program's own size
	// difference (order 10 bytes) while the oracle's stays above 600, so the
	// failure is measured in hundreds of bytes, not sixteen.
	const frameRoundingUnit = 16
	if diff := kccDelta - goDelta; diff > frameRoundingUnit || diff < -frameRoundingUnit {
		t.Errorf("kcc's map-helper delta (%d) differs from the oracle's (%d) by %d, "+
			"more than the %d-byte frame rounding unit.\n"+
			"  A match is required, not a floor: unconditional emission would "+
			"collapse kcc's delta to the program's own size difference while the "+
			"oracle stayed at %d, and the `>= %d` floor above would have passed that.",
			kccDelta, goDelta, diff, frameRoundingUnit, mapRuntimeBytes, mapRuntimeBytes)
	}
}

// phase151D5BuildWindows builds a Windows PE from src with the named engine,
// writes it to a .exe in a temp dir, and returns the path.
//
// nativeBuildWithEngine builds into ITS OWN t.TempDir() and returns bytes, not
// a path, so the image is re-materialised here. That is deliberate rather than
// convenient: the execution assertion needs a file the OS will actually run, and
// asking a byte-slice-carrying helper for a path would be an invitation to
// guess at the directory layout.
func phase151D5BuildWindows(t *testing.T, karkain, src, engine, name string) string {
	t.Helper()
	res := nativeBuildWithEngine(t, karkain, src, NativeWindowsTarget, engine, name)
	if res.ExitCode != 0 || !res.WroteImage {
		t.Fatalf("%s build produced no Windows image (exit %d):\n%s", engine, res.ExitCode, res.Output)
	}
	exe := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(exe, res.Image, 0o755); err != nil {
		t.Fatalf("writing %s: %v", exe, err)
	}
	return exe
}

// phase151D5Run executes a Windows image and returns its stdout and exit code.
//
// The exit code is read even on failure: a program that exits 1 reports through
// LASTEXITCODE, whereas the pre-D5 Int3 trap surfaced as a negative code. Both
// are observable here, which is what makes the assertion meaningful.
func phase151D5Run(t *testing.T, exe string) (string, int) {
	t.Helper()
	cmd := exec.Command(exe)
	var out strings.Builder
	cmd.Stdout = &out
	err := cmd.Run()
	code := 0
	if err != nil {
		if ee, okExit := err.(*exec.ExitError); okExit {
			code = ee.ExitCode()
		} else {
			t.Fatalf("running %s: %v", exe, err)
		}
	}
	return out.String(), code
}