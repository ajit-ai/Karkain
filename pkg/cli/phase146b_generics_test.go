package cli

// Phase 146B — stdlib generic Stack[T]/Queue[T] (new std.generics
// module, persistent over plain arrays) + sibling examples + goldens.
//
// Go-engine goldens + check-clean contract live here; kcc parity for
// the same corpus lives in phase146c_generics_test.go (146C). The kcc
// leg below asserts that a COMPLETED kcc run on the generic corpus
// succeeds with the Go engine's golden stdout (silent acceptance
// without monomorphization would be a soundness hole); the documented
// slow-host self-build skip (timeout / error[K127]) still applies.

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

// TestPhase146B_KccParity asserts the post-146C contract for the stdlib
// generic corpus: a COMPLETED kcc run reproduces the Go engine's golden
// stdout byte for byte. The kcc self-build is slow on small hosts, so
// timeouts and the K127 low-RAM guard skip with a note (the parity
// subtests in phase146c_generics_test.go carry the same contract).
func TestPhase146B_KccParity(t *testing.T) {
	karkain := phase130Karkain(t)
	for _, c := range phase146bCases(t) {
		ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
		cmd := exec.CommandContext(ctx, karkain, "run", c.file, "--engine", "kcc")
		cmd.Env = append(os.Environ(), "KARKAIN_ENGINE=kcc")
		out, err := cmd.CombinedOutput()
		cancel()
		msg := string(out)
		if ctx.Err() == context.DeadlineExceeded {
			t.Skipf("%s: kcc self-build exceeds the 90s parity probe (slow host)", filepath.Base(c.file))
		}
		if strings.Contains(msg, "error[K127]") {
			t.Skipf("%s: low-RAM host guards the kcc self-build — parity deferred", filepath.Base(c.file))
		}
		if err != nil {
			t.Errorf("%s: kcc run failed (want parity): %v\n%s", filepath.Base(c.file), err, msg)
			continue
		}
		got := stripKCCBuildBanner(t, msg)
		got = strings.ReplaceAll(got, "\r\n", "\n")
		if got != c.want {
			t.Errorf("%s (engine=kcc): parity mismatch\nwant:\n%q\ngot:\n%q", filepath.Base(c.file), c.want, got)
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
