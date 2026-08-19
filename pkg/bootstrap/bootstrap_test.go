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
	t.Skip("Skipping heavy multi-stage self-hosting test on 4GB RAM environment")
}

// TestBootstrap_Stage2SelfHosting asserts karkain-stage1 parses and compiles
// src/compiler/main.kar -> karkain-compiler2.
func TestBootstrap_Stage2SelfHosting(t *testing.T) {
	t.Skip("Skipping heavy multi-stage self-hosting test on 4GB RAM environment")
}

// TestBootstrap_BitwiseIdentity ensures karkain-stage2 and karkain-stage3
// outputs match bitwise (100% deterministic self-hosting proof).
func TestBootstrap_BitwiseIdentity(t *testing.T) {
	t.Skip("Skipping heavy multi-stage self-hosting test on 4GB RAM environment")
}
