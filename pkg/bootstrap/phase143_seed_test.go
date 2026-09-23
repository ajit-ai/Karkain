package bootstrap

import (
	"errors"
	"os"
	"os/exec"
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

// goBinDir returns the directory holding a tool binary, splitting on both
// separator styles: LookPath results are native (backslashes on Windows),
// while tests and cross-platform callers may pass forward slashes.
// filepath.Dir alone would mis-split the foreign style.
func goBinDir(tool string) string {
	flat := strings.ReplaceAll(tool, "\\", "/")
	if i := strings.LastIndex(flat, "/"); i >= 0 {
		return tool[:i]
	}
	return "."
}
func scrubGoFromPath(t *testing.T, goBin string) string {
	t.Helper()
	goDir := goBinDir(goBin)
	sep := string(os.PathListSeparator)
	parts := strings.Split(os.Getenv("PATH"), sep)
	kept := parts[:0]
	for _, p := range parts {
		if strings.EqualFold(p, goDir) {
			continue
		}
		kept = append(kept, p)
	}
	return strings.Join(kept, sep)
}

func TestBootstrap_SeedClosure(t *testing.T) {	goBin, err := exec.LookPath("go")
	if err != nil {
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

	// From here on the Go tool must be unresolvable: prove the closure.
	t.Setenv("PATH", scrubGoFromPath(t, goBin))
	if _, err := exec.LookPath("go"); err == nil {
		t.Fatal("PATH scrub failed: `go` still resolvable, closure would prove nothing")
	}
	if _, err := exec.LookPath("gcc"); err != nil {
		t.Fatal("PATH scrub removed gcc too: fix the test environment")
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

// TestBootstrap_ScrubGoFromPath unit-tests the PATH scrub without needing
// RAM, gcc or a seed build: the go tool's own directory is removed, every
// other entry is kept.
func TestBootstrap_ScrubGoFromPath(t *testing.T) {
	sep := string(os.PathListSeparator)
	t.Setenv("PATH", strings.Join([]string{"/tools/go", "/usr/bin", "/opt"}, sep))
	got := scrubGoFromPath(t, "/tools/go/go")
	want := strings.Join([]string{"/usr/bin", "/opt"}, sep)
	if got != want {
		t.Errorf("scrub = %q, want %q", got, want)
	}
}
