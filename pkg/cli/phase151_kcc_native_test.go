package cli

import (
	"encoding/hex"
	"strconv"
	"strings"
	"testing"

	"karkain/pkg/codegen"
	"karkain/pkg/lexer"
	"karkain/pkg/native"
	"karkain/pkg/parser"
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
// gate. For every C-free native target, under kcc, the outcome must be one of
// two HONEST things and never a silent Go hand-off:
//
//	native-x86_64-windows  -> kcc EMITS: exit 0, an image on disk, and kcc
//	                          provenance proving the self-hosted engine
//	                          produced those bytes.
//	native-x86_64-linux    -> kcc REFUSES: non-zero exit naming K116 and the
//	native-x86_64-macos      target, kcc provenance, and NO image.
//
// "Must not write an image" is the load-bearing half for the refusing targets.
// A refusal that still left a Go image on disk would satisfy the letter of the
// rule while preserving the dishonesty, because a stale artifact is
// indistinguishable from real output.
//
// Phase 151D made this target-dependent rather than uniformly negative. kcc
// owns Windows only, so asserting "never writes an image" for all three would
// now be a WRONG expectation that fails on correct behaviour — and, worse, it
// would forbid the very thing 151D delivers. The refusal assertions are kept
// verbatim for the two targets kcc genuinely cannot emit.
//
// Phase 151A-1 added the provenance requirement: the refusal must come FROM kcc,
// not be synthesised by the Go driver. A Go-side refusal is honest about the
// engine but proves nothing about kcc, which is part of why the original §1.1
// finding was so easy to miss.
func TestPhase151_NoSilentGoFallback(t *testing.T) {
	karkain := phase130Karkain(t)
	const src = "func main() {\n\tprint(42)\n}\n"

	for _, tgt := range []string{NativeLinuxTarget, NativeWindowsTarget, NativeMacOSTarget} {
		tgt := tgt
		t.Run("build/"+tgt, func(t *testing.T) {
			res := nativeBuildWithEngine(t, karkain, src, tgt, "kcc", "probe.out")

			// Provenance is required in BOTH outcomes, and it is checked
			// before any claim about the image, so no branch can be reached
			// without kcc having been consulted.
			//
			// The check is for the marker FOLLOWED BY THE TARGET, not the bare
			// marker. A diagnostic that merely names the expected marker (for
			// instance the "kcc did not answer" path) would satisfy a bare
			// Contains and make this gate pass while kcc printed nothing —
			// which is exactly the failure a mutation of kcc's provenance line
			// produced before the check was tightened.
			if !strings.Contains(res.Output, kccNativeProvenance+" "+tgt) {
				t.Fatalf("%s: carries no kcc provenance line for this target (%q + target), "+
					"so the self-hosted engine was not actually consulted: %s",
					tgt, kccNativeProvenance, res.Output)
			}

			if tgt == NativeWindowsTarget {
				assertKccOwnedWindowsBuild(t, res, tgt)
				return
			}

			assertKccRefused(t, res, tgt)
		})
	}
}

// assertKccOwnedWindowsBuild is the 151D positive half: kcc owns this target, so
// it must emit, and the emission must be attributable to kcc.
func assertKccOwnedWindowsBuild(t *testing.T, res nativeBuildResult, tgt string) {
	t.Helper()
	if !res.WroteImage {
		t.Fatalf("%s: kcc owns this target in 151D but wrote no image: %s", tgt, res.Output)
	}
	if res.ExitCode != 0 {
		t.Errorf("%s: kcc native build exited %d: %s", tgt, res.ExitCode, res.Output)
	}
	// The image-marker line is what distinguishes "kcc emitted" from "kcc was
	// consulted and refused", so it must be present alongside the provenance.
	if !strings.Contains(res.Output, kccNativeImageMarker) {
		t.Errorf("%s: kcc build emits no image marker, so its bytes cannot be "+
			"attributed to the self-hosted engine: %s", tgt, res.Output)
	}
	// A Go native build banner would mean the request never reached kcc.
	if strings.Contains(res.Output, "[native-") {
		t.Errorf("%s: output still carries a Go native build banner: %s", tgt, res.Output)
	}
	// And the bytes must be a real container, not hex that happened to decode.
	if _, _, err := native.ParsePE(res.Image); err != nil {
		t.Errorf("%s: the kcc-emitted image is not a structurally valid PE: %v", tgt, err)
	}
}

// assertKccRefused is the negative half, unchanged in substance: for a target
// kcc cannot emit, the outcome is a refusal and never an image.
func assertKccRefused(t *testing.T, res nativeBuildResult, tgt string) {
	t.Helper()
	if res.WroteImage {
		t.Fatalf("%s: kcc does not own this target, so an image here is output kcc cannot "+
			"have produced (sha %s); refusing must leave nothing behind", tgt, shortSHA(res.SHA))
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

// TestPhase151D_ProvenanceGateAcceptsOnlyAttributedBytes is the mutation gate
// for the 151D acceptance requirement "require and validate KCC provenance
// before the CLI accepts and writes emitted image bytes".
//
// The whole risk in 151D is a SUCCESS that looks right. An image is a bag of
// bytes: nothing in the bytes themselves says which engine produced them, so
// the provenance line is the only thing separating a kcc build from a Go build
// with a rename. This test drives the real acceptance path -- kccNativeBuild's
// own hex extraction and provenance gate -- over kcc's ACTUAL output shapes, and
// then over the shapes a dishonest or broken emitter would produce.
//
// It is mutation-verified by construction: each case below is a distinct way
// bytes can be presented without provenance, and the implementation must reject
// every one. Deleting the provenance check makes them pass, which is exactly
// the regression this pins.
func TestPhase151D_ProvenanceGateAcceptsOnlyAttributedBytes(t *testing.T) {
	// The real, valid shape: provenance, then the image marker, then hex.
	const goodHex = "4d5a9000"

	t.Run("provenance plus image marker is accepted", func(t *testing.T) {
		out := kccNativeProvenance + " " + NativeWindowsTarget + " (os=windows): probe.kark\n" +
			kccNativeImageMarker + "4 bytes\n" + goodHex + "\n"
		hexImg, ok := kccNativeHexFromOutput(out)
		if !ok || hexImg != goodHex {
			t.Fatalf("valid kcc output was rejected: ok=%v hex=%q (want %q)", ok, hexImg, goodHex)
		}
	})

	// Bytes with NO provenance must not even be reachable as an image. This is
	// the mutation the 151A-1 gate already caught once, expressed at the unit
	// level so it cannot come back silently.
	t.Run("hex without provenance yields no image", func(t *testing.T) {
		out := goodHex + "\n"
		if _, ok := kccNativeHexFromOutput(out); ok {
			t.Fatal("bare hex was accepted as an image: output with no provenance line must " +
				"produce nothing, or Go bytes could be relabelled as kcc's")
		}
	})

	// Provenance alone proves kcc was consulted, not that it emitted. Without
	// the image marker there is nothing to accept.
	t.Run("provenance without an image marker yields no image", func(t *testing.T) {
		out := kccNativeProvenance + " " + NativeWindowsTarget + "\n" + goodHex + "\n"
		if _, ok := kccNativeHexFromOutput(out); ok {
			t.Fatal("provenance without an image marker was accepted: provenance proves kcc " +
				"was consulted, not that it emitted")
		}
	})

	// A hex-looking string INSIDE a diagnostic must not be mistaken for the
	// image. This is why extraction is positional rather than a search for
	// anything hex-shaped.
	t.Run("hex inside a diagnostic is not the image", func(t *testing.T) {
		out := "error[K145]: native target: diagnostic mentioning " + goodHex + " inline\n"
		if _, ok := kccNativeHexFromOutput(out); ok {
			t.Fatal("a hex string inside a diagnostic was accepted as the image: extraction " +
				"must be positional, after the image marker")
		}
	})

	// The provenance gate itself, not just the extractor: an output carrying
	// bytes but no provenance is refused, and -- the part that matters -- the
	// caller is left with nothing it could write.
	//
	// This drives the real acceptance path (kccNativeAcceptOutput), so deleting
	// or weakening the provenance check makes this subtest fail.
	t.Run("accept refuses bytes without provenance", func(t *testing.T) {
		img, refusal := kccNativeAcceptOutput("unattributed hex payload with no marker",
			NativeWindowsTarget)
		if img != nil {
			t.Fatalf("kccNativeAcceptOutput returned %d bytes for output with no provenance: "+
				"bytes that cannot be attributed to kcc must never be returned", len(img))
		}
		if refusal == nil {
			t.Fatal("output with no provenance was accepted: this is the silent-Go-fallback " +
				"regression 151 exists to prevent")
		}
		if !strings.Contains(refusal.Message, "WITHOUT its provenance line") {
			t.Errorf("refusal does not explain that provenance was missing: %s", refusal.Message)
		}
		// And it must not be a K116: kcc did not refuse, the SEAM refused.
		if strings.Contains(refusal.Message, "K116") {
			t.Errorf("a missing-provenance refusal must not be reported as a backend-capability "+
				"code; kcc was never asked: %s", refusal.Message)
		}
	})

	// Conversely, a fully attributed real build IS accepted. Without this the
	// subtests above would pass trivially against an implementation that
	// accepted nothing at all.
	//
	// The image is produced by the ORACLE (pkg/native), which is legitimate
	// here: what this subtest pins is that ATTRIBUTED bytes are accepted, not
	// that these particular bytes came from kcc -- that is the end-to-end
	// gate's job. Using the oracle keeps a real, structurally valid container
	// without hand-assembling one.
	t.Run("accept returns bytes for a properly attributed build", func(t *testing.T) {
		prog := parseKccNativeProbe(t, "func main() {\n\tprint(42)\n}\n")
		pe, err := native.CompileProgramForOS(prog, native.OSWindows)
		if err != nil {
			t.Fatalf("building a valid PE for the acceptance fixture: %v", err)
		}
		out := kccNativeProvenance + " " + NativeWindowsTarget + " (os=windows): probe.kark\n" +
			kccNativeImageMarker + strconv.Itoa(len(pe)) + " bytes\n" +
			hexForTest(pe)

		img, refusal := kccNativeAcceptOutput(out, NativeWindowsTarget)
		if refusal != nil {
			t.Fatalf("a properly attributed kcc build was refused: %s", refusal.Message)
		}
		if len(img) != len(pe) {
			t.Errorf("accepted %d bytes, built %d: the payload was not carried through intact",
				len(img), len(pe))
		}
	})
}

// parseKccNativeProbe parses a probe program with the Go parser. Used only to
// obtain a structurally valid container for the 151D acceptance fixtures.
func parseKccNativeProbe(t *testing.T, src string) *parser.Program {
	t.Helper()
	p := parser.New(lexer.New(src))
	prog := p.ParseProgram()
	if len(p.Errors) > 0 {
		t.Fatalf("probe does not parse: %s", strings.Join(p.Errors, "; "))
	}
	return prog
}

func hexForTest(b []byte) string { return hex.EncodeToString(b) }

// TestPhase151_KccNativeRefusalIsNotAFallback pins the seam itself, without
// spawning a process.
//
// This is the test the old version said would have to be inverted in the same
// commit as the ownership flip, and that is what happened: while the flag was
// false it asserted the flag must not be flipped; now that 151D has landed real
// emission it asserts the flip is CONSISTENT -- the flag is true, exactly one
// target is claimed, and it is the target kcc can actually emit. Naming that
// obligation in the test body is what made it impossible to forget: the constant
// could not have been flipped silently.
//
// The unit seam cannot run kcc (that needs a real kcc binary and a real file),
// so what is checked here is the ownership DATA. The end-to-end assertion -- an
// emitted image with provenance -- is TestPhase151_NoSilentGoFallback.
func TestPhase151_KccNativeRefusalIsNotAFallback(t *testing.T) {
	// The flip is inverted from the pre-151D expectation: it must now be true,
	// or the emission arm would be dead code behind a false flag.
	if !kccOwnsNativeTargets {
		t.Fatal("kccOwnsNativeTargets is false after 151D landed real emission: " +
			"the flag was supposed to be inverted in the same change that wires it")
	}

	// Exactly the target kcc can emit. Claiming more would be a lie this gate
	// cannot detect at runtime, because an unsupported target is refused by
	// kcc and the user sees a plausible-looking error.
	if !kccNativeOwns(NativeWindowsTarget) {
		t.Fatal("kccOwnsNativeTargets is true but the owned-target list omits Windows: " +
			"the flag claims ownership the data does not support")
	}
	for _, tgt := range []string{NativeLinuxTarget, NativeMacOSTarget} {
		if kccNativeOwns(tgt) {
			t.Errorf("%s is listed as kcc-owned, but kcc has no ELF/Mach-O emitter for a "+
				"user program (native_elf.kark/native_macho.kark lower fixed reference "+
				"corpora only). Claiming it would turn a refusal into a false success", tgt)
		}
	}
}
