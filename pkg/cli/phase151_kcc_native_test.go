package cli

import (
	"strings"
	"testing"

	"karkain/pkg/codegen"
)

// nativeCfg is a minimal Config selecting one C-free native target. The refusal
// under test reads only Target, so nothing else needs filling in.
func nativeCfg(target string) codegen.Config { return codegen.Config{Target: target} }

// Phase 151D-first — the "no silent Go fallback" gate.
//
// Increment 150 implemented the three C-free native targets in Go (pkg/native)
// and made kcc the default engine for everything else. The two facts combined
// into a silent hand-off: a native target requested under kcc was built by the
// Go backend, with no error and no signal that the engine had changed. The 151
// baseline measured it (docs/audit/PHASE-151-BASELINE.md §1.1) and found the
// resulting image byte-identical to the Go one, which made "parity" untestable.
//
// This gate pins the fix. It is deliberately NOT a parity test — 151A is what
// earns parity. This is the smaller, independent requirement that the baseline
// states as non-negotiable:
//
//	"No silent Go fallback. If kcc cannot lower a construct it must be a loud
//	 refusal naming the construct, never a quiet hand-off to the Go engine."

// TestPhase151_NoSilentGoFallback is the whole point of the slice, stated as a
// gate. For every C-free native target, under kcc:
//
//	must exit non-zero, must name K116, and must NOT write an image.
//
// "Must not write an image" is the load-bearing half. A refusal that still left
// a Go image on disk would satisfy the letter of the rule while preserving the
// dishonesty, because a stale artifact is indistinguishable from real output.
func TestPhase151_NoSilentGoFallback(t *testing.T) {
	karkain := phase130Karkain(t)
	const src = "func main() {\n\tprint(42)\n}\n"

	targets := []string{NativeLinuxTarget, NativeWindowsTarget, NativeMacOSTarget}
	for _, tgt := range targets {
		tgt := tgt
		t.Run("build/"+tgt, func(t *testing.T) {
			res := nativeBuildWithEngine(t, karkain, src, tgt, "kcc", "probe.out")

			if res.WroteImage {
				t.Fatalf("%s: kcc build wrote an image (%s) but kcc has no native backend; "+
					"that image is Go output presented as kcc output", tgt, shortSHA(res.SHA))
			}
			if res.ExitCode == 0 {
				t.Errorf("%s: kcc native build exited 0 with no image and no diagnostic: %s", tgt, res.Output)
			}
			if !strings.Contains(res.Output, "K116") {
				t.Errorf("%s: refusal does not name its diagnostic code (want K116): %s", tgt, res.Output)
			}
			// The message must name the target, so the user can tell WHICH
			// request was refused without re-reading their own command line.
			if !strings.Contains(res.Output, tgt) {
				t.Errorf("%s: refusal does not name the target: %s", tgt, res.Output)
			}
			// And it must not pretend the Go engine is equivalent: the whole
			// point is that a hand-off is not the same engine.
			if strings.Contains(res.Output, "[native-") {
				t.Errorf("%s: refusal still printed a Go native build banner: %s", tgt, res.Output)
			}
		})
	}
}

// TestPhase151_NativeGoBackendUnaffected is the no-regression half. Closing the
// fallback must not cost the Go engine anything: the three native targets are
// still built by Go, from any host, with no C compiler. If this ever fails, the
// refusal has leaked into the Go path and 150's deliverable has been broken.
func TestPhase151_NativeGoBackendUnaffected(t *testing.T) {
	karkain := phase130Karkain(t)
	const src = "func main() {\n\tprint(42)\n}\n"

	for _, tgt := range []string{NativeLinuxTarget, NativeWindowsTarget, NativeMacOSTarget} {
		tgt := tgt
		t.Run(tgt, func(t *testing.T) {
			res := nativeBuildWithEngine(t, karkain, src, tgt, "go", "probe.out")
			if !res.WroteImage {
				t.Fatalf("%s: go engine stopped producing native images: %s", tgt, res.Output)
			}
			if res.ExitCode != 0 {
				t.Errorf("%s: go native build exited %d: %s", tgt, res.ExitCode, res.Output)
			}
			if strings.Contains(res.Output, "K116") {
				t.Errorf("%s: the K116 refusal leaked into the Go path: %s", tgt, res.Output)
			}
		})
	}
}

// TestPhase151_KccNativeRefusalIsNotAFallback pins the seam itself, without
// spawning a process, so the invariant is checked at the unit level too: while
// kcc does not own native targets, the command must refuse. When 151A lands
// and kccOwnsNativeTargets flips to true, this test is the one that must be
// deleted or inverted in the same commit as the flip — which is the point of
// naming it here rather than leaving the switch untested.
func TestPhase151_KccNativeRefusalIsNotAFallback(t *testing.T) {
	res := KCCNativeBuildCommand("hello.kark", "out.elf", nativeCfg(NativeLinuxTarget), false)
	if res.ExitCode == ExitSuccess {
		t.Errorf("kcc native build reported success while kcc has no native backend: %+v", res)
	}
	if !strings.Contains(res.Message, "K116") {
		t.Errorf("kcc native build message does not name K116: %s", res.Message)
	}
	if res.Message == "" {
		t.Error("kcc native build produced an empty diagnostic")
	}
	// While kccOwnsNativeTargets is false the message must be the USER-facing
	// refusal, not the internal guard. If this arm is the internal one, the
	// constant was flipped without 151A landing, which is the failure mode the
	// guard exists to catch.
	if kccOwnsNativeTargets {
		t.Fatalf("kccOwnsNativeTargets is true but 151A has not landed: %s", res.Message)
	}
	if !strings.Contains(res.Message, "increment 151A") {
		t.Errorf("refusal does not say what will fix it: %s", res.Message)
	}

	run := KCCNativeRunCommand("hello.kark", nativeCfg(NativeLinuxTarget), false)
	if run.ExitCode == ExitSuccess {
		t.Errorf("kcc native run reported success while kcc has no native backend: %+v", run)
	}
	if !strings.Contains(run.Message, "K116") {
		t.Errorf("kcc native run message does not name K116: %s", run.Message)
	}
}
