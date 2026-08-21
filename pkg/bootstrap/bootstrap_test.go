package bootstrap

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// findProjectRoot locates the project root by walking up from the test directory
// looking for go.mod.
func findProjectRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Cannot get working directory: %v", err)
	}
	for {
		goMod := filepath.Join(dir, "go.mod")
		if _, err := os.Stat(goMod); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("Cannot find project root (no go.mod found)")
		}
		dir = parent
	}
}

// cleanupBinaries removes binary artifacts created during the test.
func cleanupBinaries(t *testing.T, projectRoot string) {
	t.Helper()
	names := []string{
		"karkain-stage1",
		"karkain-compiler1",
		"karkain-compiler2",
		"karkain-compiler3",
	}
	for _, name := range names {
		os.Remove(binPath(projectRoot, name))
	}
}

// TestBootstrap_Stage1Compilation verifies Go compiler successfully emits runnable karkain-stage1.
func TestBootstrap_Stage1Compilation(t *testing.T) {
	projectRoot := findProjectRoot(t)
	cleanupBinaries(t, projectRoot)
	defer cleanupBinaries(t, projectRoot)

	s1, err := RunStage1(projectRoot)
	if err != nil {
		t.Fatalf("Stage 1 failed: %v", err)
	}

	if s1.Size == 0 {
		t.Fatal("Stage 1 binary is empty")
	}
	if s1.SHA256 == "" {
		t.Fatal("Stage 1 SHA256 is empty")
	}
	t.Logf("Stage 1: %s (%d bytes) in %v", s1.Binary, s1.Size, s1.Duration)
}

// TestBootstrap_Stage2SelfHosting asserts karkain-stage1 parses and compiles
// src/compiler/main.kar -> karkain-compiler2.
func TestBootstrap_Stage2SelfHosting(t *testing.T) {
	projectRoot := findProjectRoot(t)
	cleanupBinaries(t, projectRoot)
	defer cleanupBinaries(t, projectRoot)

	s1, err := RunStage1(projectRoot)
	if err != nil {
		t.Fatalf("Stage 1 failed: %v", err)
	}

	s2, err := RunStage2(projectRoot, s1.Binary)
	if err != nil {
		t.Fatalf("Stage 2 failed: %v", err)
	}

	if s2.Size == 0 {
		t.Fatal("Stage 2 binary is empty")
	}
	t.Logf("Stage 1: %s (%d bytes) in %v", s1.Binary, s1.Size, s1.Duration)
	t.Logf("Stage 2: %s (%d bytes) in %v", s2.Binary, s2.Size, s2.Duration)
}

// TestBootstrap_BitwiseIdentity ensures karkain-stage2 and karkain-stage3
// outputs match bitwise (100% deterministic self-hosting proof).
func TestBootstrap_BitwiseIdentity(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping full bootstrap identity test in short mode")
	}

	projectRoot := findProjectRoot(t)
	cleanupBinaries(t, projectRoot)
	defer cleanupBinaries(t, projectRoot)

	start := time.Now()

	s1, err := RunStage1(projectRoot)
	if err != nil {
		t.Fatalf("Stage 1 failed: %v", err)
	}

	s2, err := RunStage2(projectRoot, s1.Binary)
	if err != nil {
		t.Fatalf("Stage 2 failed: %v", err)
	}

	s3, err := RunStage3(projectRoot, s2.Binary)
	if err != nil {
		t.Fatalf("Stage 3 failed: %v", err)
	}

	identical, err := VerifyIdentity(s2, s3)
	if err != nil {
		t.Fatalf("VerifyIdentity error: %v", err)
	}

	if !identical {
		t.Fatalf("Bitwise identity FAILED: stage2 SHA256=%s != stage3 SHA256=%s", s2.SHA256, s3.SHA256)
	}

	t.Logf("Bootstrap complete in %v", time.Since(start))
	t.Logf("Stage 1: %s (%d bytes)", s1.Binary, s1.Size)
	t.Logf("Stage 2: %s (%d bytes) SHA256=%s", s2.Binary, s2.Size, s2.SHA256[:16])
	t.Logf("Stage 3: %s (%d bytes) SHA256=%s", s3.Binary, s3.Size, s3.SHA256[:16])
	t.Logf("Stage 2 == Stage 3: bitwise identical ✓")
}
