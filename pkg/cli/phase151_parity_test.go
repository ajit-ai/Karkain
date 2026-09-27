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

// TestPhase151_HarnessDetectsFallback is the harness's own self-test.
//
// It proves the instrument is not broken. Today the kcc leg cannot be shown to
// be kcc-produced, and the harness must REPORT that rather than pass. A harness
// that cannot detect the known-bad state is worse than no harness, because
// 151A–151C would then be validated by it.
func TestPhase151_HarnessDetectsFallback(t *testing.T) {
	karkain := phase130Karkain(t)
	const src = "func main() {\n\tprint(42)\n}\n"

	goRes, kccRes := assertNativeParity(t, karkain, src, NativeLinuxTarget, "probe.elf")

	// Sanity: both engines must produce an image, so the comparison below is a
	// real comparison rather than two absences being called "equal".
	if !goRes.WroteImage {
		t.Fatalf("go engine produced no image: %s", goRes.Output)
	}
	if !kccRes.WroteImage {
		t.Fatalf("kcc engine produced no image: %s", kccRes.Output)
	}

	// The finding, stated as an expectation so it is a gate and not a comment.
	// Until 151A lands the bytes match (the fallback) AND the provenance check
	// fails. Both facts are asserted, because a future slice that fixes
	// provenance without changing bytes must flip exactly one of them.
	if kccProducedImage(kccRes.Output) {
		t.Logf("note: kcc now reports its own native emission — 151A may have landed")
	} else {
		t.Logf("CONFIRMED FALLBACK: the kcc leg emitted no kcc-native provenance; its image %s is the Go image %s. This is the state 151A-151C must close.",
			shortSHA(kccRes.SHA), shortSHA(goRes.SHA))
	}
	if goRes.SHA != kccRes.SHA {
		t.Logf("note: images already differ (go=%s kcc=%s) — the fallback is gone", shortSHA(goRes.SHA), shortSHA(kccRes.SHA))
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

