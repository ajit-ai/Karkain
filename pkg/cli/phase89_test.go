package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// Phase 89: Self-hosted runtime and toolchain tests.

func phase89Root(t *testing.T) string {
	t.Helper()
	_, f, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot determine test file path")
	}
	return filepath.Join(filepath.Dir(f), "..", "..")
}

func TestPhase89_RuntimeDirectoryExists(t *testing.T) {
	root := phase89Root(t)
	runtimeDir := filepath.Join(root, "runtime")
	if _, err := os.Stat(runtimeDir); os.IsNotExist(err) {
		t.Fatalf("runtime directory not found at %s", runtimeDir)
	}
}

func TestPhase89_RuntimeFilesExist(t *testing.T) {
	root := phase89Root(t)
	runtimeDir := filepath.Join(root, "runtime")

	expected := []string{
		"types.kark",
		"io.kark",
		"platform.kark",
		"init.kark",
		"boundary.kark",
	}

	for _, name := range expected {
		path := filepath.Join(runtimeDir, name)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("runtime file not found: %s", name)
		}
	}
}

func TestPhase89_RuntimeFilesCompilable(t *testing.T) {
	root := phase89Root(t)
	karkain := buildBootstrap(t, root)
	runtimeDir := filepath.Join(root, "runtime")

	entries, err := os.ReadDir(runtimeDir)
	if err != nil {
		t.Fatalf("cannot read runtime directory: %v", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".kark") {
			continue
		}
		name := entry.Name()
		t.Run(name, func(t *testing.T) {
			srcFile := filepath.Join(runtimeDir, name)
			cmd := exec.Command(karkain, "check", srcFile)
			out, err := cmd.CombinedOutput()
			t.Logf("output: %s", string(out))
			if err != nil {
				t.Fatalf("bootstrap compiler failed to check runtime/%s: %v", name, err)
			}
		})
	}
}

func TestPhase89_AcceptanceTestExists(t *testing.T) {
	root := phase89Root(t)
	acceptance := filepath.Join(root, "kcc-tests", "acceptance.kark")
	if _, err := os.Stat(acceptance); os.IsNotExist(err) {
		t.Fatalf("acceptance test not found at %s", acceptance)
	}
}

func TestPhase89_AcceptanceTestChecks(t *testing.T) {
	root := phase89Root(t)
	karkain := buildBootstrap(t, root)

	acceptance := filepath.Join(root, "kcc-tests", "acceptance.kark")
	cmd := exec.Command(karkain, "check", acceptance)
	out, err := cmd.CombinedOutput()
	t.Logf("output: %s", string(out))
	if err != nil {
		t.Fatalf("bootstrap compiler failed to check acceptance test: %v", err)
	}
}

func TestPhase89_AcceptanceTestBuilds(t *testing.T) {
	root := phase89Root(t)
	karkain := buildBootstrap(t, root)

	acceptance := filepath.Join(root, "kcc-tests", "acceptance.kark")
	cmd := exec.Command(karkain, "build", acceptance, "--target", "c23")
	out, err := cmd.CombinedOutput()
	t.Logf("output: %s", string(out))
	if err != nil {
		t.Fatalf("bootstrap compiler failed to build acceptance test: %v", err)
	}
}

func TestPhase89_SelfHostedCompilesAcceptance(t *testing.T) {
	root := phase89Root(t)

	// Check if kcc.exe exists
	kcc := filepath.Join(root, "kcc.exe")
	if _, err := os.Stat(kcc); os.IsNotExist(err) {
		t.Skip("kcc.exe not found, skipping self-hosted acceptance test")
	}

	acceptance := filepath.Join(root, "kcc-tests", "acceptance.kark")
	cmd := exec.Command(kcc, "check", acceptance)
	out, err := cmd.CombinedOutput()
	t.Logf("output: %s", string(out))
	if err != nil {
		t.Fatalf("self-hosted compiler failed to check acceptance test: %v", err)
	}
}

func TestPhase89_SelfHostedBuildsAcceptance(t *testing.T) {
	root := phase89Root(t)

	kcc := filepath.Join(root, "kcc.exe")
	if _, err := os.Stat(kcc); os.IsNotExist(err) {
		t.Skip("kcc.exe not found, skipping self-hosted acceptance test")
	}

	acceptance := filepath.Join(root, "kcc-tests", "acceptance.kark")
	cmd := exec.Command(kcc, "build", acceptance, "--target", "c23")
	out, err := cmd.CombinedOutput()
	t.Logf("output: %s", string(out))
	if err != nil {
		t.Fatalf("self-hosted compiler failed to build acceptance test: %v", err)
	}
}

func TestPhase89_NativeExecutableGeneration(t *testing.T) {
	root := phase89Root(t)

	gccPath, err := exec.LookPath("gcc")
	if err != nil {
		t.Skip("gcc not available, skipping native executable test")
	}

	karkain := buildBootstrap(t, root)
	acceptance := filepath.Join(root, "kcc-tests", "acceptance.kark")

	// Build to C
	cmd := exec.Command(karkain, "build", acceptance, "--target", "c23")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build to C failed: %v\n%s", err, string(out))
	}

	// Compile C to native
	cFile := filepath.Join(root, "kcc-tests", "acceptance.c")
	exeFile := filepath.Join(root, "kcc-tests", "acceptance.exe")
	cmd = exec.Command(gccPath, "-std=c99", "-o", exeFile, cFile, "-lm", "-lgmp")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("gcc compilation failed: %v\n%s", err, string(out))
	}

	// Verify executable exists
	if _, err := os.Stat(exeFile); os.IsNotExist(err) {
		t.Fatalf("native executable not created at %s", exeFile)
	}
}

func TestPhase89_BootstrapStillWorks(t *testing.T) {
	root := phase89Root(t)
	karkain := buildBootstrap(t, root)

	// Test that bootstrap can still check the self-hosted compiler
	srcFile := filepath.Join(root, "src", "compiler", "main.kark")
	cmd := exec.Command(karkain, "check", srcFile)
	out, err := cmd.CombinedOutput()
	t.Logf("output: %s", string(out))
	if err != nil {
		t.Fatalf("bootstrap compiler failed to check self-hosted compiler: %v", err)
	}
}

func TestPhase89_AllTestProgramsCheck(t *testing.T) {
	root := phase89Root(t)
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
		name := entry.Name()
		t.Run(name, func(t *testing.T) {
			srcFile := filepath.Join(testDir, name)
			cmd := exec.Command(karkain, "check", srcFile)
			out, err := cmd.CombinedOutput()
			t.Logf("output: %s", string(out))
			// 07_error.kark is expected to fail (undefined function)
			if name == "07_error.kark" {
				if err == nil {
					t.Fatalf("expected error for 07_error.kark, got none")
				}
				if !strings.Contains(string(out), "undefined function") {
					t.Fatalf("expected 'undefined function' error, got: %s", string(out))
				}
				return
			}
			if err != nil {
				t.Fatalf("bootstrap compiler failed to check %s: %v", name, err)
			}
		})
	}
}

func TestPhase89_GoTestsPass(t *testing.T) {
	root := phase89Root(t)

	cmd := exec.Command("go", "test", "./pkg/lexer/...", "./pkg/parser/...", "./pkg/codegen/...", "-count=1")
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	t.Logf("output: %s", string(out))
	if err != nil {
		t.Fatalf("existing tests failed: %v", err)
	}
}
