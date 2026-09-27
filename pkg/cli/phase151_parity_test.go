package cli

// Phase 151 — the byte-parity HARNESS. Written before any kcc native codegen,
// because without it every parity claim in increment 151 would be an
// assertion. This file is the measuring instrument, not the thing measured.
//
// WHY A HARNESS IS THE FIRST DELIVERABLE. The measured starting position
// (PHASE-151-BASELINE.md §1.1) is that a native build under
// KARKAIN_ENGINE=kcc is a SILENT GO FALLBACK: the image is byte-identical to
// the Go engine's, so "the bytes match" proves nothing. A naive parity test
// would therefore pass TODAY, with zero lines of kcc native code.
//
// THE FALLACY THIS HARNESS EXISTS TO CATCH. Image bytes are not evidence of
// parity when a fallback can produce the same bytes. The only sound check is
// to establish PROVENANCE: which engine actually emitted the image. A fallback
// is caught by a signal the fallback cannot imitate — kcc printing its own
// native banner, and eventually a stamp inside the image itself.
//
// So every comparison pairs the byte comparison with a PROVENANCE assertion.
// When 151A lands, kcc-emitted images must satisfy the kcc provenance check;
// today they cannot, and the harness says so out loud rather than passing
// quietly.

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// nativeBuildResult is one build's observable output, captured for comparison.
type nativeBuildResult struct {
	Image      []byte // raw native image bytes, nil when none was written
	SHA        string // image digest, empty when there is no image
	Output     string // combined stdout+stderr
	ExitCode   int
	WroteImage bool
}

// hashImage returns the hex SHA-256 of an image, "" for no image.
func hashImage(img []byte) string {
	if len(img) == 0 {
		return ""
	}
	sum := sha256.Sum256(img)
	return hex.EncodeToString(sum[:])
}

// nativeBuildWithEngine builds src for the given native target with an
// explicitly chosen engine, returning everything observable about the run.
//
// The engine is pinned through KARKAIN_ENGINE rather than a --engine flag
// because the fallback happens in the Go-side dispatch in main.go, and pinning
// the env var is what makes the fallback observable at all.
func nativeBuildWithEngine(t *testing.T, karkain, src, target, engine, outName string) nativeBuildResult {
	t.Helper()
	dir := t.TempDir()
	srcPath := filepath.Join(dir, "probe.kark")
	if err := os.WriteFile(srcPath, []byte(src), 0o644); err != nil {
		t.Fatalf("writing probe: %v", err)
	}
	outPath := filepath.Join(dir, outName)

	cmd := exec.Command(karkain, "build", srcPath, "--target", target, "-o", outPath)
	cmd.Env = append(os.Environ(), "KARKAIN_ENGINE="+engine)
	out, err := cmd.CombinedOutput()
	code := 0
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			code = ee.ExitCode()
		} else {
			t.Fatalf("running %s build (%s): %v", engine, target, err)
		}
	}
	res := nativeBuildResult{Output: string(out), ExitCode: code}
	if img, rerr := os.ReadFile(outPath); rerr == nil && len(img) > 0 {
		res.Image = img
		res.SHA = hashImage(img)
		res.WroteImage = true
	}
	return res
}

// kccProducedImage reports whether a build output shows the SELF-HOSTED engine
// actually doing the native lowering, as opposed to the Go fallback.
//
// Today this is deliberately false for native targets, and that is the honest
// state: kcc has no native codegen (baseline §1), so every "kcc" native image
// is the Go one. The 151A slices replace this with a real provenance signal.
//
// It is a function rather than a constant so the slices change ONE place.
func kccProducedImage(out string) bool {
	return strings.Contains(out, "[kcc] native image")
}

// assertNativeParity compares a Go build against a kcc build for one source and
// target. It returns BOTH results so a caller can report a precise diagnosis
// rather than a single pass/fail that hides which half failed.
func assertNativeParity(t *testing.T, karkain, src, target, outName string) (goRes, kccRes nativeBuildResult) {
	t.Helper()
	goRes = nativeBuildWithEngine(t, karkain, src, target, "go", outName)
	kccRes = nativeBuildWithEngine(t, karkain, src, target, "kcc", outName)
	return goRes, kccRes
}

