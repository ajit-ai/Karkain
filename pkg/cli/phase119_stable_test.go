package cli

// Phase 119 — Language QA / Public Repository Finalization & Karkain 1.0.0
// Stable Release Preparation gate.
//
// This suite guards the Phase 119 finalization invariants: the unified
// v1.0.0 / Stable Build identity, the removal of superseded/placeholder
// material, the presence of the release documentation + master QA gate, and
// the current-status label markers. It deliberately does not re-run the
// Phase 114/115/116/117/118 behavior suites (those remain the ground truth
// for the stable core).

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// mustRead returns the file contents or fails the test.
func mustRead(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}

// assertNotExist fails the test if path exists (cleanup invariant).
func assertNotExist(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); err == nil {
		t.Errorf("superseded path %q must not exist", path)
	}
}

// TestPhase119_StableReleaseReadiness assembles the whole Phase 119 gate.
func TestPhase119_StableReleaseReadiness(t *testing.T) {
	bin := buildPreviewBinary(t)
	t.Run("VersionIdentity", func(t *testing.T) {
		out, err := runBin(t, bin, t.TempDir(), "--version")
		if err != nil {
			t.Fatalf("karkain --version failed: %v\n%s", err, out)
		}
		for _, want := range []string{"Karkain Compiler v1.0.0", "Stable Build"} {
			if !strings.Contains(out, want) {
				t.Errorf("--version does not contain %q: %s", want, out)
			}
		}
	})
	t.Run("VersionFile", func(t *testing.T) {
		root := repoRoot(t)
		ver := strings.TrimSpace(mustRead(t, filepath.Join(root, "VERSION")))
		if ver != "1.0.0" {
			t.Errorf("VERSION = %q, want %q", ver, "1.0.0")
		}
	})
	t.Run("CleanupInvariants", func(t *testing.T) {
		root := repoRoot(t)
		// Superseded material must stay removed.
		for _, rel := range []string{
			"docs/ROADMAP-PRODUCTION.md",
			"scripts/package.ps1",
			"scripts/release.ps1",
			"scripts/release.sh",
			"scripts/build.ps1",
			"scripts/build.sh",
			"pkg/stdlib/actor.go",
			"pkg/stdlib/http.go",
			"pkg/stdlib/reflect.go",
			"pkg/stdlib/rpc.go",
			"std/io.kark",
			"std/string.kark",
			"src/analyzer.kark",
			"src/codegen.kark",
			"src/lexer.kark",
			"src/main.kark",
			"src/main.rs",
			"src/parser.kark",
			"compiler/ast.c",
			"compiler/codegen.c",
			"compiler/main.c",
		} {
			assertNotExist(t, filepath.Join(root, filepath.FromSlash(rel)))
		}
		// Kept dependencies must still exist (bootstrap + tests reference them).
		for _, rel := range []string{
			"compiler/ast.kark",
			"compiler/main.kark",
			"examples/app.kark",
			"stdlib/string/string.kark",
			"stdlib/crypto/crypto.kark",
			"stdlib/io/io.kark",
		} {
			if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(rel))); err != nil {
				t.Errorf("required %q missing: %v", rel, err)
			}
		}
	})
	t.Run("ReleaseDeliverables", func(t *testing.T) {
		root := repoRoot(t)
		for _, rel := range []string{
			"docs/release/KARKAIN-1.0-CHECKLIST.md",
			"docs/release/KARKAIN-1.0-QA-PLAN.md",
			"docs/release/KARKAIN-1.0-RELEASE-NOTES.md",
			"scripts/qa/run-full-qa.ps1",
			"docs/audit/PHASE-119-LANGUAGE-QA-FINAL-REPORT.md",
		} {
			if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(rel))); err != nil {
				t.Errorf("release deliverable %q missing: %v", rel, err)
			}
		}
	})
	t.Run("LabelMarkers", func(t *testing.T) {
		root := repoRoot(t)
		if s := mustRead(t, filepath.Join(root, "README.md")); !strings.Contains(s, "Karkain 1.0.0") {
			t.Error("README.md does not carry the 1.0.0 Stable label")
		}
		if s := mustRead(t, filepath.Join(root, "docs", "source", "status", "index.rst")); !strings.Contains(s, "Karkain 1.0.0") {
			t.Error("status/index.rst does not carry the 1.0.0 status")
		}
		if s := mustRead(t, filepath.Join(root, "docs", "source", "status", "compatibility.rst")); !strings.Contains(s, "Karkain 1.0.0 versioning") {
			t.Error("compatibility.rst does not carry the 1.0.0 versioning section")
		}
	})
	t.Run("CorpusPresence", func(t *testing.T) {
		root := repoRoot(t)
		for _, rel := range []string{
			"examples/01-fundamentals/01_hello_world.kark",
			"examples/02-algorithms/06_fibonacci.kark",
			"examples/stdlib_v2/main.kark",
			"conformance",
		} {
			if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(rel))); err != nil {
				t.Errorf("corpus path %q missing: %v", rel, err)
			}
		}
	})
}