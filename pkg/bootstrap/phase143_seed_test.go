package bootstrap

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Phase 143 — Seed-Binary Bootstrap Closure (Go-free).
//
// Proves that FROM any working seed compiler, the self-hosting closure
// completes with no Go tool on PATH: the seed transpiles
// src/compiler/main.kark (stages 2 and 3), gcc links both, and the two
// outputs are bitwise identical. The only external tools in the closure
// are the seed binary itself (absolute path) and gcc (kept on PATH).
//
// Environment contract (each absence skips, never fails — the gate runs
// for real on capable hosts and CI, and skips fast elsewhere):
//   - `go` present: needed once to BUILD the seed (stage 1). A host with
//     no Go at all has nothing to seed from (that flow is the 142
//     downloadable-seed ceremony, not this gate).
//   - `gcc` present: the C linker for every stage.
//   - ≥1.5 GiB free (K127 guard): stages 2/3 abort cleanly below it.
//
// KARKAIN_ENGINE=go is pinned on the child commands by runCmdOutput, but
// the seed is a native kcc binary that never consults it — the pin is
// Go-CLI-side configuration and is inert here by construction.
//
// Go-neutralization is by SHADOWING (a failing `go` stub prepended to
// PATH), not directory removal: on some hosts `go` shares its directory
// with gcc, and scrubbing the directory would amputate the linker.

// shadowGoWithStub prepends a directory holding a `go` stub that always
// fails, so every PATH-based `go` invocation from here on errors loudly
// instead of running the real toolchain. Directory scrubbing was tried
// first and abandoned: on some hosts (notably CI images) `go` shares its
// directory with gcc, so removing the directory amputates the linker the
// closure genuinely needs. Shadowing is layout-agnostic: gcc and every
// other tool resolve untouched, while `go` provably cannot succeed.
// It returns the stub directory for the proof assertion below.
func shadowGoWithStub(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	sh := "#!/bin/sh\necho 'go: toolchain disabled for the seed-closure proof' >&2\nexit 1\n"
	if err := os.WriteFile(filepath.Join(dir, "go"), []byte(sh), 0o755); err != nil {
		t.Fatal(err)
	}
	// Windows resolves `go` via PATHEXT (.bat); a bare shell script would
	// be skipped there and the real toolchain found instead.
	bat := "@echo off\r\necho go: toolchain disabled for the seed-closure proof 1>&2\r\nexit /b 1\r\n"
	if err := os.WriteFile(filepath.Join(dir, "go.bat"), []byte(bat), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	return dir
}

func TestBootstrap_SeedClosure(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go toolchain absent: nothing to build the seed from (see the 142 seed ceremony)")
	}
	if _, err := exec.LookPath("gcc"); err != nil {
		t.Skip("gcc absent: no linker for any bootstrap stage")
	}
	if err := CheckBootstrapMemory(2); err != nil {
		if errors.Is(err, errInsufficientMemory) {
			t.Skipf("insufficient free RAM for self-hosted stages: %v", err)
		}
		t.Fatalf("memory probe failed: %v", err)
	}

	projectRoot := findProjectRoot(t)
	cleanupBinaries(t, projectRoot)
	defer cleanupBinaries(t, projectRoot)

	// Stage 1 (Go present): build the seed compiler.
	s1, err := RunStage1(projectRoot)
	if err != nil {
		t.Fatalf("Stage 1 (seed build) failed: %v", err)
	}
	t.Logf("seed: %s (%d bytes) SHA256=%s", s1.Binary, s1.Size, s1.SHA256[:16])

	// From here on the Go tool must be unusable: shadow it with a stub that
	// always fails, then prove the shadow is active. Any stage that
	// secretly needs Go now fails loudly instead of silently succeeding.
	stubDir := shadowGoWithStub(t)
	resolved, err := exec.LookPath("go")
	if err != nil {
		t.Fatalf("PATH shadow broken: `go` unresolvable at all (%v) — want the stub", err)
	}
	if filepath.Dir(resolved) != stubDir && !strings.EqualFold(filepath.Dir(resolved), stubDir) {
		// Resolve symlinks/relative spellings before declaring defeat.
		if eval, everr := filepath.EvalSymlinks(resolved); everr != nil || filepath.Dir(eval) != stubDir {
			t.Fatalf("PATH shadow failed: `go` resolves to %s, not the stub in %s", resolved, stubDir)
		}
	}
	if out, err := exec.Command(resolved).CombinedOutput(); err == nil {
		t.Fatalf("stub `go` unexpectedly succeeded: %s", out)
	}
	if _, err := exec.LookPath("gcc"); err != nil {
		t.Fatalf("gcc must stay resolvable for the linker: %v", err)
	}

	s2, err := RunStage2(projectRoot, s1.Binary)
	if err != nil {
		t.Fatalf("Stage 2 (go-less) failed: %v", err)
	}
	s3, err := RunStage3(projectRoot, s2.Binary)
	if err != nil {
		t.Fatalf("Stage 3 (go-less) failed: %v", err)
	}

	identical, err := VerifyIdentity(s2, s3)
	if err != nil {
		t.Fatalf("VerifyIdentity error: %v", err)
	}
	if !identical {
		t.Fatalf("seed closure FAILED: stage2 SHA256=%s != stage3 SHA256=%s", s2.SHA256, s3.SHA256)
	}
	t.Logf("seed closure: stage2 == stage3 bitwise identical (%d bytes), no Go tool used", s2.Size)
}

// TestBootstrap_GoStubShadow unit-tests the shadow without needing RAM,
// gcc or a seed build: after shadowing, `go` resolves inside the stub dir
// and invoking it fails, while unrelated tools keep resolving.
func TestBootstrap_GoStubShadow(t *testing.T) {
	stubDir := shadowGoWithStub(t)
	resolved, err := exec.LookPath("go")
	if err != nil {
		t.Fatalf("`go` unresolvable after shadowing: %v", err)
	}
	if d := filepath.Dir(resolved); d != stubDir && !strings.EqualFold(d, stubDir) {
		t.Fatalf("`go` resolves to %s, want the stub in %s", resolved, stubDir)
	}
	if out, err := exec.Command(resolved).CombinedOutput(); err == nil {
		t.Fatalf("stub `go` unexpectedly succeeded: %s", out)
	}
}
