package cli

// Phase 121 — Compiler Independence Foundation gate.
//
// Phase 121 makes the self-hosted compiler verify its OWN intermediate
// representation on the default engine path. KIR (Phase 120) was CLI-only; in
// Phase 121 it becomes an engine-internal invariant:
//
//   - src/compiler/kir.kark gains kirVerify, a structural verifier that checks
//     every emitted line against the KIR v1 contract (header, basename source
//     line, two-space indentation discipline, monotone nesting depth, and a
//     ` line: <decimal>` statement suffix with the block/fallback exemptions);
//   - src/compiler/main.kark checkFile runs the verifier after the Phase 99
//     type checker — silent on success, error[K121] + no [ok] on drift, so the
//     CLI maps failures to ExitCompile;
//   - a new `verifykir` kcc command and `karkain kir --verify <file>` expose
//     the component standalone (`[ok] kir verify: N lines ok`).
//
// The verifier is written in Karkain using only language builtins and the
// in-tree user helpers, so it is itself engine-independent: no Go front-end,
// external IR, or library participates in proving the invariant.
//
// Success criterion (baseline Q4 NO -> YES): the self-hosted pipeline emits AND
// verifies its own IR on the default check path.
//
// NOTE: with the current emitter no valid source file can fail kirVerify — the
// point. The failure path exists to make ANY future emitter-vs-contract drift
// (indentation, depth, missing line metadata, header corruption) a hard,
// visible error on every kcc check instead of a silent divergence. The gate
// therefore proves the positive invariant (verifier accepts the full corpus
// incl. the compiler's own sources), the wiring (K121 string + checkFile hook +
// CLI), and the exit-code contract (parse failures and missing files map to
// failure without ever printing the [ok] verdicts).

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// TestPhase121_CompilerIndependence assembles the Phase 121 gate.
func TestPhase121_CompilerIndependence(t *testing.T) {
	bin := buildPreviewBinary(t)
	root := repoRoot(t)

	t.Run("VerifyCommand", func(t *testing.T) {
		dir := t.TempDir()
		fixture := filepath.Join(dir, "main.kark")
		if err := os.WriteFile(fixture, []byte(phase120Surface), 0o644); err != nil {
			t.Fatal(err)
		}
		out, err := runBin(t, bin, root, "kir", "--verify", fixture)
		if err != nil {
			t.Fatalf("karkain kir --verify failed: %v\n%s", err, out)
		}
		for _, want := range []string{
			"[ok] kir text: ",
			"[ok] kir verify: ",
		} {
			if !strings.Contains(out, want) {
				t.Errorf("verify output missing %q\n---\n%s", want, out)
			}
		}
		if tc, vc := phase121Count(t, out, "[ok] kir text: "), phase121Count(t, out, "[ok] kir verify: "); tc != vc {
			t.Errorf("count mismatch: kir text %d lines, verify %d lines\n%s", tc, vc, out)
		}
		if strings.Contains(out, "error[K121]") {
			t.Errorf("valid fixture must not report K121 problems:\n%s", out)
		}

		// Determinism: two invocations must be byte-identical so the verifier
		// cannot depend on host paths or sandbox locations.
		second, err := runBin(t, bin, root, "kir", "--verify", fixture)
		if err != nil {
			t.Fatalf("second verify run failed: %v\n%s", err, second)
		}
		if out != second {
			t.Errorf("kir --verify is not byte-deterministic:\n--- run 1 ---\n%s\n--- run 2 ---\n%s", out, second)
		}
	})

	t.Run("CompilerSelfVerify", func(t *testing.T) {
		// The strongest independence proof: the compiler verifies its own
		// sources. src/compiler/*.kark is assembled as one unit (ProjectSource
		// assembly), so a single verify run covers every compiler file;
		// kir.kark is the ideal target because it contains both the emitter
		// and the verifier being proven. The DEFAULT kcc check of main.kark
		// repeats the invariant on the full assembly internally through the
		// checkFile hook (silent success).
		kirSrc := filepath.Join(root, "src", "compiler", "kir.kark")
		out, err := runBin(t, bin, root, "kir", "--verify", kirSrc)
		if err != nil {
			t.Fatalf("kir --verify on compiler source failed: %v\n%s", err, out)
		}
		if !strings.Contains(out, "[ok] kir verify:") {
			t.Errorf("kir --verify on kir.kark printed no verdict:\n%s", out)
		}
		if tc, vc := phase121Count(t, out, "[ok] kir text: "), phase121Count(t, out, "[ok] kir verify: "); tc != vc {
			t.Errorf("kir.kark count mismatch: %d vs %d", tc, vc)
		}

		whole, err := runBin(t, bin, root, "check", filepath.Join(root, "src", "compiler", "main.kark"))
		if err != nil {
			t.Fatalf("default kcc check of full compiler assembly failed: %v\n%s", err, whole)
		}
		if !strings.Contains(whole, "[ok]") {
			t.Errorf("full-compiler check must succeed through the checkFile KIR hook:\n%s", whole)
		}
	})

	t.Run("DefaultCheckInvariant", func(t *testing.T) {
		// The verifier is wired into checkFile on the DEFAULT kcc check path:
		// a valid file still [ok]'s (silent success), while the Phase 99/117
		// semantic gate still rejects bad programs first (no [ok], exit 3).
		dir := t.TempDir()
		good := filepath.Join(dir, "good.kark")
		if err := os.WriteFile(good, []byte("func main() {\n    print 42\n}\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		out, err := runBin(t, bin, root, "check", good)
		if err != nil {
			t.Fatalf("kcc check on valid file failed: %v\n%s", err, out)
		}
		if !strings.Contains(out, "[ok]") {
			t.Errorf("valid file must [ok] through the default check path:\n%s", out)
		}

		bad := filepath.Join(dir, "bad.kark")
		if err := os.WriteFile(bad, []byte("func main() {\n    print unknown_identifier\n}\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		out2, err2 := runBin(t, bin, root, "check", bad)
		if err2 == nil {
			t.Fatalf("check on undefined identifier must fail, got success:\n%s", out2)
		}
		if strings.Contains(out2, "[ok]") {
			t.Errorf("[ok] must never appear when the KIR invariant or semantic gate fails:\n%s", out2)
		}
	})

	t.Run("VerifierPresence", func(t *testing.T) {
		// Guards the failure-path wiring itself: the K121 diagnostic, the
		// checkFile hook, and the standalone command must exist so an emitter
		// regression becomes a hard, visible error instead of silent drift.
		kirSrc := mustRead(t, filepath.Join(root, "src", "compiler", "kir.kark"))
		if !strings.Contains(kirSrc, "func kirVerify(lines, path)") {
			t.Error("kir.kark missing func kirVerify")
		}
		for _, fn := range []string{"func kirIndentCount", "func kirIsDigits", "func kirHasLineNo", "func kirIsStructureLine"} {
			if !strings.Contains(kirSrc, fn) {
				t.Errorf("kir.kark missing %s", fn)
			}
		}
		mainSrc := mustRead(t, filepath.Join(root, "src", "compiler", "main.kark"))
		if !strings.Contains(mainSrc, "error[K121]") {
			t.Error("main.kark missing error[K121] diagnostic")
		}
		if !strings.Contains(mainSrc, "kirVerify(kirLines, path)") {
			t.Error("main.kark checkFile missing kirVerify invocation")
		}
		if !strings.Contains(mainSrc, "func verifyFile(path)") {
			t.Error("main.kark missing func verifyFile")
		}
		cliSrc := mustRead(t, filepath.Join(root, "cmd", "karkain", "main.go"))
		if !strings.Contains(cliSrc, "--verify") {
			t.Error("cmd/karkain/main.go missing --verify flag")
		}
	})

	t.Run("ExitCodeContract", func(t *testing.T) {
		dir := t.TempDir()
		missing := filepath.Join(dir, "nope.kark")
		out, err := runBin(t, bin, root, "kir", "--verify", missing)
		if err == nil {
			t.Fatalf("kir --verify on missing file should fail, got success:\n%s", out)
		}
		if !strings.Contains(out, "file not found") {
			t.Errorf("missing-file error should name the file: %s", out)
		}

		bad := filepath.Join(dir, "bad.kark")
		if err := os.WriteFile(bad, []byte("func main() {\n  let = 42\n}\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		out2, err2 := runBin(t, bin, root, "kir", "--verify", bad)
		if err2 == nil {
			t.Fatalf("kir --verify on unparseable file should fail, got success:\n%s", out2)
		}
		for _, forbidden := range []string{"[ok] kir text:", "[ok] kir verify:"} {
			if strings.Contains(out2, forbidden) {
				t.Errorf("verdict marker %q must never appear on parse failure:\n%s", forbidden, out2)
			}
		}
	})
}

// TestPhase121_VerifyCommandExists guards the --verify registration itself
// without requiring the self-hosted engine to be present.
func TestPhase121_VerifyCommandExists(t *testing.T) {
	bin := buildPreviewBinary(t)
	out, err := runBin(t, bin, t.TempDir(), "kir", "--verify")
	if err == nil {
		t.Fatalf("kir --verify without args should fail with usage, got success:\n%s", out)
	}
	if !strings.Contains(out, "kir") {
		t.Errorf("usage text should mention kir: %s", out)
	}
}

// phase121Count extracts the integer immediately after marker (e.g.
// "[ok] kir text: " -> 6399) from kcc output.
func phase121Count(t *testing.T, out, marker string) int {
	t.Helper()
	idx := strings.Index(out, marker)
	if idx < 0 {
		t.Fatalf("output missing %q\n---\n%s", marker, out)
	}
	rest := out[idx+len(marker):]
	end := strings.Index(rest, " ")
	if end < 0 {
		end = len(rest)
	}
	n, err := strconv.Atoi(strings.TrimSpace(rest[:end]))
	if err != nil {
		t.Fatalf("bad count after %q: %q", marker, rest)
	}
	return n
}