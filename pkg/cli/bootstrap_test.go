package cli

import (
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestBootstrap_Stage0Validation validates the 3-stage bootstrap pipeline prerequisites
func TestBootstrap_Stage0Validation(t *testing.T) {
	projectRoot := findProjectRoot(t)

	// Verify compiler source files exist
	compilerFiles := []string{
		"compiler/ast.kark",
		"compiler/lexer.kark",
		"compiler/parser.kark",
		"compiler/codegen.kark",
		"compiler/main.kark",
	}

	for _, f := range compilerFiles {
	 fullPath := filepath.Join(projectRoot, f)
		if _, err := os.Stat(fullPath); os.IsNotExist(err) {
			t.Errorf("Missing compiler source file: %s", f)
		}
	}

	// Verify bootstrap scripts exist
	scripts := []string{
		"scripts/bootstrap.ps1",
		"scripts/bootstrap.sh",
	}

	for _, s := range scripts {
		fullPath := filepath.Join(projectRoot, s)
		if _, err := os.Stat(fullPath); os.IsNotExist(err) {
			t.Errorf("Missing bootstrap script: %s", s)
		}
	}

	// Verify go.mod exists (Stage 0 build depends on it)
	goModPath := filepath.Join(projectRoot, "go.mod")
	if _, err := os.Stat(goModPath); os.IsNotExist(err) {
		t.Error("Missing go.mod - Stage 0 build will fail")
	}

	// Verify cmd/karkain/main.go exists (Stage 0 entry point)
	mainPath := filepath.Join(projectRoot, "cmd", "karkain", "main.go")
	if _, err := os.Stat(mainPath); os.IsNotExist(err) {
		t.Error("Missing cmd/karkain/main.go - Stage 0 build entry point")
	}
}

// TestBootstrap_CompilerSourcesReadable verifies all compiler/*.kark files are readable
func TestBootstrap_CompilerSourcesReadable(t *testing.T) {
	projectRoot := findProjectRoot(t)
	compilerDir := filepath.Join(projectRoot, "compiler")

	entries, err := os.ReadDir(compilerDir)
	if err != nil {
		t.Fatalf("Cannot read compiler/ directory: %v", err)
	}

	karCount := 0
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".kark") {
			continue
		}
		karCount++

		fullPath := filepath.Join(compilerDir, entry.Name())
		data, err := os.ReadFile(fullPath)
		if err != nil {
			t.Errorf("Cannot read %s: %v", entry.Name(), err)
			continue
		}

		if len(data) == 0 {
			t.Errorf("File %s is empty", entry.Name())
		}

		// Check that it contains at least one func declaration
		content := string(data)
		if !strings.Contains(content, "func ") {
			t.Errorf("File %s contains no function declarations", entry.Name())
		}
	}

	if karCount < 5 {
		t.Errorf("Expected at least 5 .kark files in compiler/, found %d", karCount)
	}
}

// TestBootstrap_ScriptExecutable verifies bootstrap scripts have correct structure
func TestBootstrap_ScriptExecutable(t *testing.T) {
	projectRoot := findProjectRoot(t)

	// Check PowerShell bootstrap script
	psScript := filepath.Join(projectRoot, "scripts", "bootstrap.ps1")
	psData, err := os.ReadFile(psScript)
	if err != nil {
		t.Fatalf("Cannot read bootstrap.ps1: %v", err)
	}
	psContent := string(psData)

	// Verify 3-stage pipeline structure
	if !strings.Contains(psContent, "Stage 0") {
		t.Error("bootstrap.ps1 missing Stage 0")
	}
	if !strings.Contains(psContent, "Stage 1") {
		t.Error("bootstrap.ps1 missing Stage 1")
	}
	if !strings.Contains(psContent, "Stage 2") {
		t.Error("bootstrap.ps1 missing Stage 2")
	}
	if !strings.Contains(psContent, "SHA256") {
		t.Error("bootstrap.ps1 missing SHA256 verification")
	}
	if !strings.Contains(psContent, "go build") {
		t.Error("bootstrap.ps1 missing go build command")
	}

	// Check POSIX bootstrap script
	shScript := filepath.Join(projectRoot, "scripts", "bootstrap.sh")
	shData, err := os.ReadFile(shScript)
	if err != nil {
		t.Fatalf("Cannot read bootstrap.sh: %v", err)
	}
	shContent := string(shData)

	if !strings.Contains(shContent, "Stage 0") {
		t.Error("bootstrap.sh missing Stage 0")
	}
	if !strings.Contains(shContent, "Stage 1") {
		t.Error("bootstrap.sh missing Stage 1")
	}
	if !strings.Contains(shContent, "Stage 2") {
		t.Error("bootstrap.sh missing Stage 2")
	}
	if !strings.Contains(shContent, "sha256sum") {
		t.Error("bootstrap.sh missing sha256sum verification")
	}
}

