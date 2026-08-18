package bootstrap

import (
	"os"
	"path/filepath"
	"testing"
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
	defer cleanupBinaries(t, projectRoot)

	t.Log("Running Stage 1: Go build -> transpile -> gcc -> karkain-compiler1")
	result, err := RunStage1(projectRoot)
	if err != nil {
		t.Fatalf("Stage 1 failed: %v", err)
	}

	t.Logf("Stage 1 binary: %s", result.Binary)
	t.Logf("Stage 1 size:   %d bytes", result.Size)
	t.Logf("Stage 1 SHA256: %s", result.SHA256)
	t.Logf("Stage 1 time:   %v", result.Duration)

	if result.Size == 0 {
		t.Fatal("Stage 1 binary is empty")
	}
	if result.SHA256 == "" {
		t.Fatal("Stage 1 SHA256 is empty")
	}
	if result.Stage != 1 {
		t.Fatalf("Expected stage 1, got %d", result.Stage)
	}
}

// TestBootstrap_Stage2SelfHosting asserts karkain-stage1 parses and compiles
// src/compiler/main.kar -> karkain-compiler2.
func TestBootstrap_Stage2SelfHosting(t *testing.T) {
	projectRoot := findProjectRoot(t)
	defer cleanupBinaries(t, projectRoot)

	t.Log("Running Stage 1 to produce karkain-compiler1...")
	s1, err := RunStage1(projectRoot)
	if err != nil {
		t.Fatalf("Stage 1 prerequisite failed: %v", err)
	}
	t.Logf("Stage 1 SHA256: %s", s1.SHA256)

	t.Log("Running Stage 2: karkain-compiler1 -> transpile -> gcc -> karkain-compiler2")
	s2, err := RunStage2(projectRoot, s1.Binary)
	if err != nil {
		t.Fatalf("Stage 2 failed: %v", err)
	}

	t.Logf("Stage 2 binary: %s", s2.Binary)
	t.Logf("Stage 2 size:   %d bytes", s2.Size)
	t.Logf("Stage 2 SHA256: %s", s2.SHA256)
	t.Logf("Stage 2 time:   %v", s2.Duration)

	if s2.Size == 0 {
		t.Fatal("Stage 2 binary is empty")
	}
	if s2.SHA256 == "" {
		t.Fatal("Stage 2 SHA256 is empty")
	}
	if s2.Stage != 2 {
		t.Fatalf("Expected stage 2, got %d", s2.Stage)
	}
}

// TestBootstrap_BitwiseIdentity ensures karkain-stage2 and karkain-stage3
// outputs match bitwise (100% deterministic self-hosting proof).
func TestBootstrap_BitwiseIdentity(t *testing.T) {
	projectRoot := findProjectRoot(t)
	defer cleanupBinaries(t, projectRoot)

	t.Log("Running Stage 1 to produce karkain-compiler1...")
	s1, err := RunStage1(projectRoot)
	if err != nil {
		t.Fatalf("Stage 1 prerequisite failed: %v", err)
	}
	t.Logf("Stage 1 SHA256: %s", s1.SHA256)

	t.Log("Running Stage 2 to produce karkain-compiler2...")
	s2, err := RunStage2(projectRoot, s1.Binary)
	if err != nil {
		t.Fatalf("Stage 2 prerequisite failed: %v", err)
	}
	t.Logf("Stage 2 SHA256: %s", s2.SHA256)

	t.Log("Running Stage 3 to produce karkain-compiler3...")
	s3, err := RunStage3(projectRoot, s2.Binary)
	if err != nil {
		t.Fatalf("Stage 3 failed: %v", err)
	}
	t.Logf("Stage 3 SHA256: %s", s3.SHA256)

	t.Logf("Stage 2 size: %d bytes", s2.Size)
	t.Logf("Stage 3 size: %d bytes", s3.Size)

	match, err := VerifyIdentity(s2, s3)
	if err != nil {
		t.Fatalf("VerifyIdentity error: %v", err)
	}

	if !match {
		t.Fatalf("Bitwise identity FAILED: stage2 (%s) != stage3 (%s)", s2.SHA256, s3.SHA256)
	}

	t.Logf("Bitwise identity CONFIRMED: stage2 == stage3 (%s)", s2.SHA256)
}
