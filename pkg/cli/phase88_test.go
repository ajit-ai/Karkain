package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// Phase 88: Self-hosted compiler foundation tests.
// Verifies the bootstrap → self-hosted → test-program pipeline.

func phase88Root(t *testing.T) string {
	t.Helper()
	_, f, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot determine test file path")
	}
	return filepath.Join(filepath.Dir(f), "..", "..")
}

func TestPhase88_SelfHostedCompilerCheck(t *testing.T) {
	root := phase88Root(t)
	karkain := buildBootstrap(t, root)

	srcFile := filepath.Join(root, "src", "compiler", "main.kark")
	cmd := exec.Command(karkain, "check", srcFile)
	out, err := cmd.CombinedOutput()
	t.Logf("output: %s", string(out))
	if err != nil {
		t.Fatalf("bootstrap compiler failed to check self-hosted compiler: %v", err)
	}
	if !strings.Contains(string(out), "passed") && !strings.Contains(string(out), "successfully") {
		t.Fatalf("unexpected output: %s", string(out))
	}
}

func TestPhase88_SelfHostedCompilerBuild(t *testing.T) {
	root := phase88Root(t)
	karkain := buildBootstrap(t, root)

	srcFile := filepath.Join(root, "src", "compiler", "main.kark")
	cmd := exec.Command(karkain, "build", srcFile, "--target", "c23")
	// Phase 88 predates the self-hosted (kcc) default engine: these tests assert
	// the bootstrap contract that the Go front end emits C23 next to the source
	// (src/compiler/main.c). Pin the Go engine so the assertion holds regardless
	// of the engine default.
	cmd.Env = append(os.Environ(), "KARKAIN_ENGINE=go")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("bootstrap compiler failed to build self-hosted compiler: %v\n%s", err, string(out))
	}
	if !strings.Contains(string(out), "successful") {
		t.Fatalf("unexpected output: %s", string(out))
	}

	// Verify C23 output exists
	cFile := filepath.Join(root, "src", "compiler", "main.c")
	if _, err := os.Stat(cFile); os.IsNotExist(err) {
		t.Fatalf("C23 output not created at %s", cFile)
	}
}

func TestPhase88_SelfHostedCompilerCompiles(t *testing.T) {
	root := phase88Root(t)

	// Check if gcc is available
	gccPath, err := exec.LookPath("gcc")
	if err != nil {
		t.Skip("gcc not available, skipping native compilation test")
	}

	cFile := filepath.Join(root, "src", "compiler", "main.c")
	exeFile := filepath.Join(root, "kcc.exe")
	if runtime.GOOS == "windows" {
		exeFile = filepath.Join(root, "kcc.exe")
	}

	cmd := exec.Command(gccPath, "-std=c99", "-o", exeFile, cFile, "-lm", "-lgmp", winsockLibFlag())
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("gcc failed to compile self-hosted compiler: %v\n%s", err, string(out))
	}

	// Verify the executable exists
	if _, err := os.Stat(exeFile); os.IsNotExist(err) {
		t.Fatalf("self-hosted compiler executable not created at %s", exeFile)
	}

	// Run --version
	cmd = exec.Command(exeFile, "--version")
	out, err = cmd.CombinedOutput()
	t.Logf("version: %s", string(out))
	if err != nil {
		t.Fatalf("self-hosted compiler failed to run: %v", err)
	}
	if !strings.Contains(string(out), "v1.0.0") {
		t.Fatalf("unexpected version output: %s", string(out))
	}
}

func TestPhase88_SelfHostedCheckTestPrograms(t *testing.T) {
	root := phase88Root(t)
	karkain := buildBootstrap(t, root)

	testDir := filepath.Join(root, "kcc-tests")
	entries, err := os.ReadDir(testDir)
	if err != nil {
		t.Fatalf("cannot read kcc-tests directory: %v", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".kark") {
			continue
		}
		if entry.Name() == "07_error.kark" {
			continue // Skip error test for check
		}
		name := entry.Name()
		t.Run(name, func(t *testing.T) {
			srcFile := filepath.Join(testDir, name)
			cmd := exec.Command(karkain, "check", srcFile)
			out, err := cmd.CombinedOutput()
			t.Logf("output: %s", string(out))
			if err != nil {
				t.Fatalf("bootstrap compiler failed to check %s: %v", name, err)
			}
		})
	}
}

func TestPhase88_SelfHostedBuildTestPrograms(t *testing.T) {
	root := phase88Root(t)
	karkain := buildBootstrap(t, root)

	testDir := filepath.Join(root, "kcc-tests")
	entries, err := os.ReadDir(testDir)
	if err != nil {
		t.Fatalf("cannot read kcc-tests directory: %v", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".kark") {
			continue
		}
		if entry.Name() == "07_error.kark" {
			continue // Skip error test for build
		}
		name := entry.Name()
		t.Run(name, func(t *testing.T) {
			srcFile := filepath.Join(testDir, name)
			cmd := exec.Command(karkain, "build", srcFile, "--target", "c23")
			out, err := cmd.CombinedOutput()
			t.Logf("output: %s", string(out))
			if err != nil {
				t.Fatalf("bootstrap compiler failed to build %s: %v", name, err)
			}

			// Verify C output exists (bootstrap compiler writes .c even with --target c23)
			cFile := filepath.Join(testDir, strings.TrimSuffix(name, ".kark")+".c")
			if _, err := os.Stat(cFile); os.IsNotExist(err) {
				t.Fatalf("C output not created for %s", name)
			}
		})
	}
}

func TestPhase88_ErrorReporting(t *testing.T) {
	root := phase88Root(t)
	karkain := buildBootstrap(t, root)

	// Test that undefined function is reported
	errorFile := filepath.Join(root, "kcc-tests", "07_error.kark")
	cmd := exec.Command(karkain, "check", errorFile)
	out, err := cmd.CombinedOutput()
	t.Logf("output: %s", string(out))
	if err == nil {
		t.Fatal("expected error for undefined function, got none")
	}
	if !strings.Contains(string(out), "undefined function") {
		t.Fatalf("expected 'undefined function' error, got: %s", string(out))
	}
}

func TestPhase88_BuildScriptExists(t *testing.T) {
	root := phase88Root(t)
	script := filepath.Join(root, "scripts", "build-kcc.sh")
	if _, err := os.Stat(script); os.IsNotExist(err) {
		t.Fatalf("build script not found at %s", script)
	}
}

func TestPhase88_TestProgramsExist(t *testing.T) {
	root := phase88Root(t)
	testDir := filepath.Join(root, "kcc-tests")

	expected := []string{
		"01_hello.kark",
		"02_arithmetic.kark",
		"03_functions.kark",
		"04_variables.kark",
		"05_return.kark",
		"06_control_flow.kark",
		"07_error.kark",
	}

	for _, name := range expected {
		path := filepath.Join(testDir, name)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("test program not found: %s", name)
		}
	}
}

// buildBootstrap builds the bootstrap compiler and returns its path.
func buildBootstrap(t *testing.T, root string) string {
	t.Helper()
	karkain := filepath.Join(root, "karkain.exe")
	if runtime.GOOS != "windows" {
		karkain = filepath.Join(root, "karkain")
	}

	// Build if not exists
	if _, err := os.Stat(karkain); os.IsNotExist(err) {
		cmd := exec.Command("go", "build", "-o", karkain, "./cmd/karkain/")
		cmd.Dir = root
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("failed to build bootstrap compiler: %v\n%s", err, string(out))
		}
	}
	return karkain
}