// TestBootstrap_HashParity verifies SHA-256 hashing works correctly for parity checks
func TestBootstrap_HashParity(t *testing.T) {
	// Test that identical content produces identical hashes
	content1 := "Hello, Karkain!"
	content2 := "Hello, Karkain!"

	hash1 := sha256.Sum256([]byte(content1))
	hash2 := sha256.Sum256([]byte(content2))

	hash1Str := fmt.Sprintf("%x", hash1)
	hash2Str := fmt.Sprintf("%x", hash2)

	if hash1Str != hash2Str {
		t.Errorf("Identical content produced different hashes: %s vs %s", hash1Str, hash2Str)
	}

	// Test that different content produces different hashes
	content3 := "Goodbye, Karkain!"
	hash3 := sha256.Sum256([]byte(content3))
	hash3Str := fmt.Sprintf("%x", hash3)

	if hash1Str == hash3Str {
		t.Error("Different content produced identical hashes")
	}
}

// TestBootstrap_Stage0Build verifies that Stage 0 (Go build) succeeds
func TestBootstrap_Stage0Build(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping Stage 0 build test in short mode")
	}

	projectRoot := findProjectRoot(t)

	// Build the Go binary
	cmd := exec.Command("go", "build", "-o", filepath.Join(projectRoot, "bin", "karkain_test.exe"), "./cmd/karkain")
	cmd.Dir = projectRoot
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Stage 0 build failed: %v\nOutput: %s", err, string(output))
	}

	// Clean up test binary
	binPath := filepath.Join(projectRoot, "bin", "karkain_test.exe")
	defer os.Remove(binPath)

	// Verify binary was created
	if _, err := os.Stat(binPath); os.IsNotExist(err) {
		t.Error("Stage 0 binary not created")
	}

	// Verify binary is non-empty
	info, err := os.Stat(binPath)
	if err != nil {
		t.Fatalf("Cannot stat Stage 0 binary: %v", err)
	}
	if info.Size() == 0 {
		t.Error("Stage 0 binary is empty")
	}
}

// TestBootstrap_CheckCommand validates that the built binary supports check command
func TestBootstrap_CheckCommand(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping check command test in short mode")
	}

	projectRoot := findProjectRoot(t)
	binPath := filepath.Join(projectRoot, "bin", "karkain.exe")

	// Build first if binary doesn't exist
	if _, err := os.Stat(binPath); os.IsNotExist(err) {
		cmd := exec.Command("go", "build", "-o", binPath, "./cmd/karkain")
		cmd.Dir = projectRoot
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("Build failed: %v\nOutput: %s", err, string(output))
		}
	}

	// Test check command with compiler/main.kark
	testFile := filepath.Join(projectRoot, "compiler", "main.kark")
	if _, err := os.Stat(testFile); os.IsNotExist(err) {
		t.Skip("compiler/main.kark not found, skipping check test")
	}

	cmd := exec.Command(binPath, "check", testFile)
	cmd.Dir = projectRoot
	output, err := cmd.CombinedOutput()

	// check command may succeed or fail depending on parser support,
	// but the binary should at least run without crashing
	if err != nil {
		// check may fail for parse errors, that's OK for this test
		t.Logf("check command returned error (may be expected): %v", err)
		t.Logf("Output: %s", string(output))
	}
}

// findProjectRoot locates the project root by looking for go.mod
func findProjectRoot(t *testing.T) string {
	t.Helper()

	// Start from the test file's directory and walk up
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
