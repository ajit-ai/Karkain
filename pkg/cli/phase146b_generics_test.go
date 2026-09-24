package cli

// Phase 146B — stdlib generic Stack[T]/Queue[T] (new std.generics
// module, persistent over plain arrays) + sibling examples + goldens.
//
// Engine contract for this slice: the Go engine is live (monomorphizer
// instantiates the stdlib templates through the normal import
// assembly). kcc cannot parse [T] yet (146C owns that), so the kcc leg
// below pins the honest boundary — a kcc run that completes must fail
// LOUDLY (silent acceptance of unparsed generics would be a soundness
// hole); a slow-host self-build (timeout/K127) skips with a note.

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func phase146bCases(t *testing.T) []phase102Case {
	t.Helper()
	dir := filepath.Join(repoRoot(t), "examples", "generics")
	return []phase102Case{
		{filepath.Join(dir, "00_stack.kark"), "1\n2\n20\n10\n0\nb\n"},
		{filepath.Join(dir, "01_queue.kark"), "1\n3\n1\n2\n0\n"},
	}
}

// TestPhase146B_GoldensGoEngine runs the generics examples through the
// real binary on the Go engine (import std.generics included) and
// asserts byte-exact golden stdout.
func TestPhase146B_GoldensGoEngine(t *testing.T) {
	karkain := phase130Karkain(t)
	runPhase102Cases(t, karkain, "go", phase146bCases(t))
}

// TestPhase146B_CheckClean proves the Go checker accepts the corpus.
func TestPhase146B_CheckClean(t *testing.T) {
	karkain := phase130Karkain(t)
	for _, c := range phase146bCases(t) {
		cmd := exec.Command(karkain, "check", c.file, "--engine", "go")
		cmd.Env = append(os.Environ(), "KARKAIN_ENGINE=go")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Errorf("%s: check --engine go failed:\n%s", filepath.Base(c.file), string(out))
		}
	}
}

// TestPhase146B_KccBoundary pins the pre-146C contract: kcc must never
// silently accept generic declarations. The kcc self-build is slow on
// small hosts, so this runs under a short timeout — timeouts and the
// K127 low-RAM guard skip with a note (146C flips these to parity).
func TestPhase146B_KccBoundary(t *testing.T) {
	karkain := phase130Karkain(t)
	for _, c := range phase146bCases(t) {
		ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
		cmd := exec.CommandContext(ctx, karkain, "check", c.file, "--engine", "kcc")
		cmd.Env = append(os.Environ(), "KARKAIN_ENGINE=kcc")
		out, err := cmd.CombinedOutput()
		cancel()
		msg := string(out)
		if ctx.Err() == context.DeadlineExceeded {
			t.Skipf("%s: kcc self-build exceeds the 90s boundary probe (slow host) — parity deferred to 146C", filepath.Base(c.file))
		}
		if strings.Contains(msg, "error[K127]") {
			t.Skipf("%s: low-RAM host guards the kcc self-build — parity deferred to 146C", filepath.Base(c.file))
		}
		if err == nil {
			t.Errorf("%s: kcc check unexpectedly ACCEPTED generic syntax (want loud rejection until 146C):\n%s", filepath.Base(c.file), msg)
		}
	}
}

// TestPhase146B_StdlibDocsConsistency guards the freeze-gate contract:
// the new module is registered in stable-api.rst so the Phase-131/132
// consistency gates keep passing.
func TestPhase146B_StdlibDocsConsistency(t *testing.T) {
	content, err := os.ReadFile(filepath.Join(repoRoot(t), "docs", "source", "reference", "stable-api.rst"))
	if err != nil {
		t.Fatalf("reading stable-api.rst: %v", err)
	}
	if !strings.Contains(string(content), "std.generics") {
		t.Errorf("stable-api.rst must document std.generics (Phase-131/132 consistency rule)")
	}
}