// TestPhase151_HarnessDetectsNonParity is the harness's own self-test.
//
// It proves the instrument is not broken, and it encodes the invariant that
// survives the whole increment:
//
//	AN IMAGE MUST NEVER EXIST WITHOUT kcc PROVENANCE.
//
// There are exactly two acceptable states for a kcc native build:
//
//	(a) 151A+ — kcc emits the image, and its output carries the kcc-native
//	    provenance marker. Bytes are then compared for real.
//	(b) today — kcc refuses loudly with K116 and writes no image.
//
// The unacceptable third state is the one this test exists to kill: an image
// written by the Go backend while the caller believes it came from kcc. That
// was the measured baseline state (§1.1) and it is exactly what made "the
// bytes match" worthless as evidence. So the assertion is deliberately written
// as a prohibition rather than as a comparison — it must hold in BOTH
// acceptable states, and it must fail the moment the fallback reappears.
func TestPhase151_HarnessDetectsNonParity(t *testing.T) {
	karkain := phase130Karkain(t)
	const src = "func main() {\n\tprint(42)\n}\n"

	goRes, kccRes := assertNativeParity(t, karkain, src, NativeLinuxTarget, "probe.elf")

	// The Go leg is the oracle and must always work; if it does not, nothing
	// below is a real comparison.
	if !goRes.WroteImage {
		t.Fatalf("go engine produced no image: %s", goRes.Output)
	}

	// THE INVARIANT. An image from the kcc request is only legitimate if kcc
	// can prove it produced it.
	if kccRes.WroteImage && !kccProducedImage(kccRes.Output) {
		t.Fatalf("silent Go fallback: the kcc request wrote an image (%s) with no kcc-native provenance.\n"+
			"  That is the exact dishonesty the 151 baseline measured: the caller asked for kcc and\n"+
			"  received Go output, so byte-equality would prove nothing.\n"+
			"  kcc output was: %s", shortSHA(kccRes.SHA), kccRes.Output)
	}

	switch {
	case kccProducedImage(kccRes.Output):
		// (a) 151A landed: kcc owns the target. Now the bytes carry meaning,
		// so compare them for real rather than logging a note.
		if goRes.SHA != kccRes.SHA {
			t.Errorf("kcc provenance present but images differ: go=%s kcc=%s\n"+
				"  151A-151C require byte-identical output for the same source and target.",
				shortSHA(goRes.SHA), shortSHA(kccRes.SHA))
		}
		t.Logf("kcc owns the target and matches the Go oracle byte for byte (%s)", shortSHA(goRes.SHA))
	case kccRes.WroteImage:
		// Unreachable: the invariant above already failed. Kept so the switch
		// is exhaustive and a future edit cannot fall through silently.
		t.Fatal("unreachable: image without provenance")
	default:
		// (b) the current state: a loud refusal and no image. Assert the
		// refusal is actually loud and names the code, because "silent" here
		// would reintroduce the same dishonesty one layer up.
		if kccRes.ExitCode == 0 {
			t.Errorf("kcc native build exited 0 with no image and no refusal: %s", kccRes.Output)
		}
		if !strings.Contains(kccRes.Output, "K116") {
			t.Errorf("kcc native refusal does not name its code (want K116): %s", kccRes.Output)
		}
		t.Logf("CONFIRMED NO FALLBACK: kcc refused with K116 and wrote no image; the Go oracle is %s. "+
			"This is the state 151A closes by making kcc emit the image itself.", shortSHA(goRes.SHA))
	}
}

// TestPhase151_HarnessComparesBytes is the positive control: the harness must be
// able to see a byte DIFFERENCE, or its "identical" verdict is vacuous.
//
// It gets that difference legitimately, by building the same source for two
// different OS containers, which cannot produce the same image. If this test
// ever fails, the harness stopped comparing bytes at all.
func TestPhase151_HarnessComparesBytes(t *testing.T) {
	karkain := phase130Karkain(t)
	const src = "func main() {\n\tprint(42)\n}\n"

	elf := nativeBuildWithEngine(t, karkain, src, NativeLinuxTarget, "go", "a.elf")
	pe := nativeBuildWithEngine(t, karkain, src, NativeWindowsTarget, "go", "b.exe")

	if !elf.WroteImage || !pe.WroteImage {
		t.Fatalf("expected both containers to build (elf=%v pe=%v)", elf.WroteImage, pe.WroteImage)
	}
	if elf.SHA == pe.SHA {
		t.Errorf("harness is vacuous: ELF and PE images hash identically (%s)", shortSHA(elf.SHA))
	}
}

// shortSHA trims a hex digest for log lines.
func shortSHA(sha string) string {
	if len(sha) <= 12 {
		return sha
	}
	return sha[:12]
}

